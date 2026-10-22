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
