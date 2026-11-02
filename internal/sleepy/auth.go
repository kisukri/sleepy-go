package sleepy

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const sessionCookie = "sleepy-session"

type role int

const (
	roleNone role = iota
	roleDevice
	roleAdmin
)

func (s *Server) tokenRole(token string) role {
	if token == "" {
		return roleNone
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.c.Main.Secret)) == 1 {
		return roleAdmin
	}
	if d := s.c.Main.DeviceSecret; d != "" && subtle.ConstantTimeCompare([]byte(token), []byte(d)) == 1 {
		return roleDevice
	}
	return roleNone
}

func (s *Server) explicitRole(r *http.Request) role {
	tokens := []string{r.Header.Get("Sleepy-Secret")}
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		tokens = append(tokens, strings.TrimPrefix(a, "Bearer "))
	}
	if s.c.Main.AllowQuerySecret {
		tokens = append(tokens, r.URL.Query().Get("secret"))
	}
	var fromBody string
	_ = json.Unmarshal(body(r)["secret"], &fromBody)
	tokens = append(tokens, fromBody)
	best := roleNone
	for _, t := range tokens {
		if got := s.tokenRole(t); got > best {
			best = got
		}
	}
	return best
}

func (s *Server) authorizeDevice(r *http.Request) error {
	if s.explicitRole(r) != roleNone {
		return nil
	}
	return s.authorize(r, true)
}

func (s *Server) sessionKeyFile() string { return filepath.Join(s.c.DataDir, "session.key") }

func (s *Server) loadSessionKey() error {
	data, err := os.ReadFile(s.sessionKeyFile())
	if err == nil && len(data) >= 32 {
		s.sessionKey = data
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read session key: %w", err)
	}
	return s.rotateSessionKey()
}

func (s *Server) rotateSessionKey() error {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	s.sessionMu.Lock()
	s.sessionKey = key
	s.sessionMu.Unlock()
	if err := os.MkdirAll(s.c.DataDir, 0700); err != nil {
		return fmt.Errorf("save session key: %w", err)
	}
	tmp, err := os.CreateTemp(s.c.DataDir, ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("save session key: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(key); err == nil {
		err = tmp.Close()
	} else {
		tmp.Close()
	}
	if err == nil {
		err = os.Rename(tmp.Name(), s.sessionKeyFile())
	}
	if err != nil {
		return fmt.Errorf("save session key: %w", err)
	}
	return nil
}

func (s *Server) signature(payload string) string {
	s.sessionMu.RLock()
	key := append([]byte(s.c.Main.Secret+"\x00"), s.sessionKey...)
	s.sessionMu.RUnlock()
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("sleepy-session-v1:" + payload))
	return hex.EncodeToString(mac.Sum(nil))
}
func (s *Server) sessionOK(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	payload, sig, ok := strings.Cut(cookie.Value, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(s.signature(payload))) {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return false
	}
	expiry, _, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return false
	}
	ts, err := strconv.ParseInt(expiry, 10, 64)
	return err == nil && time.Now().Unix() < ts
}

func originMismatch(r *http.Request) string {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return "Sec-Fetch-Site is cross-site"
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return ""
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	u, err := url.Parse(origin)
	if err == nil && u.Host == r.Host && u.Scheme == scheme {
		return ""
	}
	return fmt.Sprintf("Origin %s does not match %s://%s (behind a reverse proxy, keep the Host header and set X-Forwarded-Proto)", origin, scheme, r.Host)
}

func (s *Server) authorize(r *http.Request, write bool) error {
	switch s.explicitRole(r) {
	case roleAdmin:
		return nil
	case roleDevice:
		return apiError{403, "device secret can only report devices"}
	}
	if s.sessionOK(r) {
		if write && r.Method == http.MethodGet {
			return apiError{403, "browser writes require POST"}
		}
		if reason := originMismatch(r); write && reason != "" {
			return apiError{403, "cross-origin browser write rejected: " + reason}
		}
		return nil
	}
	return apiError{401, "Wrong Secret"}
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	if reason := originMismatch(r); reason != "" {
		return apiError{403, "cross-origin login rejected: " + reason}
	}
	if err := s.authorize(r, false); err != nil {
		return err
	}
	random := make([]byte, 24)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	expiry := time.Now().Add(30 * 24 * time.Hour)
	payload := base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(expiry.Unix(), 10) + ":" + hex.EncodeToString(random)))
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: payload + "." + s.signature(payload), Path: "/", MaxAge: 30 * 24 * 3600, Expires: expiry, HttpOnly: true, Secure: r.TLS != nil || s.c.Main.HTTPS || r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteLaxMode})
	return writeJSON(w, 200, map[string]any{"success": true, "code": "OK", "message": "Login successful"})
}

func clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// rotate key = everyone logged out. ok, only me
func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if s.sessionOK(r) && originMismatch(r) == "" {
		if err := s.rotateSessionKey(); err != nil {
			log.Printf("logout: %v", err)
		}
	}
	clearSession(w)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	return nil
}
