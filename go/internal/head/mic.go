package head

// ReSpeaker 2-Mic HAT on the head: capture (arecord, no cgo), an energy VAD,
// direction of arrival per utterance (GCC-PHAT) and the voiced audio, so the
// body only receives sound while someone speaks.

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os/exec"
	"sync/atomic"
	"time"
)

const (
	MicRate       = 16000 // what the body gets: voice
	CaptureRate   = 48000 // what the HAT records: 3× the time resolution for the direction
	decim         = CaptureRate / MicRate
	frameSamples  = 320  // 20 ms at MicRate
	gccBlock      = 512  // 32 ms per GCC-PHAT block at MicRate (×decim at CaptureRate)
	prerollFrames = 15   // 300 ms kept before the voice starts
	chunkBytes    = 3200 // 100 ms of 16-bit mono per audio frame on the link
	hangoverMS    = 600  // silence that ends an utterance
	maxUtterMS    = 15000
	noiseWindow   = 100   // 2 s of frames: the floor is the quietest of them
	minVoiceRMS   = 120.0 // int16 units, after the high-pass; below this is never voice
	// highPassHz cuts the rumble a minisplit, a fan or a servo makes before the
	// level is measured: measured on the robot, a minisplit put 95 % of the room
	// noise under 200 Hz (RMS 413, three times the floor without it) and the
	// voice threshold above the voice. Speech carries its words above it.
	highPassHz     = 200.0
	voiceOverNoise = 3.2 // ≈ +10 dB over the noise floor
	// dBFS → approximate dB SPL for the HAT's MEMS mics (calibrate on the robot).
	levelOffsetDB = 120.0
)

// AudioChunk is one piece of the voiced audio stream sent to the body.
type AudioChunk struct {
	Kind string // "start", "data" or "end"
	ID   uint64
	Seq  int
	PCM  []byte // s16le mono 16 kHz, only for "data"
}

// vadStream turns 20 ms stereo frames into utterances: audio chunks plus one
// SoundEvent (direction and level) at the end of each.
type vadStream struct {
	spacing float64
	emit    func(AudioChunk)
	sound   func(SoundEvent)

	noise      float64
	recent     [noiseWindow]float64 // frame levels, for the noise floor
	recentN    int
	active     bool
	voicedRun  int
	quietMS    int
	lenMS      int
	preroll    [][]byte
	pending    []byte
	id         uint64
	seq        int
	bl, br     []float64
	gcc        *GCC
	d, blk     int        // capture rate / MicRate; GCC block length at the capture rate
	lp         *decimator // capture rate → MicRate for the voice (nil when d == 1)
	hp         [2]biquad  // 4th-order high-pass at highPassHz, at MicRate
	sumSq      float64
	voicedSamp int
}

func newVADStream(spacing float64, emit func(AudioChunk), sound func(SoundEvent)) *vadStream {
	return newVADStreamAt(1, spacing, emit, sound)
}

// newVADStreamAt takes frames recorded at d×MicRate: the direction is computed
// at that rate and the voice is low-passed and decimated to MicRate.
func newVADStreamAt(d int, spacing float64, emit func(AudioChunk), sound func(SoundEvent)) *vadStream {
	blk := gccBlock
	for blk < gccBlock*d { // the FFT wants a power of two: 2048 (43 ms) at 48 kHz
		blk *= 2
	}
	v := &vadStream{spacing: spacing, emit: emit, sound: sound, noise: 60, d: d, blk: blk, gcc: NewGCC(blk)}
	v.hp[0], v.hp[1] = highPass(highPassHz, MicRate), highPass(highPassHz, MicRate)
	if d > 1 {
		v.lp = newDecimator(d)
	}
	return v
}

func rms(x []float64) float64 {
	s := 0.0
	for _, v := range x {
		s += v * v
	}
	return math.Sqrt(s / float64(len(x)))
}

func pcm(mono []float64) []byte {
	b := make([]byte, 2*len(mono))
	for i, v := range mono {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(int16(math.Max(-32768, math.Min(32767, v)))))
	}
	return b
}

// feed takes one 20 ms frame of left/right samples at the capture rate. muted
// drops the frame (and ends any utterance), e.g. while the head itself is speaking.
func (v *vadStream) feed(l, r []float64, muted bool) {
	if muted {
		if v.active {
			v.end()
		}
		v.preroll, v.voicedRun = nil, 0
		return
	}
	mono := make([]float64, len(l))
	for i := range l {
		mono[i] = (l[i] + r[i]) / 2
	}
	if v.lp != nil {
		mono = v.lp.run(mono)
	}
	mono = v.hp[1].run(v.hp[0].run(mono))
	level := rms(mono)
	// The EMA below only learns while nobody speaks, so a steady noise above
	// the voice threshold (more mic gain, a fan) froze it and every frame looked
	// voiced. Speech always has gaps: the quietest frame of the last 2 s is a
	// floor the noise estimate can't stay under.
	v.recent[v.recentN%noiseWindow] = level
	v.recentN++
	if v.recentN >= noiseWindow {
		floor := v.recent[0]
		for _, x := range v.recent[1:] {
			floor = math.Min(floor, x)
		}
		v.noise = math.Max(v.noise, floor)
	}
	voiced := level > math.Max(v.noise*voiceOverNoise, minVoiceRMS)
	frame := pcm(mono)
	if !v.active {
		if !voiced {
			v.noise = math.Max(20, 0.97*v.noise+0.03*level)
			v.voicedRun = 0
		} else {
			v.voicedRun++
		}
		v.preroll = append(v.preroll, frame)
		if len(v.preroll) > prerollFrames {
			v.preroll = v.preroll[1:]
		}
		if v.voicedRun < 2 {
			return
		}
		v.start()
		return
	}
	v.data(frame)
	v.lenMS += 20
	v.bl, v.br = append(v.bl, l...), append(v.br, r...)
	for len(v.bl) >= v.blk {
		if rms(v.bl[:v.blk]) > v.noise*2 {
			v.gcc.Add(v.bl[:v.blk], v.br[:v.blk])
		}
		v.bl, v.br = v.bl[v.blk:], v.br[v.blk:]
	}
	if voiced {
		v.quietMS = 0
		v.sumSq += level * level * float64(len(mono))
		v.voicedSamp += len(mono)
	} else {
		v.quietMS += 20
	}
	if v.quietMS >= hangoverMS || v.lenMS >= maxUtterMS {
		v.end()
	}
}

func (v *vadStream) start() {
	v.active, v.id, v.seq = true, v.id+1, 0
	v.quietMS, v.lenMS, v.sumSq, v.voicedSamp = 0, 0, 0, 0
	v.bl, v.br = v.bl[:0], v.br[:0]
	v.gcc.Reset()
	v.emit(AudioChunk{Kind: "start", ID: v.id})
	for _, f := range v.preroll {
		v.data(f)
	}
	v.preroll = nil
}

func (v *vadStream) data(frame []byte) {
	v.pending = append(v.pending, frame...)
	if len(v.pending) >= chunkBytes {
		v.flush()
	}
}

func (v *vadStream) flush() {
	if len(v.pending) == 0 {
		return
	}
	v.emit(AudioChunk{Kind: "data", ID: v.id, Seq: v.seq, PCM: v.pending})
	v.seq++
	v.pending = nil
}

func (v *vadStream) end() {
	if v.voicedSamp > 0 {
		slog.Info("utterance", "ms", v.lenMS, "voice_rms", int(math.Sqrt(v.sumSq/float64(v.voicedSamp))), "noise_rms", int(v.noise),
			"threshold", int(math.Max(v.noise*voiceOverNoise, minVoiceRMS)))
	}
	v.flush()
	v.emit(AudioChunk{Kind: "end", ID: v.id})
	v.active, v.voicedRun = false, 0
	if v.gcc.Blocks() >= 3 && v.voicedSamp > 0 {
		ang, ok := v.gcc.Angle(v.spacing, float64(MicRate*v.d))
		if ok {
			db := 20*math.Log10(math.Sqrt(v.sumSq/float64(v.voicedSamp))/32768) + levelOffsetDB
			v.sound(SoundEvent{AngleDeg: math.Round(ang*10) / 10, LevelDB: math.Round(db*10) / 10, ID: v.id})
		}
	}
}

// Mic captures from the HAT with arecord. It implements SoundDir and streams audio.
type Mic struct {
	Device  string  // ALSA device, e.g. "plughw:CARD=wm8960soundcard"
	Spacing float64 // distance between the capsules, m
	Swap    bool    // the card reports right first

	events     chan SoundEvent
	audio      chan AudioChunk
	mutedUntil atomic.Int64
	ok         atomic.Bool
}

func NewMic(device string, spacing float64, swap bool) *Mic {
	return &Mic{Device: device, Spacing: spacing, Swap: swap,
		events: make(chan SoundEvent, 8), audio: make(chan AudioChunk, 64)}
}

func (m *Mic) Name() string {
	if m.ok.Load() {
		return "respeaker"
	}
	return "respeaker (not capturing)"
}

func (m *Mic) Events() <-chan SoundEvent { return m.events }
func (m *Mic) Audio() <-chan AudioChunk  { return m.audio }
func (m *Mic) Mute(d time.Duration)      { m.mutedUntil.Store(time.Now().Add(d).UnixNano()) }
func (m *Mic) muted() bool               { return time.Now().UnixNano() < m.mutedUntil.Load() }

// Run captures until ctx ends, restarting arecord if it dies.
func (m *Mic) Run(ctx context.Context) {
	if _, err := exec.LookPath("arecord"); err != nil {
		slog.Warn("no arecord: head runs without microphone")
		return
	}
	vs := newVADStreamAt(decim, m.Spacing, func(c AudioChunk) {
		select {
		case m.audio <- c:
		default: // the link is slow: drop rather than block capture
		}
	}, func(e SoundEvent) {
		select {
		case m.events <- e:
		default:
		}
	})
	for ctx.Err() == nil {
		if err := m.capture(ctx, vs); err != nil {
			m.ok.Store(false)
			slog.Warn("microphone", "device", m.Device, "err", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}
}

func (m *Mic) capture(ctx context.Context, vs *vadStream) error {
	cmd := exec.CommandContext(ctx, "arecord", "-q", "-D", m.Device, "-f", "S16_LE", "-r", fmt.Sprint(CaptureRate), "-c", "2", "-t", "raw")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer cmd.Wait()
	m.ok.Store(true)
	slog.Info("microphone capturing", "device", m.Device)
	rd := bufio.NewReaderSize(out, 8192)
	const n = frameSamples * decim
	buf := make([]byte, n*4)
	l, r := make([]float64, n), make([]float64, n)
	for {
		if _, err := io.ReadFull(rd, buf); err != nil {
			return err
		}
		for i := 0; i < n; i++ {
			a := float64(int16(binary.LittleEndian.Uint16(buf[4*i:])))
			b := float64(int16(binary.LittleEndian.Uint16(buf[4*i+2:])))
			if m.Swap {
				a, b = b, a
			}
			l[i], r[i] = a, b
		}
		vs.feed(l, r, m.muted())
	}
}

// decimator low-passes (windowed sinc, cut at 0.45 of the output rate) and
// keeps every d-th sample, carrying its filter state across frames. Frames
// must be a multiple of d long (a 20 ms frame always is).
type decimator struct {
	d    int
	taps []float64
	hist []float64
}

func newDecimator(d int) *decimator {
	const n = 48
	taps := make([]float64, n)
	fc := 0.45 / float64(d) // cutoff, cycles per input sample
	sum := 0.0
	for i := range taps {
		x := float64(i) - float64(n-1)/2
		sinc := 2 * fc
		if x != 0 {
			sinc = math.Sin(2*math.Pi*fc*x) / (math.Pi * x)
		}
		taps[i] = sinc * (0.54 - 0.46*math.Cos(2*math.Pi*float64(i)/float64(n-1))) // Hamming
		sum += taps[i]
	}
	for i := range taps {
		taps[i] /= sum
	}
	return &decimator{d: d, taps: taps, hist: make([]float64, n-1)}
}

func (z *decimator) run(x []float64) []float64 {
	buf := append(z.hist, x...)
	out := make([]float64, len(x)/z.d)
	for j := range out {
		acc := 0.0
		for k, t := range z.taps {
			acc += t * buf[j*z.d+k]
		}
		out[j] = acc
	}
	z.hist = append(make([]float64, 0, len(z.taps)-1), buf[len(buf)-(len(z.taps)-1):]...)
	return out
}

// biquad is one second-order section (RBJ cookbook), state kept across frames.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

// highPass is a Butterworth (Q = 1/√2) high-pass section; two in a row make 4th order.
func highPass(fc, fs float64) biquad {
	w0 := 2 * math.Pi * fc / fs
	alpha := math.Sin(w0) / math.Sqrt2 // sin(w0) / (2Q), Q = 1/√2
	c := math.Cos(w0)
	a0 := 1 + alpha
	return biquad{b0: (1 + c) / 2 / a0, b1: -(1 + c) / a0, b2: (1 + c) / 2 / a0, a1: -2 * c / a0, a2: (1 - alpha) / a0}
}

func (f *biquad) run(x []float64) []float64 {
	out := make([]float64, len(x))
	for i, v := range x {
		y := f.b0*v + f.b1*f.x1 + f.b2*f.x2 - f.a1*f.y1 - f.a2*f.y2
		f.x2, f.x1, f.y2, f.y1 = f.x1, v, f.y1, y
		out[i] = y
	}
	return out
}
