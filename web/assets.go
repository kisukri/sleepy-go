package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"html/template"
	"io/fs"
)

//go:embed templates/*.html static/* editor/* favicon.ico
var Files embed.FS

func Version() string {
	h := sha256.New()
	_ = fs.WalkDir(Files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := Files.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write([]byte(path))
		h.Write([]byte{0})
		h.Write(data)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func Templates() (*template.Template, error) { return template.ParseFS(Files, "templates/*.html") }
