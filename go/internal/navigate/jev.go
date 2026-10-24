package navigate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
)

// JEV calls TypeSafe's System One endpoint with one choice question.
// https://docs.typesafe.ai/primitives/choice
type JEV struct {
	URL  string
	HTTP *http.Client
}

func NewJEV() *JEV {
	return &JEV{URL: "https://api.typesafe.ai/v1/systemone", HTTP: &http.Client{Timeout: 3 * time.Second}}
}

const instructions = "You drive BB-8, a 30 cm spherical home robot, toward the goal one short step at a time. " +
	"Pick the next action. waypoint gives turn_deg (+ right) and distance_m to the next point of the route; " +
	"ahead_m is the free distance straight ahead (null = clear or no reading); the robot brakes by itself at 0.25 m. " +
	"history holds the last actions and their results (blocked means an obstacle stopped it)."

type jevReq struct {
	Model     string                 `json:"model"`
	State     string                 `json:"state"`
	Questions map[string]jevQuestion `json:"questions"`
}

type jevQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

type jevResp struct {
	Model   string `json:"model"`
	Answers map[string]struct {
		Choice        string             `json:"choice"`
		Confidence    float64            `json:"confidence"`
		Probabilities map[string]float64 `json:"probabilities"`
	} `json:"answers"`
	Error any `json:"error"`
}

func (j *JEV) Decide(ctx context.Context, cfg *config.Store, st State) (Decision, error) {
	state, _ := json.Marshal(st)
	model := cfg.String("navigation.jev_model")
	if model == "" {
		model = "jev-latest"
	}
	body, _ := json.Marshal(jevReq{Model: model, State: string(state), Questions: map[string]jevQuestion{
		"next_action": {Type: "choice", Instructions: instructions, Criteria: ActionHelp},
	}})
	req, _ := http.NewRequestWithContext(ctx, "POST", j.URL, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cfg.Secret("navigation.jev_api_key"))
	req.Header.Set("Content-Type", "application/json")
	res, err := j.HTTP.Do(req)
	if err != nil {
		return Decision{}, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != 200 {
		return Decision{}, fmt.Errorf("jev HTTP %d: %.200s", res.StatusCode, b)
	}
	var out jevResp
	if err := json.Unmarshal(b, &out); err != nil {
		return Decision{}, err
	}
	a, ok := out.Answers["next_action"]
	if !ok {
		return Decision{}, errors.New("jev: no next_action answer")
	}
	if _, valid := ActionHelp[a.Choice]; !valid {
		return Decision{}, fmt.Errorf("jev: unknown action %q", a.Choice)
	}
	return Decision{Action: a.Choice, Confidence: a.Confidence, By: "jev", Probabilities: a.Probabilities}, nil
}
