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

//go:embed publico
var public embed.FS

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

// PublicRoutes mounts the pages anyone may see without signing in: what BB-8
// is, the privacy policy and the terms (Google's sign-in screen links to them),
// and the logo.
func PublicRoutes(mux *http.ServeMux) {
	for path, file := range map[string]string{"GET /acerca": "acerca.html", "GET /privacidad": "privacidad.html",
		"GET /terminos": "terminos.html", "GET /logo.svg": "logo.svg"} {
		b, _ := public.ReadFile("publico/" + file)
		ct := "text/html; charset=utf-8"
		if file == "logo.svg" {
			ct = "image/svg+xml"
		}
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Cache-Control", "max-age=3600")
			_, _ = w.Write(b)
		})
	}
}
