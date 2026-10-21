package motion

import "testing"

func TestObstacleMemory(t *testing.T) {
	m := obstacleMemory{limit: 0.7}
	m.see(0.85, 0) // the floor: not remembered
	if _, near := m.tooClose(0.5); near {
		t.Fatal("floor reading remembered")
	}
	m.see(0.60, 0.10) // a shoe at 0.6 m
	m.see(0.90, 0.20) // it drops under the cone: the floor again
	if left, near := m.tooClose(0.40); near || left < 0.29 || left > 0.31 {
		t.Fatal("too early", left, near)
	}
	if left, near := m.tooClose(0.46); !near || left > StopM {
		t.Fatal("should brake 25 cm before the shoe", left, near)
	}
	off := obstacleMemory{}
	off.see(0.3, 0)
	if _, near := off.tooClose(1); near {
		t.Fatal("limit 0 must disable it")
	}
}
