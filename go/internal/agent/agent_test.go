package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/tools"
)

// fakeRouter answers like OpenRouter: first a tool call, then text using the tool result.
func fakeRouter(t *testing.T, tool, args string, seen *[]chatReq) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":{"message":"bad key"}}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		var req chatReq
		json.Unmarshal(b, &req)
		*seen = append(*seen, req)
		last := req.Messages[len(req.Messages)-1]
		if last.Role == "user" {
			w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"` + tool + `","arguments":` + quote(args) + `}}]}}]}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"Bip. Resultado: ` + strings.ReplaceAll(last.Content.(string), `"`, `'`) + `"}}]}`))
	}))
}

func setup(t *testing.T, url string) (*Agent, *int) {
	c, _ := config.Open(t.TempDir())
	c.Register(Fields()...)
	c.Set(map[string]any{"agent.api_key": "sk-test", "agent.base_url": url}, config.SourceWeb, nil)
	reg := tools.NewRegistry()
	moved := 0
	reg.Set("robot", []tools.Tool{
		{Name: "move", Moves: true, Schema: tools.Object(map[string]any{"meters": tools.Num("m")}),
			Handler: tools.Bind(func(_ context.Context, in struct{ Meters float64 }) (tools.Output, error) {
				moved++
				return tools.Data(map[string]any{"result": "done", "meters": in.Meters})
			})},
	})
	return New(c, reg, func() string { return "You are BB-8." }), &moved
}

func TestToolLoop(t *testing.T) {
	var seen []chatReq
	srv := fakeRouter(t, "move", `{"meters":0.5}`, &seen)
	defer srv.Close()
	a, moved := setup(t, srv.URL)
	rep, err := a.Ask(context.Background(), "avanza medio metro", true)
	if err != nil {
		t.Fatal(err)
	}
	if *moved != 1 || len(rep.ToolsUsed) != 1 || !strings.Contains(rep.Text, "'meters':0.5") {
		t.Fatal(rep, *moved)
	}
	if len(seen) != 2 || len(seen[0].Tools) != 1 || seen[0].Messages[0].Role != "system" {
		t.Fatal("request shape", seen)
	}
}

func TestUnknownVoiceCannotMove(t *testing.T) {
	var seen []chatReq
	srv := fakeRouter(t, "move", `{"meters":1}`, &seen)
	defer srv.Close()
	a, moved := setup(t, srv.URL)
	rep, err := a.Ask(context.Background(), "avanza", false)
	if err != nil {
		t.Fatal(err)
	}
	if *moved != 0 || !strings.Contains(rep.Text, "unknown voice") {
		t.Fatal(rep, *moved)
	}
}

func TestNeedsKey(t *testing.T) {
	c, _ := config.Open("")
	c.Register(Fields()...)
	if _, err := New(c, tools.NewRegistry(), func() string { return "" }).Ask(context.Background(), "hola", true); err == nil {
		t.Fatal("ask without key must fail")
	}
}
