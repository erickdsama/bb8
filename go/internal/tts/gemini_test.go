package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/erickdsama/bb8/go/internal/config"
)

func store(t *testing.T) *config.Store {
	s, err := config.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.Register(Fields()...)
	return s
}

func TestGeminiSpeak(t *testing.T) {
	pcm := []byte{1, 0, 2, 0, 3, 0}
	var gotPath, gotKey, gotVoice, gotText string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey = r.URL.Path, r.Header.Get("x-goog-api-key")
		var in struct {
			Contents []struct {
				Parts []struct{ Text string } `json:"parts"`
			} `json:"contents"`
			GenerationConfig struct {
				ResponseModalities []string `json:"responseModalities"`
				SpeechConfig       struct {
					VoiceConfig struct {
						PrebuiltVoiceConfig struct {
							VoiceName string `json:"voiceName"`
						} `json:"prebuiltVoiceConfig"`
					} `json:"voiceConfig"`
				} `json:"speechConfig"`
			} `json:"generationConfig"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		gotVoice, gotText = in.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName, in.Contents[0].Parts[0].Text
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{
			map[string]any{"inlineData": map[string]string{"mimeType": "audio/L16;codec=pcm;rate=24000", "data": base64.StdEncoding.EncodeToString(pcm)}}}}}}})
	}))
	defer fake.Close()
	s := store(t)
	g := NewGemini(s)
	g.URL = fake.URL

	// Off by default: no call, no audio.
	if b, _, err := g.Speak(context.Background(), "hola"); b != nil || err != nil {
		t.Fatal("espeak is the default", b, err)
	}
	if _, err := s.Set(map[string]any{"voice.tts": "gemini", "voice.gemini_key": "k1", "voice.gemini_voice": "Kore",
		"voice.gemini_style": "alegre"}, config.SourceWeb, nil); err != nil {
		t.Fatal(err)
	}
	b, rate, err := g.Speak(context.Background(), "hola")
	if err != nil || rate != 24000 || string(b) != string(pcm) {
		t.Fatal(b, rate, err)
	}
	if gotPath != "/models/gemini-3.8-flash-tts:generateContent" || gotKey != "k1" || gotVoice != "Kore" ||
		gotText != "### DIRECTOR NOTES\nStyle: alegre\n### TRANSCRIPT\nhola" {
		t.Fatal(gotPath, gotKey, gotVoice, gotText)
	}
}

func TestGeminiError(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"quota"}`, http.StatusTooManyRequests)
	}))
	defer fake.Close()
	s := store(t)
	_, _ = s.Set(map[string]any{"voice.tts": "gemini", "voice.gemini_key": "k"}, config.SourceWeb, nil)
	g := NewGemini(s)
	g.URL = fake.URL
	if b, _, err := g.Speak(context.Background(), "hola"); err == nil || b != nil {
		t.Fatal("want an error so the head falls back to espeak")
	}
}

func TestToPCMFromWAV(t *testing.T) {
	wav := append([]byte("RIFF\x00\x00\x00\x00WAVEfmt \x10\x00\x00\x00\x01\x00\x01\x00\x80\x3e\x00\x00\x00\x7d\x00\x00\x02\x00\x10\x00data\x04\x00\x00\x00"), 9, 0, 8, 0)
	b, rate, err := toPCM(wav, "audio/wav")
	if err != nil || rate != 16000 || len(b) != 4 || b[0] != 9 {
		t.Fatal(b, rate, err)
	}
}

func TestGeminiPreview(t *testing.T) {
	var gotVoice string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if strings.Contains(string(b), `"voiceName":"Kore"`) {
			gotVoice = "Kore"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{
			map[string]any{"inlineData": map[string]string{"mimeType": "audio/L16;codec=pcm;rate=24000", "data": base64.StdEncoding.EncodeToString([]byte{1, 0, 2, 0})}}}}}}})
	}))
	defer fake.Close()
	s := store(t)
	g := NewGemini(s)
	g.URL = fake.URL
	if _, err := g.Preview(context.Background(), "Kore", "hola"); err != ErrNoKey {
		t.Fatal("no key:", err)
	}
	s.Set(map[string]any{"voice.gemini_key": "k"}, config.SourceWeb, nil) // voice.tts stays espeak: preview works anyway
	wav, err := g.Preview(context.Background(), "Kore", "hola")
	if err != nil || gotVoice != "Kore" || string(wav[:4]) != "RIFF" || len(wav) != 44+4 {
		t.Fatal(err, gotVoice, len(wav))
	}
	if _, err := g.Preview(context.Background(), "Nadie", "hola"); err == nil {
		t.Fatal("unknown voice accepted")
	}
}
