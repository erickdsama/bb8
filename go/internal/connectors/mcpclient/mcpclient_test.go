package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connections"
	"github.com/erickdsama/bb8/go/internal/mcpserver"
	"github.com/erickdsama/bb8/go/internal/tools"
)

// A remote MCP server with "whoami" (read-only by name) and "send", behind a bearer check.
func fakeServer(t *testing.T, token, who string) *httptest.Server {
	reg := tools.NewRegistry()
	reg.Set("x", []tools.Tool{
		{Name: "get_whoami", Description: "who", Schema: tools.Object(map[string]any{}),
			Handler: func(context.Context, json.RawMessage) (tools.Output, error) { return tools.Data(who) }},
		{Name: "send", Description: "send", Schema: tools.Object(map[string]any{"to": tools.Str("")}),
			Handler: func(context.Context, json.RawMessage) (tools.Output, error) { return tools.Data("sent by " + who) }},
	})
	h := mcpserver.Handler(mcpserver.New(reg, ""), func() string { return "" })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", 401)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAccountsPerPerson(t *testing.T) {
	erickSrv, anaSrv, houseSrv := fakeServer(t, "e", "erick"), fakeServer(t, "a", "ana"), fakeServer(t, "h", "house")
	st, _ := connections.Open("")
	c := New(st)
	c.Owner = func() string { return "Erick" }
	defer c.Close()
	s, _ := config.Open("")
	s.Register(c.Fields()...)
	if ts := c.Tools(s); len(ts) != 0 {
		t.Fatal("nothing connected should give no tools", ts)
	}
	st.Add(connections.Connection{Kind: "zapier", Name: "calc", URL: erickSrv.URL, Owner: "Erick", Confirm: true, Auth: connections.Auth{Type: "bearer", Token: "e"}})
	st.Add(connections.Connection{Kind: "zapier", Name: "calc", URL: anaSrv.URL, Owner: "ana", Confirm: true, Auth: connections.Auth{Type: "bearer", Token: "a"}})
	house, _ := st.Add(connections.Connection{Kind: "home", Name: "home", URL: houseSrv.URL, Strangers: true, Auth: connections.Auth{Type: "bearer", Token: "h"}})
	ts := c.Tools(s)
	names := map[string]tools.Tool{}
	for _, tl := range ts {
		names[tl.Name] = tl
	}
	if len(ts) != 4 || !names["calc_send"].Confirm || names["calc_get_whoami"].Confirm || names["home_send"].Confirm {
		t.Fatalf("%d tools: %+v", len(ts), names)
	}
	reg := tools.NewRegistry()
	reg.Set("connector:mcp", ts)
	who := func(o tools.CallOpts, tool string) string {
		out, err := reg.Call(context.Background(), tool, nil, o)
		if err != nil {
			return err.Error()
		}
		return out.Data.(map[string]any)["text"].(string)
	}
	erick := tools.CallOpts{SpeakerKnown: true, Speaker: "erick", Turn: 1}
	ana := tools.CallOpts{SpeakerKnown: true, Speaker: "Ana", Turn: 1}
	if who(erick, "calc_get_whoami") != `"erick"` || who(ana, "calc_get_whoami") != `"ana"` {
		t.Fatal("each person must use their own account")
	}
	if who(tools.CallOpts{SpeakerKnown: true}, "calc_get_whoami") != `"erick"` {
		t.Fatal("the page acts as home.owner")
	}
	bob := tools.CallOpts{SpeakerKnown: true, Speaker: "Bob"}
	if _, err := reg.Call(context.Background(), "calc_get_whoami", nil, bob); !errors.Is(err, tools.ErrNotYours) {
		t.Fatal("Bob has no calc account", err)
	}
	if len(reg.ListFor(bob)) != 2 || who(bob, "home_get_whoami") != `"house"` {
		t.Fatal("Bob sees only the shared home tools")
	}
	stranger := tools.CallOpts{}
	if who(stranger, "home_get_whoami") != `"house"` {
		t.Fatal("strangers may read shared accounts that allow it")
	}
	if _, err := reg.Call(context.Background(), "home_send", nil, stranger); !errors.Is(err, tools.ErrNotYours) {
		t.Fatal("but not act through them", err)
	}
	if _, err := reg.Call(context.Background(), "calc_get_whoami", nil, stranger); !errors.Is(err, tools.ErrNotYours) {
		t.Fatal("and never use a person's account", err)
	}
	if out, _ := reg.Call(context.Background(), "calc_send", json.RawMessage(`{"to":"x"}`), ana); out.Data.(map[string]any)["result"] != "needs_confirmation" {
		t.Fatal("sending must ask first", out.Data)
	}
	if c.Live(house.ID).State != "ok" || c.Status(s).State != "ok" {
		t.Fatal(c.Live(house.ID), c.Status(s))
	}

	// A wrong token: that account errors, the others keep working.
	st.Update(house.ID, true, func(cn *connections.Connection) { cn.Auth.Token = "nope" })
	ts = c.Tools(s)
	if len(ts) != 2 || c.Live(house.ID).State != "error" || c.Status(s).State != "error" {
		t.Fatal(len(ts), c.Live(house.ID))
	}
}

func TestToolName(t *testing.T) {
	if n := toolName("home assistant", "light.turn_on"); n != "home_assistant_light_turn_on" {
		t.Fatal(n)
	}
}
