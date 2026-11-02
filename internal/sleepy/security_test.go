package sleepy

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func secApp(t *testing.T, tweak func(*Config)) *Server {
	t.Helper()
	c := defaults()
	c.DataDir = t.TempDir()
	c.Main.Secret = "admin-secret-long-enough"
	tweak(&c)
	store, err := openStore(c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.db.Close() })
	s, err := newServer(c, store)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func do(s *Server, method, target, bearer string, body string) *httptest.ResponseRecorder {
	var r = httptest.NewRequest(method, target, nil)
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestDeviceSecretOnlyReports(t *testing.T) {
	s := secApp(t, func(c *Config) { c.Main.DeviceSecret = "device-secret-long-enough" })
	requireCode(t, do(s, "POST", "/api/device/set", "device-secret-long-enough", `{"id":"pc","show_name":"PC","using":true}`), 200)
	requireCode(t, do(s, "GET", "/device/set?secret=device-secret-long-enough&id=ipad&show_name=iPad&using=true", "", ""), 200)
	for _, path := range []string{"/api/status/set?status=1", "/api/device/clear", "/api/device/private?private=true", "/admin/data"} {
		method := "POST"
		if path == "/admin/data" {
			method = "GET"
		}
		requireCode(t, do(s, method, path, "device-secret-long-enough", ""), 403)
	}
	requireCode(t, do(s, "POST", "/admin/auth", "", `{"secret":"device-secret-long-enough"}`), 403)
	requireCode(t, do(s, "POST", "/api/status/set?status=1", "admin-secret-long-enough", ""), 200)
	requireCode(t, do(s, "POST", "/api/device/set", "admin-secret-long-enough", `{"id":"pc","using":false}`), 200)
}

func TestQuerySecretCanBeDisabled(t *testing.T) {
	s := secApp(t, func(c *Config) { c.Main.AllowQuerySecret = false })
	requireCode(t, do(s, "GET", "/api/device/set?secret=admin-secret-long-enough&id=pc&show_name=PC", "", ""), 401)
	requireCode(t, do(s, "GET", "/api/device/set?id=pc&show_name=PC", "admin-secret-long-enough", ""), 200)
}

func TestAllowedHosts(t *testing.T) {
	s := secApp(t, func(c *Config) { c.Main.AllowedHosts = []string{"status.example.com"} })
	for host, want := range map[string]int{"status.example.com": 200, "STATUS.example.com:443": 200, "203.0.113.5:9010": 421, "evil.example": 421} {
		r := httptest.NewRequest("GET", "/api/status/query", nil)
		r.Host = host
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("Host %s: %d, want %d", host, w.Code, want)
		}
	}
	r := httptest.NewRequest("GET", "/none", nil)
	r.Host = "127.0.0.1:9010"
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	requireCode(t, w, 204)
}

func TestSecurityHeaders(t *testing.T) {
	s := secApp(t, func(*Config) {})
	w := do(s, "GET", "/", "", "")
	if w.Header().Get("X-Frame-Options") != "DENY" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal("anti-framing headers missing")
	}
	if w.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS on plain HTTP")
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Header().Get("Strict-Transport-Security") == "" {
		t.Fatal("HSTS missing behind HTTPS proxy")
	}
}

func TestSecurityConfigFromEnv(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("sleepy_main_secret=admin-secret\nsleepy_main_device_secret=dev-secret\nsleepy_main_allow_query_secret=false\nsleepy_main_allowed_hosts=status.example.com, Other.example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Main.DeviceSecret != "dev-secret" || c.Main.AllowQuerySecret || strings.Join(c.Main.AllowedHosts, ",") != "status.example.com,other.example" {
		t.Fatalf("unexpected: %+v", c.Main)
	}
	t.Setenv("sleepy_main_device_secret", "admin-secret")
	if _, err := loadConfig(dir); err == nil {
		t.Fatal("same device and admin secret accepted")
	}
}
