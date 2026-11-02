package sleepy

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Status struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Desc  string `json:"desc"`
	Color string `json:"color"`
}
type Config struct {
	Main struct {
		Database    string `json:"database"`
		Host        string `json:"host"`
		Port        int    `json:"port"`
		Secret      string `json:"secret"`
		Timezone    string `json:"timezone"`
		CacheAge    int    `json:"cache_age"`
		CORSOrigins any    `json:"cors_origins"`
		HTTPS       bool   `json:"https"`
		SSLCert     string `json:"ssl_cert"`
		SSLKey      string `json:"ssl_key"`

		DeviceSecret     string   `json:"device_secret"`
		AllowQuerySecret bool     `json:"allow_query_secret"`
		AllowedHosts     []string `json:"allowed_hosts"`
	} `json:"main"`
	Page struct {
		Name          string `json:"name"`
		Title         string `json:"title"`
		Desc          string `json:"desc"`
		Favicon       string `json:"favicon"`
		Background    string `json:"background"`
		Theme         string `json:"theme"`
		Language      string `json:"language"`
		MoreText      string `json:"more_text"`
		LearnMoreText string `json:"learn_more_text"`
		LearnMoreLink string `json:"learn_more_link"`
	} `json:"page"`
	Status struct {
		DeviceSlice     int      `json:"device_slice"`
		RefreshInterval int      `json:"refresh_interval"`
		NotUsing        string   `json:"not_using"`
		Sorted          bool     `json:"sorted"`
		UsingFirst      bool     `json:"using_first"`
		List            []Status `json:"status_list"`
	} `json:"status"`
	PluginsEnabled []string                  `json:"plugins_enabled"`
	Plugin         map[string]map[string]any `json:"plugin"`
	DataDir        string                    `json:"-"`
}

func defaults() Config {
	var c Config
	c.Main.Database = "sqlite:///../data/data.db"
	c.Main.Host = "127.0.0.1"
	c.Main.Port = 9010
	c.Main.Timezone = "Asia/Shanghai"
	c.Main.CacheAge = 1200
	c.Main.CORSOrigins = "*"
	c.Main.SSLCert = "data/cert.pem"
	c.Main.SSLKey = "data/key.pem"
	c.Main.AllowQuerySecret = true
	c.Page.Name = "User"
	c.Page.Title = "User Alive?"
	c.Page.Desc = "User's Online Status Page"
	c.Page.Theme = "paper"
	c.Page.Language = "ja"
	c.Page.Favicon = "/favicon.ico"
	c.Status.DeviceSlice = 50
	c.Status.RefreshInterval = 5000
	c.Status.NotUsing = "未在使用"
	c.Status.List = []Status{{0, "活着", "目前在线，可以通过任何可用的联系方式联系本人。", "awake"}, {1, "似了", "睡似了或其他原因不在线，紧急情况请使用电话联系。", "sleeping"}}
	c.PluginsEnabled = []string{"v4_compatible", "custom_text"}
	return c
}
func (c Config) enabled(name string) bool {
	for _, s := range c.PluginsEnabled {
		if s == name {
			return true
		}
	}
	return false
}
func mergeMap(dst, src map[string]any) {
	for k, v := range src {
		if nested, ok := v.(map[string]any); ok {
			existing, ok := dst[k].(map[string]any)
			if !ok {
				existing = map[string]any{}
			}
			mergeMap(existing, nested)
			dst[k] = existing
		} else {
			dst[k] = v
		}
	}
}

func loadConfig(dir string) (Config, error) {
	c := defaults()
	c.DataDir = dir
	env := map[string]string{}
	dotenv, err := godotenv.Read(filepath.Join(dir, ".env"))
	if err != nil && !os.IsNotExist(err) {
		return c, err
	}
	for k, v := range dotenv {
		env[strings.ToLower(k)] = v
	}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		env[strings.ToLower(k)] = v
	}
	raw, _ := json.Marshal(c)
	var schema map[string]any
	_ = json.Unmarshal(raw, &schema)

	// env stays raw str till files merged; file wins even if env is garbage
	overrides := map[string]any{}
	for section, value := range schema {
		fields, ok := value.(map[string]any)
		if !ok {
			fields = map[string]any{"": value}
		}
		for key := range fields {
			name := "sleepy_" + section
			if key != "" {
				name += "_" + key
			}
			val, exists := env[name]
			if !exists {
				continue
			}
			if key == "" {
				overrides[section] = envValue(val)
				continue
			}
			nested, _ := overrides[section].(map[string]any)
			if nested == nil {
				nested = map[string]any{}
				overrides[section] = nested
			}
			nested[key] = envValue(val)
		}
	}
	for _, ext := range []string{"yaml", "toml", "json"} {
		data, e := os.ReadFile(filepath.Join(dir, "config."+ext))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return c, e
		}
		var values map[string]any
		switch ext {
		case "yaml":
			e = yaml.Unmarshal(data, &values)
		case "toml":
			e = toml.Unmarshal(data, &values)
		default:
			e = json.Unmarshal(data, &values)
		}
		if e != nil {
			return c, fmt.Errorf("config.%s: %w", ext, e)
		}
		mergeMap(overrides, values)
	}
	if err = convertEnv(overrides, schema); err != nil {
		return c, err
	}
	raw, err = json.Marshal(overrides)
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	c.Page.Language = normLang(c.Page.Language)
	if c.Page.Language == "" {
		return c, fmt.Errorf("page.language must be ja, zh or en")
	}
	st, _ := overrides["status"].(map[string]any)
	if _, ok := st["status_list"]; !ok {
		c.Status.List = append([]Status(nil), langStatus[c.Page.Language]...)
	}
	if _, ok := st["not_using"]; !ok {
		c.Status.NotUsing = langNotUsing[c.Page.Language]
	}
	c.Main.DeviceSecret = strings.TrimSpace(c.Main.DeviceSecret)
	if c.Main.DeviceSecret != "" && c.Main.DeviceSecret == c.Main.Secret {
		return c, fmt.Errorf("main.device_secret must differ from main.secret")
	}
	for i, h := range c.Main.AllowedHosts {
		c.Main.AllowedHosts[i] = strings.ToLower(strings.TrimSpace(h))
	}
	if strings.TrimSpace(c.Main.Secret) == "" {
		return c, fmt.Errorf("set a non-empty sleepy_main_secret or main.secret in %s/config.yaml", dir)
	}
	if c.Main.Port < 1 || c.Main.Port > 65535 {
		return c, fmt.Errorf("port must be 1..65535")
	}
	if c.Main.CacheAge < 0 || c.Status.RefreshInterval < 1 {
		return c, fmt.Errorf("invalid cache_age or refresh_interval")
	}
	if len(c.Status.List) == 0 {
		return c, fmt.Errorf("status_list cannot be empty")
	}
	for i := range c.Status.List {
		c.Status.List[i].ID = i
	}
	if _, err = time.LoadLocation(c.Main.Timezone); err != nil {
		return c, fmt.Errorf("timezone: %w", err)
	}
	if c.Page.Theme != "paper" {
		return c, fmt.Errorf("Go server supports page.theme=paper; Python themes/plugins are not loaded")
	}
	for _, p := range c.PluginsEnabled {
		if p != "custom_text" && p != "v4_compatible" && p != "auto_dead" {
			return c, fmt.Errorf("unsupported Python plugin: %s", p)
		}
	}
	return c, nil
}

type envValue string

func convertEnv(overrides, schema map[string]any) error {
	for section, value := range overrides {
		if v, ok := value.(envValue); ok {
			parsed, err := parseEnv("sleepy_"+section, section, "", schema[section], string(v))
			if err != nil {
				return err
			}
			overrides[section] = parsed
			continue
		}
		nested, ok := value.(map[string]any)
		if !ok {
			continue
		}
		defaults, _ := schema[section].(map[string]any)
		for key, item := range nested {
			if v, ok := item.(envValue); ok {
				parsed, err := parseEnv("sleepy_"+section+"_"+key, section, key, defaults[key], string(v))
				if err != nil {
					return err
				}
				nested[key] = parsed
			}
		}
	}
	return nil
}

func parseEnv(name, section, key string, def any, val string) (any, error) {
	var parsed any
	if section == "main" && key == "allowed_hosts" && !strings.HasPrefix(strings.TrimSpace(val), "[") {
		var hosts []any
		for _, h := range strings.Split(val, ",") {
			if h = strings.TrimSpace(h); h != "" {
				hosts = append(hosts, h)
			}
		}
		return hosts, nil
	}
	if section == "main" && key == "cors_origins" {
		if strings.HasPrefix(strings.TrimSpace(val), "[") {
			if err := json.Unmarshal([]byte(val), &parsed); err != nil {
				return nil, fmt.Errorf("%s must contain a JSON array or an origin", name)
			}
			return parsed, nil
		}
		return val, nil
	}
	switch def.(type) {
	case string:
		return val, nil
	case float64:
		n, err := strconv.Atoi(strings.TrimSpace(val))
		if err != nil {
			return nil, fmt.Errorf("%s must be an integer", name)
		}
		return n, nil
	case bool:
		b, err := parseBool(strings.TrimSpace(val))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return b, nil
	}
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		return nil, fmt.Errorf("%s must contain JSON", name)
	}
	return parsed, nil
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "1", "t", "yes", "y", "on", "enable", "v":
		return true, nil
	case "false", "0", "f", "no", "n", "off", "disable", "x":
		return false, nil
	}
	return false, fmt.Errorf("expected a boolean")
}

// zh keeps the old defaults
var langStatus = map[string][]Status{
	"ja": {{0, "オンライン", "いつでも気軽に連絡してください。", "awake"}, {1, "オフライン", "今は連絡がつきません。急ぎの用件はお電話でお願いします。", "sleeping"}},
	"zh": {{0, "活着", "目前在线，可以通过任何可用的联系方式联系本人。", "awake"}, {1, "似了", "睡似了或其他原因不在线，紧急情况请使用电话联系。", "sleeping"}},
	"en": {{0, "Online", "Feel free to reach out anytime.", "awake"}, {1, "Offline", "Away right now. For anything urgent, please call.", "sleeping"}},
}
var langNotUsing = map[string]string{"ja": "退席中", "zh": "未在使用", "en": "Idle"}

func normLang(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case s == "" || s == "ja" || strings.HasPrefix(s, "ja-") || strings.HasPrefix(s, "ja_"):
		return "ja"
	case s == "zh" || strings.HasPrefix(s, "zh-") || strings.HasPrefix(s, "zh_"):
		return "zh"
	case s == "en" || strings.HasPrefix(s, "en-") || strings.HasPrefix(s, "en_"):
		return "en"
	}
	return ""
}

func (c Config) address() string { return net.JoinHostPort(c.Main.Host, strconv.Itoa(c.Main.Port)) }
