package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
)

type fakeSampler struct{ s Sample }

func (f *fakeSampler) Sample() (Sample, error) { return f.s, nil }

type fakeServer struct {
	mu     sync.Mutex
	got    []map[string]any
	auth   []string
	status int
}

func (f *fakeServer) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.got = append(f.got, body)
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	if f.status != 0 {
		w.WriteHeader(f.status)
		return
	}
	w.Write([]byte(`{"success":true}`))
}

func setup(t *testing.T, tweak func(*Config)) (*Client, *fakeSampler, *fakeServer, *time.Time) {
	t.Helper()
	srv := &fakeServer{}
	ts := httptest.NewServer(http.HandlerFunc(srv.handler))
	t.Cleanup(ts.Close)
	cfg := defaultConfig()
	cfg.Server, cfg.Secret, cfg.DeviceID, cfg.DeviceName = ts.URL+"/", "dev-secret", "pc", "My PC"
	if tweak != nil {
		tweak(&cfg)
	}
	fs := &fakeSampler{}
	c := NewClient(cfg, fs, t.Logf)
	clock := time.Unix(1_700_000_000, 0)
	c.now = func() time.Time { return clock }
	return c, fs, srv, &clock
}

func TestSendsOnChangeAndHeartbeat(t *testing.T) {
	c, fs, srv, clock := setup(t, nil)
	ctx := context.Background()
	fs.s = Sample{Title: "Visual Studio Code"}
	for i := 0; i < 3; i++ {
		if err := c.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if len(srv.got) != 1 || srv.got[0]["status"] != "Visual Studio Code" || srv.got[0]["using"] != true || srv.auth[0] != "Bearer dev-secret" {
		t.Fatalf("unexpected sends: %v %v", srv.got, srv.auth)
	}
	fs.s.Title = "Chrome"
	c.Tick(ctx)
	*clock = clock.Add(299 * time.Second)
	c.Tick(ctx)
	*clock = clock.Add(2 * time.Second)
	c.Tick(ctx)
	if len(srv.got) != 3 || srv.got[1]["status"] != "Chrome" || srv.got[2]["status"] != "Chrome" {
		t.Fatalf("want change + heartbeat, got %v", srv.got)
	}
}

func TestIdleSkipAndNotUsing(t *testing.T) {
	c, fs, srv, _ := setup(t, func(c *Config) { c.NotUsingTitles = []string{"LockApp"} })
	ctx := context.Background()
	fs.s = Sample{Title: "Editor"}
	c.Tick(ctx)
	fs.s = Sample{Title: "Task Switching"}
	c.Tick(ctx)
	fs.s = Sample{Title: ""}
	c.Tick(ctx)
	if len(srv.got) != 1 {
		t.Fatalf("skip titles should not send: %v", srv.got)
	}
	fs.s = Sample{Title: "Editor", Idle: 16 * time.Minute}
	c.Tick(ctx)
	fs.s = Sample{Title: "LockApp"}
	c.Tick(ctx)
	if len(srv.got) != 2 || srv.got[1]["using"] != false {
		t.Fatalf("idle should send using=false once: %v", srv.got)
	}
	if _, has := srv.got[1]["status"]; has {
		t.Fatal("not-using report must keep the old title (no status field)")
	}
}

func TestBatteryAndAppFirst(t *testing.T) {
	c, fs, srv, _ := setup(t, func(c *Config) { c.AppFirst = true })
	fs.s = Sample{Title: "notes.txt - Notepad", Battery: &Battery{Percent: 82, Charging: true}}
	c.Tick(context.Background())
	if srv.got[0]["status"] != "[🔋82%⚡] Notepad - notes.txt" {
		t.Fatalf("got %v", srv.got[0]["status"])
	}
}

func TestRetryAfterServerError(t *testing.T) {
	c, fs, srv, _ := setup(t, nil)
	srv.status = 500
	fs.s = Sample{Title: "Editor"}
	if err := c.Tick(context.Background()); err == nil {
		t.Fatal("want error on 500")
	}
	srv.status = 0
	if err := c.Tick(context.Background()); err != nil || len(srv.got) != 2 {
		t.Fatalf("want retry on next tick: err=%v sends=%d", err, len(srv.got))
	}
}

func TestEndpointAndValidate(t *testing.T) {
	for in, want := range map[string]string{
		"https://s.example.com":                 "https://s.example.com/api/device/set",
		"https://s.example.com/":                "https://s.example.com/api/device/set",
		"http://1.2.3.4:9010/api/device/set":    "http://1.2.3.4:9010/api/device/set",
		" http://1.2.3.4:9010/api/device/set/ ": "http://1.2.3.4:9010/api/device/set",
	} {
		if got := endpoint(in); got != want {
			t.Fatalf("endpoint(%q) = %q", in, got)
		}
	}
	if err := defaultConfig().validate(); err == nil {
		t.Fatal("empty config accepted")
	}
}

func TestTemplateParses(t *testing.T) {
	cfg := defaultConfig()
	path := filepath.Join(t.TempDir(), "c.toml")
	if err := os.WriteFile(path, []byte(template), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.IntervalSeconds != 5 || cfg.HeartbeatSeconds != 300 || cfg.DeviceID != "my-pc" || !cfg.Battery {
		t.Fatalf("template values: %+v", cfg)
	}
}
