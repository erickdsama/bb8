package head

import (
	"math/rand/v2"
	"testing"
)

// Silence, then 0.8 s of "voice" coming from the right (left channel delayed
// by 2 samples), then silence: one utterance with audio and a positive angle.
func TestVADStreamUtterance(t *testing.T) {
	var chunks []AudioChunk
	var events []SoundEvent
	vs := newVADStream(0.058, func(c AudioChunk) { chunks = append(chunks, c) }, func(e SoundEvent) { events = append(events, e) })
	r := rand.New(rand.NewPCG(3, 4))
	const total = 160 * 320
	src := make([]float64, total+4)
	for i := range src {
		amp := 20.0
		if i > 30*320 && i < 70*320 {
			amp = 3000
		}
		src[i] = amp * r.NormFloat64()
	}
	for f := 0; f < 150; f++ {
		off := 4 + f*320
		vs.feed(src[off-2:off-2+320], src[off:off+320], false)
	}
	if len(chunks) < 3 || chunks[0].Kind != "start" || chunks[len(chunks)-1].Kind != "end" {
		t.Fatalf("chunks: %d first %v", len(chunks), chunks[0].Kind)
	}
	bytes := 0
	for _, c := range chunks {
		bytes += len(c.PCM)
	}
	if bytes < 2*320*40 { // the voiced 0.8 s at least
		t.Fatalf("only %d bytes of audio", bytes)
	}
	if len(events) != 1 || events[0].AngleDeg < 30 || events[0].LevelDB < 50 {
		t.Fatalf("events %+v", events)
	}
	// Muted: nothing starts.
	chunks = nil
	for f := 0; f < 70; f++ {
		off := 4 + f*320
		vs.feed(src[off-2:off-2+320], src[off:off+320], true)
	}
	if len(chunks) != 0 {
		t.Fatal("muted mic streamed audio")
	}
}
