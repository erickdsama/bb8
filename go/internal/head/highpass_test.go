package head

import (
	"math"
	"testing"
)

// gain of the 4th-order high-pass at f, in dB, after it settles.
func hpGainDB(f float64) float64 {
	h := [2]biquad{highPass(highPassHz, MicRate), highPass(highPassHz, MicRate)}
	x := make([]float64, MicRate)
	for i := range x {
		x[i] = 1000 * math.Sin(2*math.Pi*f*float64(i)/MicRate)
	}
	y := h[1].run(h[0].run(x))
	return 20 * math.Log10(rms(y[MicRate/2:])/rms(x[MicRate/2:]))
}

func TestHighPass(t *testing.T) {
	for f, want := range map[float64][2]float64{ // [min, max] dB
		60:   {-60, -38}, // mains hum / minisplit
		100:  {-30, -20},
		200:  {-7, -5}, // the corner: two Butterworth sections, −3 dB each
		1000: {-0.2, 0.2},
		3000: {-0.2, 0.2},
	} {
		if g := hpGainDB(f); g < want[0] || g > want[1] {
			t.Errorf("%v Hz: %.1f dB, want %v", f, g, want)
		}
	}
}

// feedNoise runs n seconds of generated noise through a fresh VAD stream.
func feedNoise(sec int, gen func(i int) float64) *vadStream {
	v := newVADStream(0.058, func(AudioChunk) {}, func(SoundEvent) {})
	n := 0
	for f := 0; f < sec*MicRate/frameSamples; f++ {
		l := make([]float64, frameSamples)
		for j := range l {
			l[j] = gen(n)
			n++
		}
		v.feed(l, l, false)
	}
	return v
}

func TestAutoHighPass(t *testing.T) {
	rnd := uint32(1)
	white := func() float64 { // tiny LCG: deterministic, no rand state
		rnd = rnd*1664525 + 1013904223
		return float64(int32(rnd)) / (1 << 31)
	}
	// A minisplit: strong hum and rumble under 200 Hz over faint hiss.
	rumble := feedNoise(20, func(i int) float64 {
		x := float64(i) / MicRate
		s := 0.0
		for _, h := range [][2]float64{{39, 250}, {58, 300}, {78, 200}, {147, 150}, {194, 120}} {
			s += h[1] * math.Sin(2*math.Pi*h[0]*x)
		}
		return s + 20*white()
	})
	if rumble.auto.fc < 200 {
		t.Errorf("rumble: corner %v Hz, want ≥ 200", rumble.auto.fc)
	}
	// A quiet room with even hiss: keep all of the voice.
	hiss := feedNoise(20, func(int) float64 { return 60 * white() })
	if hiss.auto.fc != 80 {
		t.Errorf("hiss: corner %v Hz, want 80", hiss.auto.fc)
	}
}
