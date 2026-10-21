package head

import (
	"math"
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

// A steady noise well above minVoiceRMS (high mic gain) must not keep an
// utterance open forever: the floor catches up and the stream goes quiet.
func TestVADStreamLoudNoise(t *testing.T) {
	var ends int
	vs := newVADStream(0.058, func(c AudioChunk) {
		if c.Kind == "end" {
			ends++
		}
	}, func(SoundEvent) {})
	r := rand.New(rand.NewPCG(5, 6))
	frame := func(amp float64) []float64 {
		x := make([]float64, 320)
		for i := range x {
			x[i] = amp * r.NormFloat64()
		}
		return x
	}
	for f := 0; f < 400; f++ { // 8 s of noise at RMS 600
		x := frame(600)
		vs.feed(x, x, false)
	}
	if vs.active {
		t.Fatalf("still in an utterance after 8 s of steady noise (noise=%.0f)", vs.noise)
	}
	if ends > 1 {
		t.Fatalf("%d utterances from steady noise", ends)
	}
}

// 48 kHz capture: a 1 kHz tone survives the decimation to 16 kHz, a 12 kHz
// one (above the new Nyquist) is filtered out instead of aliasing.
func TestDecimator(t *testing.T) {
	amp := func(f float64) float64 {
		z := newDecimator(3)
		var out []float64
		for fr := 0; fr < 20; fr++ {
			x := make([]float64, 960)
			for i := range x {
				n := fr*960 + i
				x[i] = math.Sin(2 * math.Pi * f * float64(n) / 48000)
			}
			out = append(out, z.run(x)...)
		}
		if len(out) != 20*320 {
			t.Fatalf("got %d samples", len(out))
		}
		return rms(out[1000:]) * math.Sqrt2
	}
	if a := amp(1000); a < 0.95 || a > 1.05 {
		t.Fatalf("1 kHz amplitude %.3f", a)
	}
	if a := amp(12000); a > 0.05 {
		t.Fatalf("12 kHz leaked: %.3f", a)
	}
}

// The direction is computed at the capture rate: a sound from the right
// (left channel late by 6 samples at 48 kHz ≈ 43 µs) gives a positive angle.
func TestVADStreamAngleAt48k(t *testing.T) {
	var events []SoundEvent
	vs := newVADStreamAt(3, 0.058, func(AudioChunk) {}, func(e SoundEvent) { events = append(events, e) })
	r := rand.New(rand.NewPCG(7, 8))
	const fs = 960
	src := make([]float64, 160*fs+12)
	for i := range src {
		amp := 20.0
		if i > 30*fs && i < 70*fs {
			amp = 3000
		}
		src[i] = amp * r.NormFloat64()
	}
	for f := 0; f < 150; f++ {
		off := 12 + f*fs
		vs.feed(src[off-6:off-6+fs], src[off:off+fs], false)
	}
	if len(events) != 1 || events[0].AngleDeg < 20 {
		t.Fatalf("events %+v", events)
	}
}
