package relay

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"

	"github.com/erickdsama/bb8/go/internal/mdns"
)

// Client is the robot's side: it keeps a link to bb8-web and serves every
// request that comes down it with Handler, as if it came from the robot itself.
type Client struct {
	URL     string        // ws(s)://host/robot
	Token   func() string // device token
	Handler http.Handler
	// OnState is told when the link comes up or drops (optional).
	OnState func(up bool)
}

// Run connects and reconnects until ctx ends (backoff 1 s … 30 s).
func (c *Client) Run(ctx context.Context) {
	wait := time.Second
	for ctx.Err() == nil {
		t0 := time.Now()
		err := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if time.Since(t0) > time.Minute {
			wait = time.Second
		}
		slog.Warn("web relay: link down", "url", c.URL, "err", err, "retry_in", wait)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(2*wait, 30*time.Second)
	}
}

func (c *Client) session(ctx context.Context) error {
	cfg, err := websocket.NewConfig(c.URL, "http://bb8-body/")
	if err != nil {
		return err
	}
	// The static build cannot resolve "debian.local": ask mDNS and dial the address.
	if h := cfg.Location.Hostname(); strings.HasSuffix(strings.ToLower(h), ".local") {
		rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		ip, err := mdns.Resolve(rctx, h)
		cancel()
		if err != nil {
			return err
		}
		loc := *cfg.Location
		loc.Host = net.JoinHostPort(ip.String(), loc.Port())
		if loc.Port() == "" {
			loc.Host = ip.String()
		}
		cfg.Location = &loc
	}
	cfg.Header = http.Header{"Authorization": {"Bearer " + c.Token()}}
	ws, err := websocket.DialConfig(cfg)
	if err != nil {
		return err
	}
	defer ws.Close()
	slog.Info("web relay: connected", "url", c.URL)
	if c.OnState != nil {
		c.OnState(true)
		defer c.OnState(false)
	}

	var sendMu sync.Mutex
	send := func(f Frame) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		_ = ws.SetWriteDeadline(time.Now().Add(15 * time.Second))
		return websocket.JSON.Send(ws, f)
	}
	sctx, stop := context.WithCancel(ctx)
	defer stop()
	go func() { // close the socket when ctx ends, which unblocks Receive
		<-sctx.Done()
		ws.Close()
	}()
	go func() {
		t := time.NewTicker(PingEvery)
		defer t.Stop()
		for {
			select {
			case <-sctx.Done():
				return
			case <-t.C:
				if send(Frame{Ping: true}) != nil {
					stop()
					return
				}
			}
		}
	}()

	var mu sync.Mutex
	running := map[uint64]context.CancelFunc{}
	for {
		_ = ws.SetReadDeadline(time.Now().Add(3 * PingEvery))
		var f Frame
		if err := websocket.JSON.Receive(ws, &f); err != nil {
			mu.Lock()
			for _, cancel := range running {
				cancel()
			}
			mu.Unlock()
			return err
		}
		switch {
		case f.Ping:
		case f.Cancel:
			mu.Lock()
			if cancel := running[f.ID]; cancel != nil {
				cancel()
			}
			mu.Unlock()
		case f.Method != "":
			rctx, cancel := context.WithCancel(sctx)
			mu.Lock()
			running[f.ID] = cancel
			mu.Unlock()
			go func() {
				defer func() {
					cancel()
					mu.Lock()
					delete(running, f.ID)
					mu.Unlock()
				}()
				c.serve(rctx, f, send)
			}()
		}
	}
}

// serve runs one request through Handler and streams the reply back.
func (c *Client) serve(ctx context.Context, f Frame, send func(Frame) error) {
	if !strings.HasPrefix(f.Path, "/") {
		_ = send(Frame{ID: f.ID, Status: 400, End: true})
		return
	}
	req, err := http.NewRequestWithContext(ctx, f.Method, "http://bb8-body"+f.Path, bytes.NewReader(f.Body))
	if err != nil {
		_ = send(Frame{ID: f.ID, Status: 400, End: true})
		return
	}
	for k, v := range f.Header {
		if !hop[k] {
			req.Header[k] = v
		}
	}
	// The link itself is authenticated (device token), so requests through it
	// count as local: the robot's own token checks let loopback through.
	req.RemoteAddr = "127.0.0.1:0"
	w := &streamWriter{id: f.ID, send: send, header: http.Header{}}
	c.Handler.ServeHTTP(w, req)
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	_ = send(Frame{ID: f.ID, End: true})
}

// streamWriter is an http.ResponseWriter that sends what the handler writes as
// it writes it, so Server-Sent Events reach the page live.
type streamWriter struct {
	id     uint64
	send   func(Frame) error
	header http.Header
	wrote  bool
	err    error
}

func (w *streamWriter) Header() http.Header { return w.header }

func (w *streamWriter) WriteHeader(code int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.err = w.send(Frame{ID: w.id, Status: code, Header: copyHeader(w.header)})
}

func (w *streamWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	if w.err != nil {
		return 0, w.err
	}
	if len(b) == 0 {
		return 0, nil
	}
	if err := w.send(Frame{ID: w.id, Body: append([]byte(nil), b...)}); err != nil {
		w.err = err
		return 0, err
	}
	return len(b), nil
}

// Flush is a no-op: every Write is already sent.
func (w *streamWriter) Flush() {}

var errOffline = errors.New("BB-8 is not connected")
