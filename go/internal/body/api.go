package body

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/erickdsama/bb8/go/internal/motion"
	"github.com/erickdsama/bb8/go/internal/tools"
)

// LocalAPI serves 127.0.0.1:8770 for the voice front end, the power service
// and the gamepad bridge. Tools: POST /tools/{name} with the tool's JSON args;
// header X-Speaker-Known: false marks an unknown voice.
func LocalAPI(m *motion.Service, h *Head, reg *tools.Registry) *http.ServeMux {
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	body := func(r *http.Request, v any) error {
		b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil || len(b) == 0 {
			return err
		}
		return json.Unmarshal(b, v)
	}
	bad := func(w http.ResponseWriter, err error) {
		writeJSON(w, 400, map[string]any{"result": "error", "reason": err.Error()})
	}

	mux.HandleFunc("POST /tools/{name}", func(w http.ResponseWriter, r *http.Request) {
		args, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		known := r.Header.Get("X-Speaker-Known") != "false"
		out, err := reg.Call(r.Context(), r.PathValue("name"), args, tools.CallOpts{SpeakerKnown: known})
		if err != nil {
			code := 400
			if errors.Is(err, tools.ErrUnknownVoice) {
				code = 403
			}
			writeJSON(w, code, map[string]any{"result": "error", "reason": err.Error()})
			return
		}
		if out.Image != nil {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(out.Image)
			return
		}
		writeJSON(w, 200, out.Data)
	})
	mux.HandleFunc("GET /tools", func(w http.ResponseWriter, r *http.Request) {
		var names []string
		for _, t := range reg.List() {
			names = append(names, t.Name)
		}
		writeJSON(w, 200, names)
	})
	mux.HandleFunc("POST /manual", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Left, Right float64 }
		if err := body(r, &in); err != nil {
			bad(w, err)
			return
		}
		writeJSON(w, 200, m.Manual(in.Left, in.Right))
	})
	mux.HandleFunc("GET /pose", func(w http.ResponseWriter, r *http.Request) {
		p := m.Pose()
		p["head"] = h.State()
		writeJSON(w, 200, p)
	})
	mux.HandleFunc("POST /sleep", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, m.Sleep()) })
	mux.HandleFunc("POST /wake", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, m.Wake()) })
	mux.HandleFunc("POST /poweroff", func(w http.ResponseWriter, r *http.Request) {
		in := struct {
			Seconds float64 `json:"seconds"`
		}{15}
		if err := body(r, &in); err != nil {
			bad(w, err)
			return
		}
		writeJSON(w, 200, m.PowerOff(in.Seconds))
	})
	mux.HandleFunc("POST /hearing", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Text    *string  `json:"text"`
			Angle   *float64 `json:"angle_deg"`
			LevelDB *float64 `json:"level_db"`
		}
		if err := body(r, &in); err != nil {
			bad(w, err)
			return
		}
		if err := m.Hear(in.Text, in.Angle, in.LevelDB); err != nil {
			bad(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"result": "done"})
	})
	mux.HandleFunc("GET /hearing", func(w http.ResponseWriter, r *http.Request) {
		wait, _ := strconv.ParseFloat(r.URL.Query().Get("wait_s"), 64)
		writeJSON(w, 200, m.Listen(wait))
	})
	// Head passthroughs for clients that used to call the head directly.
	mux.HandleFunc("POST /head/rest", func(w http.ResponseWriter, r *http.Request) {
		in := struct {
			Active bool `json:"active"`
		}{true}
		if err := body(r, &in); err != nil {
			bad(w, err)
			return
		}
		writeJSON(w, 200, h.Rest(r.Context(), in.Active))
	})
	mux.HandleFunc("GET /head/photo", func(w http.ResponseWriter, r *http.Request) {
		width, err := strconv.Atoi(r.URL.Query().Get("width"))
		if err != nil {
			width = 640
		}
		jpg, err := h.Photo(r.Context(), width)
		if err != nil {
			writeJSON(w, 503, map[string]any{"result": "error", "reason": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(jpg)
	})
	mux.HandleFunc("GET /head", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, h.State()) })
	return mux
}

// Logged wraps a handler with a debug log line.
func Logged(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("http", "method", r.Method, "path", r.URL.Path)
		h.ServeHTTP(w, r)
	})
}
