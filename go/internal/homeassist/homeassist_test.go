package homeassist

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"golang.org/x/net/websocket"

	"github.com/erickdsama/bb8/go/internal/connections"
)

// fakeHA answers the few REST and WebSocket calls the bridge makes.
type fakeHA struct {
	mu      sync.Mutex
	exposed []string
	posts   []map[string]any
}

func (f *fakeHA) server(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer ha-token" {
				w.WriteHeader(401)
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/config/config_entries/entry", auth(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"domain":"tuya","title":"Smart Life","state":"loaded"},{"domain":"cast","title":"Cast","state":"loaded"}]`))
	}))
	mux.HandleFunc("POST /api/config/config_entries/flow", auth(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		if in["handler"] != "tuya" {
			w.Write([]byte(`{"type":"abort","flow_id":"f2","handler":"` + in["handler"].(string) + `","reason":"already_configured"}`))
			return
		}
		w.Write([]byte(`{"type":"form","flow_id":"f1","handler":"tuya","step_id":"user",
			"data_schema":[{"name":"user_code","type":"string","required":true}],"errors":{}}`))
	}))
	mux.HandleFunc("POST /api/config/config_entries/flow/f1", auth(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		f.posts = append(f.posts, in)
		n := len(f.posts)
		f.mu.Unlock()
		if n == 1 {
			w.Write([]byte(`{"type":"form","flow_id":"f1","handler":"tuya","step_id":"scan",
				"data_schema":[{"name":"QR","selector":{"qr_code":{"data":"tuyaSmart--qrLogin?token=abc"}}}]}`))
			return
		}
		w.Write([]byte(`{"type":"create_entry","flow_id":"f1","handler":"tuya","title":"Smart Life"}`))
	}))
	mux.Handle("/api/websocket", websocket.Handler(func(ws *websocket.Conn) {
		websocket.JSON.Send(ws, map[string]any{"type": "auth_required"})
		var m map[string]any
		websocket.JSON.Receive(ws, &m)
		if m["access_token"] != "ha-token" {
			websocket.JSON.Send(ws, map[string]any{"type": "auth_invalid"})
			return
		}
		websocket.JSON.Send(ws, map[string]any{"type": "auth_ok"})
		for {
			var c map[string]any
			if websocket.JSON.Receive(ws, &c) != nil {
				return
			}
			var res any
			switch c["type"] {
			case "get_states":
				res = []map[string]any{{"entity_id": "light.sala"}, {"entity_id": "switch.patio"}, {"entity_id": "lock.puerta"}, {"entity_id": "sun.sun"}}
			case "config_entries/flow/progress":
				res = []map[string]any{
					{"flow_id": "d1", "handler": "samsungtv", "step_id": "confirm", "context": map[string]any{"source": "zeroconf", "title_placeholders": map[string]any{"device": "Samsung The Frame 65"}}},
					{"flow_id": "f1", "handler": "tuya", "context": map[string]any{"source": "user"}}}
			case "homeassistant/expose_entity":
				f.mu.Lock()
				for _, e := range c["entity_ids"].([]any) {
					f.exposed = append(f.exposed, e.(string))
				}
				f.mu.Unlock()
			case "frontend/get_translations":
				res = map[string]any{"resources": map[string]string{
					"component.tuya.config.step.user.data.user_code": "Código de usuario",
					"component.tuya.config.step.user.title":          "Integración Tuya",
					"component.tuya.config.step.scan.description":    "Escanea el código QR con **Smart Life**.",
				}}
			}
			websocket.JSON.Send(ws, map[string]any{"id": c["id"], "type": "result", "success": true, "result": res})
		}
	}))
	return httptest.NewServer(mux)
}

func TestAddBrand(t *testing.T) {
	fake := &fakeHA{}
	srv := fake.server(t)
	defer srv.Close()
	store, _ := connections.Open("")
	b := New(store, http.DefaultClient)
	ctx := context.Background()
	if h := b.House(ctx); h.Connected {
		t.Fatal("connected without a home connection")
	}
	store.Add(connections.Connection{Kind: "home", Name: "home", URL: srv.URL + "/api/mcp", Auth: connections.Auth{Type: "bearer", Token: "ha-token"}})

	h := b.House(ctx)
	if !h.Connected || h.Devices != 2 || len(h.Found) != 1 || h.Found[0].Label != "Samsung The Frame 65" || h.Found[0].FlowID != "d1" {
		t.Fatalf("%+v", h)
	}
	if !h.Brands[0].Linked || h.Brands[0].Handler != "tuya" || h.Brands[1].Linked {
		t.Fatalf("brands %+v", h.Brands)
	}

	s, err := b.Start(ctx, "tuya", "")
	if err != nil || s.Type != "form" || s.FlowID != "f1" || len(s.Fields) != 1 || s.Fields[0].Label != "Código de usuario" || !s.Fields[0].Required || s.Title != "Integración Tuya" {
		t.Fatalf("%+v %v", s, err)
	}
	s, err = b.Next(ctx, "f1", map[string]any{"user_code": "abc"})
	if err != nil || s.Type != "form" || !strings.HasPrefix(s.QR, "data:image/png;base64,") || len(s.Fields) != 0 || s.Text != "Escanea el código QR con Smart Life." {
		t.Fatalf("%+v %v", s.Fields, err)
	}
	s, err = b.Next(ctx, "f1", map[string]any{})
	if err != nil || s.Type != "done" || s.Devices != 2 {
		t.Fatalf("%+v %v", s, err)
	}
	if strings.Join(fake.exposed, ",") != "light.sala,switch.patio" { // no locks, no sun
		t.Fatal(fake.exposed)
	}
	s, _ = b.Start(ctx, "hue", "")
	if s.Type != "abort" || s.Error != "Already configured" {
		t.Fatalf("%+v", s)
	}

	// Through the page's routes.
	mux := http.NewServeMux()
	b.Routes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/home/link", bytes.NewReader([]byte(`{"handler":"tuya"}`))))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"flow_id":"f1"`) || strings.Contains(rec.Body.String(), "ha-token") {
		t.Fatal(rec.Code, rec.Body.String())
	}
}

func TestField(t *testing.T) {
	f := field(map[string]any{"name": "country", "type": "select", "options": []any{[]any{"mx", "México"}}})
	if f.Type != "select" || f.Options[0].Value != "mx" || f.Options[0].Label != "México" {
		t.Fatalf("%+v", f)
	}
	if f := field(map[string]any{"name": "password", "type": "string"}); f.Type != "password" {
		t.Fatal(f)
	}
	if f := field(map[string]any{"name": "x", "selector": map[string]any{"boolean": map[string]any{}}}); f.Type != "bool" {
		t.Fatal(f)
	}
}
