package body

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/link"
	"github.com/erickdsama/bb8/go/internal/motion"
	"github.com/erickdsama/bb8/go/internal/tools"
)

// RobotFields is the core schema. Connectors register their own fields.
func RobotFields() []config.Field {
	d := motion.Defaults()
	n := func(key, label string, def, lo, hi float64, help string) config.Field {
		return config.Field{Key: key, Label: label, Type: config.Number, Default: def, Min: config.F(lo), Max: config.F(hi), Help: help}
	}
	return []config.Field{
		n("motion.pwm_cruise", "Cruise speed (PWM)", d.PWMCruise, 0, 160, "Straight-line speed. 140 ≈ 0.5 m/s. Hard ceiling 160."),
		n("motion.pwm_min", "Minimum PWM", d.PWMMin, 0, 160, "Below this the motors do not overcome friction."),
		n("motion.pwm_turn", "Turn speed (PWM)", d.PWMTurn, 0, 160, ""),
		n("motion.pwm_turn_min", "Minimum turn PWM", d.PWMTurnMin, 0, 160, ""),
		n("motion.pwm_manual_max", "Gamepad max PWM", d.PWMManualMax, 0, 160, ""),
		n("motion.move_max_m", "Max distance per move (m)", d.MoveMaxM, 0, 3, ""),
		n("motion.turn_max_deg", "Max turn per order (°)", d.TurnMaxDeg, 0, 360, ""),
		n("motion.head_max_deg", "Head range (±°)", d.HeadMaxDeg, 0, 90, ""),
		n("motion.order_max_s", "Max order time (s)", d.OrderMaxS, 0.5, 10, ""),
		n("motion.manual_hold_s", "Gamepad priority (s)", d.ManualHoldS, 0.2, 5, "The gamepad wins over the agent for this long after a touch."),
		n("motion.wheel_diameter_m", "Wheel diameter (m)", d.WheelDiameterM, 0.03, 0.15, "Calibration."),
		n("motion.ticks_per_rev", "Encoder ticks per wheel turn", d.TicksPerRev, 100, 10000, "Calibration: turn a wheel 10 times by hand and read E."),
		n("motion.slip_factor", "Slip factor", d.SlipFactor, 0.5, 2, "Calibration: >1 if the wheels slip inside the sphere."),
		n("motion.track_width_m", "Track width (m)", d.TrackWidthM, 0.05, 0.4, "Calibration."),
		{Key: "head.eye_brightness", Label: "Eye brightness", Type: config.Number, Default: 0.3, Min: config.F(0.02), Max: config.F(0.3), Agent: true},
		{Key: "head.volume", Label: "Speaker volume", Type: config.Number, Default: 0.7, Min: config.F(0), Max: config.F(1), Agent: true},
		{Key: "voice.audio_log", Label: "Keep recent voice clips", Type: config.Bool, Default: false,
			Help: "The last 20 phrases the head heard, in RAM only, playable below. For tuning the microphone."},
		{Key: "voice.speed", Label: "Speech speed", Type: config.Number, Default: 1.0, Min: config.F(0.5), Max: config.F(2), Agent: true,
			Help: "Read by the voice front end."},
		{Key: "home.city", Label: "City", Type: config.String, Default: "", Agent: true, Help: "Used by weather and to talk about local time."},
		{Key: "home.latitude", Label: "Latitude", Type: config.Number, Default: 0.0, Min: config.F(-90), Max: config.F(90)},
		{Key: "home.longitude", Label: "Longitude", Type: config.Number, Default: 0.0, Min: config.F(-180), Max: config.F(180)},
		{Key: "home.timezone", Label: "Time zone", Type: config.String, Default: "America/Mexico_City"},
		{Key: "home.owner", Label: "Owner name", Type: config.String, Default: "Erick", Agent: true},
	}
}

// MotionParams reads the motion section into motion.Params.
func MotionParams(s *config.Store) motion.Params {
	p := motion.Defaults()
	b, _ := json.Marshal(s.Section("motion"))
	_ = json.Unmarshal(b, &p)
	return p
}

// Wire applies config changes live: motion params now, head section over the link.
func Wire(s *config.Store, m *motion.Service, h *Head) {
	m.SetParams(MotionParams(s))
	push := func() {
		if err := h.PushConfig(s.Section("head"), s.Rev()); err != nil && !errors.Is(err, link.ErrDisconnected) {
			slog.Warn("push head config", "err", err)
		}
	}
	prev := h.Link.OnConnect
	h.Link.OnConnect = func(p *link.Peer) {
		if prev != nil {
			prev(p)
		}
		push()
	}
	s.Subscribe(func(c config.Change) {
		if c.Touches("motion.") {
			m.SetParams(MotionParams(s))
			slog.Info("motion params updated", "rev", c.Rev, "source", c.Source)
		}
		if c.Touches("head.") {
			push()
		}
	})
}

// SettingTools let the agent read and change agent-writable settings.
func SettingTools(s *config.Store) []tools.Tool {
	var keys []string
	for _, f := range s.Schema() {
		if f.Agent {
			keys = append(keys, f.Key)
		}
	}
	return []tools.Tool{
		{Name: "get_setting",
			Description: "Read robot settings. key: one setting, or empty for all you may change. Writable by you: " + strings.Join(keys, ", "),
			Schema:      tools.Object(map[string]any{"key": tools.Str("setting key, e.g. head.volume")}),
			Handler: tools.Bind(func(_ context.Context, in struct {
				Key string `json:"key"`
			}) (tools.Output, error) {
				out := map[string]any{}
				for _, f := range s.Schema() {
					if f.Type == config.Secret || (in.Key == "" && !f.Agent) || (in.Key != "" && f.Key != in.Key) {
						continue
					}
					out[f.Key] = map[string]any{"value": s.Get(f.Key), "min": f.Min, "max": f.Max, "options": f.Options, "writable": f.Agent}
				}
				if len(out) == 0 {
					return tools.Output{}, errors.New("unknown setting " + in.Key)
				}
				return tools.Data(out)
			})},
		{Name: "set_setting",
			Description: "Change one robot setting when someone asks you (\"habla más bajo\", \"vivo en Monterrey\"). " +
				"Only these keys: " + strings.Join(keys, ", ") + ". Safety limits are not yours to change.",
			Schema: tools.Object(map[string]any{"key": tools.Enum("setting", keys...), "value": map[string]any{"description": "new value (number, text or true/false)"}}, "key", "value"),
			Handler: tools.Bind(func(_ context.Context, in struct {
				Key   string `json:"key"`
				Value any    `json:"value"`
			}) (tools.Output, error) {
				rev, err := s.Set(map[string]any{in.Key: in.Value}, config.SourceAgent, nil)
				if err != nil {
					return tools.Data(map[string]any{"result": "error", "reason": err.Error()})
				}
				return tools.Data(map[string]any{"result": "done", "key": in.Key, "value": s.Get(in.Key), "rev": rev})
			})},
	}
}
