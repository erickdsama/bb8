package head

import (
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
