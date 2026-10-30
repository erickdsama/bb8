package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestConfirmNeedsALaterTurn(t *testing.T) {
	r := NewRegistry()
	ran := 0
	r.Set("x", []Tool{{Name: "send", Confirm: true, Schema: Object(map[string]any{"to": Str("")}),
		Handler: func(ctx context.Context, args json.RawMessage) (Output, error) {
			ran++
			if string(args) != `{"to":"ana"}` {
				t.Errorf("args %s", args)
			}
			if CallerFrom(ctx).Speaker != "Erick" {
				t.Error("caller not set")
			}
			return Data("sent")
		}}})
	if _, ok := r.List()[0].Schema["properties"].(map[string]any)["confirm_id"]; !ok {
		t.Fatal("confirm_id not added to the schema")
	}
	erick := CallOpts{SpeakerKnown: true, Speaker: "Erick", Turn: 1}
	out, _ := r.Call(context.Background(), "send", json.RawMessage(`{"to":"ana"}`), erick)
	id := out.Data.(map[string]any)["confirm_id"].(string)
	if ran != 0 || id == "" {
		t.Fatal("ran without confirmation")
	}
	// The model answering itself in the same turn does not count.
	r.Call(context.Background(), "send", json.RawMessage(`{"to":"ana","confirm_id":"`+id+`"}`), erick)
	if ran != 0 {
		t.Fatal("confirmed in the same turn")
	}
	// Another person cannot confirm it, nor different arguments.
	ana := CallOpts{SpeakerKnown: true, Speaker: "Ana", Turn: 2}
	r.Call(context.Background(), "send", json.RawMessage(`{"to":"ana","confirm_id":"`+id+`"}`), ana)
	erick.Turn = 2
	r.Call(context.Background(), "send", json.RawMessage(`{"to":"bob","confirm_id":"`+id+`"}`), erick)
	if ran != 0 {
		t.Fatal("confirmed by someone else or with other arguments")
	}
	r.Call(context.Background(), "send", json.RawMessage(`{"to":"ana","confirm_id":"`+id+`"}`), erick)
	if ran != 1 {
		t.Fatal("yes in the next turn should run it")
	}
	// Unknown voices never; admins (MCP clients) without asking.
	if _, err := r.Call(context.Background(), "send", json.RawMessage(`{"to":"ana"}`), CallOpts{}); err != ErrConfirmVoice {
		t.Fatal(err)
	}
	r.Call(context.Background(), "send", json.RawMessage(`{"to":"ana"}`), CallOpts{SpeakerKnown: true, Admin: true, Speaker: "Erick"})
	if ran != 2 {
		t.Fatal("admin should not need confirmation")
	}
}

func TestForFiltersByPerson(t *testing.T) {
	r := NewRegistry()
	r.Set("x", []Tool{{Name: "mail", For: func(o CallOpts) bool { return o.Speaker == "Erick" },
		Handler: func(context.Context, json.RawMessage) (Output, error) { return Data("ok") }}})
	if len(r.ListFor(CallOpts{Speaker: "Ana", SpeakerKnown: true})) != 0 || len(r.ListFor(CallOpts{Speaker: "Erick"})) != 1 {
		t.Fatal("ListFor")
	}
	if _, err := r.Call(context.Background(), "mail", nil, CallOpts{Speaker: "Ana", SpeakerKnown: true}); err != ErrNotYours {
		t.Fatal(err)
	}
}
