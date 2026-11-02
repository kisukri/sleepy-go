package sleepy

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testApp(t *testing.T) (*Server, Config) {
	t.Helper()
	c := defaults()
	c.DataDir = t.TempDir()
	c.Main.Secret = "test-secret"
	store, err := openStore(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.db.Close() })
	s, err := newServer(c, store)
	if err != nil {
		t.Fatal(err)
	}
	return s, c
}
func call(t *testing.T, s *Server, method, path string, value any, token bool) *httptest.ResponseRecorder {
	t.Helper()
	var payload io.Reader
	if value != nil {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewReader(raw)
	}
	r := httptest.NewRequest(method, path, payload)
	if payload != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if token {
		r.Header.Set("Authorization", "Bearer test-secret")
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func object(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON (%d): %s", w.Code, w.Body.String())
	}
	return out
}
func requireCode(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("HTTP %d, want %d: %s", w.Code, want, w.Body.String())
	}
}
func TestDevicePrivacyAndRestart(t *testing.T) {
	s, c := testApp(t)
	requireCode(t, call(t, s, "POST", "/api/device/set", map[string]any{"id": "pc", "show_name": "电脑", "using": true, "status": "编辑器", "fields": map[string]any{"battery": 80}}, false), 401)
	requireCode(t, call(t, s, "POST", "/api/device/set", map[string]any{"id": "pc", "show_name": "电脑", "using": true, "status": "编辑器", "fields": map[string]any{"battery": 80}}, true), 200)
	requireCode(t, call(t, s, "POST", "/api/device/set", map[string]any{"id": "pc", "status": "", "using": false}, true), 200)
	snap, _ := s.store.snapshot()
	if snap.Devices["pc"].Status == nil || *snap.Devices["pc"].Status != "" || *snap.Devices["pc"].Using {
		t.Fatal("empty status/false update lost")
	}
	if snap.Devices["pc"].Fields["battery"] != json.Number("80") {
		t.Fatal("fields lost")
	}
	requireCode(t, call(t, s, "POST", "/api/device/private?private=true", nil, true), 200)
	public := object(t, call(t, s, "GET", "/api/status/query", nil, false))
	if len(public["device"].(map[string]any)) != 0 {
		t.Fatal("privacy leak")
	}
	requireCode(t, call(t, s, "GET", "/admin/data", nil, false), 401)
	panel := object(t, call(t, s, "GET", "/admin/data", nil, true))
	if len(panel["devices"].(map[string]any)) != 1 {
		t.Fatal("admin cannot see private device")
	}
	if err := s.store.db.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := openStore(c)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.db.Close()
	snap, _ = restored.snapshot()
	if !snap.Private || snap.Devices["pc"].ShowName != "电脑" {
		t.Fatal("restart lost data")
	}
}
func TestValidationAndRollback(t *testing.T) {
	s, _ := testApp(t)
	for _, tc := range []struct {
		path    string
		payload any
		code    int
	}{
		{"/api/status/set?status=-1", nil, 400}, {"/api/status/set?status=99", nil, 400},
		{"/api/device/set", map[string]any{"id": "missing-name"}, 400},
		{"/api/device/set", map[string]any{"id": "pc", "show_name": "电脑", "using": "maybe"}, 400},
		{"/admin/auth", []string{"not-an-object"}, 400},
		{"/plugin/custom_text/text", map[string]any{"text": map[string]any{"page": "wrong"}}, 400},
	} {
		requireCode(t, call(t, s, "POST", tc.path, tc.payload, true), tc.code)
	}
	snap, _ := s.store.snapshot()
	if len(snap.Devices) != 0 || snap.Status != 0 {
		t.Fatal("invalid request changed state")
	}
}
func TestSessionAndCSRF(t *testing.T) {
	s, _ := testApp(t)
	w := call(t, s, "POST", "/admin/auth", map[string]any{"secret": "test-secret"}, false)
	requireCode(t, w, 200)
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || strings.Contains(cookies[0].Value, "test-secret") {
		t.Fatal("invalid session cookie")
	}
	send := func(method, path, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.AddCookie(cookies[0])
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	requireCode(t, send("GET", "/admin/data", ""), 200)
	requireCode(t, send("GET", "/api/device/clear", ""), 403)
	requireCode(t, send("POST", "/api/status/set?status=1", "https://attacker.invalid"), 403)
	requireCode(t, send("POST", "/api/status/set?status=1", "http://example.com"), 200)
	requireCode(t, call(t, s, "GET", "/api/status/set?status=0&secret=test-secret", nil, false), 200)
	cookies[0].Value += "tamper"
	requireCode(t, send("GET", "/admin/data", ""), 401)
}
func TestTextSanitizationAndPersistence(t *testing.T) {
	s, _ := testApp(t)
	request := map[string]any{"text": map[string]any{"page": map[string]any{"name": "中文🌙", "footer": "<a href='/'>链接</a>", "battery_tag": "off", "unknown": "drop"}, "status": map[string]any{"0": map[string]any{"title": []string{"你好", "再见"}, "color": "invalid"}}}}
	w := call(t, s, "POST", "/plugin/custom_text/text", request, true)
	requireCode(t, w, 200)
	saved := object(t, w)["text"].(map[string]any)
	page := saved["page"].(map[string]any)
	if _, ok := page["unknown"]; ok {
		t.Fatal("unknown key retained")
	}
	other := &TextStore{filename: s.text.filename}
	if other.load()["page"].(map[string]any)["name"] != "中文🌙" {
		t.Fatal("text restart failed")
	}
	requireCode(t, call(t, s, "POST", "/plugin/custom_text/text", request, false), 401)
	bad := map[string]any{"page": map[string]any{"name": "</script><script>alert(1)</script>"}}
	if _, err := s.text.save(bad); err != nil {
		t.Fatal(err)
	}
	html := call(t, s, "GET", "/", nil, false).Body.String()
	if strings.Contains(html, "</script><script>alert(1)") {
		t.Fatal("JSON injection not escaped")
	}
}
func TestEmbeddedFilesAndPublicIsolation(t *testing.T) {
	s, c := testApp(t)
	for _, path := range []string{"/", "/admin/login", "/static-themed/paper/paper.js", "/static-themed/paper/render.js", s.assets + "/static/paper.js", s.assets + "/editor/editor.js", "/favicon.ico"} {
		first := call(t, s, "GET", path, nil, false)
		second := call(t, s, "GET", path, nil, false)
		requireCode(t, first, 200)
		requireCode(t, second, 200)
		if first.Body.Len() == 0 || !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
			t.Fatalf("nonrepeatable asset: %s", path)
		}
	}
	if err := os.MkdirAll(filepath.Join(c.DataDir, "public"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.DataDir, "outside.txt"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside.txt", filepath.Join(c.DataDir, "public", "escape.txt")); err != nil {
		t.Fatal(err)
	}
	requireCode(t, call(t, s, "GET", "/escape.txt", nil, false), 404)
	requireCode(t, call(t, s, "GET", "/not-found", nil, false), 404)
}
func TestLegacyContracts(t *testing.T) {
	s, _ := testApp(t)
	requireCode(t, call(t, s, "GET", "/device/set?id=legacy&show_name=旧设备&using=true&app_name=旧应用&secret=test-secret", nil, false), 200)
	data := object(t, call(t, s, "GET", "/query", nil, false))
	if data["status"] != float64(0) {
		t.Fatal("legacy status must be numeric")
	}
	device := data["device"].(map[string]any)["legacy"].(map[string]any)
	if device["app_name"] != "旧应用" {
		t.Fatal("legacy app_name missing")
	}
	if _, ok := device["status"]; ok {
		t.Fatal("v5 field leaked into v4")
	}
	requireCode(t, call(t, s, "GET", "/set?status=1&secret=test-secret", nil, false), 200)
	w := call(t, s, "GET", "/device/clear", nil, false)
	requireCode(t, w, 401)
	if object(t, w)["code"] != "not authorized" {
		t.Fatal("legacy error code changed")
	}
}
func readEvent(reader *bufio.Reader) (string, error) {
	var event strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		event.WriteString(line)
		if line == "\n" {
			return event.String(), nil
		}
	}
}
func TestSSEUpdateHeartbeatAndCancellation(t *testing.T) {
	s, _ := testApp(t)
	s.heartbeat = 30 * time.Millisecond
	server := httptest.NewServer(s)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/status/events", nil)
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(response.Body)
	first, err := readEvent(reader)
	if err != nil || !strings.Contains(first, "event: update") {
		t.Fatalf("initial SSE: %s %v", first, err)
	}
	if err = s.store.setStatus(1); err != nil {
		t.Fatal(err)
	}
	for {
		event, err := readEvent(reader)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(event, "event: update") {
			if !strings.Contains(event, `"id":1`) {
				t.Fatalf("stale SSE: %s", event)
			}
			break
		}
	}
	heartbeat, err := readEvent(reader)
	if err != nil || !strings.Contains(heartbeat, "event: heartbeat") {
		t.Fatalf("heartbeat: %s %v", heartbeat, err)
	}
	response.Body.Close()
	cancel()
	deadline := time.Now().Add(time.Second)
	for len(s.streams) != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if len(s.streams) != 0 {
		t.Fatal("stream subscription leaked")
	}
}
func TestConcurrentReadersWriters(t *testing.T) {
	s, _ := testApp(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 15; j++ {
				if j%3 == 0 {
					if err := s.store.setStatus(j % 2); err != nil {
						t.Error(err)
					}
				} else {
					snap, _ := s.store.snapshot()
					if _, err := json.Marshal(s.query(snap, false)); err != nil {
						t.Error(err)
					}
				}
			}
		}()
	}
	wg.Wait()
}
func TestAutoDeadUsesLatestActivity(t *testing.T) {
	s, _ := testApp(t)
	s.autoInterval = 5 * time.Millisecond
	s.autoTimeout = 10 * time.Millisecond
	s.autoStatus = 1
	if err := s.store.setStatus(0); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.autoDead(ctx)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snap, _ := s.store.snapshot()
		if snap.Status == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("auto_dead did not expire")
}
func TestLoginWithCookieJar(t *testing.T) {
	s, _ := testApp(t)
	server := httptest.NewServer(s)
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: time.Second}
	response, err := client.Post(server.URL+"/admin/auth", "application/json", strings.NewReader(`{"secret":"test-secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	response, err = client.Get(server.URL + "/admin")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 || !bytes.Contains(data, []byte("pn-status")) {
		t.Fatal("admin page unavailable")
	}
}

func TestOriginalPythonDatabase(t *testing.T) {
	c := defaults()
	c.DataDir = t.TempDir()
	c.Main.Secret = "test-secret"
	db, err := sql.Open("sqlite", filepath.Join(c.DataDir, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile("testdata/python-schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO main(id,status,private_mode,last_updated) VALUES (0,1,1,1700000000.125)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO device_status(id,show_name,"using",status,fields,last_updated) VALUES ('old','原设备',1,'[77%]Safari','{"nested":{"value":"中文🌙"}}',1700000000.125)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO plugin(id,data) VALUES ('legacy','{"preserve":true}')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	store, err := openStore(c)
	if err != nil {
		t.Fatal(err)
	}
	defer store.db.Close()
	snap, _ := store.snapshot()
	if snap.Status != 1 || !snap.Private || snap.Updated != 1700000000.125 || snap.Devices["old"].ShowName != "原设备" {
		t.Fatalf("Python state not preserved: %+v", snap)
	}
	var plugin string
	if err = store.db.QueryRow(`SELECT data FROM plugin WHERE id='legacy'`).Scan(&plugin); err != nil || plugin != `{"preserve":true}` {
		t.Fatal("legacy plugin data lost")
	}
	status := ""
	if err = store.setDevice(DeviceUpdate{ID: "old", Status: &status}); err != nil {
		t.Fatal(err)
	}
	snap, _ = store.snapshot()
	if snap.Devices["old"].Fields["nested"].(map[string]any)["value"] != "中文🌙" {
		t.Fatal("legacy extension fields lost")
	}
}

func TestInternalErrorsAndBodyLimits(t *testing.T) {
	s, _ := testApp(t)
	s.route("GET /test-error", func(http.ResponseWriter, *http.Request) error { return errors.New("private diagnostic") })
	w := call(t, s, "GET", "/test-error", nil, false)
	requireCode(t, w, 500)
	if strings.Contains(w.Body.String(), "private diagnostic") {
		t.Fatal("internal error leaked")
	}
	r := httptest.NewRequest("POST", "/admin/auth", strings.NewReader(`{"secret":"`+strings.Repeat("a", 1<<20)+`"}`))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	requireCode(t, w, 413)
	r = httptest.NewRequest("POST", "/admin/auth", strings.NewReader(`{} {}`))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	requireCode(t, w, 400)
	requireCode(t, call(t, s, "POST", "/api/device/clear", nil, true), 200)
}
func TestPageLanguage(t *testing.T) {
	s, _ := testApp(t)
	for _, path := range []string{"/", "/admin/login"} {
		html := call(t, s, "GET", path, nil, false).Body.String()
		if !strings.Contains(html, `<html lang="ja"`) || !strings.Contains(html, `id="sl-words"`) || !strings.Contains(html, "接続しています") && path == "/" {
			t.Fatalf("%s: ja page missing lang or words", path)
		}
	}
	s.c.Page.Language = "zh"
	html := call(t, s, "GET", "/admin/login", nil, false).Body.String()
	if !strings.Contains(html, `<html lang="zh-CN"`) || !strings.Contains(html, "登录管理") {
		t.Fatal("zh login page")
	}
	w := call(t, s, "GET", s.assets+"/static/fonts/newsreader.woff2", nil, false)
	requireCode(t, w, 200)
	if w.Header().Get("Content-Type") != "font/woff2" {
		t.Fatalf("font type %q", w.Header().Get("Content-Type"))
	}
}

func TestOldPanelRedirects(t *testing.T) {
	s, _ := testApp(t)
	for old, want := range map[string]string{"/panel": "/admin", "/panel/login": "/admin/login"} {
		w := call(t, s, "GET", old, nil, false)
		requireCode(t, w, 301)
		if got := w.Header().Get("Location"); got != want {
			t.Fatalf("%s redirects to %q", old, got)
		}
	}
}
