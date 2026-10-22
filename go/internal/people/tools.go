package people

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/erickdsama/bb8/go/internal/tools"
)

func view(p Person) map[string]any {
	notes := make([]string, len(p.Notes))
	for i, n := range p.Notes {
		notes[i] = n.Text
	}
	m := map[string]any{"name": p.Name, "role": p.Role, "notes": notes, "voice_registered": p.Voice}
	if !p.LastSeen.IsZero() {
		m["last_seen"] = p.LastSeen.Format("Mon 2 Jan 15:04")
	}
	return m
}

// Tools for the agent. Writing notes is KnownOnly: a stranger cannot plant facts.
func (s *Store) Tools() []tools.Tool {
	return []tools.Tool{
		{Name: "people", Description: "Everyone BB-8 knows: name, role (owner, family, guest), notes and when they were last heard.",
			Schema: tools.Object(map[string]any{}),
			Handler: func(context.Context, json.RawMessage) (tools.Output, error) {
				out := []map[string]any{}
				for _, p := range s.List() {
					out = append(out, view(p))
				}
				return tools.Data(map[string]any{"people": out})
			}},
		{Name: "remember_about", KnownOnly: true,
			Description: "Save a short fact about a person to use later (likes, birthday, allergies, what they asked you to remember). " +
				"A new name is added as a guest. Max 200 characters; the oldest of 20 notes is dropped. " +
				"Use the person's name, not \"me\": the speaker's name is in [Speaker: …].",
			Schema: tools.Object(map[string]any{"name": tools.Str("person's name"), "note": tools.Str("the fact, in a short sentence")}, "name", "note"),
			Handler: tools.Bind(func(_ context.Context, in struct{ Name, Note string }) (tools.Output, error) {
				p, err := s.Remember(in.Name, in.Note)
				if err != nil {
					return tools.Data(map[string]any{"result": "error", "reason": err.Error()})
				}
				return tools.Data(map[string]any{"result": "done", "person": view(p)})
			})},
		{Name: "forget_about", KnownOnly: true,
			Description: "Delete the notes about a person that contain the given text. Returns how many were removed.",
			Schema:      tools.Object(map[string]any{"name": tools.Str("person's name"), "text": tools.Str("part of the note to delete")}, "name", "text"),
			Handler: tools.Bind(func(_ context.Context, in struct{ Name, Text string }) (tools.Output, error) {
				n, err := s.Forget(in.Name, in.Text)
				if err != nil {
					return tools.Data(map[string]any{"result": "error", "reason": err.Error()})
				}
				return tools.Data(map[string]any{"result": "done", "removed": n})
			})},
	}
}

// Routes for the people page (webui, served by bb8-web), behind the token:
//
//	GET    /api/people             [person…]
//	PUT    /api/people/{name}      {role} → creates or changes the role
//	POST   /api/people/{name}/notes {text}
//	DELETE /api/people/{name}/notes?text=…
//	DELETE /api/people/{name}
func (s *Store) Routes(mux *http.ServeMux) {
	j := func(w http.ResponseWriter, code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	fail := func(w http.ResponseWriter, err error) {
		code := 400
		if errors.Is(err, ErrUnknown) {
			code = 404
		}
		j(w, code, map[string]any{"error": err.Error()})
	}
	body := func(r *http.Request, v any) error {
		return json.NewDecoder(io.LimitReader(r.Body, 1<<14)).Decode(v)
	}
	mux.HandleFunc("GET /api/people", func(w http.ResponseWriter, r *http.Request) { j(w, 200, s.List()) })
	mux.HandleFunc("PUT /api/people/{name}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Role Role `json:"role"`
		}
		if err := body(r, &in); err != nil {
			fail(w, err)
			return
		}
		p, err := s.Upsert(r.PathValue("name"), in.Role)
		if err != nil {
			fail(w, err)
			return
		}
		j(w, 200, p)
	})
	mux.HandleFunc("POST /api/people/{name}/notes", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Text string `json:"text"`
		}
		if err := body(r, &in); err != nil {
			fail(w, err)
			return
		}
		p, err := s.Remember(r.PathValue("name"), in.Text)
		if err != nil {
			fail(w, err)
			return
		}
		j(w, 200, p)
	})
	mux.HandleFunc("DELETE /api/people/{name}/notes", func(w http.ResponseWriter, r *http.Request) {
		n, err := s.Forget(r.PathValue("name"), r.URL.Query().Get("text"))
		if err != nil {
			fail(w, err)
			return
		}
		j(w, 200, map[string]any{"removed": n})
	})
	mux.HandleFunc("DELETE /api/people/{name}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Remove(r.PathValue("name")); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
