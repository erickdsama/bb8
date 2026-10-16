// Package body wires the body service: motion, head link, tools and local API.
package body

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/erickdsama/bb8/go/internal/link"
	"github.com/erickdsama/bb8/go/internal/persona"
)

// ToFFresh is how long a pushed ToF reading stays valid (same as D on the Arduino).
const ToFFresh = 300 * time.Millisecond

// Head talks to the head over the secure link.
type Head struct {
	Link *link.Server
	// OnSound receives the voice direction from the head's mic array: the angle
	// relative to the head and the head angle when the utterance started.
	OnSound func(text *string, angleDeg, levelDB, headDeg *float64)
	// HeadDeg is the current post angle (from the Arduino status). The head does
	// not know where it points; the body stamps it on each utterance.
	HeadDeg func() float64
	// Audio fans the head's voiced audio out to local listeners (voz).
	Audio *AudioHub

	mu      sync.Mutex
	tofMM   int
	tofAt   time.Time
	state   map[string]any
	uttHead map[uint64]float64 // utterance id → head angle at audio_start
}

func NewHead(srv *link.Server) *Head {
	h := &Head{Link: srv, state: map[string]any{}, Audio: NewAudioHub(), uttHead: map[uint64]float64{}}
	srv.OnEvent = h.onEvent
	return h
}

func (h *Head) onEvent(_ *link.Peer, typ string, body json.RawMessage, bin []byte) {
	switch typ {
	case "audio_start", "audio", "audio_end":
		var v struct {
			ID  uint64 `json:"id"`
			Seq int    `json:"seq"`
		}
		_ = json.Unmarshal(body, &v)
		if typ == "audio_start" && h.HeadDeg != nil {
			h.mu.Lock()
			h.uttHead[v.ID] = h.HeadDeg()
			for id := range h.uttHead { // keep only the last few
				if id+8 < v.ID {
					delete(h.uttHead, id)
				}
			}
			h.mu.Unlock()
		}
		h.Audio.Publish(AudioFrame{Type: typ, ID: v.ID, Seq: v.Seq, PCM: bin})
	case "tof":
		var v struct {
			MM int `json:"mm"`
		}
		if json.Unmarshal(body, &v) == nil {
			h.mu.Lock()
			h.tofMM, h.tofAt = v.MM, time.Now()
			h.mu.Unlock()
		}
	case "sound_dir":
		var v struct {
			Angle   *float64 `json:"angle_deg"`
			LevelDB *float64 `json:"level_db"`
			ID      uint64   `json:"id"`
			Text    string   `json:"text"`
		}
		if json.Unmarshal(body, &v) == nil && h.OnSound != nil {
			var t *string
			if v.Text != "" {
				t = &v.Text
			}
			var hd *float64
			h.mu.Lock()
			if x, ok := h.uttHead[v.ID]; ok {
				hd = &x
			}
			h.mu.Unlock()
			h.OnSound(t, v.Angle, v.LevelDB, hd)
		}
	case "config_ack":
		var v struct {
			Rev int64 `json:"rev"`
		}
		if json.Unmarshal(body, &v) == nil {
			h.mu.Lock()
			h.state["config_rev"] = v.Rev
			h.mu.Unlock()
		}
	case "state":
		var v map[string]any
		if json.Unmarshal(body, &v) == nil {
			h.mu.Lock()
			for k, x := range v {
				h.state[k] = x
			}
			h.mu.Unlock()
		}
	}
}

// ToF returns the latest forward distance if fresh.
func (h *Head) ToF() (int, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if time.Since(h.tofAt) > ToFFresh || !h.Link.Connected() {
		return 0, false
	}
	return h.tofMM, true
}

// State is what the head reported (camera present, applied config rev…).
func (h *Head) State() map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := map[string]any{"connected": h.Link.Connected()}
	for k, v := range h.state {
		out[k] = v
	}
	return out
}

func (h *Head) request(ctx context.Context, typ string, body any, timeout time.Duration) (map[string]any, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	raw, bin, err := h.Link.Request(ctx, typ, body)
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out, bin, err
}

var ErrNoCamera = errors.New("the head has no camera: I cannot see")

func (h *Head) Photo(ctx context.Context, width int) ([]byte, error) {
	width = max(64, min(1920, width))
	_, jpg, err := h.request(ctx, "photo", map[string]int{"width": width}, 10*time.Second)
	if err != nil {
		if err.Error() == "no camera" {
			return nil, ErrNoCamera
		}
		return nil, errors.New("no photo from the head: " + err.Error())
	}
	if len(jpg) == 0 {
		return nil, errors.New("the head sent an empty photo")
	}
	return jpg, nil
}

func fail(err error) map[string]any { return map[string]any{"result": "error", "reason": err.Error()} }

func done(m map[string]any) map[string]any {
	out := map[string]any{"result": "done"}
	for k, v := range m {
		if k != "ok" {
			out[k] = v
		}
	}
	return out
}

func (h *Head) Eye(ctx context.Context, color, pattern string, brightness float64) map[string]any {
	if pattern == "" {
		pattern = "steady"
	}
	rgb, err := persona.ParseColor(color)
	if err != nil {
		return fail(err)
	}
	if !persona.ValidPattern(pattern) {
		return fail(errors.New("unknown pattern " + pattern + ". Use steady, breathe, blink or off"))
	}
	if brightness <= 0 || brightness > persona.MaxBrightness {
		brightness = persona.MaxBrightness
	}
	r, _, err := h.request(ctx, "eye", map[string]any{"rgb": rgb, "pattern": pattern, "brightness": brightness}, 3*time.Second)
	if err != nil {
		return fail(err)
	}
	return done(r)
}

func (h *Head) Say(ctx context.Context, text, sound string) map[string]any {
	if sound != "" {
		if _, ok := persona.Emotions[sound]; !ok {
			return fail(errors.New("unknown sound " + sound))
		}
	}
	if text == "" && sound == "" {
		return fail(errors.New("send 'text' or 'sound'"))
	}
	r, _, err := h.request(ctx, "say", map[string]string{"text": text, "sound": sound}, 30*time.Second)
	if err != nil {
		return fail(err)
	}
	return done(r)
}

// Rest: active=false dims the eye and pauses the camera; true wakes them.
func (h *Head) Rest(ctx context.Context, active bool) map[string]any {
	r, _, err := h.request(ctx, "rest", map[string]bool{"active": active}, 5*time.Second)
	if err != nil {
		return fail(err)
	}
	return done(r)
}

// PushConfig sends the head's config section; the head answers config_ack.
func (h *Head) PushConfig(section map[string]any, rev int64) error {
	return h.Link.Notify("config", map[string]any{"rev": rev, "values": section})
}
