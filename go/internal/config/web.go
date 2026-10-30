package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Web serves the config API (the pages live in webui and are served by bb8-web,
// off the robot; they reach this API through the relay):
//
//	GET   /api/config           {rev, values, live}
//	GET   /api/config/schema    fields
//	PATCH /api/config           {baseRev, values} → 409 with the latest config on a stale rev
//	GET   /api/config/events    SSE: one "change" event per applied write, from any source
//	GET   /api/config/audit     last changes
//
// live() adds read-only robot state (head connected, applied rev…).
func Web(s *Store, live func() map[string]any) http.Handler {
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	current := func() map[string]any {
		vals, rev := s.Snapshot()
		out := map[string]any{"rev": rev, "values": vals}
		if live != nil {
			out["live"] = live()
		}
		return out
	}
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, current()) })
	mux.HandleFunc("GET /api/config/schema", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.Schema()) })
	mux.HandleFunc("GET /api/config/audit", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.Audit()) })
	mux.HandleFunc("PATCH /api/config", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			BaseRev *int64         `json:"baseRev"`
			Values  map[string]any `json:"values"`
		}
		b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err := json.Unmarshal(b, &in); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
		rev, err := s.Set(in.Values, SourceWeb, in.BaseRev)
		switch {
		case errors.Is(err, ErrConflict):
			out := current()
			out["error"] = "the config changed meanwhile; reloaded"
			writeJSON(w, 409, out)
		case err != nil:
			writeJSON(w, 400, map[string]any{"error": err.Error()})
		default:
			out := current()
			out["rev"] = rev
			writeJSON(w, 200, out)
		}
	})
	mux.HandleFunc("GET /api/config/events", func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no streaming", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		ch := make(chan Change, 16)
		unsub := s.Subscribe(func(c Change) {
			select {
			case ch <- c:
			default: // slow client: it will resync on the next event
			}
		})
		defer unsub()
		_, rev := s.Snapshot()
		fmt.Fprintf(w, "event: hello\ndata: {\"rev\":%d}\n\n", rev)
		fl.Flush()
		ping := time.NewTicker(20 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case c := <-ch:
				b, _ := json.Marshal(c)
				fmt.Fprintf(w, "event: change\ndata: %s\n\n", b)
				fl.Flush()
			case <-ping.C:
				fmt.Fprint(w, ": ping\n\n")
				fl.Flush()
			}
		}
	})
	return mux
}
