// Package serial is the link to the Arduino: one command, one reply, one at a time.
// It opens /dev/ttyACM0 on the Pi or socket://host:port against the simulated Arduino.
package serial

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	bugserial "go.bug.st/serial"

	"github.com/erickdsama/bb8/go/internal/protocol"
)

const ReplyTimeout = 200 * time.Millisecond

type port interface {
	io.ReadWriteCloser
	setDeadline(time.Duration) error
}

type netPort struct{ net.Conn }

func (p netPort) setDeadline(d time.Duration) error { return p.SetReadDeadline(time.Now().Add(d)) }

type ttyPort struct{ bugserial.Port }

func (p ttyPort) setDeadline(d time.Duration) error { return p.SetReadTimeout(d) }

type Link struct {
	URL   string
	Ident string

	mu  sync.Mutex
	p   port
	buf []byte
}

// Open connects, waits for the Uno reset and checks the "I" identification.
func Open(url string, baud int) (*Link, error) {
	l := &Link{URL: url}
	if addr, ok := strings.CutPrefix(url, "socket://"); ok {
		c, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			return nil, err
		}
		l.p = netPort{c}
	} else {
		p, err := bugserial.Open(url, &bugserial.Mode{BaudRate: baud})
		if err != nil {
			return nil, err
		}
		l.p = ttyPort{p}
		time.Sleep(2 * time.Second) // opening the port resets the Uno
		if rp, ok := p.(interface{ ResetInputBuffer() error }); ok {
			_ = rp.ResetInputBuffer()
		}
	}
	id, err := l.Send(protocol.CmdIdent)
	if err != nil {
		l.Close()
		return nil, fmt.Errorf("identify: %w", err)
	}
	l.Ident = "OK " + id
	if !strings.HasPrefix(l.Ident, "OK BB8") {
		l.Close()
		return nil, fmt.Errorf("device at %s is not a BB-8: %q", url, l.Ident)
	}
	slog.Info("arduino", "url", url, "ident", l.Ident)
	return l, nil
}

func (l *Link) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.p != nil {
		l.p.Close()
		l.p = nil
	}
}

// Send writes a line and returns the text after OK. ERR → *protocol.Error.
func (l *Link) Send(line string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.p == nil {
		return "", errors.New("serial port closed")
	}
	l.buf = l.buf[:0]
	if _, err := l.p.Write([]byte(line + "\n")); err != nil {
		return "", err
	}
	reply, err := l.readLine()
	if err != nil {
		return "", fmt.Errorf("no reply to %q: %w", line, err)
	}
	slog.Debug("serial", "tx", line, "rx", reply)
	return protocol.ParseReply(reply)
}

func (l *Link) readLine() (string, error) {
	deadline := time.Now().Add(ReplyTimeout)
	chunk := make([]byte, 64)
	for {
		if i := strings.IndexByte(string(l.buf), '\n'); i >= 0 {
			return strings.TrimSpace(string(l.buf[:i])), nil
		}
		left := time.Until(deadline)
		if left <= 0 {
			return "", errors.New("timeout")
		}
		if err := l.p.setDeadline(left); err != nil {
			return "", err
		}
		n, err := l.p.Read(chunk)
		l.buf = append(l.buf, chunk[:n]...)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return "", err
		}
		if n == 0 { // tty read timeout
			continue
		}
	}
}
