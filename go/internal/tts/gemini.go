// Package tts makes BB-8's spoken voice on the body, where there is internet
// and CPU to spare, so the head (a Zero W) only plays the samples. Today the one
// cloud engine is Gemini TTS; with it off, without a key or when the call fails,
// the head speaks with its own espeak-ng (or Wyoming) as before.
package tts

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
)

const (
	EngineEspeak = "espeak"
	EngineGemini = "gemini"
)

// Voices are Gemini's prebuilt voices (all speak Spanish).
var Voices = []string{"Puck", "Zephyr", "Charon", "Kore", "Fenrir", "Leda", "Orus", "Aoede", "Callirrhoe", "Autonoe",
	"Enceladus", "Iapetus", "Umbriel", "Algieba", "Despina", "Erinome", "Algenib", "Rasalgethi", "Laomedeia", "Achernar",
	"Alnilam", "Schedar", "Gacrux", "Pulcherrima", "Achird", "Zubenelgenubi", "Vindemiatrix", "Sadachbia", "Sadaltager", "Sulafat"}

func Fields() []config.Field {
	return []config.Field{
		{Key: "voice.tts", Label: "Voice", Type: config.Enum, Options: []string{EngineEspeak, EngineGemini}, Default: EngineEspeak,
			Help: "espeak: robotic, works offline. gemini: natural, needs internet and a key."},
		{Key: "voice.gemini_key", Label: "Gemini API key", Type: config.Secret},
		{Key: "voice.gemini_voice", Label: "Gemini voice", Type: config.Enum, Options: Voices, Default: "Puck", Agent: true},
		{Key: "voice.gemini_style", Label: "Speaking style", Type: config.String, Default: "", Advanced: true,
			Help: "Optional, e.g. \"alegre y curioso\"."},
		{Key: "voice.gemini_model", Label: "Gemini model", Type: config.Enum,
			Options: []string{"gemini-3.8-flash-tts", "gemini-3.8-flash-lite-tts"}, Default: "gemini-3.8-flash-tts", Advanced: true},
	}
}

// Gemini synthesizes speech with the Gemini API (generateContent, AUDIO out).
type Gemini struct {
	Cfg    *config.Store
	Client *http.Client
	URL    string // API base; tests point it at a fake
}

func NewGemini(cfg *config.Store) *Gemini {
	return &Gemini{Cfg: cfg, Client: &http.Client{Timeout: 20 * time.Second},
		URL: "https://generativelanguage.googleapis.com/v1beta"}
}

// Speak returns 16-bit little-endian mono PCM and its sample rate. pcm is nil
// with no error when Gemini is not the chosen voice or has no key.
func (g *Gemini) Speak(ctx context.Context, text string) (pcm []byte, rate int, err error) {
	key := g.Cfg.Secret("voice.gemini_key")
	if g.Cfg.Get("voice.tts") != EngineGemini || key == "" || strings.TrimSpace(text) == "" {
		return nil, 0, nil
	}
	voice, _ := g.Cfg.Get("voice.gemini_voice").(string)
	return g.synth(ctx, key, voice, text)
}

// ErrNoKey: there is no Gemini key to preview with.
var ErrNoKey = errors.New("add the Gemini API key first")

// Preview says text with one voice as a WAV file, whatever voice.tts is, so the
// page can play a voice before choosing it.
func (g *Gemini) Preview(ctx context.Context, voice, text string) ([]byte, error) {
	key := g.Cfg.Secret("voice.gemini_key")
	if key == "" {
		return nil, ErrNoKey
	}
	if !slices.Contains(Voices, voice) {
		return nil, fmt.Errorf("unknown voice %q", voice)
	}
	pcm, rate, err := g.synth(ctx, key, voice, text)
	if err != nil {
		return nil, err
	}
	return WAV(pcm, rate), nil
}

// WAV wraps 16-bit mono PCM in a WAV header.
func WAV(pcm []byte, rate int) []byte {
	h := make([]byte, 44)
	copy(h, "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+len(pcm)))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], 1)
	binary.LittleEndian.PutUint32(h[24:], uint32(rate))
	binary.LittleEndian.PutUint32(h[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(len(pcm)))
	return append(h, pcm...)
}

func (g *Gemini) synth(ctx context.Context, key, voice, text string) (pcm []byte, rate int, err error) {
	model, _ := g.Cfg.Get("voice.gemini_model").(string)
	if style, _ := g.Cfg.Get("voice.gemini_style").(string); strings.TrimSpace(style) != "" {
		// Director notes: Gemini follows them without reading them out (a "Dilo
		// con voz…:" prefix was spoken; system instructions are refused).
		text = "### DIRECTOR NOTES\nStyle: " + strings.TrimSpace(style) + "\n### TRANSCRIPT\n" + text
	}
	body, _ := json.Marshal(map[string]any{
		"contents": []any{map[string]any{"parts": []any{map[string]string{"text": text}}}},
		"generationConfig": map[string]any{
			"responseModalities": []string{"AUDIO"},
			"speechConfig": map[string]any{"voiceConfig": map[string]any{
				"prebuiltVoiceConfig": map[string]string{"voiceName": voice}}},
		},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", g.URL+"/models/"+model+":generateContent", bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", key)
	r, err := g.Client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(r.Body, 300))
		return nil, 0, fmt.Errorf("gemini tts: HTTP %d: %s", r.StatusCode, strings.TrimSpace(string(msg)))
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&out); err != nil {
		return nil, 0, fmt.Errorf("gemini tts: %w", err)
	}
	for _, c := range out.Candidates {
		for _, p := range c.Content.Parts {
			if p.InlineData.Data == "" {
				continue
			}
			b, err := base64.StdEncoding.DecodeString(p.InlineData.Data)
			if err != nil {
				return nil, 0, fmt.Errorf("gemini tts: %w", err)
			}
			return toPCM(b, p.InlineData.MimeType)
		}
	}
	return nil, 0, errors.New("gemini tts: no audio in the answer")
}

// toPCM accepts raw L16 ("audio/L16;codec=pcm;rate=24000") or a 16-bit WAV.
func toPCM(b []byte, mime string) ([]byte, int, error) {
	if len(b) >= 44 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WAVE" {
		rate, bits, data := 0, 0, []byte(nil)
		for i := 12; i+8 <= len(b); {
			id, n := string(b[i:i+4]), int(uint32(b[i+4])|uint32(b[i+5])<<8|uint32(b[i+6])<<16|uint32(b[i+7])<<24)
			body := b[i+8 : min(len(b), i+8+n)]
			switch id {
			case "fmt ":
				if len(body) >= 16 {
					if body[2] != 1 || body[3] != 0 { // channels
						return nil, 0, errors.New("gemini tts: only mono audio")
					}
					rate = int(uint32(body[4]) | uint32(body[5])<<8 | uint32(body[6])<<16 | uint32(body[7])<<24)
					bits = int(body[14])
				}
			case "data":
				data = body
			}
			i += 8 + n + n%2
		}
		if bits != 16 || rate == 0 || data == nil {
			return nil, 0, errors.New("gemini tts: unsupported WAV")
		}
		return data, rate, nil
	}
	rate := 24000
	for _, part := range strings.Split(mime, ";") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(strings.ToLower(part)), "rate="); ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				rate = n
			}
		}
	}
	return b, rate, nil
}
