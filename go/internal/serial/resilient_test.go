package serial

import (
	"bufio"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeUno answers like the firmware on a TCP socket: "I" → OK BB8, anything else → OK.
type fakeUno struct {
	ln   net.Listener
	mu   sync.Mutex
	conn []net.Conn
}

func startFakeUno(t *testing.T, addr string) *fakeUno {
	t.Helper()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	u := &fakeUno{ln: ln}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			u.mu.Lock()
			u.conn = append(u.conn, c)
			u.mu.Unlock()
			go func() {
				sc := bufio.NewScanner(c)
				for sc.Scan() {
					if strings.TrimSpace(sc.Text()) == "I" {
						c.Write([]byte("OK BB8 fw=test\n"))
					} else {
						c.Write([]byte("OK\n"))
					}
				}
			}()
		}
	}()
	return u
}

func (u *fakeUno) stop() {
	u.ln.Close()
	u.mu.Lock()
	for _, c := range u.conn {
		c.Close()
	}
	u.mu.Unlock()
}

func TestResilientLateStartAndReconnect(t *testing.T) {
	// A free port, with nothing listening yet: the Arduino is "not ready" at boot.
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	l.Close()

	r := NewResilient("socket://"+addr, 115200)
	defer r.Close()
	var ups []bool
	r.OnConnect = func(ident string, first bool) { ups = append(ups, first) }

	if _, err := r.Send("S"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("before the Arduino exists: %v", err)
	}
	if r.Connected() {
		t.Fatal("must not be connected")
	}

	uno := startFakeUno(t, addr)
	deadline := time.Now().Add(8 * time.Second) // the retry back-off is at most 5 s
	for {
		if _, err := r.Send("S"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("never connected after the Arduino appeared")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !r.Connected() || r.Ident() != "OK BB8 fw=test" || len(ups) != 1 || !ups[0] {
		t.Fatalf("connected=%v ident=%q ups=%v", r.Connected(), r.Ident(), ups)
	}

	// The link breaks (USB glitch): Sends fail, then it reconnects by itself.
	uno.stop()
	if _, err := r.Send("S"); err == nil {
		t.Fatal("send on a dead link must fail")
	}
	uno = startFakeUno(t, addr)
	defer uno.stop()
	deadline = time.Now().Add(8 * time.Second)
	for {
		if _, err := r.Send("S"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("never reconnected")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(ups) != 2 || ups[1] {
		t.Fatalf("OnConnect must run again with first=false: %v", ups)
	}
}
