package head

import "math"

// autoHP picks the high-pass corner from the room's noise. While nobody speaks
// it keeps the noise spectrum; every second it tries each corner and keeps the
// one with the best expected voice-to-noise ratio, weighing the noise it heard
// against the long-term spectrum of speech. A quiet room keeps 80 Hz (all of the
// voice); a minisplit or a fan rumbling under 200 Hz moves it up as far as it pays.
//
// The corner only changes between utterances (the caller checks), and only for
// a clear gain, so it does not hop back and forth.

var hpCorners = []float64{80, 120, 160, 200, 250, 300}

const (
	hpFFT        = 512 // at MicRate: 31.25 Hz bins
	hpEvery      = 4   // analyze one quiet frame in four (the Zero is slow)
	hpDecideAt   = 50  // frames analyzed between decisions (~4 s of quiet)
	hpMinGainDB  = 0.3 // switch only for a better ratio than this
	hpNoiseAlpha = 0.05
)

type autoHP struct {
	fc     float64
	psd    []float64 // noise power per bin, averaged
	seen   int       // quiet frames offered
	used   int       // frames analyzed since the last decision
	buf    []complex128
	win    []float64
	speech []float64 // long-term speech spectrum per bin (relative power)
	primed bool
}

func newAutoHP(start float64) *autoHP {
	a := &autoHP{fc: start, psd: make([]float64, hpFFT/2+1), buf: make([]complex128, hpFFT),
		win: make([]float64, frameSamples), speech: make([]float64, hpFFT/2+1)}
	for i := range a.win {
		a.win[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(len(a.win)-1))
	}
	for k := range a.speech {
		f := math.Max(float64(k)*MicRate/hpFFT, 20)
		// ltassDB is per third of an octave; a bin is a fixed width: per Hz is
		// that divided by the band width, which grows with f.
		a.speech[k] = math.Pow(10, (ltassDB(f)-10*math.Log10(f/400))/10)
	}
	return a
}

// ltassDB is a rough long-term average speech spectrum (dB per third of an
// octave, 0 at its peak around 400 Hz): little under 100 Hz, falling above 1 kHz.
func ltassDB(f float64) float64 {
	pts := [][2]float64{{50, -28}, {100, -14}, {200, -4}, {400, 0}, {800, -4}, {1600, -12}, {3200, -20}, {6400, -30}, {8000, -34}}
	if f <= pts[0][0] {
		return pts[0][1]
	}
	for i := 1; i < len(pts); i++ {
		if f <= pts[i][0] {
			t := math.Log(f/pts[i-1][0]) / math.Log(pts[i][0]/pts[i-1][0])
			return pts[i-1][1] + t*(pts[i][1]-pts[i-1][1])
		}
	}
	return pts[len(pts)-1][1]
}

// hpGain2 is the power gain of two Butterworth high-pass sections at corner c.
func hpGain2(f, c float64) float64 {
	r := math.Pow(f/c, 4)
	g := r / (1 + r)
	return g * g
}

// quiet feeds one frame of unfiltered mono (MicRate) heard while nobody speaks.
func (a *autoHP) quiet(x []float64) {
	a.seen++
	if a.seen%hpEvery != 0 {
		return
	}
	for i := range a.buf {
		a.buf[i] = 0
	}
	for i, v := range x {
		a.buf[i] = complex(v*a.win[i], 0)
	}
	fft(a.buf)
	alpha := hpNoiseAlpha
	if !a.primed {
		alpha = 1
		a.primed = true
	}
	for k := range a.psd {
		re, im := real(a.buf[k]), imag(a.buf[k])
		a.psd[k] += alpha * (re*re + im*im - a.psd[k])
	}
	a.used++
}

// ratio is the expected voice-to-noise power ratio with corner c.
func (a *autoHP) ratio(c float64) (voice, noise float64) {
	for k := 1; k < len(a.psd); k++ {
		g := hpGain2(float64(k)*MicRate/hpFFT, c)
		voice += a.speech[k] * g
		noise += a.psd[k] * g
	}
	return voice, noise
}

// decide returns the corner to use and whether it changed. scale is how much
// the noise power changes with the new corner, to carry the noise estimate over.
func (a *autoHP) decide() (fc float64, changed bool, scale float64) {
	if a.used < hpDecideAt {
		return a.fc, false, 1
	}
	a.used = 0
	best, bestR := a.fc, 0.0
	curV, curN := a.ratio(a.fc)
	if curN <= 0 {
		return a.fc, false, 1
	}
	cur := curV / curN
	var bestN float64
	for _, c := range hpCorners {
		v, n := a.ratio(c)
		if n <= 0 {
			continue
		}
		if r := v / n; r > bestR {
			best, bestR, bestN = c, r, n
		}
	}
	if best == a.fc || 10*math.Log10(bestR/cur) < hpMinGainDB {
		return a.fc, false, 1
	}
	a.fc = best
	return best, true, bestN / curN
}
