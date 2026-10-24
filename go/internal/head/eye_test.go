package head

import (
	"context"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"
)

// The ws2812-pwm eye streams 3*N-byte frames to the bb8-eye helper socket.
func TestHelperEyeFrames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "eye.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan []byte, 64)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		for {
			buf := make([]byte, 3*16)
			if _, err := io.ReadFull(c, buf); err != nil {
				return
			}
			got <- buf
		}
	}()
	e, err := newRingEye("ws2812-pwm", 16, path)
	if err != nil {
		t.Fatal(err)
	}
	e.set([3]uint8{0, 220, 255}, "steady", 0.3)
	deadline := time.After(3 * time.Second)
	for {
		select {
		case f := <-got:
			// cyan at 0.3: 0, 66, 76 on every LED
			if f[0] == 0 && f[1] == 66 && f[2] == 76 && f[45] == 0 && f[46] == 66 {
				return
			}
		case <-deadline:
			t.Fatal("no cyan frame")
		}
	}
}

type eyeRec struct {
	Backend
	got []float64
}

func (e *eyeRec) Eye(rgb [3]uint8, pattern string, b float64) error {
	e.got = append(e.got, b)
	return nil
}

func TestBrightnessAppliesNow(t *testing.T) {
	b := &eyeRec{}
	h := &Head{B: b}
	cfg := func(v float64) {
		h.onEvent(nil, "config", []byte(fmt.Sprintf(`{"rev":1,"values":{"eye_brightness":%g,"volume":0.5}}`, v)), nil)
	}
	cfg(0.1) // before any eye command: the boot color dims too
	h.handle(context.Background(), "eye", []byte(`{"rgb":[255,0,0],"pattern":"steady","brightness":0.3}`))
	cfg(0.25) // brighter: back up towards what was asked
	cfg(0.25) // same value: nothing to redo
	if fmt.Sprint(b.got) != "[0.1 0.1 0.25]" {
		t.Fatal(b.got)
	}
}
