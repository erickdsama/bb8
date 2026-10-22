// Package people keeps the profiles of the people BB-8 knows: a role that sets
// what they may do, short notes the agent remembers about them, and when they
// were last heard. Stored in <data>/people.json.
//
// Roles:
//   - owner:  everything.
//   - family: move the robot and use external tools (the default for a registered voice).
//   - guest:  chat only; treated like an unknown voice for Moves and KnownOnly tools.
//
// The agent may add people (as guests) and notes. Only the web page changes roles.
package people

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Role string

const (
	Owner  Role = "owner"
	Family Role = "family"
	Guest  Role = "guest"
)

func ValidRole(r Role) bool { return r == Owner || r == Family || r == Guest }

const (
	MaxNotes   = 20
	MaxNoteLen = 200
	MaxPeople  = 50
)

type Note struct {
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

type Person struct {
	Name     string    `json:"name"`
	Role     Role      `json:"role"`
	Notes    []Note    `json:"notes"`
	Voice    bool      `json:"voice"` // heard through a registered voice at least once
	Created  time.Time `json:"created"`
	LastSeen time.Time `json:"last_seen,omitempty"`
}

// CanAct says whether this person may move the robot and use external tools.
func (p Person) CanAct() bool { return p.Role == Owner || p.Role == Family }

type Store struct {
	path string
	Now  func() time.Time

	mu     sync.Mutex
	people map[string]*Person // key: lower-case name
	subs   []func(Person, string)
}

var (
	ErrUnknown = errors.New("no such person")
	ErrFull    = errors.New("too many people")
)

func key(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// Open loads <dir>/people.json; an empty dir keeps everything in memory (tests).
func Open(dir string) (*Store, error) {
	s := &Store{people: map[string]*Person{}, Now: time.Now}
	if dir == "" {
		return s, nil
	}
	s.path = filepath.Join(dir, "people.json")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var list []*Person
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, err
	}
	for _, p := range list {
		s.people[key(p.Name)] = p
	}
	return s, nil
}

// Subscribe gets every change: the person and what happened (added, seen, note, role, removed).
func (s *Store) Subscribe(f func(Person, string)) {
	s.mu.Lock()
	s.subs = append(s.subs, f)
	s.mu.Unlock()
}

func (s *Store) changed(p *Person, what string, save bool) error {
	var err error
	if save {
		err = s.persist()
	}
	cp := clone(p)
	subs := append([]func(Person, string){}, s.subs...)
	s.mu.Unlock()
	for _, f := range subs {
		f(cp, what)
	}
	s.mu.Lock()
	return err
}

func clone(p *Person) Person {
	cp := *p
	cp.Notes = append([]Note{}, p.Notes...)
	return cp
}

// persist writes atomically. Called with mu held.
func (s *Store) persist() error {
	if s.path == "" {
		return nil
	}
	list := s.sorted()
	b, _ := json.MarshalIndent(list, "", "  ")
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) sorted() []*Person {
	list := make([]*Person, 0, len(s.people))
	for _, p := range s.people {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

func (s *Store) List() []Person {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Person{}
	for _, p := range s.sorted() {
		out = append(out, clone(p))
	}
	return out
}

func (s *Store) Get(name string) (Person, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.people[key(name)]
	if !ok {
		return Person{}, false
	}
	return clone(p), true
}

func (s *Store) add(name string, role Role) (*Person, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 40 {
		return nil, errors.New("name must be 1–40 characters")
	}
	if len(s.people) >= MaxPeople {
		return nil, ErrFull
	}
	p := &Person{Name: name, Role: role, Notes: []Note{}, Created: s.Now()}
	s.people[key(name)] = p
	return p, nil
}

// Seen records a registered voice. A name heard for the first time becomes family:
// registering a voice needs access to the robot, so it was the owner's choice.
func (s *Store) Seen(name string) (Person, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.people[key(name)]
	what := "seen"
	if !ok {
		var err error
		if p, err = s.add(name, Family); err != nil {
			return Person{}, err
		}
		what = "added"
	}
	p.Voice = true
	p.LastSeen = s.Now()
	err := s.changed(p, what, true)
	return clone(p), err
}

// Upsert creates or updates a person from the web page.
func (s *Store) Upsert(name string, role Role) (Person, error) {
	if !ValidRole(role) {
		return Person{}, errors.New("role must be owner, family or guest")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.people[key(name)]
	what := "role"
	if !ok {
		var err error
		if p, err = s.add(name, role); err != nil {
			return Person{}, err
		}
		what = "added"
	}
	p.Role = role
	err := s.changed(p, what, true)
	return clone(p), err
}

// Remember adds a note, creating the person as a guest if new.
func (s *Store) Remember(name, text string) (Person, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Person{}, errors.New("note is empty")
	}
	if len(text) > MaxNoteLen {
		text = text[:MaxNoteLen]
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.people[key(name)]
	if !ok {
		var err error
		if p, err = s.add(name, Guest); err != nil {
			return Person{}, err
		}
	}
	for _, n := range p.Notes {
		if strings.EqualFold(n.Text, text) {
			return clone(p), nil
		}
	}
	p.Notes = append(p.Notes, Note{Text: text, At: s.Now()})
	if len(p.Notes) > MaxNotes { // the oldest note goes first
		p.Notes = p.Notes[len(p.Notes)-MaxNotes:]
	}
	err := s.changed(p, "note", true)
	return clone(p), err
}

// Forget removes the notes that contain text (case-insensitive). It returns how many.
func (s *Store) Forget(name, text string) (int, error) {
	text = strings.ToLower(strings.TrimSpace(text))
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.people[key(name)]
	if !ok {
		return 0, ErrUnknown
	}
	if text == "" {
		return 0, errors.New("say which note")
	}
	kept := p.Notes[:0]
	n := 0
	for _, x := range p.Notes {
		if strings.Contains(strings.ToLower(x.Text), text) {
			n++
			continue
		}
		kept = append(kept, x)
	}
	p.Notes = kept
	if n == 0 {
		return 0, nil
	}
	return n, s.changed(p, "note", true)
}

func (s *Store) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.people[key(name)]
	if !ok {
		return ErrUnknown
	}
	delete(s.people, key(name))
	return s.changed(p, "removed", true)
}

// Summary is the one line the agent gets every turn: "Erick (owner), Ana (family)".
func (s *Store) Summary() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var parts []string
	for _, p := range s.sorted() {
		parts = append(parts, p.Name+" ("+string(p.Role)+")")
	}
	return strings.Join(parts, ", ")
}
