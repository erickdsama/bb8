package websearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erickdsama/bb8/go/internal/config"
)

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		pl := in["plugins"].([]any)[0].(map[string]any)
		if in["model"] != "cheap/model" || pl["id"] != "web" || pl["max_results"] != 3.0 {
			t.Errorf("request %v", in)
		}
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{"choices":[{"message":{"content":" Ganó Tigres 2-1. ","annotations":[
			{"type":"url_citation","url_citation":{"url":"https://a.mx/x","title":"A"}},
			{"type":"url_citation","url_citation":{"url":"https://a.mx/x","title":"A"}}]}}]}`))
	}))
	defer srv.Close()
	w := New()
	c, _ := config.Open("")
	c.Register(w.Fields()...)
	c.Register(config.Field{Key: "agent.api_key", Type: config.Secret}, config.Field{Key: "agent.base_url", Type: config.String},
		config.Field{Key: "agent.model", Type: config.String}, config.Field{Key: "home.city", Type: config.String},
		config.Field{Key: "home.timezone", Type: config.String})
	if _, err := w.Search(context.Background(), c, "x"); err == nil {
		t.Fatal("no key should fail")
	}
	c.Set(map[string]any{"agent.api_key": "k", "agent.base_url": srv.URL, keyModel: "cheap/model", keyResults: 3.0}, config.SourceWeb, nil)
	r, err := w.Search(context.Background(), c, "¿Quién ganó?")
	if err != nil || r["answer"] != "Ganó Tigres 2-1." || len(r["sources"].([]map[string]string)) != 1 {
		t.Fatal(r, err)
	}
}
