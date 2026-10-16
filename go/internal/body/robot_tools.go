package body

import (
	"context"
	"errors"

	"github.com/erickdsama/bb8/go/internal/floorplan"
	"github.com/erickdsama/bb8/go/internal/motion"
	"github.com/erickdsama/bb8/go/internal/persona"
	. "github.com/erickdsama/bb8/go/internal/tools"
)

// RobotTools are the tools that drive the body and the head.
func RobotTools(m *motion.Service, h *Head) []Tool {
	type deg struct {
		Degrees float64 `json:"degrees"`
	}
	emotions := persona.EmotionNames()
	return []Tool{
		{Name: "move", Moves: true,
			Description: "Move forward (positive) or back (negative) in a straight line, -2 to 2 meters; stops by itself.\n\n" +
				"Returns result: done (with real meters), blocked (obstacle; includes distance_m), manual_override, " +
				"cancelled, timeout or error. Look with take_photo before moving more than 1 m.",
			Schema: Object(map[string]any{"meters": Num("meters, -2 to 2")}, "meters"),
			Handler: Bind(func(_ context.Context, in struct {
				Meters float64 `json:"meters"`
			}) (Output, error) {
				return Data(m.Move(in.Meters))
			})},
		{Name: "turn", Moves: true,
			Description: "Rotate the whole body in place. Positive = right, negative = left (-360 to 360).",
			Schema:      Object(map[string]any{"degrees": Num("degrees, + right")}, "degrees"),
			Handler:     Bind(func(_ context.Context, in deg) (Output, error) { return Data(m.Turn(in.Degrees)) })},
		{Name: "stop",
			Description: "Brake now and cancel any movement running or queued.",
			Schema:      Object(map[string]any{}),
			Handler:     Bind(func(_ context.Context, _ struct{}) (Output, error) { return Data(m.Stop()) })},
		{Name: "look_at", Moves: true,
			Description: "Turn only the head relative to the body: 0 forward, positive right, -90 to 90.",
			Schema:      Object(map[string]any{"degrees": Num("degrees, + right")}, "degrees"),
			Handler:     Bind(func(_ context.Context, in deg) (Output, error) { return Data(m.LookAt(in.Degrees)) })},
		{Name: "take_photo",
			Description: "Take a photo (640 px) with the head camera, in the direction the head is looking.",
			Schema:      Object(map[string]any{}),
			Handler: Bind(func(ctx context.Context, _ struct{}) (Output, error) {
				jpg, err := h.Photo(ctx, 640)
				if err != nil {
					return Output{}, err
				}
				return Output{Image: jpg}, nil
			})},
		{Name: "set_eye_color",
			Description: "Change the eye (NeoPixel ring). color: #RRGGBB or red, green, blue, white, orange, yellow, " +
				"purple, cyan. Use it to show state: blue calm, green happy, orange thinking, red alert.",
			Schema: Object(map[string]any{"color": Str("name or #RRGGBB"),
				"pattern": Enum("default steady", persona.Patterns...)}, "color"),
			Handler: Bind(func(ctx context.Context, in struct {
				Color   string `json:"color"`
				Pattern string `json:"pattern"`
			}) (Output, error) {
				return Data(h.Eye(ctx, in.Color, in.Pattern, 0))
			})},
		{Name: "say",
			Description: "Speak through the head speaker (short Spanish text, under 15 words) or make a droid beep.\n\n" +
				"With text, BB-8 babbles a few beeps and then says it; with text and sound, it beeps that sound first. " +
				"To react with the eye too, use express.",
			Schema: Object(map[string]any{"text": Str("what to say, in Spanish"), "sound": Enum("beep (an emotion)", emotions...)}),
			Handler: Bind(func(ctx context.Context, in struct {
				Text  string `json:"text"`
				Sound string `json:"sound"`
			}) (Output, error) {
				return Data(h.Say(ctx, in.Text, in.Sound))
			})},
		{Name: "express",
			Description: "Show an emotion: that emotion's droid beep and its eye color. Use it to react before " +
				"speaking or moving (see Personality in the instructions).",
			Schema: Object(map[string]any{"emotion": Enum("emotion", emotions...)}, "emotion"),
			Handler: Bind(func(ctx context.Context, in struct {
				Emotion string `json:"emotion"`
			}) (Output, error) {
				e, ok := persona.Emotions[in.Emotion]
				if !ok {
					return Output{}, errors.New("unknown emotion " + in.Emotion)
				}
				eye := h.Eye(ctx, e.Color, e.Pattern, 0)
				if eye["result"] != "done" {
					return Data(eye)
				}
				r := h.Say(ctx, "", in.Emotion)
				r["emotion"] = in.Emotion
				r["eye"] = e.Color + " " + e.Pattern
				return Data(r)
			})},
		{Name: "listen",
			Description: "The last thing the body microphone (ReSpeaker 2-Mic) heard and where the voice came from.\n\n" +
				"wait_s (0 to 10): if above 0, waits for someone to speak before answering. Returns heard (false if " +
				"nothing yet), text, level_db (60+ near, < 40 far or behind a wall), ago_s, angle_deg (−90 to 90, + right, " +
				"relative to the front you had when hearing it) and candidates_deg: the two possible headings relative to " +
				"your current front, already corrected by how much you turned since (turned_since_deg). With two mics " +
				"you cannot tell front (the first) from back (the second).",
			Schema: Object(map[string]any{"wait_s": Num("seconds to wait, 0 to 10")}),
			Handler: Bind(func(_ context.Context, in struct {
				Wait float64 `json:"wait_s"`
			}) (Output, error) {
				return Data(m.Listen(in.Wait))
			})},
		{Name: "get_map",
			Description: "House plan and where you are on it.\n\nAxes: x east (right of the sketch), y north (up), in meters. " +
				"robot has x, y, heading_deg (clockwise from east: 0 east, 90 south, -90 north, 180 west) and the room " +
				"(or at_door if you are in a doorway). position says if it is uncertain and how far you rolled since the " +
				"last fix. Every room, door and piece of furniture has distance_m, turn_deg (what you would turn with turn, " +
				"+ right, to face it) and direct (true if no wall or closed door is in between). Every room has route: the " +
				"doors to cross to get there. sketch is a drawing, north up, one char per 25 cm: # wall, m furniture, . floor, " +
				"= closed door, the arrow is you.",
			Schema:  Object(map[string]any{}),
			Handler: Bind(func(_ context.Context, _ struct{}) (Output, error) { return Data(m.GetMap()) })},
		{Name: "set_position",
			Description: "Fix where you think you are on the plan (does not move the robot).\n\nx, y in meters with the " +
				"get_map axes; heading_deg clockwise from east, or omit it to keep the heading. Use it when a photo tells " +
				"you that you are not where get_map thinks.",
			Schema: Object(map[string]any{"x": Num("meters east"), "y": Num("meters north"),
				"heading_deg": Num("clockwise from east")}, "x", "y"),
			Handler: Bind(func(_ context.Context, in struct {
				X       float64  `json:"x"`
				Y       float64  `json:"y"`
				Heading *float64 `json:"heading_deg"`
			}) (Output, error) {
				return Data(m.SetPosition(in.X, in.Y, in.Heading))
			})},
		{Name: "update_map",
			Description: "Change the house plan and save it (does not move the robot).\n\nadd_furniture: new name, x, y of " +
				"the center, width_m (east-west), length_m (north-south) and height_m. move_furniture: name and the new " +
				"center x, y. remove_furniture: name. close_door / open_door: name \"living-kitchen\" (the two rooms). " +
				"Only use it with what you confirmed with take_photo.",
			Schema: Object(map[string]any{"action": Enum("what to change", floorplan.Actions...), "name": Str("furniture or door"),
				"x": Num("center x"), "y": Num("center y"), "width_m": Num("east-west"), "length_m": Num("north-south"),
				"height_m": Num("height")}, "action", "name"),
			Handler: Bind(func(_ context.Context, in floorplan.Change) (Output, error) { return Data(m.ChangeMap(in)) })},
		{Name: "get_pose",
			Description: "Robot state: estimated position (x, y in m; y positive to the right), heading, tilt, battery, " +
				"head angle, forward ToF distance, who controls (llm, manual, idle) and whether the head is connected.",
			Schema: Object(map[string]any{}),
			Handler: Bind(func(_ context.Context, _ struct{}) (Output, error) {
				p := m.Pose()
				p["head"] = h.State()
				return Data(p)
			})},
	}
}
