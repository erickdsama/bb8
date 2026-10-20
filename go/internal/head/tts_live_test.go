package head

import (
	"context"
	"os"
	"testing"
)

func TestWyomingTTSLive(t *testing.T) {
	addr := os.Getenv("BB8_TTS_WYOMING")
	if addr == "" {
		t.Skip("BB8_TTS_WYOMING not set")
	}
	wav, err := wyomingTTS(context.Background(), addr, "Hola, soy BB-8.", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(wav) < 1000 || string(wav[:4]) != "RIFF" {
		t.Fatalf("bad wav: %d bytes", len(wav))
	}
	t.Logf("%d bytes", len(wav))
}
