// Package navigate is BB-8's "System 1": a fast loop that drives to a room or a
// piece of furniture one short step at a time. Each step builds a small state
// (next waypoint from the house plan, distance ahead, last scan, last results)
// and asks a planner for one action from a fixed set:
//
//   - JEV (TypeSafe System One model, ~70–500 ms) picks it as a typed choice with
//     a calibrated confidence;
//   - the local rule planner is the fallback when JEV is off, fails or is unsure.
//
// The LLM ("System 2") only sets the goal with navigate_to. Every action goes
// through motion.Service, so the clamps, the gamepad arbiter and the Arduino
// reflexes apply exactly as for any other order.
package navigate

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/floorplan"
	"github.com/erickdsama/bb8/go/internal/motion"
)

// Actions the planner can choose. The executor sets the size of each one.
const (
	TurnToWaypoint = "turn_to_waypoint"
	Forward        = "forward"
	ForwardShort   = "forward_short"
	TurnToClearest = "turn_to_clearest"
	TurnLeft       = "turn_left"
	TurnRight      = "turn_right"
	BackUp         = "back_up"
	Scan           = "scan"
	Arrived        = "arrived"
	GiveUp         = "give_up"
)

// ActionHelp is what JEV reads for each option (its choice criteria).
var ActionHelp = map[string]string{
	TurnToWaypoint: "rotate in place to face the next waypoint (by waypoint.turn_deg)",
	Forward:        "drive straight toward the waypoint, up to 0.6 m; only when facing it (|turn_deg| small) and the way ahead is clear",
	ForwardShort:   "drive straight 0.25 m; for a tight spot or when ahead_m is short but above 0.4 m",
	TurnToClearest: "rotate toward last_scan.clearest_deg, the most open direction; after being blocked",
	TurnLeft:       "rotate 45° left",
	TurnRight:      "rotate 45° right",
	BackUp:         "drive back 0.2 m; when blocked right after turning or stuck",
	Scan:           "sweep the head to measure distances around; when blocked or ahead_m is short and there is no recent scan",
	Arrived:        "the goal is reached: waypoint.final is true and waypoint.distance_m <= waypoint.arrive_m (or the furniture goal is right in front)",
	GiveUp:         "stop and ask for help: blocked many times, no path, or the position is uncertain",
}

func Fields() []config.Field {
	return []config.Field{
		{Key: "navigation.planner", Label: "Planner", Type: config.Enum, Options: []string{"auto", "jev", "local"}, Default: "auto",
			Help: "auto: JEV if its key is set, else local rules."},
		{Key: "navigation.jev_api_key", Label: "TypeSafe API key (JEV)", Type: config.Secret},
		{Key: "navigation.jev_model", Label: "JEV model", Type: config.String, Default: "jev-latest"},
		{Key: "navigation.min_confidence", Label: "Min JEV confidence", Type: config.Number, Default: 0.6, Min: config.F(0), Max: config.F(1),
			Help: "Below this the local rules decide that step."},
		{Key: "navigation.max_steps", Label: "Max steps per trip", Type: config.Number, Default: 30.0, Min: config.F(1), Max: config.F(60)},
	}
}

// Step is one entry of the trip log (and of the state the planner sees).
type Step struct {
	Action     string  `json:"action"`
	By         string  `json:"by"` // jev, local
	Confidence float64 `json:"confidence,omitempty"`
	Result     string  `json:"result"`
	Reason     string  `json:"reason,omitempty"`
}

// ScanInfo is the last head sweep, relative to the body when it was taken.
type ScanInfo struct {
	Points     []motion.Result `json:"points"`
	ClearestDg float64         `json:"clearest_deg"`
	ClearestM  float64         `json:"clearest_m"`
	ClosestDg  float64         `json:"closest_deg"`
	ClosestM   float64         `json:"closest_m"`
	StepsAgo   int             `json:"steps_ago"`
}

// State is what a planner decides on.
type State struct {
	Goal      string             `json:"goal"`
	Waypoint  floorplan.Waypoint `json:"waypoint"`
	AheadM    *float64           `json:"ahead_m"` // forward distance; null = nothing within range or no sensor
	LastScan  *ScanInfo          `json:"last_scan"`
	History   []Step             `json:"history"` // last 5, oldest first
	Blocked   int                `json:"blocked_count"`
	StepIndex int                `json:"step"`
	MaxSteps  int                `json:"max_steps"`
}

type Decision struct {
	Action        string             `json:"action"`
	Confidence    float64            `json:"confidence"`
	By            string             `json:"by"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

type Planner interface {
	Decide(ctx context.Context, st State) (Decision, error)
}

// Robot is the part of motion.Service the navigator drives (a fake in tests).
type Robot interface {
	Move(meters float64) motion.Result
	Turn(degrees float64) motion.Result
	Scan(rangeDeg, stepDeg float64) motion.Result
	Waypoint(target string) (floorplan.Waypoint, error)
	Ahead() (float64, bool)
}

type Navigator struct {
	Cfg     *config.Store
	Robot   Robot
	Local   Planner
	JEV     *JEV
	Publish func(typ string, data any)
}

func New(cfg *config.Store, r Robot) *Navigator {
	return &Navigator{Cfg: cfg, Robot: r, Local: Local{}, JEV: NewJEV()}
}

func (n *Navigator) publish(d any) {
	if n.Publish != nil {
		n.Publish("nav", d)
	}
}

// decide asks JEV (if configured) and falls back to the local rules.
func (n *Navigator) decide(ctx context.Context, st State) Decision {
	mode := n.Cfg.String("navigation.planner")
	key := n.Cfg.Secret("navigation.jev_api_key")
	if mode != "local" && key != "" && n.JEV != nil {
		d, err := n.JEV.Decide(ctx, n.Cfg, st)
		if err == nil && d.Confidence >= n.Cfg.Float("navigation.min_confidence") {
			return d
		}
		if err != nil {
			slog.Warn("jev", "err", err)
		}
		loc, _ := n.Local.Decide(ctx, st)
		if err == nil {
			loc.By = fmt.Sprintf("local (jev unsure: %s %.2f)", d.Action, d.Confidence)
		} else {
			loc.By = "local (jev failed)"
		}
		return loc
	}
	d, _ := n.Local.Decide(ctx, st)
	return d
}

// Go drives to target. It returns when it arrives, gives up, runs out of steps,
// or a move is cancelled, overridden by the gamepad or fails.
func (n *Navigator) Go(ctx context.Context, target string) motion.Result {
	maxSteps := int(n.Cfg.Float("navigation.max_steps"))
	if maxSteps <= 0 {
		maxSteps = 30
	}
	st := State{Goal: target, MaxSteps: maxSteps}
	log := []Step{}
	t0 := time.Now()
	finish := func(result, reason string) motion.Result {
		out := motion.Result{"result": result, "steps": log, "seconds": math.Round(time.Since(t0).Seconds()*10) / 10}
		if reason != "" {
			out["reason"] = reason
		}
		if wp, err := n.Robot.Waypoint(target); err == nil {
			out["waypoint"] = wp
		}
		n.publish(map[string]any{"target": target, "done": result, "reason": reason, "steps": len(log)})
		return out
	}
	for st.StepIndex = 0; st.StepIndex < maxSteps; st.StepIndex++ {
		if ctx.Err() != nil {
			return finish("cancelled", ctx.Err().Error())
		}
		wp, err := n.Robot.Waypoint(target)
		if err != nil {
			return finish("error", err.Error())
		}
		st.Waypoint = wp
		st.AheadM = nil
		if d, ok := n.Robot.Ahead(); ok {
			st.AheadM = &d
		}
		if st.LastScan != nil {
			st.LastScan.StepsAgo++
		}
		st.History = lastN(log, 5)

		// Arriving is a fact, not a guess: never leave it to the planner.
		if arrived(st) {
			log = append(log, Step{Action: Arrived, By: "rule", Result: "done"})
			return finish("arrived", "")
		}

		d := n.decide(ctx, st)
		step := Step{Action: d.Action, By: d.By, Confidence: d.Confidence}
		res := n.do(d.Action, &st)
		step.Result, _ = res["result"].(string)
		step.Reason, _ = res["reason"].(string)
		log = append(log, step)
		n.publish(map[string]any{"target": target, "step": st.StepIndex + 1, "action": d.Action, "by": d.By,
			"confidence": d.Confidence, "result": step.Result, "reason": step.Reason, "waypoint": wp.Name, "distance_m": wp.DistanceM})

		switch step.Result {
		case "done":
		case "blocked":
			st.Blocked++
		case "arrived":
			return finish("arrived", "")
		case "give_up":
			return finish("gave_up", step.Reason)
		default: // manual_override, cancelled, timeout, error: stop the trip
			return finish(step.Result, step.Reason)
		}
	}
	return finish("gave_up", fmt.Sprintf("not there after %d steps", maxSteps))
}

// arrived: within arrive_m of the target, or facing a piece of furniture that is
// right in front (the ToF sees the target itself and the robot cannot get closer).
func arrived(st State) bool {
	wp := st.Waypoint
	if !wp.Final {
		return false
	}
	if wp.DistanceM <= wp.ArriveM {
		return true
	}
	return wp.Kind == "furniture" && math.Abs(wp.TurnDeg) < 20 && st.AheadM != nil && *st.AheadM < 0.5 &&
		wp.DistanceM <= wp.ArriveM+0.6
}

func lastN(s []Step, n int) []Step {
	if len(s) > n {
		s = s[len(s)-n:]
	}
	return append([]Step{}, s...)
}

// do runs one action and returns a motion-style result.
func (n *Navigator) do(action string, st *State) motion.Result {
	wp := st.Waypoint
	switch action {
	case TurnToWaypoint:
		if math.Abs(wp.TurnDeg) < 3 {
			return motion.Result{"result": "done", "degrees": 0}
		}
		return n.Robot.Turn(wp.TurnDeg)
	case Forward:
		m := 0.6
		if rest := wp.DistanceM - wp.ArriveM + 0.05; rest < m {
			m = math.Max(rest, 0.1)
		}
		if st.AheadM != nil && *st.AheadM-motion.StopM-0.05 < m {
			m = math.Max(*st.AheadM-motion.StopM-0.05, 0.1)
		}
		return n.Robot.Move(m)
	case ForwardShort:
		return n.Robot.Move(0.25)
	case TurnToClearest:
		if st.LastScan == nil {
			return n.scan(st)
		}
		deg := st.LastScan.ClearestDg
		st.LastScan = nil // the scan is relative to the old heading
		if math.Abs(deg) < 3 {
			return motion.Result{"result": "done", "degrees": 0}
		}
		return n.Robot.Turn(deg)
	case TurnLeft:
		st.LastScan = nil
		return n.Robot.Turn(-45)
	case TurnRight:
		st.LastScan = nil
		return n.Robot.Turn(45)
	case BackUp:
		return n.Robot.Move(-0.2)
	case Scan:
		return n.scan(st)
	case Arrived:
		if arrived(*st) || (wp.Final && wp.DistanceM <= wp.ArriveM+0.15) {
			return motion.Result{"result": "arrived"}
		}
		return motion.Result{"result": "done", "reason": "not there yet: the waypoint is still " + fmt.Sprint(wp.DistanceM) + " m away"}
	case GiveUp:
		return motion.Result{"result": "give_up", "reason": "the planner gave up"}
	}
	return motion.Result{"result": "error", "reason": "unknown action " + action}
}

func (n *Navigator) scan(st *State) motion.Result {
	r := n.Robot.Scan(90, 30)
	if r["result"] != "done" {
		return r
	}
	si := &ScanInfo{}
	si.Points, _ = r["points"].([]motion.Result)
	si.ClearestDg, _ = r["clearest_deg"].(float64)
	si.ClearestM, _ = r["clearest_m"].(float64)
	si.ClosestDg, _ = r["closest_deg"].(float64)
	si.ClosestM, _ = r["closest_m"].(float64)
	st.LastScan = si
	return motion.Result{"result": "done"}
}
