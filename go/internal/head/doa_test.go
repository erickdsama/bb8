package head

import (
	"math"
	"math/rand/v2"
	"testing"
)

// A source on the right reaches the right capsule first: the left channel is
// a delayed copy, and the angle must come out positive.
func TestGCCAngle(t *testing.T) {
	const rate, spacing, n = 16000.0, 0.058, 512
	r := rand.New(rand.NewPCG(1, 2))
	src := make([]float64, 20*n+8)
	for i := range src {
		src[i] = r.NormFloat64()
	}
	for _, delay := range []int{2, -2, 0, 1} {
		g := NewGCC(n)
		for b := 0; b < 20; b++ {
			off := 4 + b*n
			left := src[off-delay : off-delay+n]
			right := src[off : off+n]
			g.Add(left, right)
		}
		ang, ok := g.Angle(spacing, rate)
		want := math.Asin(float64(delay)*SpeedOfSound/(rate*spacing)) * 180 / math.Pi
		if !ok || math.Abs(ang-want) > 4 {
			t.Fatalf("delay %d: angle %.1f, want %.1f", delay, ang, want)
		}
	}
}

func TestFFT(t *testing.T) {
	a := make([]complex128, 8)
	a[1] = 1
	fft(a)
	for k, v := range a {
		want := complex(math.Cos(-2*math.Pi*float64(k)/8), math.Sin(-2*math.Pi*float64(k)/8))
		if math.Abs(real(v-want)) > 1e-9 || math.Abs(imag(v-want)) > 1e-9 {
			t.Fatal(k, v, want)
		}
	}
}
