package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/mcpserver"
	"github.com/erickdsama/bb8/go/internal/tools"
)

// A remote MCP server built from a registry with one "add" tool, behind a bearer check.
func fakeServer(t *testing.T) *httptest.Server {
	reg := tools.NewRegistry()
	reg.Set("x", []tools.Tool{{Name: "add", Description: "a + b",
		Schema: tools.Object(map[string]any{"a": tools.Num("a"), "b": tools.Num("b")}, "a", "b"),
		Handler: tools.Bind(func(_ context.Context, in struct{ A, B float64 }) (tools.Output, error) {
			return tools.Data(map[string]any{"sum": in.A + in.B})
		})}})
	h := mcpserver.Handler(mcpserver.New(reg, ""), func() string { return "" })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer s3cret" {
			http.Error(w, "unauthorized", 401)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRemoteTools(t *testing.T) {
	srv := fakeServer(t)
	c := New()
	defer c.Close()
	s, _ := config.Open("")
	s.Register(c.Fields()...)
	if ts := c.Tools(s); len(ts) != 0 {
		t.Fatal("no slots should give no tools", ts)
	}
	s.Set(map[string]any{key(0, "name"): "calc", key(0, "url"): srv.URL, key(0, "token"): "s3cret"}, config.SourceWeb, nil)
	ts := c.Tools(s)
	if len(ts) != 1 || ts[0].Name != "calc_add" || !ts[0].KnownOnly {
		t.Fatalf("%+v", ts)
	}
	reg := tools.NewRegistry()
	reg.Set("connector:mcp", ts)
	out, err := reg.Call(context.Background(), "calc_add", json.RawMessage(`{"a":2,"b":3}`), tools.CallOpts{SpeakerKnown: true})
	if err != nil {
		t.Fatal(err)
	}
	m := out.Data.(map[string]any)
	if m["result"] != "done" || m["text"] != `{"sum":5}` {
		t.Fatal(m)
	}
	if _, err := reg.Call(context.Background(), "calc_add", nil, tools.CallOpts{}); !errors.Is(err, tools.ErrKnownOnly) {
		t.Fatal("unknown voice must be refused", err)
	}
	if st := c.Status(s); st.State != "ok" {
		t.Fatal(st)
	}

	// Wrong token: no tools, and the status says why.
	s.Set(map[string]any{key(0, "token"): "nope"}, config.SourceWeb, nil)
	if ts := c.Tools(s); len(ts) != 0 {
		t.Fatal(ts)
	}
	if st := c.Status(s); st.State != "error" {
		t.Fatal(st)
	}
}

func TestToolName(t *testing.T) {
	if n := toolName("home assistant", "light.turn_on"); n != "home_assistant_light_turn_on" {
		t.Fatal(n)
	}
}
