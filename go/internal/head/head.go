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
		var in struct {
			RGB        [3]uint8 `json:"rgb"`
			Pattern    string   `json:"pattern"`
			Brightness float64  `json:"brightness"`
		}
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, nil, err
		}
		h.mu.Lock()
		if h.cfg.EyeBrightness > 0 && in.Brightness > h.cfg.EyeBrightness {
			in.Brightness = h.cfg.EyeBrightness
		}
		h.mu.Unlock()
		return map[string]any{"ok": true}, nil, h.B.Eye(in.RGB, in.Pattern, in.Brightness)
	case "say":
		var in struct{ Text, Sound string }
		if err := json.Unmarshal(body, &in); err != nil {
			return nil, nil, err
		}
		secs, err := h.B.Say(ctx, in.Text, in.Sound)
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

func (h *Head) onEvent(p *link.Peer, typ string, body json.RawMessage) {
	if typ == "config" {
		var in struct {
			Rev    int64  `json:"rev"`
			Values Config `json:"values"`
		}
		if json.Unmarshal(body, &in) != nil {
			return
		}
		h.mu.Lock()
		h.cfg, h.rev = in.Values, in.Rev
		h.mu.Unlock()
		if v, ok := h.B.(interface{ SetVolume(float64) }); ok {
			v.SetVolume(in.Values.Volume)
		}
		slog.Info("config applied", "rev", in.Rev, "eye_brightness", in.Values.EyeBrightness, "volume", in.Values.Volume)
		if p != nil {
			_ = p.Notify("config_ack", map[string]int64{"rev": in.Rev})
		}
	}
}

// Run keeps the link to the body up until ctx ends.
func (h *Head) Run(ctx context.Context, addr string, cfg *tls.Config) {
	link.Dial(ctx, addr, cfg, h.handle, h.onEvent, h.pump)
}

// pump pushes state once and the ToF every 100 ms while connected.
func (h *Head) pump(ctx context.Context, p *link.Peer) {
	st := h.B.State()
	h.mu.Lock()
	st["config_rev"] = h.rev
	h.mu.Unlock()
	_ = p.Notify("state", st)
	var sound <-chan SoundEvent
	if sd, ok := h.B.(interface{ Sound() SoundDir }); ok {
		sound = sd.Sound().Events()
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
		case <-t.C:
		}
		if mm, ok := h.B.ToF(); ok {
			if err := p.Notify("tof", map[string]int{"mm": mm}); err != nil {
				return
			}
		}
	}
}
