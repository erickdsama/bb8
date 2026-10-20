package body

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"
)

// VoiceFeed is the live view of the voice pipeline for the /voice page: the head
// mic level (computed here from the audio hub) plus what voz reports (wake word
// score, state, transcript, reply). Nothing is stored; late viewers see from now.
type VoiceFeed struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func NewVoiceFeed() *VoiceFeed { return &VoiceFeed{subs: map[chan []byte]struct{}{}} }

func (f *VoiceFeed) Publish(ev map[string]any) {
	if _, ok := ev["t"]; !ok {
		ev["t"] = time.Now().UnixMilli()
	}
	b, _ := json.Marshal(ev)
	f.mu.Lock()
	defer f.mu.Unlock()
	for ch := range f.subs {
		select {
		case ch <- b:
		default: // slow viewer: drop
		}
	}
}

// RunLevels publishes the head mic level (dBFS, 0 = full scale) every chunk
// while the head streams, and -90 every 250 ms while it is quiet.
func (f *VoiceFeed) RunLevels(ctx context.Context, hub *AudioHub) {
	ch, cancel := hub.Subscribe()
	defer cancel()
	idle := time.NewTicker(250 * time.Millisecond)
	defer idle.Stop()
	last := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case a := <-ch:
			if a.Type != "audio" || len(a.PCM) < 2 {
				continue
			}
			var sum float64
			n := len(a.PCM) / 2
			for i := 0; i < n; i++ {
				s := float64(int16(binary.LittleEndian.Uint16(a.PCM[2*i:])))
				sum += s * s
			}
			db := 20 * math.Log10(math.Sqrt(sum/float64(n))/32768+1e-9)
			f.Publish(map[string]any{"type": "level", "db": math.Round(db*10) / 10})
			last = time.Now()
		case <-idle.C:
			if time.Since(last) > 300*time.Millisecond {
				f.Publish(map[string]any{"type": "level", "db": -90})
			}
		}
	}
}

// Routes: POST {post} (voz reports an event, local API) and GET {sse} (viewers).
func (f *VoiceFeed) PostHandler(w http.ResponseWriter, r *http.Request) {
	var ev map[string]any
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&ev); err != nil || ev["type"] == nil {
		http.Error(w, `send {"type": …}`, http.StatusBadRequest)
		return
	}
	f.Publish(ev)
	w.WriteHeader(http.StatusNoContent)
}

func (f *VoiceFeed) SSEHandler(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no streaming", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	ch := make(chan []byte, 64)
	f.mu.Lock()
	f.subs[ch] = struct{}{}
	f.mu.Unlock()
	defer func() { f.mu.Lock(); delete(f.subs, ch); f.mu.Unlock() }()
	fmt.Fprint(w, ": hello\n\n")
	fl.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		}
	}
}
