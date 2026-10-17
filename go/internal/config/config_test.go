package config

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newStore(t *testing.T) *Store {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.Register(
		Field{Key: "motion.pwm_cruise", Type: Number, Default: 140.0, Min: F(0), Max: F(160)},
		Field{Key: "head.volume", Type: Number, Default: 0.7, Min: F(0), Max: F(1), Agent: true},
		Field{Key: "agent.api_key", Type: Secret},
		Field{Key: "agent.model", Type: Enum, Options: []string{"a", "b"}, Default: "a"},
	)
	return s
}

func TestSetValidatesAndPersists(t *testing.T) {
	s := newStore(t)
	if _, err := s.Set(map[string]any{"motion.pwm_cruise": 999.0}, SourceWeb, nil); err == nil {
		t.Fatal("range not enforced")
	}
	if _, err := s.Set(map[string]any{"agent.model": "zzz"}, SourceWeb, nil); err == nil {
		t.Fatal("enum not enforced")
	}
	rev, err := s.Set(map[string]any{"motion.pwm_cruise": 120.0, "agent.api_key": "sk-1"}, SourceWeb, nil)
	if err != nil || rev != 1 {
		t.Fatal(rev, err)
	}
	vals, _ := s.Snapshot()
	if vals["agent.api_key"] != "set" || s.Secret("agent.api_key") != "sk-1" {
		t.Fatal("secret not masked/stored", vals)
	}
	re, _ := Open(s.dir)
	re.Register(s.Schema()...)
	if re.Float("motion.pwm_cruise") != 120 || re.Rev() != 1 || re.Secret("agent.api_key") != "sk-1" {
		t.Fatal("not persisted")
	}
	if fi, _ := os.Stat(filepath.Join(s.dir, "secrets.json")); fi.Mode().Perm() != 0o600 {
		t.Fatal("secrets.json must be 0600")
	}
}

func TestAgentLimitsAndConflict(t *testing.T) {
	s := newStore(t)
	if _, err := s.Set(map[string]any{"motion.pwm_cruise": 100.0}, SourceAgent, nil); !errors.Is(err, ErrDenied) {
		t.Fatal("agent wrote a safety limit:", err)
	}
	var got []Change
	s.Subscribe(func(c Change) { got = append(got, c) })
	if _, err := s.Set(map[string]any{"head.volume": 0.3}, SourceAgent, nil); err != nil {
		t.Fatal(err)
	}
	stale := int64(0)
	if _, err := s.Set(map[string]any{"head.volume": 0.5}, SourceWeb, &stale); !errors.Is(err, ErrConflict) {
		t.Fatal("stale rev accepted")
	}
	if len(got) != 1 || got[0].Source != SourceAgent || !got[0].Touches("head.") {
		t.Fatal(got)
	}
	if s.Section("head")["volume"] != 0.3 {
		t.Fatal(s.Section("head"))
	}
}

func TestWebPatchConflict(t *testing.T) {
	s := newStore(t)
	srv := httptest.NewServer(Web(s, nil))
	defer srv.Close()
	patch := func(body string) (int, map[string]any) {
		req, _ := http.NewRequest("PATCH", srv.URL+"/api/config", strings.NewReader(body))
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(r.Body).Decode(&out)
		return r.StatusCode, out
	}
	if code, out := patch(`{"baseRev":0,"values":{"head.volume":0.4}}`); code != 200 || out["rev"].(float64) != 1 {
		t.Fatal(code, out)
	}
	if code, out := patch(`{"baseRev":0,"values":{"head.volume":0.9}}`); code != 409 || out["values"].(map[string]any)["head.volume"] != 0.4 {
		t.Fatal(code, out)
	}
}
