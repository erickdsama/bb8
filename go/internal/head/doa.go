package head

// Direction of arrival for the ReSpeaker 2-Mic HAT: GCC-PHAT between the left
// and right capsules. With two mics only the time difference is measured, which
// depends on the sine of the angle, so front and back fold together (−90…90°).

import (
	"math"
	"math/cmplx"
)

const SpeedOfSound = 343.0 // m/s at ~20 °C

// fft is an in-place iterative radix-2 FFT. len(a) must be a power of two.
func fft(a []complex128) {
	n := len(a)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		w := cmplx.Exp(complex(0, -2*math.Pi/float64(size)))
		for start := 0; start < n; start += size {
			wk := complex(1, 0)
			for k := 0; k < size/2; k++ {
				u, v := a[start+k], a[start+k+size/2]*wk
				a[start+k], a[start+k+size/2] = u+v, u-v
				wk *= w
			}
		}
	}
}

// GCC accumulates PHAT-weighted cross spectra over the voiced blocks of one
// utterance, then finds the inter-mic delay with sub-sample resolution.
type GCC struct {
	N      int
	acc    []complex128
	win    []float64
	blocks int
	b0, b1 []complex128
}

func NewGCC(n int) *GCC {
	g := &GCC{N: n, acc: make([]complex128, n/2), win: make([]float64, n), b0: make([]complex128, n), b1: make([]complex128, n)}
	for i := range g.win {
		g.win[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1)) // Hann
	}
	return g
}

func (g *GCC) Reset() {
	clear(g.acc)
	g.blocks = 0
}

func (g *GCC) Blocks() int { return g.blocks }

// Add one block of left and right samples (len N each).
func (g *GCC) Add(left, right []float64) {
	for i := 0; i < g.N; i++ {
		g.b0[i] = complex(left[i]*g.win[i], 0)
		g.b1[i] = complex(right[i]*g.win[i], 0)
	}
	fft(g.b0)
	fft(g.b1)
	for k := 1; k < g.N/2; k++ {
		c := g.b0[k] * cmplx.Conj(g.b1[k])
		if m := cmplx.Abs(c); m > 1e-12 {
			g.acc[k] += c / complex(m, 0)
		}
	}
	g.blocks++
}

// Delay returns the delay of the left channel relative to the right, in
// samples, searched within ±maxLag with 0.05-sample steps.
func (g *GCC) Delay(maxLag float64) (float64, bool) {
	if g.blocks == 0 {
		return 0, false
	}
	best, bestTau := math.Inf(-1), 0.0
	for tau := -maxLag; tau <= maxLag+1e-9; tau += 0.05 {
		r := 0.0
		for k := 1; k < g.N/2; k++ {
			ph := 2 * math.Pi * float64(k) * tau / float64(g.N)
			r += real(g.acc[k] * complex(math.Cos(ph), math.Sin(ph)))
		}
		if r > best {
			best, bestTau = r, tau
		}
	}
	return bestTau, true
}

// Angle is the arrival angle relative to the mic array front, + right
// (the left capsule hears a sound from the right later), −90…90.
func (g *GCC) Angle(spacingM, rate float64) (float64, bool) {
	maxLag := spacingM / SpeedOfSound * rate
	tau, ok := g.Delay(math.Ceil(maxLag))
	if !ok {
		return 0, false
	}
	s := math.Max(-1, math.Min(1, tau/maxLag))
	return math.Asin(s) * 180 / math.Pi, true
}
