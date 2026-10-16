package floorplan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHouseRouteAndPosition(t *testing.T) {
	src, err := os.ReadFile("../../../partes/3-base-andando/mapas/ejemplo_casa.json")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	plan := filepath.Join(dir, "casa.json")
	os.WriteFile(plan, src, 0o644)
	m := New(plan, filepath.Join(dir, "pose.json"))
	d, err := m.Describe(Odo{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if d["robot"].(map[string]any)["room"] != "sala" {
		t.Fatal(d["robot"])
	}
	var estudio map[string]any
	for _, r := range d["rooms"].([]any) {
		if r.(map[string]any)["name"] == "estudio" {
			estudio = r.(map[string]any)
		}
	}
	if rt, _ := estudio["route"].([]string); strings.Join(rt, ",") != "sala↔cocina,cocina↔estudio" {
		t.Fatal(estudio["route"])
	}
	if err := m.Apply(Change{Action: "close_door", Name: "sala-cocina"}); err != nil {
		t.Fatal(err)
	}
	d, _ = m.Describe(Odo{}, 0)
	for _, r := range d["rooms"].([]any) {
		if r.(map[string]any)["name"] == "estudio" && r.(map[string]any)["route"] != "no path through open doors" {
			t.Fatal("closed door still routed")
		}
	}
	if err := m.SetPosition(99, 1, nil, Odo{}, 0); err == nil {
		t.Fatal("outside the house accepted")
	}
	h := 90.0
	if err := m.SetPosition(6, 2, &h, Odo{X: 1, Y: 0, Heading: 0}, 3); err != nil {
		t.Fatal(err)
	}
	r := m.describeRobot(t, Odo{X: 1, Y: 0, Heading: 0})
	if r["x"] != 6.0 || r["y"] != 2.0 || r["heading_deg"] != 90.0 || r["room"] != "cocina" {
		t.Fatal(r)
	}
	if _, err := os.Stat(plan + ".bak"); err != nil {
		t.Fatal("no .bak after a change")
	}
}

func (m *Map) describeRobot(t *testing.T, o Odo) map[string]any {
	d, err := m.Describe(o, 0)
	if err != nil {
		t.Fatal(err)
	}
	return d["robot"].(map[string]any)
}
