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
	"github.com/erickdsama/bb8/go/internal/people"
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

func TestGuestCannotMove(t *testing.T) {
	var seen []chatReq
	srv := fakeRouter(t, "move", `{"meters":1}`, &seen)
	defer srv.Close()
	a, moved := setup(t, srv.URL)
	a.People, _ = people.Open("")
	a.People.Upsert("Juan", people.Guest)
	a.People.Remember("Erick", "le gusta el café")
	a.People.Upsert("Erick", people.Owner)
	var evs []string
	a.Publish = func(typ string, _ any) { evs = append(evs, typ) }

	if _, err := a.AskAs(context.Background(), Turn{Text: "avanza", Speaker: "juan", Known: true}); err != nil {
		t.Fatal(err)
	}
	if *moved != 0 {
		t.Fatal("a guest moved the robot")
	}
	if _, err := a.AskAs(context.Background(), Turn{Text: "avanza", Speaker: "Erick", Known: true}); err != nil {
		t.Fatal(err)
	}
	if *moved != 1 {
		t.Fatal("the owner could not move it")
	}
	u := seen[len(seen)-2].Messages
	if c, _ := u[len(u)-1].Content.(string); !strings.Contains(c, "Erick is owner") || !strings.Contains(c, "café") {
		t.Fatal("profile missing from the turn:", c)
	}
	if sys := seen[0].Messages[0].Content.([]any)[1].(map[string]any)["text"].(string); !strings.Contains(sys, "Juan (guest)") {
		t.Fatal("people line:", sys)
	}
	if strings.Join(evs, ",") != "heard,tool,reply,heard,tool,reply" {
		t.Fatal(evs)
	}
}

// A tool that sends needs a yes in the next turn; the model cannot confirm by itself.
func TestConfirmAcrossTurns(t *testing.T) {
	var confirmID string
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var req chatReq
		json.Unmarshal(b, &req)
		last := req.Messages[len(req.Messages)-1]
		switch {
		case last.Role == "user" && strings.HasPrefix(last.Content.(string), "manda"):
			w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"a","type":"function","function":{"name":"mail_send","arguments":"{\"to\":\"ana\"}"}}]}}]}`))
		case last.Role == "tool" && strings.Contains(last.Content.(string), "needs_confirmation"):
			var m map[string]any
			json.Unmarshal([]byte(last.Content.(string)), &m)
			confirmID = m["confirm_id"].(string)
			if calls == 0 { // a sneaky model tries to confirm in the same turn
				calls++
				w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"b","type":"function","function":{"name":"mail_send","arguments":"{\"to\":\"ana\",\"confirm_id\":\"` + confirmID + `\"}"}}]}}]}`))
				return
			}
			w.Write([]byte(`{"choices":[{"message":{"content":"¿Le mando el correo a Ana?"}}]}`))
		case last.Role == "user": // "sí"
			w.Write([]byte(`{"choices":[{"message":{"content":"","tool_calls":[{"id":"c","type":"function","function":{"name":"mail_send","arguments":"{\"to\":\"ana\",\"confirm_id\":\"` + confirmID + `\"}"}}]}}]}`))
		default:
			w.Write([]byte(`{"choices":[{"message":{"content":"Listo."}}]}`))
		}
	}))
	defer srv.Close()
	a, _ := setup(t, srv.URL)
	sent := 0
	a.Reg.Set("x", []tools.Tool{{Name: "mail_send", Confirm: true, Schema: tools.Object(map[string]any{"to": tools.Str("")}),
		Handler: func(context.Context, json.RawMessage) (tools.Output, error) { sent++; return tools.Data("sent") }}})
	rep, err := a.AskAs(context.Background(), Turn{Text: "manda un correo a ana", Speaker: "Erick", Known: true})
	if err != nil || sent != 0 || !strings.Contains(rep.Text, "¿Le mando") {
		t.Fatal(rep, err, sent)
	}
	if _, err := a.AskAs(context.Background(), Turn{Text: "sí", Speaker: "Erick", Known: true}); err != nil || sent != 1 {
		t.Fatal(err, sent)
	}
}
