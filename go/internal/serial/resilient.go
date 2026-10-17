package serial

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/erickdsama/bb8/go/internal/protocol"
)

// ErrUnavailable: the Arduino is not connected right now (the link keeps retrying).
var ErrUnavailable = errors.New("arduino_unavailable")

const (
	retryMin        = 500 * time.Millisecond
	retryMax        = 5 * time.Second
	timeoutsToReset = 3 // consecutive replies missed before reopening the port
	logEvery        = 30 * time.Second
)

// Resilient is a Link that survives the Arduino not being ready at boot, being
// unplugged, or resetting: it connects lazily on the first Send, backs off while
// the port is missing, and reopens the port when the link breaks. Callers just
// see Send errors while it is down.
type Resilient struct {
	URL  string
	Baud int
	// OnConnect runs after every successful (re)connection, outside the lock, so it
	// may call Send. first is true for the first connection since the process started.
	OnConnect func(ident string, first bool)

	mu       sync.Mutex
	cur      *Link
	ident    string
	everUp   bool
	timeouts int
	delay    time.Duration
	nextTry  time.Time
	lastErr  error
	lastLog  time.Time
	closed   bool
}

func NewResilient(url string, baud int) *Resilient { return &Resilient{URL: url, Baud: baud} }

// Ident is the last identification line ("" if it never connected).
func (r *Resilient) Ident() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ident
}

func (r *Resilient) Connected() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cur != nil
}

func (r *Resilient) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	if r.cur != nil {
		r.cur.Close()
		r.cur = nil
	}
}

// acquire returns the live link, connecting if needed. fresh reports a new connection.
func (r *Resilient) acquire() (l *Link, fresh bool, first bool, ident string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, false, false, "", errors.New("serial closed")
	}
	if r.cur != nil {
		return r.cur, false, false, r.ident, nil
	}
	if time.Now().Before(r.nextTry) {
		return nil, false, false, "", fmt.Errorf("%w: %v", ErrUnavailable, r.lastErr)
	}
	nl, err := Open(r.URL, r.Baud) // may block ~2 s: opening a tty resets the Uno
	if err != nil {
		r.lastErr = err
		r.delay = min(max(r.delay*2, retryMin), retryMax)
		r.nextTry = time.Now().Add(r.delay)
		if time.Since(r.lastLog) > logEvery {
			r.lastLog = time.Now()
			slog.Warn("arduino not available, retrying", "url", r.URL, "err", err, "every", r.delay)
		}
		return nil, false, false, "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	r.cur, r.ident, r.delay, r.timeouts, r.lastLog = nl, nl.Ident, 0, 0, time.Time{}
	first = !r.everUp
	r.everUp = true
	return nl, true, first, nl.Ident, nil
}

func (r *Resilient) lose(l *Link, why error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cur != l {
		return
	}
	r.cur.Close()
	r.cur = nil
	r.lastErr = why
	r.nextTry = time.Now().Add(retryMin)
	r.delay = 0
	slog.Warn("arduino link lost, will reconnect", "err", why)
}

func (r *Resilient) Send(line string) (string, error) {
	l, fresh, first, ident, err := r.acquire()
	if err != nil {
		return "", err
	}
	if fresh && r.OnConnect != nil {
		r.OnConnect(ident, first)
	}
	reply, err := l.Send(line)
	if err == nil {
		r.mu.Lock()
		r.timeouts = 0
		r.mu.Unlock()
		return reply, nil
	}
	var pe *protocol.Error
	if errors.As(err, &pe) { // ERR <reason> is an answer, not a broken link
		r.mu.Lock()
		r.timeouts = 0
		r.mu.Unlock()
		return reply, err
	}
	if errors.Is(err, ErrTimeout) {
		r.mu.Lock()
		r.timeouts++
		n := r.timeouts
		r.mu.Unlock()
		if n < timeoutsToReset {
			return "", err
		}
	}
	r.lose(l, err)
	return "", err
}
