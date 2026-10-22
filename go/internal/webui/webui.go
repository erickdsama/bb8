// Package webui holds BB-8's web pages: settings (/), drive (/control), voice
// (/voice) and people (/people). bb8-web serves them, off the robot; they call
// the robot's API (/api/…, /mcp) through the relay the robot dials out to.
package webui

import (
	"embed"
	"net/http"
)

//go:embed index.html control.html voice.html people.html
var files embed.FS

// Routes mounts the pages.
func Routes(mux *http.ServeMux) {
	for path, file := range map[string]string{"GET /{$}": "index.html", "GET /control": "control.html",
		"GET /voice": "voice.html", "GET /people": "people.html"} {
		b, _ := files.ReadFile(file)
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(b)
		})
	}
}
