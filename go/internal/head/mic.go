package head

// ReSpeaker 2-Mic HAT on the head: capture (arecord, no cgo), an energy VAD,
// direction of arrival per utterance (GCC-PHAT) and the voiced audio, so the
// body only receives sound while someone speaks.

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"math"
	"os/exec"
	"sync/atomic"
	"time"
)

const (
	MicRate        = 16000
	frameSamples   = 320  // 20 ms
	gccBlock       = 512  // 32 ms per GCC-PHAT block
	prerollFrames  = 15   // 300 ms kept before the voice starts
	chunkBytes     = 3200 // 100 ms of 16-bit mono per audio frame on the link
	hangoverMS     = 600  // silence that ends an utterance
	maxUtterMS     = 15000
	minVoiceRMS    = 150.0 // int16 units; below this is never voice
	voiceOverNoise = 3.2   // ≈ +10 dB over the noise floor
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
	sumSq      float64
	voicedSamp int
}

func newVADStream(spacing float64, emit func(AudioChunk), sound func(SoundEvent)) *vadStream {
	return &vadStream{spacing: spacing, emit: emit, sound: sound, noise: 60, gcc: NewGCC(gccBlock)}
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

// feed takes one 20 ms frame of left/right samples. muted drops the frame
// (and ends any utterance), e.g. while the head itself is speaking.
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
	level := rms(mono)
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
	for len(v.bl) >= gccBlock {
		if rms(v.bl[:gccBlock]) > v.noise*2 {
			v.gcc.Add(v.bl[:gccBlock], v.br[:gccBlock])
		}
		v.bl, v.br = v.bl[gccBlock:], v.br[gccBlock:]
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
	v.flush()
	v.emit(AudioChunk{Kind: "end", ID: v.id})
	v.active, v.voicedRun = false, 0
	if v.gcc.Blocks() >= 3 && v.voicedSamp > 0 {
		ang, ok := v.gcc.Angle(v.spacing, MicRate)
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
	vs := newVADStream(m.Spacing, func(c AudioChunk) {
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
	cmd := exec.CommandContext(ctx, "arecord", "-q", "-D", m.Device, "-f", "S16_LE", "-r", "16000", "-c", "2", "-t", "raw")
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
	buf := make([]byte, frameSamples*4)
	l, r := make([]float64, frameSamples), make([]float64, frameSamples)
	for {
		if _, err := io.ReadFull(rd, buf); err != nil {
			return err
		}
		for i := 0; i < frameSamples; i++ {
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
