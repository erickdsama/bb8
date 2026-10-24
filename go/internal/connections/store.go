// Package connections keeps the accounts BB-8 is connected to (Zapier, Notion,
// Home Assistant, any MCP server): who owns each one, how it signs in and its
// tokens. Stored in <data>/connections.json (0600: it holds tokens).
//
// An account has an owner (a person's name) or none (shared by the house). When
// someone talks, BB-8 uses that person's accounts plus the shared ones; see
// mcpclient. Tokens never leave bb8-body: the page only sees View.
package connections

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Auth is how bb8-body signs its requests to the server.
type Auth struct {
	Type         string    `json:"type"` // none, bearer, oauth, homeassistant
	Token        string    `json:"token,omitempty"`
	Refresh      string    `json:"refresh,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
	ClientID     string    `json:"client_id,omitempty"`
	ClientSecret string    `json:"client_secret,omitempty"`
	TokenURL     string    `json:"token_url,omitempty"`
	AuthMethod   string    `json:"auth_method,omitempty"` // none, client_secret_post, client_secret_basic
	Resource     string    `json:"resource,omitempty"`
}

type Connection struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"` // a Catalog key
	Name      string    `json:"name"` // tool prefix: zapier → zapier_gmail_send
	URL       string    `json:"url"`  // MCP endpoint
	Owner     string    `json:"owner"`
	Account   string    `json:"account,omitempty"`   // who it signed in as (email, user, home name)
	Strangers bool      `json:"strangers,omitempty"` // shared only: unknown voices may use its read-only tools
	Confirm   bool      `json:"confirm"`             // ask before sending, creating or deleting
	Auth      Auth      `json:"auth"`
	Created   time.Time `json:"created"`
}

// View is what the page sees: no tokens.
type View struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Owner     string `json:"owner"`
	Account   string `json:"account,omitempty"`
	Strangers bool   `json:"strangers"`
	Confirm   bool   `json:"confirm"`
	SignIn    string `json:"sign_in"` // none, token, oauth, homeassistant
}

func (c Connection) View() View {
	v := View{ID: c.ID, Kind: c.Kind, Label: c.Kind, Name: c.Name, URL: c.URL, Owner: c.Owner, Account: c.Account,
		Strangers: c.Strangers, Confirm: c.Confirm, SignIn: c.Auth.Type}
	if k, ok := Catalog[c.Kind]; ok {
		v.Label = k.Label
	}
	if c.Kind == "custom" && c.Name != "" { // "Other" says nothing: show the service's name
		v.Label = strings.ToUpper(c.Name[:1]) + c.Name[1:]
	}
	if v.SignIn == "bearer" {
		v.SignIn = "token"
	}
	return v
}

// Owns says whether this person's requests use this account. Names compare
// case-insensitively; an empty owner means shared.
func (c Connection) Owns(person string) bool {
	return c.Owner != "" && strings.EqualFold(strings.TrimSpace(c.Owner), strings.TrimSpace(person))
}

const MaxConnections = 12

var (
	ErrUnknown = errors.New("no such connection")
	ErrFull    = errors.New("too many connections")
)

type Store struct {
	path string

	mu    sync.Mutex
	conns map[string]*Connection
	subs  []func()
}

// Open loads <dir>/connections.json; an empty dir keeps everything in memory (tests).
func Open(dir string) (*Store, error) {
	s := &Store{conns: map[string]*Connection{}}
	if dir == "" {
		return s, nil
	}
	s.path = filepath.Join(dir, "connections.json")
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
	var list []*Connection
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, err
	}
	for _, c := range list {
		if c.Kind == "custom" && IsHomeAssistant(c.URL) { // linked before Home Assistant links were named
			c.Kind, c.Name = "home", "home"
		}
		s.conns[c.ID] = c
	}
	return s, nil
}

// Subscribe is called after every change (add, update, remove, token refresh excluded).
func (s *Store) Subscribe(f func()) {
	s.mu.Lock()
	s.subs = append(s.subs, f)
	s.mu.Unlock()
}

func (s *Store) notify() {
	s.mu.Lock()
	subs := append([]func(){}, s.subs...)
	s.mu.Unlock()
	for _, f := range subs {
		f()
	}
}

// persist writes atomically. Called with mu held.
func (s *Store) persist() error {
	if s.path == "" {
		return nil
	}
	b, _ := json.MarshalIndent(s.sorted(), "", "  ")
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) sorted() []*Connection {
	out := make([]*Connection, 0, len(s.conns))
	for _, c := range s.conns {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out
}

// List returns copies, oldest first.
func (s *Store) List() []Connection {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Connection
	for _, c := range s.sorted() {
		out = append(out, *c)
	}
	return out
}

func (s *Store) Get(id string) (Connection, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.conns[id]
	if !ok {
		return Connection{}, false
	}
	return *c, true
}

var badName = regexp.MustCompile(`[^a-z0-9]+`)

// CleanName makes a tool prefix: lower-case letters and digits, max 16.
func CleanName(s string) string {
	n := badName.ReplaceAllString(strings.ToLower(s), "")
	if len(n) > 16 {
		n = n[:16]
	}
	if n == "" {
		n = "app"
	}
	return n
}

// Add stores a new connection and gives it an id.
func (s *Store) Add(c Connection) (Connection, error) {
	s.mu.Lock()
	if len(s.conns) >= MaxConnections {
		s.mu.Unlock()
		return Connection{}, ErrFull
	}
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	c.ID = hex.EncodeToString(b)
	c.Name = CleanName(c.Name)
	if c.Created.IsZero() {
		c.Created = time.Now()
	}
	cp := c
	s.conns[c.ID] = &cp
	err := s.persist()
	s.mu.Unlock()
	s.notify()
	return c, err
}

// Update changes a connection with f. notify=false for token refreshes, which
// must not reconnect everything.
func (s *Store) Update(id string, notify bool, f func(*Connection)) (Connection, error) {
	s.mu.Lock()
	c, ok := s.conns[id]
	if !ok {
		s.mu.Unlock()
		return Connection{}, ErrUnknown
	}
	f(c)
	c.Name = CleanName(c.Name)
	out := *c
	err := s.persist()
	s.mu.Unlock()
	if notify {
		s.notify()
	}
	return out, err
}

func (s *Store) Remove(id string) error {
	s.mu.Lock()
	if _, ok := s.conns[id]; !ok {
		s.mu.Unlock()
		return ErrUnknown
	}
	delete(s.conns, id)
	err := s.persist()
	s.mu.Unlock()
	s.notify()
	return err
}
