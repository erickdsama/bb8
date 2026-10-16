package body

import (
	"encoding/binary"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// AudioFrame is one piece of the head's voiced audio: audio_start, audio (PCM,
// s16le mono 16 kHz) or audio_end, with the utterance id.
type AudioFrame struct {
	Type string `json:"type"`
	ID   uint64 `json:"id"`
	Seq  int    `json:"seq,omitempty"`
	PCM  []byte `json:"-"`
}

// AudioHub fans frames out to every local listener; a slow one loses frames.
type AudioHub struct {
	mu   sync.Mutex
	subs map[chan AudioFrame]struct{}
}

func NewAudioHub() *AudioHub { return &AudioHub{subs: map[chan AudioFrame]struct{}{}} }

func (a *AudioHub) Publish(f AudioFrame) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for ch := range a.subs {
		select {
		case ch <- f:
		default:
		}
	}
}

func (a *AudioHub) Subscribe() (<-chan AudioFrame, func()) {
	ch := make(chan AudioFrame, 256)
	a.mu.Lock()
	a.subs[ch] = struct{}{}
	a.mu.Unlock()
	return ch, func() { a.mu.Lock(); delete(a.subs, ch); a.mu.Unlock() }
}

// ServeStream writes frames as [u32 BE length][JSON] and, for "audio", a second
// [u32 BE length][PCM]. A {"type":"ping"} every 5 s keeps idle connections honest.
func (a *AudioHub) ServeStream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no streaming", 500)
		return
	}
	ch, cancel := a.Subscribe()
	defer cancel()
	w.Header().Set("Content-Type", "application/x-bb8-audio")
	w.Header().Set("X-Audio-Format", "s16le; rate=16000; channels=1")
	write := func(b []byte) error {
		var hdr [4]byte
		binary.BigEndian.PutUint32(hdr[:], uint32(len(b)))
		if _, err := w.Write(hdr[:]); err != nil {
			return err
		}
		_, err := w.Write(b)
		return err
	}
	hello, _ := json.Marshal(map[string]any{"type": "hello", "rate": 16000, "channels": 1, "format": "s16le"})
	if write(hello) != nil {
		return
	}
	fl.Flush()
	ping := time.NewTicker(5 * time.Second)
	defer ping.Stop()
	for {
		var f AudioFrame
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			f = AudioFrame{Type: "ping"}
		case f = <-ch:
		}
		hdr, _ := json.Marshal(f)
		if write(hdr) != nil {
			return
		}
		if f.Type == "audio" && write(f.PCM) != nil {
			return
		}
		fl.Flush()
	}
}
