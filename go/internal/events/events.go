// Package events is the robot → page feed: one bus every part of bb8-body
// publishes to (pose, obstacles, control, head link, what BB-8 heard and said,
// people, settings), served as SSE at GET /api/events.
//
// Event types (data fields in wiki/Protocolo.md § 2.4):
//
//	pose      x, y, heading_deg, room, battery_v, tilt_deg   (≤ 2 Hz, only when it changed)
//	alert     kind (obstacle, tilt, battery_low, watchdog), distance_m…
//	control   who (llm, manual, idle)
//	head      connected (bool)
//	heard     text, speaker
//	reply     text, tools
//	tool      name, result
//	person    name, role, what (added, seen, note, role, removed)
//	config    rev, source, keys
package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Event struct {
	ID   int64     `json:"id"`
	Type string    `json:"type"`
	At   time.Time `json:"at"`
	Data any       `json:"data"`
}

const keep = 100 // replayed to a page that reconnects with Last-Event-ID

type Bus struct {
	mu   sync.Mutex
	next int64
	ring []Event
	subs map[chan Event]struct{}
}

func New() *Bus { return &Bus{subs: map[chan Event]struct{}{}} }

// Publish never blocks: a slow page drops events instead of stalling the robot.
func (b *Bus) Publish(typ string, data any) {
	b.mu.Lock()
	b.next++
	e := Event{ID: b.next, Type: typ, At: time.Now(), Data: data}
	b.ring = append(b.ring, e)
	if len(b.ring) > keep {
		b.ring = b.ring[len(b.ring)-keep:]
	}
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
	b.mu.Unlock()
}

// Since returns the kept events after id (oldest first).
func (b *Bus) Since(id int64) []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []Event
	for _, e := range b.ring {
		if e.ID > id {
			out = append(out, e)
		}
	}
	return out
}

func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}
}

// Handler serves SSE. ?types=pose,alert filters; Last-Event-ID (or ?since=) replays.
func (b *Bus) Handler(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	want := map[string]bool{}
	for _, t := range strings.Split(r.URL.Query().Get("types"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			want[t] = true
		}
	}
	since, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	if s := r.URL.Query().Get("since"); s != "" {
		since, _ = strconv.ParseInt(s, 10, 64)
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	ch, cancel := b.Subscribe()
	defer cancel()
	send := func(e Event) {
		if len(want) > 0 && !want[e.Type] {
			return
		}
		bs, _ := json.Marshal(e)
		fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.ID, e.Type, bs)
	}
	last := since
	if since > 0 {
		for _, e := range b.Since(since) {
			send(e)
			last = e.ID
		}
	}
	fmt.Fprint(w, ": hi\n\n")
	fl.Flush()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			if e.ID <= last {
				continue
			}
			send(e)
			fl.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		}
	}
}
