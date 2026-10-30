package navigate

import (
	"context"

	"github.com/erickdsama/bb8/go/internal/tools"
)

// Tools: navigate_to for the agent. It moves, so unknown voices and guests cannot use it.
func (n *Navigator) Tools() []tools.Tool {
	return []tools.Tool{{
		Name: "navigate_to", Moves: true,
		Description: "Drive by yourself to a room or a piece of furniture of the house plan (names from get_map), " +
			"in short steps decided by the fast navigation planner (JEV or local rules): it turns, scans and goes around obstacles. " +
			"Takes from seconds to about a minute; stop cancels it. Returns result: arrived, gave_up (with reason), " +
			"manual_override, cancelled, timeout or error, plus the steps taken and the final waypoint.",
		Schema: tools.Object(map[string]any{"target": tools.Str("room or furniture name, as in get_map")}, "target"),
		Handler: tools.Bind(func(ctx context.Context, in struct {
			Target string `json:"target"`
		}) (tools.Output, error) {
			return tools.Data(n.Go(ctx, in.Target))
		}),
	}}
}
