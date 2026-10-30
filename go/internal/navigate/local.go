package navigate

import (
	"context"
	"math"
)

// Local is the rule planner: always available, deterministic, a bit dumb.
type Local struct{}

func (Local) Decide(_ context.Context, st State) (Decision, error) {
	d := func(a string) (Decision, error) { return Decision{Action: a, Confidence: 1, By: "local"}, nil }
	wp := st.Waypoint
	var last Step
	if len(st.History) > 0 {
		last = st.History[len(st.History)-1]
	}
	switch {
	case wp.Final && wp.DistanceM <= wp.ArriveM:
		return d(Arrived)
	case st.Blocked >= 6:
		return d(GiveUp)
	case last.Result == "blocked" && last.Action == TurnToClearest:
		return d(BackUp) // turned to the open side and still blocked: make room
	case last.Result == "blocked":
		if st.LastScan == nil || st.LastScan.StepsAgo > 0 {
			return d(Scan)
		}
		return d(TurnToClearest)
	case last.Action == Scan && st.LastScan != nil && st.AheadM != nil && *st.AheadM < 0.45:
		return d(TurnToClearest) // looked around because it was tight ahead: go the open way
	case math.Abs(wp.TurnDeg) > 12 && !(last.Action == TurnToClearest && last.Result == "done"):
		return d(TurnToWaypoint)
	case st.AheadM != nil && *st.AheadM < 0.45:
		if st.LastScan == nil || st.LastScan.StepsAgo > 1 {
			return d(Scan)
		}
		return d(TurnToClearest)
	case last.Action == TurnToClearest && last.Result == "done":
		return d(ForwardShort) // get around it before facing the waypoint again
	}
	return d(Forward)
}
