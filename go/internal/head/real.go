package head

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/devices/v3/apa102"
	"periph.io/x/devices/v3/nrzled"
	"periph.io/x/host/v3"
)

// Real is the Pi Zero W hardware. Each part is optional: what is missing is
// logged and reported in State, and the head still runs.
//
//   - Camera: rpicam-jpeg on demand (no process stays alive between photos).
//   - Eye: 16-LED WS2812 ring. With the ReSpeaker HAT (default "ws2812-pwm") its
//     data goes to GPIO12 (PWM0, the HAT's Grove digital port), driven by the
//     bb8-eye C helper (rpi_ws281x, PWM + DMA, root) through /run/bb8-eye.sock:
//     SPI0 belongs to the HAT's APA102 LEDs and GPIO18 to its I2S. Without the HAT
//     it can go on SPI0 MOSI ("ws2812", periph.io nrzled). "apa102" drives the HAT's
//     own LEDs. Pin notes in wiki/Materiales.md.
//   - Speaker: beeps synthesized in Go, played with aplay; text with espeak-ng
//     (Piper does not run on the armv6 Zero W).
//   - ToF: VL53L0X on I2C1 (0x29) once it moves from the Arduino to the head.
//   - Microphone: ReSpeaker 2-Mic HAT (Mic): VAD, direction of arrival and the
//     voiced audio streamed to the body. Its speaker output is the HAT's codec.
type Real struct {
	camera string
	eye    *ringEye
	tof    *VL53L0X
	tofMM  atomic.Int64 // -1 = no reading
	sound  SoundDir
	mic    *Mic

	sayMu  sync.Mutex
	volume atomic.Value // float64
	paused atomic.Bool
}

// RealConfig of the real head.
//   - Eye: "ws2812-pwm" (WS2812 on GPIO12 via the bb8-eye helper at EyeSocket),
//     "ws2812" (WS2812 on SPI0 MOSI, no HAT), "apa102" (the HAT's LEDs) or "none".
//   - MicDevice: ALSA device of the ReSpeaker ("" = no mic); MicSpacing in m; MicSwap
//     if the card reports the right capsule first.
type RealConfig struct {
	Eye        string
	EyeLEDs    int
	EyeSocket  string // bb8-eye socket for ws2812-pwm
	MicDevice  string
	MicSpacing float64
	MicSwap    bool
}

func NewReal(ctx context.Context, c RealConfig) *Real {
	r := &Real{sound: NoSoundDir{}}
	if c.MicDevice != "" {
		m := NewMic(c.MicDevice, c.MicSpacing, c.MicSwap)
		r.sound, r.mic = m, m
		go m.Run(ctx)
	}
	r.volume.Store(0.7)
	r.tofMM.Store(-1)
	for _, c := range []string{"rpicam-jpeg", "libcamera-jpeg"} {
		if _, err := exec.LookPath(c); err == nil {
			r.camera = c
			break
		}
	}
	if r.camera == "" {
		slog.Warn("no rpicam-jpeg: head runs without camera")
	}
	if _, err := host.Init(); err != nil {
		slog.Warn("periph host init", "err", err)
		return r
	}
	if c.Eye != "none" {
		if e, err := newRingEye(c.Eye, max(1, c.EyeLEDs), c.EyeSocket); err != nil {
			slog.Warn("no eye LEDs", "kind", c.Eye, "err", err)
		} else {
			r.eye = e
		}
	}
	if bus, err := i2creg.Open(""); err != nil {
		slog.Warn("no I2C bus", "err", err)
	} else if t, err := NewVL53L0X(bus); err != nil {
		slog.Info("no ToF on the head (still on the Arduino?)", "err", err)
	} else {
		r.tof = t
		go r.tofLoop()
	}
	return r
}

func (r *Real) tofLoop() {
	for {
		mm, err := r.tof.RangeMM()
		if err != nil {
			r.tofMM.Store(-1)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		r.tofMM.Store(int64(mm))
		time.Sleep(30 * time.Millisecond)
	}
}

func (r *Real) Photo(ctx context.Context, width int) ([]byte, error) {
	if r.camera == "" {
		return nil, ErrNoCamera
	}
	cmd := exec.CommandContext(ctx, r.camera, "-n", "-t", "1", "--width", fmt.Sprint(width),
		"--height", fmt.Sprint(width*3/4), "-q", "80", "-o", "-")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %v: %s", r.camera, err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

func (r *Real) Eye(rgb [3]uint8, pattern string, brightness float64) error {
	if r.eye == nil {
		slog.Info("eye (no ring)", "rgb", rgb, "pattern", pattern)
		return nil
	}
	r.eye.set(rgb, pattern, brightness)
	return nil
}

func play(ctx context.Context, wav []byte) error {
	cmd := exec.CommandContext(ctx, "aplay", "-q", "-")
	cmd.Stdin = bytes.NewReader(wav)
	return cmd.Run()
}

func (r *Real) Say(ctx context.Context, text, sound string) (float64, error) {
	r.sayMu.Lock()
	defer r.sayMu.Unlock()
	t0 := time.Now()
	// The mic would hear BB-8 itself: mute it while speaking and a bit after.
	if r.mic != nil {
		r.mic.Mute(30 * time.Second)
		defer r.mic.Mute(400 * time.Millisecond)
	}
	vol := r.volume.Load().(float64)
	key := sound
	if key == "" {
		key = text
	}
	wav, _ := BeepWAV(key, 0.6*vol, int(time.Now().UnixNano()%4))
	if err := play(ctx, wav); err != nil {
		return 0, fmt.Errorf("aplay: %w", err)
	}
	if text != "" {
		if _, err := exec.LookPath("espeak-ng"); err != nil {
			slog.Warn("espeak-ng not installed: beeps only")
		} else {
			cmd := exec.CommandContext(ctx, "espeak-ng", "-v", "es", "-a", fmt.Sprint(int(vol*200)), "--stdout", text)
			out, err := cmd.Output()
			if err == nil {
				err = play(ctx, out)
			}
			if err != nil {
				return 0, fmt.Errorf("espeak-ng: %w", err)
			}
		}
	}
	return math.Round(time.Since(t0).Seconds()*100) / 100, nil
}

// Rest: inactive dims the eye to a faint breathing and drops camera use.
func (r *Real) Rest(active bool) error {
	r.paused.Store(!active)
	if r.eye != nil {
		if active {
			r.eye.set([3]uint8{40, 110, 255}, "steady", 0.3)
		} else {
			r.eye.set([3]uint8{40, 110, 255}, "breathe", 0.03)
		}
	}
	return nil
}

func (r *Real) ToF() (int, bool) {
	v := r.tofMM.Load()
	return int(v), v >= 0
}

// SetVolume is applied from the head config section.
func (r *Real) SetVolume(v float64) { r.volume.Store(max(0, min(1, v))) }

func (r *Real) Sound() SoundDir { return r.sound }

func (r *Real) State() map[string]any {
	return map[string]any{"backend": "real", "camera": r.camera != "", "eye": r.eye != nil, "mic": r.mic != nil,
		"tof": r.tof != nil, "sound_dir": r.sound.Name()}
}

// --- eye LEDs (NeoPixel or APA102 on SPI0) ------------------------------------

type pixelWriter interface {
	Write(pixels []byte) (int, error)
}

type ringEye struct {
	dev pixelWriter
	n   int

	mu         sync.Mutex
	rgb        [3]uint8
	pattern    string
	brightness float64
}

func newRingEye(kind string, n int, socket string) (*ringEye, error) {
	var d pixelWriter
	if kind == "ws2812-pwm" {
		d = &helperEye{path: socket}
	} else {
		p, err := spireg.Open("")
		if err != nil {
			return nil, err
		}
		switch kind {
		case "apa102":
			d, err = apa102.New(p, &apa102.Opts{NumPixels: n, Intensity: 255, Temperature: apa102.NeutralTemp})
		default:
			d, err = nrzled.NewSPI(p, &nrzled.Opts{NumPixels: n, Channels: 3, Freq: 2500 * physic.KiloHertz})
		}
		if err != nil {
			return nil, err
		}
	}
	e := &ringEye{dev: d, n: n, rgb: [3]uint8{40, 110, 255}, pattern: "steady", brightness: 0.3}
	go e.loop()
	return e, nil
}

// helperEye sends RGB frames (3 bytes per LED) to the bb8-eye helper over a
// unix socket, reconnecting when the helper restarts.
type helperEye struct {
	path   string
	conn   net.Conn
	warned bool
}

func (h *helperEye) Write(pixels []byte) (int, error) {
	if h.conn == nil {
		c, err := net.DialTimeout("unix", h.path, time.Second)
		if err != nil {
			if !h.warned {
				slog.Warn("eye helper not reachable (bb8-eye.socket)", "path", h.path, "err", err)
				h.warned = true
			}
			time.Sleep(time.Second)
			return 0, err
		}
		h.conn, h.warned = c, false
	}
	_ = h.conn.SetWriteDeadline(time.Now().Add(200 * time.Millisecond))
	n, err := h.conn.Write(pixels)
	if err != nil {
		h.conn.Close()
		h.conn = nil
	}
	return n, err
}

func (e *ringEye) set(rgb [3]uint8, pattern string, b float64) {
	e.mu.Lock()
	e.rgb, e.pattern, e.brightness = rgb, pattern, min(b, 0.3)
	e.mu.Unlock()
}

func (e *ringEye) loop() {
	t0 := time.Now()
	buf := make([]byte, 3*e.n)
	for {
		e.mu.Lock()
		rgb, pat, k := e.rgb, e.pattern, e.brightness
		e.mu.Unlock()
		t := time.Since(t0).Seconds()
		switch pat {
		case "off":
			k = 0
		case "breathe":
			k *= 0.15 + 0.85*(0.5+0.5*math.Sin(t*2))
		case "blink":
			if int(t*4)%2 == 1 {
				k = 0
			}
		}
		for i := 0; i < e.n; i++ {
			for c := range 3 {
				buf[3*i+c] = byte(float64(rgb[c]) * k)
			}
		}
		_, _ = e.dev.Write(buf)
		time.Sleep(33 * time.Millisecond)
	}
}
