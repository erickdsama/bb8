// Package config is the robot's single source of truth for settings.
//
// Every write goes through Store.Set(patch, source, baseRev): validate against
// the schema, bump rev, persist atomically, notify subscribers and log it in a
// short audit. The web UI, the agent (set_setting), calibration and the head
// all use that one path, so a change from anywhere shows up everywhere.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

type Type string

const (
	Number Type = "number"
	String Type = "string"
	Bool   Type = "bool"
	Enum   Type = "enum"
	Secret Type = "secret" // write-only from outside; stored in secrets.json
)

// Who may write a field. "web" and "system" (calibration, internal) may write
// everything; "agent" only fields that list it.
const (
	SourceWeb    = "web"
	SourceAgent  = "agent"
	SourceSystem = "system"
	SourceHead   = "head"
)

type Field struct {
	Key      string   `json:"key"`   // "motion.pwm_cruise"
	Label    string   `json:"label"` // short, for the form
	Help     string   `json:"help,omitempty"`
	Type     Type     `json:"type"`
	Default  any      `json:"default,omitempty"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Options  []string `json:"options,omitempty"`
	Agent    bool     `json:"agent"`              // the agent may write it with set_setting
	Restart  bool     `json:"restart"`            // applies after a restart, not live
	Advanced bool     `json:"advanced,omitempty"` // folded under "Advanced" in the form
}

func F(v float64) *float64 { return &v }

// Section returns the part of the key before the first dot ("motion").
func (f Field) Section() string { s, _, _ := strings.Cut(f.Key, "."); return s }

var (
	ErrConflict = errors.New("config changed meanwhile")
	ErrUnknown  = errors.New("unknown setting")
	ErrDenied   = errors.New("not writable by this source")
)

// Change is one applied write.
type Change struct {
	Rev    int64          `json:"rev"`
	Source string         `json:"source"`
	At     time.Time      `json:"at"`
	Values map[string]any `json:"values"` // secrets masked
}

type Store struct {
	dir string

	mu      sync.RWMutex
	fields  map[string]Field
	order   []string
	values  map[string]any
	secrets map[string]string
	rev     int64
	audit   []Change
	subs    map[int]func(Change)
	nextSub int
}

type file struct {
	Rev    int64          `json:"rev"`
	Values map[string]any `json:"values"`
}

// Open loads config.json and secrets.json from dir (both optional).
func Open(dir string) (*Store, error) {
	s := &Store{dir: dir, fields: map[string]Field{}, values: map[string]any{}, secrets: map[string]string{}, subs: map[int]func(Change){}}
	if dir == "" {
		return s, nil
	}
	if b, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
		var f file
		if err := json.Unmarshal(b, &f); err != nil {
			return nil, fmt.Errorf("config.json: %w", err)
		}
		s.rev, s.values = f.Rev, f.Values
		if s.values == nil {
			s.values = map[string]any{}
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "secrets.json")); err == nil {
		if err := json.Unmarshal(b, &s.secrets); err != nil {
			return nil, fmt.Errorf("secrets.json: %w", err)
		}
	}
	return s, nil
}

// Register adds fields to the schema (robot core and each connector).
func (s *Store) Register(fs ...Field) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range fs {
		if _, ok := s.fields[f.Key]; !ok {
			s.order = append(s.order, f.Key)
		}
		s.fields[f.Key] = f
	}
}

func (s *Store) Schema() []Field {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Field, 0, len(s.order))
	for _, k := range s.order {
		out = append(out, s.fields[k])
	}
	return out
}

func (s *Store) Rev() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rev
}

// Get returns a value (or its default). Secrets are not returned here.
func (s *Store) Get(key string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.get(key)
}

func (s *Store) get(key string) any {
	if v, ok := s.values[key]; ok {
		return v
	}
	return s.fields[key].Default
}

func (s *Store) Float(key string) float64 {
	switch v := s.Get(key).(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}

func (s *Store) String(key string) string { v, _ := s.Get(key).(string); return v }
func (s *Store) Bool(key string) bool     { v, _ := s.Get(key).(bool); return v }

func (s *Store) Secret(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.secrets[key]
}

// Snapshot is what the web sees: every field's value; secrets as "set"/"".
func (s *Store) Snapshot() (map[string]any, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]any{}
	for _, k := range s.order {
		if s.fields[k].Type == Secret {
			out[k] = mask(s.secrets[k])
		} else {
			out[k] = s.get(k)
		}
	}
	return out, s.rev
}

// Section returns the values of one section without the prefix ("head" → {"volume": …}).
func (s *Store) Section(name string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]any{}
	for _, k := range s.order {
		if rest, ok := strings.CutPrefix(k, name+"."); ok && s.fields[k].Type != Secret {
			out[rest] = s.get(k)
		}
	}
	return out
}

func mask(v string) string {
	if v == "" {
		return ""
	}
	return "set"
}

func (s *Store) validate(f Field, v any) (any, error) {
	switch f.Type {
	case Number:
		var x float64
		switch n := v.(type) {
		case float64:
			x = n
		case int:
			x = float64(n)
		case json.Number:
			var err error
			if x, err = n.Float64(); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%s must be a number", f.Key)
		}
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, fmt.Errorf("%s must be a number", f.Key)
		}
		if (f.Min != nil && x < *f.Min) || (f.Max != nil && x > *f.Max) {
			return nil, fmt.Errorf("%s must be between %g and %g", f.Key, deref(f.Min, math.Inf(-1)), deref(f.Max, math.Inf(1)))
		}
		return x, nil
	case Bool:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("%s must be true or false", f.Key)
		}
		return b, nil
	case String, Secret:
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be text", f.Key)
		}
		if len(str) > 4096 {
			return nil, fmt.Errorf("%s is too long", f.Key)
		}
		return strings.TrimSpace(str), nil
	case Enum:
		str, ok := v.(string)
		if !ok || !slices.Contains(f.Options, str) {
			return nil, fmt.Errorf("%s must be one of %s", f.Key, strings.Join(f.Options, ", "))
		}
		return str, nil
	}
	return nil, fmt.Errorf("%s: bad type", f.Key)
}

func deref(p *float64, d float64) float64 {
	if p == nil {
		return d
	}
	return *p
}

// Set validates and applies a patch atomically: all keys or none. baseRev, if
// given, must match the current rev (optimistic concurrency for the web UI).
func (s *Store) Set(patch map[string]any, source string, baseRev *int64) (int64, error) {
	s.mu.Lock()
	if baseRev != nil && *baseRev != s.rev {
		rev := s.rev
		s.mu.Unlock()
		return rev, ErrConflict
	}
	clean := map[string]any{}
	secrets := map[string]string{}
	for k, v := range patch {
		f, ok := s.fields[k]
		if !ok {
			s.mu.Unlock()
			return s.rev, fmt.Errorf("%w: %s", ErrUnknown, k)
		}
		if source == SourceAgent && !f.Agent {
			s.mu.Unlock()
			return s.rev, fmt.Errorf("%w: %s", ErrDenied, k)
		}
		x, err := s.validate(f, v)
		if err != nil {
			s.mu.Unlock()
			return s.rev, err
		}
		if f.Type == Secret {
			secrets[k] = x.(string)
		} else {
			clean[k] = x
		}
	}
	if len(clean) == 0 && len(secrets) == 0 {
		rev := s.rev
		s.mu.Unlock()
		return rev, nil
	}
	oldVals, oldSecrets, oldRev := cloneAny(s.values), cloneStr(s.secrets), s.rev
	for k, v := range clean {
		s.values[k] = v
	}
	for k, v := range secrets {
		if v == "" {
			delete(s.secrets, k)
		} else {
			s.secrets[k] = v
		}
	}
	s.rev++
	if err := s.persist(len(secrets) > 0); err != nil {
		s.values, s.secrets, s.rev = oldVals, oldSecrets, oldRev
		s.mu.Unlock()
		return oldRev, err
	}
	shown := map[string]any{}
	for k, v := range clean {
		shown[k] = v
	}
	for k, v := range secrets {
		shown[k] = mask(v)
	}
	ch := Change{Rev: s.rev, Source: source, At: time.Now(), Values: shown}
	s.audit = append(s.audit, ch)
	if len(s.audit) > 50 {
		s.audit = s.audit[len(s.audit)-50:]
	}
	subs := make([]func(Change), 0, len(s.subs))
	ids := make([]int, 0, len(s.subs))
	for id := range s.subs {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		subs = append(subs, s.subs[id])
	}
	rev := s.rev
	s.mu.Unlock()
	for _, f := range subs {
		f(ch)
	}
	return rev, nil
}

func cloneAny(m map[string]any) map[string]any {
	o := make(map[string]any, len(m))
	for k, v := range m {
		o[k] = v
	}
	return o
}

func cloneStr(m map[string]string) map[string]string {
	o := make(map[string]string, len(m))
	for k, v := range m {
		o[k] = v
	}
	return o
}

func writeAtomic(path string, v any, mode os.FileMode) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) persist(withSecrets bool) error {
	if s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(s.dir, "config.json"), file{Rev: s.rev, Values: s.values}, 0o644); err != nil {
		return err
	}
	if withSecrets {
		return writeAtomic(filepath.Join(s.dir, "secrets.json"), s.secrets, 0o600)
	}
	return nil
}

// Subscribe calls f after every applied change. Returns an unsubscribe func.
func (s *Store) Subscribe(f func(Change)) func() {
	s.mu.Lock()
	id := s.nextSub
	s.nextSub++
	s.subs[id] = f
	s.mu.Unlock()
	return func() { s.mu.Lock(); delete(s.subs, id); s.mu.Unlock() }
}

func (s *Store) Audit() []Change {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.audit)
}

// Touches reports whether a change includes any key with the prefix.
func (c Change) Touches(prefix string) bool {
	for k := range c.Values {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}
