package body

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AudioLog keeps the last few phrases voz recorded after the wake word, with what
// the transcriber understood, in RAM only, so they can be played back from the
// config page (voice.audio_log).
type AudioLog struct {
	Enabled func() bool
	Max     int

	mu    sync.Mutex
	clips []*Clip
	next  uint64
}

type Clip struct {
	ID      uint64    `json:"id"`
	At      time.Time `json:"at"`
	Seconds float64   `json:"seconds"`
	Text    string    `json:"text,omitempty"`
	Reply   string    `json:"reply,omitempty"`
	pcm     []byte
}

func NewAudioLog(enabled func() bool) *AudioLog {
	return &AudioLog{Enabled: enabled, Max: 20}
}

// Add stores one phrase: s16le mono 16 kHz PCM, its transcript and BB-8's reply.
func (a *AudioLog) Add(pcm []byte, text, reply string) {
	if !a.Enabled() || len(pcm) == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.next++
	a.clips = append(a.clips, &Clip{ID: a.next, At: time.Now(), Seconds: float64(len(pcm)) / 32000, Text: text, Reply: reply, pcm: pcm})
	if len(a.clips) > a.Max {
		a.clips = a.clips[len(a.clips)-a.Max:]
	}
}

// Routes: GET {prefix} lists the clips (newest first), GET {prefix}/{id}.wav plays one.
func (a *AudioLog) Routes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc("GET "+prefix, func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		out := make([]Clip, 0, len(a.clips))
		for i := len(a.clips) - 1; i >= 0; i-- {
			out = append(out, *a.clips[i])
		}
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"enabled": a.Enabled(), "clips": out})
	})
	mux.HandleFunc("GET "+prefix+"/{file}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseUint(strings.TrimSuffix(r.PathValue("file"), ".wav"), 10, 64)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		a.mu.Lock()
		var pcm []byte
		for _, c := range a.clips {
			if c.ID == id {
				pcm = c.pcm
			}
		}
		a.mu.Unlock()
		if pcm == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(wav16k(pcm))
	})
}

// wav16k wraps s16le mono 16 kHz PCM in a WAV header.
func wav16k(pcm []byte) []byte {
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+len(pcm)))
	b.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(16000), uint32(32000), uint16(2), uint16(16)} {
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(pcm)))
	b.Write(pcm)
	return b.Bytes()
}
