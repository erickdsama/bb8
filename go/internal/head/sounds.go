package head

// Droid beeps synthesized with plain math (port of partes/5-cabeza/head/sounds.py).
// Each sound is a list of syllables: a frequency sweep with vibrato and a timbre.
// Every play picks one of 4 variants (slightly different pitch and tempo) so it
// does not sound recorded. Any text gets its own babble: same text, same beeps,
// with the intonation of its punctuation (¿…? rises at the end, ¡…! is higher).

import (
	"bytes"
	"encoding/binary"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"regexp"
	"strings"
	"sync"
)

const Rate = 22050

type syl struct {
	f0, f1, dur, vib, vibHz float64
	timbre                  string
	vol                     float64
}

func s(f0, f1, dur float64) syl { return syl{f0: f0, f1: f1, dur: dur, timbre: "soft", vol: 1} }

func (x syl) v(vib, hz float64) syl { x.vib, x.vibHz = vib, hz; return x }
func (x syl) t(timbre string) syl   { x.timbre = timbre; return x }
func (x syl) loud(v float64) syl    { x.vol = v; return x }
func pause(d float64) syl           { return syl{dur: d} }
func trill(fa, fb float64, n int, dur float64, timbre string) []syl {
	out := make([]syl, n)
	for i := range out {
		a, b := fa, fb
		if i%2 == 1 {
			a, b = fb, fa
		}
		out[i] = s(a, b, dur).t(timbre)
	}
	return out
}

func cat(parts ...any) []syl {
	var out []syl
	for _, p := range parts {
		switch v := p.(type) {
		case syl:
			out = append(out, v)
		case []syl:
			out = append(out, v...)
		}
	}
	return out
}

// Harmonics (multiplier, weight) per timbre.
var timbres = map[string][][2]float64{
	"soft":   {{1, 0.8}, {2, 0.2}},
	"bright": {{1, 0.6}, {2, 0.25}, {3, 0.15}},
	"buzz":   {{1, 0.55}, {3, 0.25}, {5, 0.12}, {7, 0.08}}, // almost square: grumpy
}

// Sounds: name = emotion (persona.Emotions).
var Sounds = map[string][]syl{
	"happy": {s(900, 1500, 0.08), s(1500, 2200, 0.07).v(0.03, 18), pause(0.03), s(1200, 1900, 0.09), s(1900, 2500, 0.06).t("bright")},
	"excited": cat(trill(1600, 2300, 6, 0.03, "bright"), pause(0.02),
		s(1200, 2900, 0.22).v(0.05, 22).t("bright")),
	"greeting": {s(1100, 1100, 0.06), pause(0.03), s(1500, 1300, 0.06), pause(0.04), s(900, 1900, 0.30).v(0.04, 9)},
	"curious":  {s(700, 1100, 0.09), s(1100, 850, 0.08), pause(0.04), s(850, 1700, 0.20).v(0.03, 7)},
	"question": {s(700, 700, 0.10), pause(0.02), s(800, 1600, 0.18)},
	"thinking": {s(950, 950, 0.04).loud(0.6), pause(0.07), s(1150, 1150, 0.04).loud(0.6), pause(0.05),
		s(1050, 1050, 0.04).loud(0.6), pause(0.09), s(1250, 1200, 0.05).loud(0.6), pause(0.06), s(1000, 1080, 0.06).loud(0.6)},
	"yes": {s(1000, 1050, 0.06), pause(0.03), s(1400, 1700, 0.09)},
	"no":  {s(900, 750, 0.09).t("buzz").loud(0.7), pause(0.04), s(700, 450, 0.16).t("buzz").loud(0.7)},
	"alert": {s(1800, 1800, 0.06).t("bright"), pause(0.04), s(1800, 1800, 0.06).t("bright"), pause(0.04),
		s(1800, 1800, 0.06).t("bright"), pause(0.04)},
	"scared": cat(s(2400, 1400, 0.12).v(0.08, 30).t("bright"), s(1500, 2800, 0.18).v(0.10, 26).t("bright"),
		pause(0.02), trill(2600, 2000, 6, 0.025, "soft")),
	"sad":   {s(900, 500, 0.30).v(0.03, 5), pause(0.05), s(600, 320, 0.45).v(0.04, 4)},
	"error": {s(420, 420, 0.12).t("buzz"), pause(0.05), s(320, 240, 0.28).v(0.02, 12).t("buzz")},
	"yawn":  {s(450, 850, 0.35).loud(0.7), s(850, 330, 0.65).v(0.02, 3).loud(0.6)},
	"wake":  {s(300, 1800, 0.35).v(0.01, 6), pause(0.04), s(1500, 2000, 0.05), pause(0.02), s(1800, 2300, 0.06)},
	"laugh": func() []syl {
		var o []syl
		for i := range 5 {
			f := float64(120 * i)
			o = append(o, s(1900-f, 1500-f, 0.05), pause(0.025))
		}
		return o
	}(),
}

func render(syls []syl, volume, pitch, tempo float64) []int16 {
	var out []int16
	phase := 0.0
	for _, x := range syls {
		n := int(Rate * x.dur / tempo)
		if x.f0 <= 0 {
			out = append(out, make([]int16, n)...)
			continue
		}
		f0, f1 := x.f0*pitch, x.f1*pitch
		ratio := f1 / f0
		harm := timbres[x.timbre]
		amp := volume * x.vol * 32767
		attack, tail := 88, 264 // 4 ms and 12 ms at 22050 Hz
		wv := 2 * math.Pi * x.vibHz / Rate
		for i := range n {
			f := f0 * math.Pow(ratio, float64(i)/float64(n)) // exponential sweep: even to the ear
			if x.vib != 0 {
				f *= 1 + x.vib*math.Sin(wv*float64(i))
			}
			phase += 2 * math.Pi * f / Rate
			env := math.Min(1, math.Min(float64(i)/float64(attack), float64(n-i)/float64(tail)))
			v := 0.0
			for _, h := range harm {
				v += h[1] * math.Sin(h[0]*phase)
			}
			out = append(out, int16(amp*env*v))
		}
	}
	return out
}

// WAV encodes mono 16-bit PCM.
func WAV(samples []int16) []byte {
	var b bytes.Buffer
	data := len(samples) * 2
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+data))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, struct {
		Size                 uint32
		Format, Channels     uint16
		Rate, ByteRate       uint32
		Align, BitsPerSample uint16
	}{16, 1, 1, Rate, Rate * 2, 2, 16})
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(data))
	binary.Write(&b, binary.LittleEndian, samples)
	return b.Bytes()
}

func seeded(key string) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(key))
	v := h.Sum64()
	return rand.New(rand.NewPCG(v, v^0x9e3779b97f4a7c15))
}

func uniform(r *rand.Rand, a, b float64) float64 { return a + (b-a)*r.Float64() }

var vowels = regexp.MustCompile(`[aeiouáéíóúü]+`)

// Babble: beeps to go with a sentence, ~1 per two syllables, 2 to 7.
func Babble(text string) []syl {
	r := seeded(text) // same text → same beeps
	t := strings.TrimSpace(text)
	n := len(vowels.FindAllString(strings.ToLower(t), -1))
	if n == 0 {
		n = len(t) / 3
	}
	n = max(2, min(7, n/2))
	tailStr := t
	if len(t) > 2 {
		tailStr = t[len(t)-2:]
	}
	question, exclaim := strings.Contains(tailStr, "?"), strings.Contains(t, "!")
	base := uniform(r, 850, 1250)
	if exclaim {
		base *= 1.25
	}
	var out []syl
	for i := range n {
		f := base * uniform(r, 0.75, 1.4)
		if i == n-1 { // intonation shows in the last one
			g, vib := f*0.65, 0.0
			if question {
				g, vib = f*1.7, 0.03
			} else if exclaim {
				g = f * 1.3
			}
			out = append(out, s(f, g, uniform(r, 0.10, 0.16)).v(vib, 8))
		} else {
			d := uniform(r, 0.04, 0.09)
			if exclaim {
				d /= 1.3
			}
			out = append(out, s(f, f*uniform(r, 0.85, 1.25), d), pause(uniform(r, 0.012, 0.04)))
		}
	}
	return out
}

type cacheKey struct {
	name    string
	variant int
	volume  float64
}

var cache sync.Map // cacheKey → beep

type beep struct {
	wav  []byte
	secs float64
}

// BeepWAV returns a WAV and its length. name is a sound or any text (babble).
// variant 0 is the base version; 1–3 vary pitch and tempo a little.
func BeepWAV(name string, volume float64, variant int) ([]byte, float64) {
	syls, known := Sounds[name]
	if !known {
		syls, variant = Babble(name), 0 // a text's babble does not vary
	}
	k := cacheKey{name, variant, volume}
	if v, ok := cache.Load(k); ok {
		b := v.(beep)
		return b.wav, b.secs
	}
	pitch, tempo := 1.0, 1.0
	if variant != 0 {
		r := seeded(name + "/" + string(rune('0'+variant)))
		pitch, tempo = uniform(r, 0.94, 1.07), uniform(r, 0.92, 1.08)
	}
	m := render(syls, volume, pitch, tempo)
	b := beep{WAV(m), math.Round(float64(len(m))/Rate*1000) / 1000}
	if known { // texts are not cached: unbounded
		cache.Store(k, b)
	}
	return b.wav, b.secs
}
