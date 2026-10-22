package relay

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// Server is bb8-web's side: the robot connects to RobotHandler, and ServeHTTP
// forwards a page's request down that link and streams the answer back.
type Server struct {
	Token func() string // device token the robot must present

	mu     sync.Mutex
	link   *link
	nextID uint64
	since  time.Time
}

type link struct {
	ws      *websocket.Conn
	sendMu  sync.Mutex
	mu      sync.Mutex
	waiting map[uint64]chan Frame
	done    chan struct{}
}

func (l *link) send(f Frame) error {
	l.sendMu.Lock()
	defer l.sendMu.Unlock()
	_ = l.ws.SetWriteDeadline(time.Now().Add(15 * time.Second))
	return websocket.JSON.Send(l.ws, f)
}

// Connected says whether the robot is on the link, and since when.
func (s *Server) Connected() (bool, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.link != nil, s.since
}

// RobotHandler is where bb8-body dials in (GET /robot, Authorization: Bearer <device token>).
func (s *Server) RobotHandler() http.Handler {
	ws := websocket.Server{
		Handshake: func(*websocket.Config, *http.Request) error { return nil }, // not a browser: no Origin check
		Handler:   s.serveRobot,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want := s.Token()
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if want == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			slog.Warn("web relay: robot refused (bad device token)", "from", r.RemoteAddr)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ws.ServeHTTP(w, r)
	})
}

func (s *Server) serveRobot(ws *websocket.Conn) {
	l := &link{ws: ws, waiting: map[uint64]chan Frame{}, done: make(chan struct{})}
	s.mu.Lock()
	old := s.link
	s.link, s.since = l, time.Now()
	s.mu.Unlock()
	if old != nil { // a new robot link replaces a stale one
		old.ws.Close()
	}
	slog.Info("web relay: robot connected", "from", ws.Request().RemoteAddr)
	defer func() {
		close(l.done)
		s.mu.Lock()
		if s.link == l {
			s.link = nil
		}
		s.mu.Unlock()
		ws.Close()
		slog.Info("web relay: robot gone")
	}()
	go func() {
		t := time.NewTicker(PingEvery)
		defer t.Stop()
		for {
			select {
			case <-l.done:
				return
			case <-t.C:
				if l.send(Frame{Ping: true}) != nil {
					ws.Close()
					return
				}
			}
		}
	}()
	for {
		_ = ws.SetReadDeadline(time.Now().Add(3 * PingEvery))
		var f Frame
		if err := websocket.JSON.Receive(ws, &f); err != nil {
			return
		}
		if f.Ping || f.ID == 0 {
			continue
		}
		l.mu.Lock()
		ch := l.waiting[f.ID]
		l.mu.Unlock()
		if ch == nil {
			continue
		}
		select {
		case ch <- f:
		default: // a page reading too slowly: drop it rather than stall every other request
			l.mu.Lock()
			delete(l.waiting, f.ID)
			l.mu.Unlock()
			close(ch)
			_ = l.send(Frame{ID: f.ID, Cancel: true})
		}
	}
}

func offline(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": errOffline.Error(), "offline": true})
}

// ServeHTTP forwards one request to the robot.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	l := s.link
	s.nextID++
	id := s.nextID
	s.mu.Unlock()
	if l == nil {
		offline(w)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxBody+1))
	if err != nil || len(body) > MaxBody {
		http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	ch := make(chan Frame, 512)
	l.mu.Lock()
	l.waiting[id] = ch
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		delete(l.waiting, id)
		l.mu.Unlock()
	}()
	if err := l.send(Frame{ID: id, Method: r.Method, Path: r.URL.RequestURI(), Header: copyHeader(r.Header), Body: body}); err != nil {
		offline(w)
		return
	}
	fl, _ := w.(http.Flusher)
	headSent := false
	timeout := time.NewTimer(30 * time.Second) // until the head arrives; streams then run freely
	defer timeout.Stop()
	for {
		select {
		case <-r.Context().Done():
			_ = l.send(Frame{ID: id, Cancel: true})
			return
		case <-l.done:
			if !headSent {
				offline(w)
			}
			return
		case <-timeout.C:
			if !headSent {
				_ = l.send(Frame{ID: id, Cancel: true})
				http.Error(w, "BB-8 did not answer", http.StatusGatewayTimeout)
				return
			}
		case f, ok := <-ch:
			if !ok {
				return
			}
			if f.Status != 0 && !headSent {
				for k, v := range f.Header {
					w.Header()[k] = v
				}
				w.WriteHeader(f.Status)
				headSent = true
				timeout.Stop()
			}
			if len(f.Body) > 0 {
				if !headSent {
					w.WriteHeader(http.StatusOK)
					headSent = true
				}
				if _, err := w.Write(f.Body); err != nil {
					_ = l.send(Frame{ID: id, Cancel: true})
					return
				}
				if fl != nil {
					fl.Flush()
				}
			}
			if f.End {
				return
			}
		}
	}
}
