// Package link is the secure head↔body channel: one mutual-TLS 1.3 connection,
// opened by the head, carrying length-prefixed JSON frames.
//
// Frame: [u32 big-endian length][JSON Msg]. If Msg.Bin is true the next frame is
// raw bytes (a photo). A Msg with an ID and no Reply is a request; the answer
// has the same ID and Reply=true. A Msg with ID 0 is an event (tof, heartbeat…).
package link

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	MaxFrame       = 8 << 20
	HeartbeatEvery = time.Second
	DeadAfter      = 1500 * time.Millisecond
)

var ErrDisconnected = errors.New("head disconnected")

type Msg struct {
	ID    uint64          `json:"id,omitempty"`
	Type  string          `json:"type"`
	Reply bool            `json:"reply,omitempty"`
	Body  json.RawMessage `json:"body,omitempty"`
	Err   string          `json:"err,omitempty"`
	Bin   bool            `json:"bin,omitempty"`
}

// Handler answers a request. bin is optional binary payload.
type Handler func(ctx context.Context, typ string, body json.RawMessage) (resp any, bin []byte, err error)

// EventFunc receives events (ID 0). bin is the binary frame that followed, if any.
type EventFunc func(p *Peer, typ string, body json.RawMessage, bin []byte)

type reply struct {
	msg Msg
	bin []byte
}

// Peer is one end of an established connection.
type Peer struct {
	conn     net.Conn
	wmu      sync.Mutex
	pmu      sync.Mutex
	pending  map[uint64]chan reply
	nextID   atomic.Uint64
	lastSeen atomic.Int64
	handler  Handler
	onEvent  EventFunc
	done     chan struct{}
	once     sync.Once
}

func newPeer(c net.Conn, h Handler, ev EventFunc) *Peer {
	p := &Peer{conn: c, pending: map[uint64]chan reply{}, handler: h, onEvent: ev, done: make(chan struct{})}
	p.lastSeen.Store(time.Now().UnixNano())
	return p
}

func writeFrame(w io.Writer, b []byte) error {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(b)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

func readFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > MaxFrame {
		return nil, fmt.Errorf("frame too big: %d", n)
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, err
}

func (p *Peer) write(m Msg, bin []byte) error {
	m.Bin = bin != nil
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	p.wmu.Lock()
	defer p.wmu.Unlock()
	_ = p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := writeFrame(p.conn, b); err != nil {
		return err
	}
	if bin != nil {
		return writeFrame(p.conn, bin)
	}
	return nil
}

// Notify sends an event.
func (p *Peer) Notify(typ string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return p.write(Msg{Type: typ, Body: b}, nil)
}

// NotifyBin sends an event followed by a binary frame (audio chunks).
func (p *Peer) NotifyBin(typ string, body any, bin []byte) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	if bin == nil {
		bin = []byte{}
	}
	return p.write(Msg{Type: typ, Body: b}, bin)
}

// Request sends a request and waits for the reply.
func (p *Peer) Request(ctx context.Context, typ string, body any) (json.RawMessage, []byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	id := p.nextID.Add(1)
	ch := make(chan reply, 1)
	p.pmu.Lock()
	p.pending[id] = ch
	p.pmu.Unlock()
	defer func() { p.pmu.Lock(); delete(p.pending, id); p.pmu.Unlock() }()
	if err := p.write(Msg{ID: id, Type: typ, Body: b}, nil); err != nil {
		return nil, nil, err
	}
	select {
	case r := <-ch:
		if r.msg.Err != "" {
			return r.msg.Body, nil, errors.New(r.msg.Err)
		}
		return r.msg.Body, r.bin, nil
	case <-p.done:
		return nil, nil, ErrDisconnected
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

func (p *Peer) Close() {
	p.once.Do(func() { close(p.done); p.conn.Close() })
}

// serve reads frames until the connection ends.
func (p *Peer) serve(ctx context.Context) error {
	defer p.Close()
	for {
		b, err := readFrame(p.conn)
		if err != nil {
			return err
		}
		var m Msg
		if err := json.Unmarshal(b, &m); err != nil {
			return err
		}
		var bin []byte
		if m.Bin {
			if bin, err = readFrame(p.conn); err != nil {
				return err
			}
		}
		p.lastSeen.Store(time.Now().UnixNano())
		switch {
		case m.Reply:
			p.pmu.Lock()
			ch := p.pending[m.ID]
			p.pmu.Unlock()
			if ch != nil {
				ch <- reply{m, bin}
			}
		case m.ID != 0:
			go p.answer(ctx, m)
		default:
			if p.onEvent != nil && m.Type != "heartbeat" {
				p.onEvent(p, m.Type, m.Body, bin)
			}
		}
	}
}

func (p *Peer) answer(ctx context.Context, m Msg) {
	out := Msg{ID: m.ID, Type: m.Type, Reply: true}
	var bin []byte
	if p.handler == nil {
		out.Err = "no handler"
	} else {
		resp, b, err := p.handler(ctx, m.Type, m.Body)
		bin = b
		if resp != nil {
			out.Body, _ = json.Marshal(resp)
		}
		if err != nil {
			out.Err = err.Error()
		}
	}
	if err := p.write(out, bin); err != nil {
		slog.Debug("link reply", "err", err)
	}
}

func (p *Peer) heartbeat(ctx context.Context) {
	t := time.NewTicker(HeartbeatEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.done:
			return
		case <-t.C:
		}
		if err := p.Notify("heartbeat", nil); err != nil {
			p.Close()
			return
		}
		if time.Since(time.Unix(0, p.lastSeen.Load())) > DeadAfter+HeartbeatEvery {
			slog.Warn("link: peer silent, closing")
			p.Close()
			return
		}
	}
}

// --- body side ---------------------------------------------------------------

// Server accepts the head. A new head connection replaces the old one.
type Server struct {
	Handler   Handler   // requests from the head (rare)
	OnEvent   EventFunc // tof, sound_dir, state, config_ack
	OnConnect func(*Peer)
	OnLost    func()

	mu  sync.Mutex
	cur *Peer
}

func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() { <-ctx.Done(); ln.Close() }()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go s.handle(ctx, c)
	}
}

func (s *Server) handle(ctx context.Context, c net.Conn) {
	tc, ok := c.(*tls.Conn)
	if ok {
		hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := tc.HandshakeContext(hctx)
		cancel()
		if err != nil {
			slog.Warn("link: rejected connection", "from", c.RemoteAddr(), "err", err)
			c.Close()
			return
		}
	}
	p := newPeer(c, s.Handler, s.OnEvent)
	s.mu.Lock()
	old := s.cur
	s.cur = p
	s.mu.Unlock()
	if old != nil {
		old.Close()
	}
	slog.Info("link: head connected", "from", c.RemoteAddr())
	if s.OnConnect != nil {
		go s.OnConnect(p)
	}
	go p.heartbeat(ctx)
	err := p.serve(ctx)
	s.mu.Lock()
	current := s.cur == p
	if current {
		s.cur = nil
	}
	s.mu.Unlock()
	if current {
		slog.Warn("link: head lost", "err", err)
		if s.OnLost != nil {
			s.OnLost()
		}
	}
}

func (s *Server) Peer() *Peer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

func (s *Server) Connected() bool { return s.Peer() != nil }

func (s *Server) Request(ctx context.Context, typ string, body any) (json.RawMessage, []byte, error) {
	p := s.Peer()
	if p == nil {
		return nil, nil, ErrDisconnected
	}
	return p.Request(ctx, typ, body)
}

func (s *Server) Notify(typ string, body any) error {
	p := s.Peer()
	if p == nil {
		return ErrDisconnected
	}
	return p.Notify(typ, body)
}

// --- head side ---------------------------------------------------------------

// Dial keeps a connection to the body, reconnecting with backoff. onConnect runs
// per connection with a context that ends when the connection drops.
func Dial(ctx context.Context, addr string, cfg *tls.Config, h Handler, ev EventFunc, onConnect func(context.Context, *Peer)) {
	backoff := 500 * time.Millisecond
	for ctx.Err() == nil {
		d := tls.Dialer{NetDialer: &net.Dialer{Timeout: 3 * time.Second}, Config: cfg}
		c, err := d.DialContext(ctx, "tcp", resolveMDNS(ctx, addr))
		if err != nil {
			slog.Warn("link: cannot reach body", "addr", addr, "err", err, "retry_in", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 5*time.Second)
			continue
		}
		backoff = 500 * time.Millisecond
		slog.Info("link: connected to body", "addr", addr)
		p := newPeer(c, h, ev)
		cctx, cancel := context.WithCancel(ctx)
		go p.heartbeat(cctx)
		if onConnect != nil {
			go onConnect(cctx, p)
		}
		err = p.serve(cctx)
		cancel()
		slog.Warn("link: disconnected", "err", err)
	}
}

// resolveMDNS turns "name.local:port" into "ip:port" through the system resolver.
// The binaries are static (no cgo), so Go's own resolver only asks DNS and never
// sees mDNS; getent goes through nsswitch (nss-mdns). The TLS ServerName is fixed
// to the body's cert name, so dialing by IP keeps the check intact. On any
// failure the address is returned unchanged.
func resolveMDNS(ctx context.Context, addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || !strings.HasSuffix(host, ".local") {
		return addr
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "getent", "ahostsv4", host).Output()
	if err != nil {
		return addr
	}
	if ip := firstIP(string(out)); ip != "" {
		return net.JoinHostPort(ip, port)
	}
	return addr
}

// firstIP reads the first address from getent output ("192.168.1.20 STREAM bb8.local").
func firstIP(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if f := strings.Fields(line); len(f) > 0 && net.ParseIP(f[0]) != nil {
			return f[0]
		}
	}
	return ""
}
