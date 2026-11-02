package sleepy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPrecedenceAndAliases(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{
		".env":        "sleepy_main_secret=test-secret\nsleepy_page_name=dotenv\n",
		"config.yaml": "page:\n  name: YAML\n  title: 测试\n",
		"config.toml": "[page]\nname = 'TOML'\n",
		"config.json": `{"page":{"name":"JSON"}}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SLEEPY_PAGE_NAME", "environment")
	t.Setenv("sleepy_status_refresh_interval", "2500")
	c, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Page.Name != "JSON" || c.Page.Title != "测试" || c.Status.RefreshInterval != 2500 {
		t.Fatalf("incorrect config: %+v", c.Page)
	}
}
func TestRejectEmptySecretAndUnsupportedFeatures(t *testing.T) {
	t.Setenv("sleepy_main_secret", "")
	if _, err := loadConfig(t.TempDir()); err == nil {
		t.Fatal("empty secret accepted")
	}
	t.Setenv("sleepy_main_secret", "test-secret")
	t.Setenv("sleepy_plugins_enabled", `["external_python_plugin"]`)
	if _, err := loadConfig(t.TempDir()); err == nil {
		t.Fatal("unsupported plugin accepted")
	}
}

func TestLanguageDefaults(t *testing.T) {
	t.Setenv("sleepy_main_secret", "test-secret")
	c, err := loadConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if c.Page.Language != "ja" || c.Status.List[0].Name != "オンライン" || c.Status.NotUsing != "退席中" {
		t.Fatalf("ja defaults: %q %+v %q", c.Page.Language, c.Status.List, c.Status.NotUsing)
	}
	t.Setenv("sleepy_page_language", "zh-CN")
	if c, err = loadConfig(t.TempDir()); err != nil || c.Page.Language != "zh" || c.Status.List[1].Name != "似了" || c.Status.NotUsing != "未在使用" {
		t.Fatalf("zh defaults: %v %+v", err, c.Status)
	}
	// explicit list / not_using always win
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("page:\n  language: en\nstatus:\n  not_using: away\n  status_list:\n    - name: Here\n      desc: hi\n      color: awake\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if c, err = loadConfig(dir); err != nil || c.Page.Language != "en" || len(c.Status.List) != 1 || c.Status.List[0].Name != "Here" || c.Status.NotUsing != "away" {
		t.Fatalf("explicit: %v %+v", err, c.Status)
	}
	t.Setenv("sleepy_page_language", "fr")
	if _, err = loadConfig(t.TempDir()); err == nil {
		t.Fatal("unknown language accepted")
	}
}
