package navigate

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/floorplan"
	"github.com/erickdsama/bb8/go/internal/motion"
)

// fakeRobot: the goal is goalM ahead after turning turnDeg; a box blocks the
// straight line at boxM until the robot side-steps it (turn_to_clearest).
type fakeRobot struct {
	heading, along, goalM, turnDeg, boxM float64
	dodged                               bool
	calls                                []string
}

func (r *fakeRobot) Waypoint(string) (floorplan.Waypoint, error) {
	return floorplan.Waypoint{Kind: "room", Name: "cocina", Final: true, ArriveM: 0.35,
		TurnDeg: math.Round(r.turnDeg - r.heading), DistanceM: math.Max(r.goalM-r.along, 0), Direct: true}, nil
}
func (r *fakeRobot) Ahead() (float64, bool) {
	if r.boxM > 0 && !r.dodged && math.Abs(r.turnDeg-r.heading) < 15 {
		return r.boxM - r.along, true
	}
	return 0, false
}
func (r *fakeRobot) Move(m float64) motion.Result {
	r.calls = append(r.calls, "move")
	if d, ok := r.Ahead(); ok && m > 0 && d-m < motion.StopM {
		r.along += d - motion.StopM
		return motion.Result{"result": "blocked"}
	}
	if math.Abs(r.turnDeg-r.heading) < 15 {
		r.along += m
	} else if m > 0 {
		r.dodged = true // went around the box
	}
	return motion.Result{"result": "done", "meters": m}
}
func (r *fakeRobot) Turn(d float64) motion.Result {
	r.calls = append(r.calls, "turn")
	r.heading += d
	return motion.Result{"result": "done", "degrees": d}
}
func (r *fakeRobot) Scan(float64, float64) motion.Result {
	r.calls = append(r.calls, "scan")
	return motion.Result{"result": "done", "points": []motion.Result{}, "clearest_deg": 60.0, "clearest_m": 2.0, "closest_deg": 0.0, "closest_m": 0.3}
}

func cfg(t *testing.T) *config.Store {
	c, _ := config.Open("")
	c.Register(Fields()...)
	return c
}

func TestLocalArrives(t *testing.T) {
	r := &fakeRobot{goalM: 2, turnDeg: 90}
	n := New(cfg(t), r)
	out := n.Go(context.Background(), "cocina")
	if out["result"] != "arrived" || r.calls[0] != "turn" {
		t.Fatal(out, r.calls)
	}
}

func TestLocalGoesAround(t *testing.T) {
	r := &fakeRobot{goalM: 2.5, turnDeg: 0, boxM: 1}
	n := New(cfg(t), r)
	var nav []map[string]any
	n.Publish = func(_ string, d any) { nav = append(nav, d.(map[string]any)) }
	out := n.Go(context.Background(), "cocina")
	calls := strings.Join(r.calls, ",")
	if out["result"] != "arrived" || !strings.Contains(calls, "scan") {
		t.Fatal(out, calls)
	}
	if len(nav) < 3 || nav[len(nav)-1]["done"] != "arrived" {
		t.Fatal("nav events", nav)
	}
}

func TestJEVWithFallback(t *testing.T) {
	answers := []string{`{"answers":{"next_action":{"choice":"turn_to_waypoint","confidence":0.92}}}`,
		`{"answers":{"next_action":{"choice":"forward","confidence":0.3}}}`} // unsure → local
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var req jevReq
		json.Unmarshal(b, &req)
		q := req.Questions["next_action"]
		if r.Header.Get("Authorization") != "Bearer ts-key" || q.Type != "choice" || len(q.Criteria) != len(ActionHelp) ||
			!strings.Contains(req.State, `"goal":"cocina"`) {
			t.Errorf("request %s", b)
		}
		w.Write([]byte(answers[min(i, len(answers)-1)]))
		i++
	}))
	defer srv.Close()
	c := cfg(t)
	c.Set(map[string]any{"navigation.jev_api_key": "ts-key"}, config.SourceWeb, nil)
	r := &fakeRobot{goalM: 1, turnDeg: -45}
	n := New(c, r)
	n.JEV.URL = srv.URL
	out := n.Go(context.Background(), "cocina")
	steps := out["steps"].([]Step)
	if out["result"] != "arrived" || steps[0].By != "jev" || !strings.HasPrefix(steps[1].By, "local (jev unsure") {
		t.Fatalf("%v %+v", out["result"], steps)
	}

	// JEV down: every step goes local and the trip still works.
	n.JEV.URL = "http://127.0.0.1:1"
	r2 := &fakeRobot{goalM: 1}
	n.Robot = r2
	if out := n.Go(context.Background(), "cocina"); out["result"] != "arrived" {
		t.Fatal(out)
	}
}

func TestStopsOnOverride(t *testing.T) {
	r := &override{fakeRobot{goalM: 3}}
	n := New(cfg(t), r)
	if out := n.Go(context.Background(), "cocina"); out["result"] != "manual_override" {
		t.Fatal(out)
	}
}

type override struct{ fakeRobot }

func (o *override) Move(float64) motion.Result { return motion.Result{"result": "manual_override"} }

func TestArrivedAtFurnitureInFront(t *testing.T) {
	ahead := 0.3
	st := State{Waypoint: floorplan.Waypoint{Kind: "furniture", Final: true, ArriveM: 0.5, DistanceM: 0.9, TurnDeg: 5}, AheadM: &ahead}
	if !arrived(st) {
		t.Fatal("the target is right in front")
	}
	st.Waypoint.Kind = "room"
	if arrived(st) {
		t.Fatal("a room is not reached by bumping into something")
	}
}
