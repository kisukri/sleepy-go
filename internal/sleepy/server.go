package sleepy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kisukri/sleepy-go/web"
)

const version = "5.2-go"

type apiError struct {
	Code    int
	Message string
}

func (e apiError) Error() string { return e.Message }

type bodyKey struct{}

type requestBody struct {
	raw    []byte
	values map[string]json.RawMessage
}

func body(r *http.Request) map[string]json.RawMessage {
	v, _ := r.Context().Value(bodyKey{}).(*requestBody)
	if v == nil {
		return nil
	}
	return v.values
}
func decodeBody(r *http.Request, v any) error {
	b, _ := r.Context().Value(bodyKey{}).(*requestBody)
	if b == nil {
		return apiError{400, "expected a JSON object"}
	}
	dec := json.NewDecoder(bytes.NewReader(b.raw))
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return apiError{400, "invalid field type: " + err.Error()}
	}
	return nil
}

func (s *Server) readBody(w http.ResponseWriter, r *http.Request) (*requestBody, error) {
	// per-req deadline. NOT srv.ReadTimeout -> that one kills SSE ctx
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(s.bodyTimeout))
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	_ = rc.SetReadDeadline(time.Time{})
	if err != nil {
		w.Header().Set("Connection", "close")
		var size *http.MaxBytesError
		var timeout net.Error
		switch {
		case errors.As(err, &size):
			return nil, apiError{413, "request body too large"}
		case errors.As(err, &timeout) && timeout.Timeout():
			return nil, apiError{408, "request body was not received in time"}
		default:
			return nil, apiError{400, "cannot read request body"}
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	var values map[string]json.RawMessage
	if err = dec.Decode(&values); err != nil || values == nil {
		return nil, apiError{400, "expected a JSON object"}
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return nil, apiError{400, "expected one JSON object"}
	}
	return &requestBody{raw: raw, values: values}, nil
}
func writeJSON(w http.ResponseWriter, status int, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(append(raw, '\n'))
	return err
}

type Server struct {
	c            Config
	store        *Store
	text         *TextStore
	templates    *template.Template
	mux          *http.ServeMux
	streams      chan struct{}
	heartbeat    time.Duration
	bodyTimeout  time.Duration
	assets       string
	location     *time.Location
	sessionMu    sync.RWMutex
	sessionKey   []byte
	autoInterval time.Duration
	autoTimeout  time.Duration
	autoStatus   int
}

func newServer(c Config, store *Store) (*Server, error) {
	tmpl, err := web.Templates()
	if err != nil {
		return nil, err
	}
	s := &Server{c: c, store: store, text: &TextStore{filename: filepath.Join(c.DataDir, "text.json")}, templates: tmpl, mux: http.NewServeMux(), streams: make(chan struct{}, 256), heartbeat: 30 * time.Second, bodyTimeout: 10 * time.Second, autoInterval: time.Minute, autoTimeout: 15 * time.Minute, autoStatus: 1}
	if c.enabled("auto_dead") {
		raw, _ := json.Marshal(c.Plugin["auto_dead"])
		conf := struct {
			Interval int `json:"check_interval"`
			Timeout  int `json:"timeout_minutes"`
			Status   int `json:"dead_status_id"`
		}{60, 15, 1}
		if err = json.Unmarshal(raw, &conf); err != nil {
			return nil, fmt.Errorf("auto_dead: %w", err)
		}
		if conf.Interval < 1 || conf.Timeout < 1 || conf.Status < 0 || conf.Status >= len(c.Status.List) {
			return nil, fmt.Errorf("invalid auto_dead configuration")
		}
		s.autoInterval = time.Duration(conf.Interval) * time.Second
		s.autoTimeout = time.Duration(conf.Timeout) * time.Minute
		s.autoStatus = conf.Status
	}
	s.assets = "/assets/" + web.Version()
	if s.location, err = time.LoadLocation(c.Main.Timezone); err != nil {
		return nil, err
	}
	if err = s.loadSessionKey(); err != nil {
		return nil, err
	}
	if len([]rune(c.Main.Secret)) < 16 {
		log.Printf("warning: main.secret is shorter than 16 characters; use a long random secret because login attempts are not rate limited")
	}
	s.routes()
	return s, nil
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("Sleepy-Version", version)
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'self'; object-src 'none'")
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
	}
	if !s.hostAllowed(r) {
		s.writeError(w, r, apiError{421, "unknown host"})
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/static") && !strings.HasPrefix(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "no-store")
	}
	origin := r.Header.Get("Origin")
	allowed := false
	switch origins := s.c.Main.CORSOrigins.(type) {
	case string:
		allowed = origins == "*" || origins == origin
	case []any:
		for _, v := range origins {
			if v == origin {
				allowed = true
			}
		}
	}
	if origin != "" && allowed {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Sleepy-Secret, Last-Event-ID")
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method == http.MethodPost && r.ContentLength != 0 {
		media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if media != "application/json" {
			s.writeError(w, r, apiError{415, "Content-Type must be application/json"})
			return
		}
		b, err := s.readBody(w, r)
		if err != nil {
			s.writeError(w, r, err)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), bodyKey{}, b))
	}
	s.mux.ServeHTTP(w, r)
}
func (s *Server) hostAllowed(r *http.Request) bool {
	if len(s.c.Main.AllowedHosts) == 0 || r.URL.Path == "/none" {
		return true
	}
	host := strings.ToLower(r.Host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	for _, allowed := range s.c.Main.AllowedHosts {
		if host == allowed {
			return true
		}
	}
	return false
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	code := 500
	message := "Internal Server Error"
	var api apiError
	if errors.As(err, &api) {
		code = api.Code
		message = api.Message
	} else {
		log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	}
	data := map[string]any{"success": false, "code": code, "details": http.StatusText(code), "message": message}
	if s.isLegacy(r.URL.Path) {
		label := "exception"
		if code == 401 {
			label = "not authorized"
		} else if code == 400 {
			label = "bad request"
		} else if code == 404 {
			label = "not found"
		}
		data["code"] = label
		delete(data, "details")
	}
	_ = writeJSON(w, code, data)
}
func (s *Server) route(pattern string, f func(http.ResponseWriter, *http.Request) error) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				s.writeError(w, r, fmt.Errorf("panic: %v", e))
			}
		}()
		if err := f(w, r); err != nil {
			s.writeError(w, r, err)
		}
	})
}
func (s *Server) routes() {
	s.route("GET /{$}", func(w http.ResponseWriter, r *http.Request) error { return s.page(w, "index.html") })
	s.route("GET /none", func(w http.ResponseWriter, r *http.Request) error { w.WriteHeader(204); return nil })
	s.route("GET /api/meta", func(w http.ResponseWriter, r *http.Request) error { return writeJSON(w, 200, s.metadata()) })
	s.route("GET /api/status/query", func(w http.ResponseWriter, r *http.Request) error {
		snap, _ := s.store.snapshot()
		data := s.query(snap, false)
		if value, _ := parseBool(r.URL.Query().Get("meta")); value {
			data["meta"] = s.metadata()
		}
		return writeJSON(w, 200, data)
	})
	s.route("GET /api/status/list", s.statusList)
	s.route("GET /api/status/events", func(w http.ResponseWriter, r *http.Request) error { return s.events(w, r, false) })
	for _, method := range []string{"GET", "POST"} {
		s.route(method+" /api/status/set", s.setStatus)
		s.route(method+" /api/device/set", s.setDevice)
		s.route(method+" /api/device/remove", s.removeDevice)
		s.route(method+" /api/device/clear", s.clearDevices)
		s.route(method+" /api/device/private", s.setPrivate)
		s.route(method+" /panel/verify", func(w http.ResponseWriter, r *http.Request) error {
			if err := s.authorize(r, false); err != nil {
				return err
			}
			return writeJSON(w, 200, map[string]any{"success": true, "code": "OK", "message": "Secret verified"})
		})
	}
	s.route("GET /panel", func(w http.ResponseWriter, r *http.Request) error {
		if err := s.authorize(r, false); err != nil {
			http.Redirect(w, r, "/panel/login", 302)
			return nil
		}
		return s.page(w, "panel.html")
	})
	s.route("GET /panel/login", func(w http.ResponseWriter, r *http.Request) error {
		if s.sessionOK(r) {
			http.Redirect(w, r, "/panel", 302)
			return nil
		}
		return s.page(w, "login.html")
	})
	s.route("POST /panel/auth", s.login)

	s.route("GET /panel/logout", func(w http.ResponseWriter, r *http.Request) error {
		clearSession(w)
		http.Redirect(w, r, "/panel/login", 302)
		return nil
	})
	s.route("POST /panel/logout", s.logout)
	s.route("GET /panel/data", func(w http.ResponseWriter, r *http.Request) error {
		if err := s.authorize(r, false); err != nil {
			return err
		}
		snap, _ := s.store.snapshot()
		return writeJSON(w, 200, map[string]any{"success": true, "status": s.status(snap.Status), "private_mode": snap.Private, "devices": snap.Devices, "last_updated": snap.Updated})
	})
	if s.c.enabled("custom_text") {
		s.route("GET /plugin/custom_text/text", func(w http.ResponseWriter, r *http.Request) error {
			return writeJSON(w, 200, map[string]any{"success": true, "text": s.text.load(), "more_text": s.c.Page.MoreText})
		})
		s.route("POST /plugin/custom_text/text", func(w http.ResponseWriter, r *http.Request) error {
			if err := s.authorize(r, true); err != nil {
				return err
			}
			var req struct {
				Text map[string]any `json:"text"`
			}
			if err := decodeBody(r, &req); err != nil {
				return err
			}
			data, err := s.text.save(req.Text)
			if err != nil {
				return err
			}
			return writeJSON(w, 200, map[string]any{"success": true, "text": data})
		})
	}
	s.route("GET /assets/{version}/{path...}", func(w http.ResponseWriter, r *http.Request) error {
		name := r.PathValue("path")
		if strings.HasPrefix(name, "templates/") {
			return apiError{404, "file not found"}
		}
		if "/assets/"+r.PathValue("version") == s.assets {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			return s.serveEmbedded(w, r, name)
		}
		return s.asset(w, r, name)
	})
	s.route("GET /static-themed/{theme}/{file}", func(w http.ResponseWriter, r *http.Request) error {
		if r.PathValue("theme") != "paper" {
			return apiError{404, "theme not found"}
		}
		return s.asset(w, r, "static/"+r.PathValue("file"))
	})
	s.route("GET /static/{file}", func(w http.ResponseWriter, r *http.Request) error {
		return s.asset(w, r, "static/"+r.PathValue("file"))
	})
	s.route("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) error {
		if s.c.Page.Favicon != "/favicon.ico" {
			http.Redirect(w, r, s.c.Page.Favicon, 302)
			return nil
		}
		return s.public(w, r, "favicon.ico")
	})
	s.route("GET /github", func(w http.ResponseWriter, r *http.Request) error {
		http.Redirect(w, r, "https://github.com/kisukri/sleepy-go", 301)
		return nil
	})
	if s.c.enabled("v4_compatible") {
		s.legacyRoutes()
	}
	s.route("GET /{path...}", func(w http.ResponseWriter, r *http.Request) error { return s.public(w, r, r.PathValue("path")) })
}
func (s *Server) metadata() map[string]any {
	return map[string]any{"success": true, "version": []int{5, 2, 0}, "version_str": version, "timezone": s.c.Main.Timezone, "page": s.c.Page, "status": map[string]any{"device_slice": s.c.Status.DeviceSlice, "refresh_interval": s.c.Status.RefreshInterval, "not_using": s.c.Status.NotUsing, "sorted": s.c.Status.Sorted, "using_first": s.c.Status.UsingFirst}, "metrics": false}
}
func (s *Server) status(id int) Status {
	if id >= 0 && id < len(s.c.Status.List) {
		return s.c.Status.List[id]
	}
	return Status{-1, "[未知]", "未知的标识符，可能是配置问题。", "error"}
}
func (s *Server) query(snap Snapshot, legacy bool) map[string]any {
	devices := map[string]any{}
	if !snap.Private {
		for id, device := range snap.Devices {
			if device.Using != nil && !*device.Using && s.c.Status.NotUsing != "" {
				v := s.c.Status.NotUsing
				device.Status = &v
			}
			if legacy {
				devices[id] = map[string]any{"show_name": device.ShowName, "using": device.Using, "app_name": device.Status}
			} else {
				devices[id] = device
			}
		}
	}
	if legacy {
		loc := s.location
		status := s.status(snap.Status)
		return map[string]any{"success": true, "time": time.Now().In(loc).Format("2006-01-02 15:04:05"), "timezone": s.c.Main.Timezone, "status": snap.Status, "info": map[string]any{"name": status.Name, "desc": status.Desc, "color": status.Color}, "device": devices, "last_updated": time.UnixMicro(int64(snap.Updated * 1e6)).In(loc).Format("2006-01-02 15:04:05"), "refresh": s.c.Status.RefreshInterval, "device_status_slice": s.c.Status.DeviceSlice}
	}
	return map[string]any{"success": true, "time": nowSeconds(), "status": s.status(snap.Status), "device": devices, "last_updated": snap.Updated}
}
func (s *Server) statusList(w http.ResponseWriter, r *http.Request) error {
	if s.isLegacy(r.URL.Path) {
		return writeJSON(w, 200, s.c.Status.List)
	}
	return writeJSON(w, 200, map[string]any{"success": true, "status_list": s.c.Status.List})
}
func (s *Server) success(w http.ResponseWriter, r *http.Request, extra map[string]any) error {
	if extra == nil {
		extra = map[string]any{}
	}
	extra["success"] = true
	if s.isLegacy(r.URL.Path) {
		extra["code"] = "OK"
	}
	return writeJSON(w, 200, extra)
}
func param(r *http.Request, key string) (string, error) {
	if raw, ok := body(r)[key]; ok {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return text, nil
		}
		var n json.Number
		if json.Unmarshal(raw, &n) == nil {
			return n.String(), nil
		}
		if string(raw) == "true" || string(raw) == "false" {
			return string(raw), nil
		}
		return "", apiError{400, key + " has an invalid type"}
	}
	return r.URL.Query().Get(key), nil
}

func parseUsing(raw string, legacy bool) (*bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "none", "null":
		return nil, nil
	}
	v, err := parseBool(strings.TrimSpace(raw))
	if err != nil {
		if legacy {
			return nil, nil
		}
		return nil, apiError{400, "using must be boolean"}
	}
	return &v, nil
}

func usingFromJSON(raw json.RawMessage, legacy bool) (*bool, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil, nil
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return &b, nil
	}
	var str string
	if json.Unmarshal(raw, &str) == nil {
		return parseUsing(str, legacy)
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return parseUsing(n.String(), legacy)
	}
	if legacy {
		return nil, nil
	}
	return nil, apiError{400, "using must be boolean"}
}

func (s *Server) setStatus(w http.ResponseWriter, r *http.Request) error {
	if err := s.authorize(r, true); err != nil {
		return err
	}
	text, err := param(r, "status")
	if err != nil {
		return err
	}
	id, err := strconv.Atoi(text)
	if err != nil || id < 0 || id >= len(s.c.Status.List) {
		return apiError{400, "status must be a valid status_list index"}
	}
	if err = s.store.setStatus(id); err != nil {
		return err
	}
	return s.success(w, r, map[string]any{"set_to": id})
}
func (s *Server) setDevice(w http.ResponseWriter, r *http.Request) error {
	if err := s.authorizeDevice(r); err != nil {
		return err
	}
	var req DeviceUpdate
	if r.Method == http.MethodPost {
		var in struct {
			ID       string          `json:"id"`
			ShowName *string         `json:"show_name"`
			Using    json.RawMessage `json:"using"`
			Status   *string         `json:"status"`
			AppName  *string         `json:"app_name"`
			Fields   map[string]any  `json:"fields"`
		}
		if err := decodeBody(r, &in); err != nil {
			return err
		}
		using, err := usingFromJSON(in.Using, s.isLegacy(r.URL.Path))
		if err != nil {
			return err
		}
		req = DeviceUpdate{ID: in.ID, ShowName: in.ShowName, Using: using, Status: in.Status, AppName: in.AppName, Fields: in.Fields}
	} else {
		q := r.URL.Query()
		req.ID = q.Get("id")
		if q.Has("show_name") {
			v := q.Get("show_name")
			req.ShowName = &v
		}
		if q.Has("status") {
			v := q.Get("status")
			req.Status = &v
		}
		if q.Has("app_name") {
			v := q.Get("app_name")
			req.AppName = &v
		}
		if q.Has("using") {
			v, err := parseUsing(q.Get("using"), s.isLegacy(r.URL.Path))
			if err != nil {
				return err
			}
			req.Using = v
		}
		req.Fields = map[string]any{}
		if !s.isLegacy(r.URL.Path) {
			for k, v := range q {
				switch k {
				case "secret", "id", "show_name", "using", "status", "app_name":
					continue
				}
				req.Fields[k] = v[0]
			}
		}
	}
	if req.ID == "" || len([]rune(req.ID)) > 1024 {
		return apiError{400, "device id must contain 1..1024 characters"}
	}
	if req.ShowName != nil && len([]rune(*req.ShowName)) > 1024 {
		return apiError{400, "device show_name is too long"}
	}
	if s.c.enabled("auto_dead") {
		req.reviveFrom = &s.autoStatus
	}
	if err := s.store.setDevice(req); err != nil {
		return err
	}
	return s.success(w, r, nil)
}
func (s *Server) removeDevice(w http.ResponseWriter, r *http.Request) error {
	if err := s.authorize(r, true); err != nil {
		return err
	}
	id, err := param(r, "id")
	if err != nil {
		return err
	}
	if id == "" {
		return apiError{400, "missing device id"}
	}
	if s.isLegacy(r.URL.Path) {
		snap, _ := s.store.snapshot()
		if _, ok := snap.Devices[id]; !ok {
			return apiError{404, "cannot find item"}
		}
	}
	if err = s.store.deleteDevice(id, false); err != nil {
		return err
	}
	return s.success(w, r, nil)
}
func (s *Server) clearDevices(w http.ResponseWriter, r *http.Request) error {
	if err := s.authorize(r, true); err != nil {
		return err
	}
	if err := s.store.deleteDevice("", true); err != nil {
		return err
	}
	return s.success(w, r, nil)
}
func (s *Server) setPrivate(w http.ResponseWriter, r *http.Request) error {
	if err := s.authorize(r, true); err != nil {
		return err
	}
	text, err := param(r, "private")
	if err != nil {
		return err
	}
	v, err := parseBool(text)
	if err != nil {
		return apiError{400, "private must be boolean"}
	}
	if err = s.store.setPrivate(v); err != nil {
		return err
	}
	return s.success(w, r, nil)
}
func (s *Server) asset(w http.ResponseWriter, r *http.Request, name string) error {
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", s.c.Main.CacheAge))
	return s.serveEmbedded(w, r, name)
}

func (s *Server) serveEmbedded(w http.ResponseWriter, r *http.Request, name string) error {
	if !fs.ValidPath(name) {
		return apiError{404, "file not found"}
	}
	data, err := web.Files.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return apiError{404, "file not found"}
	}
	if err != nil {
		return err
	}
	if strings.HasSuffix(name, ".woff2") {
		w.Header().Set("Content-Type", "font/woff2")
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	return nil
}
func (s *Server) public(w http.ResponseWriter, r *http.Request, name string) error {
	if !fs.ValidPath(name) {
		return apiError{404, "file not found"}
	}
	for _, dir := range []string{filepath.Join(s.c.DataDir, "public"), "public"} {
		root, err := os.OpenRoot(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		file, err := root.Open(name)
		root.Close()
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return apiError{404, "file not found"}
		}
		defer file.Close()
		stat, err := file.Stat()
		if err != nil {
			return err
		}
		if stat.IsDir() {
			return apiError{404, "file not found"}
		}
		http.ServeContent(w, r, name, stat.ModTime(), file)
		return nil
	}
	if name == "favicon.ico" {
		return s.asset(w, r, "favicon.ico")
	}
	return apiError{404, "file not found"}
}
func (s *Server) page(w http.ResponseWriter, name string) error {
	snap, _ := s.store.snapshot()
	status := s.status(snap.Status)
	color := map[string]string{"awake": "green", "sleeping": "grey", "error": "red"}[status.Color]
	if color == "" {
		color = "green"
	}
	text, _ := json.Marshal(s.text.load())
	words := web.Words(s.c.Page.Language)
	dict, _ := json.Marshal(words)
	data := struct {
		Title, Desc, Favicon, Name, Color, Assets, Lang string
		Status                                          Status
		MoreText                                        template.HTML
		TextJSON, Words                                 template.JS
		T                                               map[string]string
		CustomText                                      bool
	}{s.c.Page.Title, s.c.Page.Desc, s.c.Page.Favicon, s.c.Page.Name, color, s.assets, web.HTMLLang(s.c.Page.Language), status, template.HTML(s.c.Page.MoreText), template.JS(text), template.JS(dict), words, s.c.enabled("custom_text")}
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write(buf.Bytes())
	return err
}
func (s *Server) isLegacy(path string) bool {
	switch path {
	case "/query", "/events", "/set", "/status_list", "/device/set", "/device/remove", "/device/clear", "/device/private_mode", "/save_data":
		return true
	}
	return false
}
func (s *Server) legacyRoutes() {
	s.route("GET /query", func(w http.ResponseWriter, r *http.Request) error {
		snap, _ := s.store.snapshot()
		return writeJSON(w, 200, s.query(snap, true))
	})
	s.route("GET /events", func(w http.ResponseWriter, r *http.Request) error { return s.events(w, r, true) })
	s.route("GET /status_list", s.statusList)
	for _, method := range []string{"GET", "POST"} {
		for path, handler := range map[string]func(http.ResponseWriter, *http.Request) error{"/set": s.setStatus, "/device/set": s.setDevice, "/device/remove": s.removeDevice, "/device/clear": s.clearDevices, "/device/private_mode": s.setPrivate} {
			s.route(method+" "+path, handler)
		}
	}
	s.route("GET /save_data", func(w http.ResponseWriter, r *http.Request) error {
		if err := s.authorize(r, false); err != nil {
			return err
		}
		if s.c.Plugin["v4_compatible"]["simulate_save_data"] != true {
			return apiError{500, "Save data function is deprecated!"}
		}
		snap, _ := s.store.snapshot()
		old := s.query(snap, true)
		return s.success(w, r, map[string]any{"data": map[string]any{"status": snap.Status, "device_status": old["device"], "last_updated": old["last_updated"]}})
	})
}
