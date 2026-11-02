package sleepy

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type TextStore struct {
	mu       sync.Mutex
	filename string
}

func cleanString(v any, limit int) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	r := []rune(strings.TrimSpace(s))
	if len(r) > limit {
		r = r[:limit]
	}
	return string(r)
}
func sanitizeText(raw map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for _, group := range []string{"page", "time", "status"} {
		if v, ok := raw[group]; ok && v != nil {
			if _, ok := v.(map[string]any); !ok {
				return nil, apiError{400, group + " must be an object"}
			}
		}
	}
	for group, keys := range map[string][]string{
		"page": {"title", "name", "device", "app", "updated", "live", "connecting", "reconnecting", "footer"},
		"time": {"just_now", "minutes", "hours", "yesterday", "days"},
	} {
		values, _ := raw[group].(map[string]any)
		clean := map[string]any{}
		for _, key := range keys {
			limit := 300
			if key == "footer" {
				limit = 2000
			}
			if s := cleanString(values[key], limit); s != "" {
				clean[key] = s
			}
		}
		if group == "page" {
			for _, key := range []string{"battery_tag", "music_tag"} {
				if values[key] == "off" {
					clean[key] = "off"
				}
			}
		}
		if len(clean) > 0 {
			out[group] = clean
		}
	}
	statuses, _ := raw["status"].(map[string]any)
	cleanStatuses := map[string]any{}
	for id, item := range statuses {
		if _, err := strconv.Atoi(id); err != nil {
			continue
		}
		fields, ok := item.(map[string]any)
		if !ok {
			continue
		}
		clean := map[string]any{}
		for _, key := range []string{"emoji", "color"} {
			if s := cleanString(fields[key], 16); s != "" {
				if key == "color" && s != "green" && s != "grey" && s != "amber" && s != "red" && s != "blue" {
					continue
				}
				clean[key] = s
			}
		}
		for _, key := range []string{"title", "desc", "idle"} {
			var list []any
			switch v := fields[key].(type) {
			case string:
				for _, s := range strings.Split(v, "\n") {
					list = append(list, s)
				}
			case []any:
				list = v
			}
			lines := []string{}
			for _, v := range list {
				if str := cleanString(v, 300); str != "" {
					lines = append(lines, str)
				}
				if len(lines) == 20 {
					break
				}
			}
			if len(lines) > 0 {
				clean[key] = lines
			}
		}
		if len(clean) > 0 {
			cleanStatuses[id] = clean
		}
	}
	if len(cleanStatuses) > 0 {
		out["status"] = cleanStatuses
	}
	return out, nil
}
func (t *TextStore) load() map[string]any {
	t.mu.Lock()
	defer t.mu.Unlock()
	data, err := os.ReadFile(t.filename)
	if os.IsNotExist(err) {
		return map[string]any{}
	}
	var raw map[string]any
	if err == nil {
		err = json.Unmarshal(data, &raw)
	}
	if err == nil {
		var clean map[string]any
		clean, err = sanitizeText(raw)
		if err == nil {
			return clean
		}
	}
	log.Printf("cannot read custom text; using defaults: %v", err)
	return map[string]any{}
}
func (t *TextStore) save(raw map[string]any) (map[string]any, error) {
	if raw == nil {
		return nil, apiError{400, "text must be an object"}
	}
	clean, err := sanitizeText(raw)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(clean, "", "  ")
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err = os.MkdirAll(filepath.Dir(t.filename), 0700); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(filepath.Dir(t.filename), ".text-*.tmp")
	if err != nil {
		return nil, err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return nil, err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	if err = os.Rename(file.Name(), t.filename); err != nil {
		return nil, fmt.Errorf("save text: %w", err)
	}
	return clean, nil
}
