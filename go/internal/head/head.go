// Package head is the head side (Pi Zero W): it dials the body over the secure
// link, answers photo/eye/say/rest and pushes ToF and state.
package head

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/erickdsama/bb8/go/internal/link"
)

var ErrNoCamera = errors.New("no camera")

// Backend is the hardware (or the simulator).
type Backend interface {
	Photo(ctx context.Context, width int) ([]byte, error)
	Eye(rgb [3]uint8, pattern string, brightness float64) error
	Say(ctx context.Context, text, sound string) (seconds float64, err error)
	Rest(active bool) error
	ToF() (mm int, ok bool) // ok=false: no sensor or no reading
	State() map[string]any
}

// Voiced is a backend that can play speech the body synthesized (16-bit mono
// PCM at rate Hz) instead of making its own from the text.
type Voiced interface {
	// beep=false plays only pcm: the body sends a long reply in parts and beeps once.
	SayPCM(ctx context.Context, text, sound string, pcm []byte, rate int, beep bool) (seconds float64, err error)
}

const ToFEvery = 100 * time.Millisecond

// Config is the head's section of the robot config, pushed by the body.
type Config struct {
	EyeBrightness float64 `json:"eye_brightness"`
	Volume        float64 `json:"volume"`
}

type Head struct {
	B Backend

	mu  sync.Mutex
	cfg Config
	rev int64
	eye eyeState // the last eye asked for, before the brightness cap
}

type eyeState struct {
	RGB        [3]uint8 `json:"rgb"`
	Pattern    string   `json:"pattern"`
	Brightness float64  `json:"brightness"`
}

// startEye is what the ring shows at boot (real.go newRingEye).
var startEye = eyeState{RGB: [3]uint8{40, 110, 255}, Pattern: "steady", Brightness: 0.3}

// capped applies the eye_brightness setting as a ceiling. Called with mu held.
func (h *Head) capped(e eyeState) eyeState {
	if h.cfg.EyeBrightness > 0 && e.Brightness > h.cfg.EyeBrightness {
		e.Brightness = h.cfg.EyeBrightness
	}
	return e
}

func (h *Head) handle(ctx context.Context, typ string, body json.RawMessage) (any, []byte, error) {
	switch typ {
	case "photo":
		var in struct {
			Width int `json:"width"`
		}
		_ = json.Unmarshal(body, &in)
		jpg, err := h.B.Photo(ctx, max(64, min(1920, in.Width)))
		if err != nil {
			return nil, nil, err
		}
		return map[string]any{"ok": true, "bytes": len(jpg)}, jpg, nil
	case "eye":
		var in eyeState
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, nil, err
		}
		h.mu.Lock()
		h.eye = in
		e := h.capped(in)
		h.mu.Unlock()
		return map[string]any{"ok": true}, nil, h.B.Eye(e.RGB, e.Pattern, e.Brightness)
	case "say":
		var in struct {
			Text, Sound string
			PCM         []byte `json:"pcm"` // optional: the body's voice for Text
			Rate        int    `json:"rate"`
			NoBeep      bool   `json:"no_beep"` // a later part of one reply: no beep before it
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, nil, err
		}
		var secs float64
		var err error
		if v, ok := h.B.(Voiced); ok && len(in.PCM) > 0 && in.Rate > 0 {
			secs, err = v.SayPCM(ctx, in.Text, in.Sound, in.PCM, in.Rate, !in.NoBeep)
		} else {
			secs, err = h.B.Say(ctx, in.Text, in.Sound)
		}
		return map[string]any{"ok": err == nil, "seconds": secs}, nil, err
	case "rest":
		var in struct {
			Active bool `json:"active"`
		}
		_ = json.Unmarshal(body, &in)
		return map[string]any{"ok": true, "active": in.Active}, nil, h.B.Rest(in.Active)
	}
	return nil, nil, fmt.Errorf("unknown request %q", typ)
}

func (h *Head) onEvent(p *link.Peer, typ string, body json.RawMessage, _ []byte) {
	if typ == "config" {
		var in struct {
			Rev    int64  `json:"rev"`
			Values Config `json:"values"`
		}
		if json.Unmarshal(body, &in) != nil {
			return
		}
		h.mu.Lock()
		changed := h.cfg.EyeBrightness != in.Values.EyeBrightness
		h.cfg, h.rev = in.Values, in.Rev
		cur := h.eye
		if cur.Pattern == "" {
			cur = startEye
		}
		e := h.capped(cur)
		h.mu.Unlock()
		if changed { // show the new brightness now, not at the next color change
			_ = h.B.Eye(e.RGB, e.Pattern, e.Brightness)
		}
		if v, ok := h.B.(interface{ SetVolume(float64) }); ok {
			v.SetVolume(in.Values.Volume)
		}
		slog.Info("config applied", "rev", in.Rev, "eye_brightness", in.Values.EyeBrightness, "volume", in.Values.Volume)
		if p != nil {
			_ = p.Notify("config_ack", map[string]int64{"rev": in.Rev})
		}
	}
}

// WaitingRGB is the eye while the head has no body: from power-on until the link
// is up, and again whenever it drops. The body restores its last color on connect.
var WaitingRGB = [3]uint8{255, 110, 0}

// Run keeps the link to the body up until ctx ends.
func (h *Head) Run(ctx context.Context, addr string, cfg *tls.Config) {
	_ = h.B.Eye(WaitingRGB, "breathe", 0.3)
	link.Dial(ctx, addr, cfg, h.handle, h.onEvent, func(ctx context.Context, p *link.Peer) {
		h.pump(ctx, p)
		_ = h.B.Eye(WaitingRGB, "breathe", 0.3)
	})
}

// pump pushes state once and the ToF every 100 ms while connected.
func (h *Head) pump(ctx context.Context, p *link.Peer) {
	st := h.B.State()
	h.mu.Lock()
	st["config_rev"] = h.rev
	h.mu.Unlock()
	_ = p.Notify("state", st)
	var sound <-chan SoundEvent
	var audio <-chan AudioChunk
	if sd, ok := h.B.(interface{ Sound() SoundDir }); ok {
		sound = sd.Sound().Events()
		if as, ok := sd.Sound().(AudioSource); ok {
			audio = as.Audio()
		}
	}
	t := time.NewTicker(ToFEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-sound:
			_ = p.Notify("sound_dir", e)
			continue
		case c := <-audio:
			var err error
			switch c.Kind {
			case "start":
				err = p.Notify("audio_start", map[string]any{"id": c.ID, "rate": MicRate, "channels": 1, "format": "s16le"})
			case "data":
				err = p.NotifyBin("audio", map[string]any{"id": c.ID, "seq": c.Seq}, c.PCM)
			case "end":
				err = p.Notify("audio_end", map[string]any{"id": c.ID})
			}
			if err != nil {
				return
			}
			continue
		case <-t.C:
		}
		if mm, ok := h.B.ToF(); ok {
			if err := p.Notify("tof", map[string]int{"mm": mm}); err != nil {
				return
			}
		}
	}
}
