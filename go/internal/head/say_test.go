package head

import (
	"context"
	"encoding/json"
	"testing"
)

type sayRec struct {
	Backend
	said, voiced string
	pcm          []byte
	rate         int
}

func (s *sayRec) Say(_ context.Context, text, _ string) (float64, error) { s.said = text; return 1, nil }

func (s *sayRec) SayPCM(_ context.Context, text, _ string, pcm []byte, rate int) (float64, error) {
	s.voiced, s.pcm, s.rate = text, pcm, rate
	return 2, nil
}

// The body's voice arrives as base64 PCM in "say"; without it the head speaks the text.
func TestSayWithBodyVoice(t *testing.T) {
	b := &sayRec{}
	h := &Head{B: b}
	body, _ := json.Marshal(map[string]any{"text": "hola", "pcm": []byte{1, 0, 2, 0}, "rate": 24000})
	if _, _, err := h.handle(context.Background(), "say", body); err != nil {
		t.Fatal(err)
	}
	if b.voiced != "hola" || len(b.pcm) != 4 || b.rate != 24000 || b.said != "" {
		t.Fatalf("%+v", b)
	}
	if _, _, err := h.handle(context.Background(), "say", []byte(`{"text":"adios"}`)); err != nil || b.said != "adios" {
		t.Fatal(err, b.said)
	}
}
