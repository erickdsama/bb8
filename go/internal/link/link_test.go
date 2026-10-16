package link

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func startServer(t *testing.T, dir string) (*Server, string) {
	cfg, err := ServerTLS(dir)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go s.Serve(ctx, ln)
	return s, ln.Addr().String()
}

func TestRoundTripAndEvents(t *testing.T) {
	dir := t.TempDir()
	if err := Pair(dir, []string{"127.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	s, addr := startServer(t, dir)
	events := make(chan string, 4)
	s.OnEvent = func(_ *Peer, typ string, body json.RawMessage) { events <- typ + string(body) }

	ccfg, _ := ClientTLS(dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go Dial(ctx, addr, ccfg, func(_ context.Context, typ string, body json.RawMessage) (any, []byte, error) {
		return map[string]string{"got": typ}, []byte{0xff, 0xd8}, nil
	}, nil, func(ctx context.Context, p *Peer) { p.Notify("tof", map[string]int{"mm": 812}) })

	deadline := time.Now().Add(3 * time.Second)
	for !s.Connected() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	body, bin, err := s.Request(ctx, "photo", map[string]int{"width": 640})
	if err != nil || string(body) != `{"got":"photo"}` || len(bin) != 2 {
		t.Fatalf("request: %s %v %v", body, bin, err)
	}
	select {
	case e := <-events:
		if e != `tof{"mm":812}` {
			t.Fatal(e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no tof event")
	}
}

func TestRejectsForeignCert(t *testing.T) {
	dir, other := t.TempDir(), t.TempDir()
	if err := Pair(dir, nil); err != nil {
		t.Fatal(err)
	}
	if err := Pair(other, nil); err != nil {
		t.Fatal(err)
	}
	s, addr := startServer(t, dir)
	// A head cert from another CA, trusting our body CA so only the server check can fail.
	cfg, _ := ClientTLS(other)
	good, _ := ClientTLS(dir)
	cfg.RootCAs = good.RootCAs
	c, err := tls.Dial("tcp", addr, cfg)
	if err == nil {
		// TLS 1.3: the client may finish before the server rejects; the read fails.
		c.SetReadDeadline(time.Now().Add(time.Second))
		_, err = c.Read(make([]byte, 1))
		c.Close()
	}
	if err == nil {
		t.Fatal("foreign cert accepted")
	}
	time.Sleep(100 * time.Millisecond)
	if s.Connected() {
		t.Fatal("server registered a foreign head")
	}
	// Plain TCP without a cert also fails.
	raw, _ := net.Dial("tcp", addr)
	raw.Write([]byte("hello"))
	raw.Close()
	if s.Connected() {
		t.Fatal("plain tcp accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "ca.key")); err != nil {
		t.Fatal(err)
	}
}
