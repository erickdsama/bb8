package head

import (
	"testing"

	"github.com/erickdsama/bb8/go/internal/persona"
)

func TestEverySoundAndBabble(t *testing.T) {
	for name := range persona.Emotions {
		wav, secs := BeepWAV(name, 0.4, 2)
		if len(wav) < 1000 || secs <= 0 || string(wav[:4]) != "RIFF" {
			t.Fatal(name, len(wav), secs)
		}
	}
	for name := range Sounds {
		if _, ok := persona.Emotions[name]; !ok {
			t.Fatal("sound without emotion", name)
		}
	}
	a, _ := BeepWAV("¿Quién anda ahí?", 0.4, 3)
	b, _ := BeepWAV("¿Quién anda ahí?", 0.4, 0)
	if string(a) != string(b) {
		t.Fatal("same text must give the same babble")
	}
}
