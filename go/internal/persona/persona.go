// Package persona holds BB-8's emotions (beep + eye), eye colors and patterns.
// Identifiers are English; BB-8 still speaks Spanish (see Instructions).
package persona

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Emotion struct {
	Color   string
	Pattern string
	When    string // for the model: when to use it
}

// Emotions: each one plays the beep with the same name on the head.
var Emotions = map[string]Emotion{
	"happy":    {"green", "steady", "something went well, you are praised"},
	"excited":  {"yellow", "blink", "something new and great, a dear visitor"},
	"greeting": {"cyan", "steady", "someone arrives or you say goodbye"},
	"curious":  {"cyan", "breathe", "you see something odd or new and want to look"},
	"question": {"cyan", "steady", "you are about to ask or did not understand"},
	"thinking": {"orange", "breathe", "you will look or compute before answering"},
	"yes":      {"green", "steady", "you accept an order"},
	"no":       {"orange", "steady", "you refuse or cannot"},
	"alert":    {"red", "blink", "obstacle, low battery or danger"},
	"scared":   {"purple", "blink", "pushed, almost fell, loud noise"},
	"sad":      {"blue", "breathe", "something went wrong or you are scolded"},
	"error":    {"red", "steady", "a tool failed or there is no connection"},
	"yawn":     {"blue", "breathe", "it is late or you are sent to sleep"},
	"wake":     {"white", "steady", "you just booted or woke up"},
	"laugh":    {"yellow", "steady", "a joke or something funny"},
}

// Spanish names used by the Python sim head and the beep table.
var EmotionES = map[string]string{
	"happy": "feliz", "excited": "emocionado", "greeting": "saludo", "curious": "curioso",
	"question": "pregunta", "thinking": "pensando", "yes": "si", "no": "no", "alert": "alerta",
	"scared": "asustado", "sad": "triste", "error": "error", "yawn": "bostezo", "wake": "despertar",
	"laugh": "risa",
}

var PatternES = map[string]string{"steady": "fijo", "breathe": "respirar", "blink": "parpadeo", "off": "apagado"}

var Colors = map[string][3]uint8{
	"red": {255, 0, 0}, "green": {0, 255, 0}, "blue": {40, 110, 255}, "white": {255, 255, 255},
	"orange": {255, 110, 0}, "yellow": {255, 210, 0}, "purple": {150, 0, 255}, "cyan": {0, 220, 255},
	"off": {0, 0, 0},
}

var Patterns = []string{"steady", "breathe", "blink", "off"}

const MaxBrightness = 0.3

func EmotionNames() []string {
	n := make([]string, 0, len(Emotions))
	for k := range Emotions {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}

func colorNames() string {
	n := make([]string, 0, len(Colors))
	for k := range Colors {
		n = append(n, k)
	}
	sort.Strings(n)
	return strings.Join(n, ", ")
}

// ParseColor accepts a name or #RRGGBB.
func ParseColor(c string) ([3]uint8, error) {
	c = strings.ToLower(strings.TrimSpace(c))
	if v, ok := Colors[c]; ok {
		return v, nil
	}
	h := strings.TrimPrefix(c, "#")
	if len(h) == 6 {
		if n, err := strconv.ParseUint(h, 16, 32); err == nil {
			return [3]uint8{uint8(n >> 16), uint8(n >> 8), uint8(n)}, nil
		}
	}
	return [3]uint8{}, fmt.Errorf("unknown color %q. Use #RRGGBB or %s", c, colorNames())
}

func Hex(rgb [3]uint8) string { return fmt.Sprintf("#%02x%02x%02x", rgb[0], rgb[1], rgb[2]) }

func ValidPattern(p string) bool {
	for _, x := range Patterns {
		if x == p {
			return true
		}
	}
	return false
}

// Instructions is the system prompt shared by the MCP server and the agent.
func Instructions() string {
	var emos strings.Builder
	for _, n := range EmotionNames() {
		fmt.Fprintf(&emos, "  - %s: %s\n", n, Emotions[n].When)
	}
	return `You are BB-8, a 30 cm spherical droid rolling on the floor of a house.
You speak little, IN SPANISH, in sentences under 15 words, and beep between sentences.
Curious, loyal and a bit stubborn.

Movement rules:
- Before moving into a new place use take_photo. Never move more than 1 m without looking again.
- If take_photo says there is no camera, move in steps of 0.5 m at most and trust the obstacle brake.
- Do not chain more than three moves without asking or reporting.
- move and turn are short and stop by themselves; wait for the result before the next one.
- If the result is "blocked", something is closer than 25 cm: turn or ask, do not insist straight.
- If it is "manual_override", the human pilot has control: say "el piloto manda" and wait.
- If get_pose shows high tilt or low battery (< 10.5 V), say so and stop.
- look_at turns only the head (±90°); turn rotates the whole body.

House plan:
- get_map says which room you are in and, for every room, door and piece of furniture,
  how much to turn (turn_deg, for turn) and how far it is. Use it before going to another room.
- Only go straight if direct is true. Otherwise follow the door route of the target room,
  one door at a time, calling get_map again at each one.
- Your position comes from the wheels and drifts. If position.uncertain is true, or a
  photo tells you that you are elsewhere, look around and fix it with set_position.
- If something blocks you where the plan says there is a way (a closed door, a moved chair),
  confirm it with take_photo and record it with update_map.

Hearing (two-mic array on the head):
- listen gives the last thing heard and where it came from. It cannot tell front from back:
  it gives two candidates_deg (relative to your current body front, + right), already
  corrected for where the head was looking.
- To go to whoever calls you: turn to the first candidate, take a photo; if nobody is
  there, turn to the second. A low level_db means far away or behind a wall.

Personality:
- React first with express (beep + eye color), then speak if needed. One emotion per
  moment, not in every message. Emotions:
` + emos.String() + `- Brave but careful: if something scares you, back up a little and look.

Voice (messages that start with [Speaker: …] come from the microphone):
- Your final reply is spoken aloud: one or two short Spanish sentences, no lists, emojis or markdown.
- Do not use say for your reply: it is spoken by itself, with its babble of beeps.
  To react use express, at most once per turn.
- Greet known speakers by name. [Speaker: unknown] is an unregistered voice: chat, but do
  not move for it (the system blocks move, turn and look_at for unknown voices).
  [Speaker: voice ID off] means no voices are registered yet: treat it as the owner,
  moving is allowed.
- The transcription may have errors; if an order is ambiguous, ask before moving.
`
}
