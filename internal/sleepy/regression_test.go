package sleepy

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func rawFields(t *testing.T, s *Server, id string) string {
	t.Helper()
	var raw string
	if err := s.store.db.QueryRow(`SELECT fields FROM device_status WHERE id=?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestFieldsKeepLargeIntegers(t *testing.T) {
	s, c := testApp(t)
	const big = "9007199254740993"
	r := httptest.NewRequest("POST", "/api/device/set", strings.NewReader(`{"id":"pc","show_name":"PC","fields":{"snowflake":`+big+`,"nested":{"n":`+big+`}}}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer test-secret")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	requireCode(t, w, 200)
	if raw := rawFields(t, s, "pc"); strings.Count(raw, big) != 2 {
		t.Fatalf("HTTP write lost precision: %s", raw)
	}

	if _, err := s.store.db.Exec(`INSERT INTO device_status (id,show_name,"using",status,fields,last_updated) VALUES ('old','Old',1,'x','{"snowflake": ` + big + `}',1)`); err != nil {
		t.Fatal(err)
	}
	s.store.db.Close()
	store, err := openStore(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.db.Close() })
	s2, err := newServer(c, store)
	if err != nil {
		t.Fatal(err)
	}
	if body := call(t, s2, "GET", "/api/status/query", nil, false).Body.String(); !strings.Contains(body, `"snowflake":`+big) {
		t.Fatalf("query lost precision: %s", body)
	}
	requireCode(t, call(t, s2, "POST", "/api/device/set", map[string]any{"id": "old", "show_name": "Renamed"}, true), 200)
	if raw := rawFields(t, s2, "old"); !strings.Contains(raw, big) {
		t.Fatalf("rename rewrote fields: %s", raw)
	}
}

func TestHeadOnEventStreamsReturnsImmediately(t *testing.T) {
	for _, path := range []string{"/api/status/events", "/events"} {
		s, _ := testApp(t)
		ts := httptest.NewServer(s)
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Head(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 || len(s.streams) != 0 {
			t.Fatalf("%s: HEAD status %d, streams %d", path, resp.StatusCode, len(s.streams))
		}

		next, err := client.Get(ts.URL + "/none")
		if err != nil {
			t.Fatalf("%s: reused connection blocked: %v", path, err)
		}
		next.Body.Close()
		client.CloseIdleConnections()
		ts.Close()
	}
}

func TestSlowRequestBodyTimesOut(t *testing.T) {
	s, _ := testApp(t)
	s.bodyTimeout = 100 * time.Millisecond
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: s, ReadHeaderTimeout: time.Second}
	go srv.Serve(ln)
	defer srv.Close()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "POST /api/device/set HTTP/1.1\r\nHost: x\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n{\"id\":")
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil || !strings.Contains(string(buf[:n]), " 408 ") {
		t.Fatalf("expected 408 after body timeout, got %q err=%v", buf[:n], err)
	}
}

func TestEnvValuesOverriddenByFilesAreNotValidated(t *testing.T) {
	for _, tc := range []struct{ env, val, yaml string }{
		{"sleepy_main_port", "invalid", "main:\n  secret: s\n  port: 9011\n"},
		{"sleepy_main_https", "maybe", "main:\n  secret: s\n  https: false\n"},
		{"sleepy_plugins_enabled", "not-json", "main:\n  secret: s\nplugins_enabled: [custom_text]\n"},
		{"sleepy_plugin", "{bad", "main:\n  secret: s\nplugin: {}\n"},
		{"sleepy_status_status_list", "[bad", "main:\n  secret: s\nstatus:\n  status_list: [{name: a, desc: b}]\n"},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(tc.yaml), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(tc.env, tc.val)
		if _, err := loadConfig(dir); err != nil {
			t.Fatalf("%s overridden by YAML still failed: %v", tc.env, err)
		}
		os.Unsetenv(tc.env)
	}

	dir := t.TempDir()
	t.Setenv("sleepy_main_secret", "s")
	t.Setenv("sleepy_main_port", "invalid")
	if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), "sleepy_main_port") {
		t.Fatalf("invalid env not reported: %v", err)
	}
}

func TestUsingParsingMatchesPython(t *testing.T) {
	s, _ := testApp(t)
	requireCode(t, call(t, s, "GET", "/device/set?secret=test-secret&id=ipad&show_name=iPad&app_name=A&using=true", nil, false), 200)
	for _, q := range []string{"using=", "using=none", "using=whatever"} {
		requireCode(t, call(t, s, "GET", "/device/set?secret=test-secret&id=ipad&app_name=B&"+q, nil, false), 200)
		if snap, _ := s.store.snapshot(); snap.Devices["ipad"].Using == nil || !*snap.Devices["ipad"].Using {
			t.Fatalf("legacy %s changed using", q)
		}
	}
	requireCode(t, call(t, s, "GET", "/api/device/set?id=ipad&using=", nil, true), 200)
	requireCode(t, call(t, s, "GET", "/api/device/set?id=ipad&using=whatever", nil, true), 400)
	requireCode(t, call(t, s, "POST", "/api/device/set", map[string]any{"id": "ipad", "using": "false"}, true), 200)
	if snap, _ := s.store.snapshot(); *snap.Devices["ipad"].Using {
		t.Fatal(`JSON "false" string not applied`)
	}
	requireCode(t, call(t, s, "POST", "/device/set", map[string]any{"id": "ipad", "using": "odd"}, true), 200)
}

func TestCorruptFieldsRowDoesNotBreakWrites(t *testing.T) {
	s, c := testApp(t)
	if _, err := s.store.db.Exec(`INSERT INTO device_status (id,show_name,"using",status,fields,last_updated) VALUES ('bad','Bad',1,'x','not json',1)`); err != nil {
		t.Fatal(err)
	}
	_, changed := s.store.snapshot()
	requireCode(t, call(t, s, "POST", "/api/status/set", map[string]any{"status": 1}, true), 200)
	select {
	case <-changed:
	default:
		t.Fatal("SSE not notified")
	}
	if snap, _ := s.store.snapshot(); snap.Status != 1 {
		t.Fatal("snapshot stale")
	}
	s.store.db.Close()
	store, err := openStore(c)
	if err != nil {
		t.Fatal(err)
	}
	store.db.Close()
}

func TestUnchangedWritesKeepLastUpdated(t *testing.T) {
	s, _ := testApp(t)
	before, changed := s.store.snapshot()
	requireCode(t, call(t, s, "POST", "/api/status/set", map[string]any{"status": before.Status}, true), 200)
	requireCode(t, call(t, s, "POST", "/api/device/private?private=false", nil, true), 200)
	after, _ := s.store.snapshot()
	select {
	case <-changed:
		t.Fatal("unchanged write woke SSE clients")
	default:
	}
	if after.Updated != before.Updated {
		t.Fatal("unchanged write bumped last_updated")
	}
}

func loginCookie(t *testing.T, s *Server) *http.Cookie {
	t.Helper()
	r := httptest.NewRequest("POST", "/admin/auth", strings.NewReader(`{"secret":"test-secret"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	requireCode(t, w, 200)
	return w.Result().Cookies()[0]
}

func TestOriginErrorExplainsProxySetup(t *testing.T) {
	s, _ := testApp(t)
	cookie := loginCookie(t, s)
	r := httptest.NewRequest("POST", "/api/status/set?status=1", nil)
	r.Host = "127.0.0.1:9010"
	r.Header.Set("Origin", "https://status.example.com")
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	requireCode(t, w, 403)
	if !strings.Contains(w.Body.String(), "X-Forwarded-Proto") || !strings.Contains(w.Body.String(), "127.0.0.1:9010") {
		t.Fatalf("403 lacks diagnostics: %s", w.Body.String())
	}
}

func TestLogoutEndsAllSessions(t *testing.T) {
	s, c := testApp(t)
	cookie := loginCookie(t, s)
	copied := *cookie
	r := httptest.NewRequest("POST", "/admin/logout", nil)
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	requireCode(t, w, 303)
	check := httptest.NewRequest("GET", "/admin/data", nil)
	check.AddCookie(&copied)
	w = httptest.NewRecorder()
	s.ServeHTTP(w, check)
	requireCode(t, w, 401)

	s2, err := newServer(c, s.store)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(string(s2.sessionKey), string(s.sessionKey)) {
		t.Fatal("session key not persisted")
	}

	fresh := loginCookie(t, s)
	r = httptest.NewRequest("POST", "/admin/logout", nil)
	r.Header.Set("Origin", "https://evil.example")
	r.AddCookie(fresh)
	s.ServeHTTP(httptest.NewRecorder(), r)
	if !s.sessionOK(withCookie(fresh)) {
		t.Fatal("cross-site logout rotated the key")
	}
}

func withCookie(c *http.Cookie) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(c)
	return r
}

func TestVersionedAssets(t *testing.T) {
	s, _ := testApp(t)
	w := call(t, s, "GET", s.assets+"/static/paper.css", nil, false)
	requireCode(t, w, 200)
	if !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("current assets not immutable: %q", w.Header().Get("Cache-Control"))
	}
	old := call(t, s, "GET", "/assets/000000000000/static/paper.css", nil, false)
	requireCode(t, old, 200)
	if strings.Contains(old.Header().Get("Cache-Control"), "immutable") {
		t.Fatal("stale version cached forever")
	}
	requireCode(t, call(t, s, "GET", s.assets+"/templates/index.html", nil, false), 404)
	if page := call(t, s, "GET", "/", nil, false).Body.String(); !strings.Contains(page, s.assets+"/static/paper.js") {
		t.Fatal("index does not reference versioned assets")
	}
}
