package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	Server           string   `toml:"server"`
	Secret           string   `toml:"secret"`
	DeviceID         string   `toml:"device_id"`
	DeviceName       string   `toml:"device_name"`
	IntervalSeconds  int      `toml:"interval_seconds"`
	HeartbeatSeconds int      `toml:"heartbeat_seconds"`
	IdleMinutes      int      `toml:"idle_minutes"`
	Battery          bool     `toml:"battery"`
	AppFirst         bool     `toml:"app_first"`
	AppOnly          bool     `toml:"app_only"`
	SkipTitles       []string `toml:"skip_titles"`
	NotUsingTitles   []string `toml:"not_using_titles"`
}

func defaultConfig() Config {
	return Config{IntervalSeconds: 5, HeartbeatSeconds: 300, IdleMinutes: 15, Battery: true}
}

// shell bits that steal focus for a moment - keep last title
var builtinSkip = []string{
	"", "Program Manager", "Task Switching", "Task View", "Search", "New notification", "Notification Center",
	"Action center", "Quick settings", "System tray overflow window.", "Snap Assist", "Date and Time Information",
	"任务切换", "任务视图", "搜索", "新通知", "通知中心", "操作中心", "快速设置", "系统托盘溢出窗口。", "贴靠助手", "日期和时间信息", "网络连接", "电池信息",
	"タスクの切り替え", "タスク ビュー", "検索", "通知センター", "アクション センター", "クイック設定",
}

var builtinNotUsing = []string{
	"Start", "Windows Default Lock Screen", "启动", "「开始」菜单", "Windows 默认锁屏界面", "スタート",
}

func (c Config) validate() error {
	switch {
	case strings.TrimSpace(c.Server) == "":
		return fmt.Errorf("server is empty")
	case strings.TrimSpace(c.Secret) == "":
		return fmt.Errorf("secret is empty")
	case strings.TrimSpace(c.DeviceID) == "" || strings.TrimSpace(c.DeviceName) == "":
		return fmt.Errorf("device_id and device_name are required")
	case c.IntervalSeconds < 1 || c.HeartbeatSeconds < c.IntervalSeconds || c.IdleMinutes < 1:
		return fmt.Errorf("need interval_seconds >= 1, heartbeat_seconds >= interval_seconds, idle_minutes >= 1")
	}
	return nil
}

// base url or full .../api/device/set, both ok
func endpoint(server string) string {
	s := strings.TrimRight(strings.TrimSpace(server), "/")
	if !strings.HasSuffix(s, "/api/device/set") {
		s += "/api/device/set"
	}
	return s
}

type Battery struct {
	Percent  int
	Charging bool
}

type Sample struct {
	Title   string
	App     string // program name of the foreground window, may be empty
	Idle    time.Duration
	Battery *Battery
}

type Sampler interface {
	Sample() (Sample, error)
}

type report struct {
	Using  bool
	Status string
}

type Client struct {
	cfg      Config
	sampler  Sampler
	http     *http.Client
	url      string
	skip     map[string]bool
	notUsing map[string]bool
	last     *report
	lastSent time.Time
	now      func() time.Time
	logf     func(string, ...any)
}

func NewClient(cfg Config, sampler Sampler, logf func(string, ...any)) *Client {
	c := &Client{cfg: cfg, sampler: sampler, http: &http.Client{Timeout: 10 * time.Second}, url: endpoint(cfg.Server),
		skip: map[string]bool{}, notUsing: map[string]bool{}, now: time.Now, logf: logf}
	for _, t := range append(append([]string{}, builtinSkip...), cfg.SkipTitles...) {
		c.skip[t] = true
	}
	for _, t := range append(append([]string{}, builtinNotUsing...), cfg.NotUsingTitles...) {
		c.notUsing[t] = true
	}
	return c
}

func (c *Client) decide(s Sample) (report, bool) {
	title := strings.TrimSpace(s.Title)
	if s.Idle >= time.Duration(c.cfg.IdleMinutes)*time.Minute || c.notUsing[title] {
		return report{Using: false}, true
	}
	if c.skip[title] {
		return report{}, false
	}
	if c.cfg.AppOnly {
		if s.App != "" {
			title = s.App
		} else if i := strings.LastIndex(title, " - "); i >= 0 {
			title = title[i+3:] // "page - Microsoft Edge": the app is usually last
		}
	} else if c.cfg.AppFirst && strings.Contains(title, " - ") {
		parts := strings.Split(title, " - ")
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		title = strings.Join(parts, " - ")
	}
	if r := []rune(title); len(r) > 500 {
		title = string(r[:500])
	}
	if c.cfg.Battery && s.Battery != nil {
		plug := ""
		if s.Battery.Charging {
			plug = "⚡"
		}
		title = fmt.Sprintf("[🔋%d%%%s] %s", s.Battery.Percent, plug, title)
	}
	return report{Using: true, Status: title}, true
}

func (c *Client) Tick(ctx context.Context) error {
	s, err := c.sampler.Sample()
	if err != nil {
		return err
	}
	r, ok := c.decide(s)
	if !ok {
		return nil
	}
	heartbeat := time.Duration(c.cfg.HeartbeatSeconds) * time.Second
	if c.last != nil && *c.last == r && c.now().Sub(c.lastSent) < heartbeat {
		return nil
	}
	if err := c.send(ctx, r); err != nil {
		return err // last untouched -> retried next tick
	}
	if c.last == nil || *c.last != r {
		c.logf("sent: using=%v %s", r.Using, r.Status)
	}
	c.last, c.lastSent = &r, c.now()
	return nil
}

func (c *Client) send(ctx context.Context, r report) error {
	body := map[string]any{"id": c.cfg.DeviceID, "show_name": c.cfg.DeviceName, "using": r.Using}
	if r.Using {
		body["status"] = r.Status
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.Secret)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

func (c *Client) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop()
	failing := false
	for {
		if err := c.Tick(ctx); err != nil && ctx.Err() == nil {
			if !failing {
				c.logf("error: %v (retrying every %ds)", err, c.cfg.IntervalSeconds)
			}
			failing = true
		} else if err == nil && failing {
			c.logf("back online")
			failing = false
		}
		select {
		case <-ctx.Done():
			bye, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := c.send(bye, report{Using: false}); err != nil {
				c.logf("could not send 'not using' on exit: %v", err)
			}
			cancel()
			return
		case <-ticker.C:
		}
	}
}
