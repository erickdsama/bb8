package body

import (
	"context"
	"math"
	"time"

	"github.com/erickdsama/bb8/go/internal/events"
	"github.com/erickdsama/bb8/go/internal/motion"
)

// flag → alert kind for the page (the Arduino's names are Spanish in places).
var alertKinds = map[string]string{"obstacle": "obstacle", "tilt": "tilt", "watchdog": "watchdog", "bateria": "battery_low", "imu": "imu"}

// Watch turns robot state into events: pose at most every 500 ms when it moved,
// alerts and control on every edge, the head link on connect and drop.
func Watch(ctx context.Context, bus *events.Bus, svc *motion.Service, head *Head) {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	var (
		lastPose   motion.Result
		lastPoseAt time.Time
		flags      = map[string]bool{}
		control    string
		headUp     *bool
	)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		p := svc.Pose()

		active := map[string]bool{}
		if fl, ok := p["flags"].([]string); ok {
			for _, f := range fl {
				active[f] = true
			}
		}
		for f := range active {
			if !flags[f] {
				if kind, ok := alertKinds[f]; ok {
					d := map[string]any{"kind": kind, "on": true}
					if f == "obstacle" && p["tof_m"] != nil {
						d["distance_m"] = p["tof_m"]
					}
					if f == "bateria" {
						d["battery_v"] = p["battery_v"]
					}
					bus.Publish("alert", d)
				}
			}
		}
		for f := range flags {
			if !active[f] {
				if kind, ok := alertKinds[f]; ok {
					bus.Publish("alert", map[string]any{"kind": kind, "on": false})
				}
			}
		}
		flags = active

		if c, _ := p["control"].(string); c != control {
			control = c
			bus.Publish("control", map[string]any{"who": c})
		}
		if up := head.Link.Connected(); headUp == nil || *headUp != up {
			headUp = &up
			bus.Publish("head", map[string]any{"connected": up})
		}

		if time.Since(lastPoseAt) >= 500*time.Millisecond && moved(lastPose, p) {
			d := map[string]any{"x": p["x_m"], "y": p["y_m"], "heading_deg": p["heading_deg"], "head_deg": p["head_deg"],
				"tilt_deg": p["tilt_deg"], "battery_v": p["battery_v"], "tof_m": p["tof_m"]}
			if x, y, hd, place, ok := svc.Where(); ok {
				d["plan"] = map[string]any{"x": x, "y": y, "heading_deg": hd, "room": place}
			}
			bus.Publish("pose", d)
			lastPose, lastPoseAt = p, time.Now()
		}
	}
}

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	}
	return 0
}

// moved: 1 cm, 1°, a head turn or a 0.1 V battery step is worth a pose event.
func moved(a, b motion.Result) bool {
	if a == nil {
		return true
	}
	return math.Abs(num(a["x_m"])-num(b["x_m"])) >= 0.01 || math.Abs(num(a["y_m"])-num(b["y_m"])) >= 0.01 ||
		math.Abs(num(a["heading_deg"])-num(b["heading_deg"])) >= 1 || num(a["head_deg"]) != num(b["head_deg"]) ||
		math.Abs(num(a["battery_v"])-num(b["battery_v"])) >= 0.1
}
