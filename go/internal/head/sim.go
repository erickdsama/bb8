package head

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/erickdsama/bb8/go/internal/persona"
)

// Sim forwards to the Python simulated head (simulador/dummy, HTTP :8080),
// so the dummy world, its camera and its viewer keep working.
type Sim struct {
	URL string
	c   *http.Client
}

func NewSim(u string) *Sim {
	return &Sim{URL: strings.TrimRight(u, "/"), c: &http.Client{Timeout: 20 * time.Second}}
}

func (s *Sim) post(ctx context.Context, path string, body any) error {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", s.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r, err := s.c.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 300 {
		msg, _ := io.ReadAll(r.Body)
		return fmt.Errorf("sim head %s: %s", path, msg)
	}
	return nil
}

func (s *Sim) Photo(ctx context.Context, width int) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", s.URL+"/foto?ancho="+url.QueryEscape(fmt.Sprint(width)), nil)
	r, err := s.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode == 503 {
		return nil, ErrNoCamera
	}
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("sim head photo: HTTP %d", r.StatusCode)
	}
	return io.ReadAll(r.Body)
}

func (s *Sim) Eye(rgb [3]uint8, pattern string, brightness float64) error {
	es := persona.PatternES[pattern]
	if es == "" {
		es = "fijo"
	}
	return s.post(context.Background(), "/ojo", map[string]any{"color": persona.Hex(rgb), "patron": es, "brillo": brightness})
}

func (s *Sim) Say(ctx context.Context, text, sound string) (float64, error) {
	body := map[string]any{}
	if text != "" {
		body["texto"] = text
	}
	if sound != "" {
		body["sonido"] = persona.EmotionES[sound]
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", s.URL+"/hablar", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r, err := s.c.Do(req)
	if err != nil {
		return 0, err
	}
	defer r.Body.Close()
	var out struct {
		OK      bool    `json:"ok"`
		Seconds float64 `json:"segundos"`
		Error   string  `json:"error"`
	}
	_ = json.NewDecoder(r.Body).Decode(&out)
	if !out.OK {
		return 0, fmt.Errorf("sim head say: %s", out.Error)
	}
	return out.Seconds, nil
}

func (s *Sim) Rest(active bool) error {
	return s.post(context.Background(), "/reposo", map[string]bool{"activo": active})
}

func (s *Sim) ToF() (int, bool) {
	r, err := s.c.Get(s.URL + "/tof")
	if err != nil {
		return 0, false
	}
	defer r.Body.Close()
	var v struct {
		MM int `json:"mm"`
	}
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		return 0, false
	}
	return v.MM, true
}

func (s *Sim) State() map[string]any {
	out := map[string]any{"backend": "sim"}
	r, err := s.c.Get(s.URL + "/estado")
	if err != nil {
		return out
	}
	defer r.Body.Close()
	var v struct {
		Camara string `json:"camara"`
		ToF    string `json:"tof"`
	}
	if json.NewDecoder(r.Body).Decode(&v) == nil {
		out["camera"] = v.Camara != "ninguna"
		out["tof"] = v.ToF
	}
	return out
}

// SayPCM: the dummy cannot play the body's voice, so it speaks the text as
// usual; the answer still covers at least the audio's length.
func (s *Sim) SayPCM(ctx context.Context, text, sound string, pcm []byte, rate int, _ bool) (float64, error) {
	secs, err := s.Say(ctx, text, sound)
	if d := float64(len(pcm)/2) / float64(rate); err == nil && d > secs {
		secs = math.Round(d*100) / 100
	}
	return secs, err
}
