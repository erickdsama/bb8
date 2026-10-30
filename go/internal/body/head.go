// Package body wires the body service: motion, head link, tools and local API.
package body

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
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
	// Voice, if set, makes the spoken voice here (16-bit mono PCM at rate Hz) and
	// the head just plays it. nil audio or an error: the head speaks the text itself.
	Voice func(ctx context.Context, text string) (pcm []byte, rate int, err error)

	mu      sync.Mutex
	tofMM   int
	tofAt   time.Time
	state   map[string]any
	uttHead map[uint64]float64 // utterance id → head angle at audio_start
	lastEye map[string]any     // replayed on reconnect: the head shows "waiting" while unlinked
}

func NewHead(srv *link.Server) *Head {
	h := &Head{Link: srv, state: map[string]any{}, Audio: NewAudioHub(), uttHead: map[uint64]float64{}}
	srv.OnEvent = h.onEvent
	prev := srv.OnConnect
	srv.OnConnect = func(p *link.Peer) {
		if prev != nil {
			prev(p)
		}
		h.mu.Lock()
		eye := h.lastEye
		h.mu.Unlock()
		if eye != nil {
			if _, _, err := h.request(context.Background(), "eye", eye, 3*time.Second); err != nil {
				slog.Warn("head: restoring eye", "err", err)
			}
		}
	}
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
	req := map[string]any{"rgb": rgb, "pattern": pattern, "brightness": brightness}
	h.mu.Lock()
	h.lastEye = req
	h.mu.Unlock()
	r, _, err := h.request(ctx, "eye", req, 3*time.Second)
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
	if text != "" && h.Voice != nil {
		if first, rest := splitFirstSentence(text); rest != "" {
			return h.sayInParts(ctx, first, rest, sound)
		}
	}
	return h.sayOne(ctx, text, sound, h.voice(ctx, text), false)
}

type voiced struct {
	pcm  []byte
	rate int
}

// voice makes the body's voice for text; nil when there is none (the head speaks it).
func (h *Head) voice(ctx context.Context, text string) *voiced {
	if text == "" || h.Voice == nil {
		return nil
	}
	t0 := time.Now()
	pcm, rate, err := h.Voice(ctx, text)
	if err != nil {
		slog.Warn("voice synthesis failed, the head speaks it", "err", err)
		return nil
	}
	if len(pcm) == 0 || len(pcm) >= 4<<20 { // a link frame is at most 8 MB, base64 included
		return nil
	}
	slog.Info("voice synthesis", "ms", time.Since(t0).Milliseconds(), "chars", len([]rune(text)), "audio_s", len(pcm)/2/rate)
	return &voiced{pcm, rate}
}

func (h *Head) sayOne(ctx context.Context, text, sound string, v *voiced, noBeep bool) map[string]any {
	req := map[string]any{"text": text, "sound": sound}
	wait := 30 * time.Second
	if v != nil {
		req["pcm"], req["rate"] = v.pcm, v.rate
		wait += time.Duration(len(v.pcm)/2/v.rate) * time.Second
	}
	if noBeep {
		req["no_beep"] = true
	}
	r, _, err := h.request(ctx, "say", req, wait)
	if err != nil {
		return fail(err)
	}
	return done(r)
}

// sayInParts starts speaking after the first sentence is synthesized instead of the
// whole reply: both parts are made at once and the rest plays without a second beep.
func (h *Head) sayInParts(ctx context.Context, first, rest, sound string) map[string]any {
	later := make(chan *voiced, 1)
	go func() { later <- h.voice(ctx, rest) }()
	r := h.sayOne(ctx, first, sound, h.voice(ctx, first), false)
	v := <-later
	if r["result"] != "done" {
		return r
	}
	r2 := h.sayOne(ctx, rest, "", v, v != nil)
	if s1, ok := r["seconds"].(float64); ok {
		if s2, ok := r2["seconds"].(float64); ok {
			r2["seconds"] = s1 + s2
		}
	}
	return r2
}

// splitFirstSentence cuts text after its first sentence, once that sentence has some
// length (a lone "¡Hola!" is not worth a separate request). rest is "" when there is
// nothing to gain.
func splitFirstSentence(text string) (first, rest string) {
	rs := []rune(text)
	for i := 0; i < len(rs)-1; i++ {
		switch rs[i] {
		case '.', '!', '?', '…':
		default:
			continue
		}
		if rs[i+1] != ' ' || i < 25 {
			continue
		}
		first, rest = string(rs[:i+1]), strings.TrimSpace(string(rs[i+1:]))
		if len([]rune(rest)) < 15 {
			return text, ""
		}
		return first, rest
	}
	return text, ""
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
