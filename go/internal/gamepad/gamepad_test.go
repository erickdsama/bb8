//go:build linux

package gamepad

import "testing"

func TestMix(t *testing.T) {
	if l, r := Mix(0, 1, 150); l != 150 || r != 150 {
		t.Fatal(l, r)
	}
	if l, r := Mix(1, 0, 150); l != 150 || r != -150 {
		t.Fatal("turn right", l, r)
	}
	if l, r := Mix(0.03, 0.02, 150); l != 0 || r != 0 {
		t.Fatal("dead zone", l, r)
	}
	if l, _ := Mix(1, 1, 150); l != 150 {
		t.Fatal("limit", l)
	}
}
