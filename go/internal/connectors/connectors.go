// Package connectors holds optional integrations (weather, calendar, later Nest…).
// Each connector declares its settings and tools; the Manager registers the tools
// of enabled connectors and re-registers them live when their settings change.
package connectors

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/tools"
)

type State string

const (
	OK        State = "ok"
	NeedsAuth State = "needs_auth"
	NeedsConf State = "needs_config"
	Error     State = "error"
	Disabled  State = "disabled"
)

type Status struct {
	ID          string   `json:"id"`
	State       State    `json:"state"`
	Detail      string   `json:"detail,omitempty"`
	Title       string   `json:"title,omitempty"`       // filled by the Manager
	Description string   `json:"description,omitempty"` // filled by the Manager
	Tools       []string `json:"tools,omitempty"`       // registered now; filled by the Manager
}

type Connector interface {
	ID() string
	Fields() []config.Field             // shown in the web form; keys start with "connectors.<id>."
	Tools(c *config.Store) []tools.Tool // added to the registry → agent and MCP
	Status(c *config.Store) Status
}

// Describer gives the config page a name and one line on what the connector does.
type Describer interface {
	Title() string
	Description() string
}

// DefaultOn is implemented by connectors that start enabled (no account needed).
type DefaultOn interface{ DefaultOn() bool }

// Closer is implemented by connectors that hold connections (MCP sessions).
type Closer interface{ Close() }

// EnabledKey is the on/off switch every connector has.
func EnabledKey(id string) string { return "connectors." + id + ".enabled" }

type Manager struct {
	cfg  *config.Store
	reg  *tools.Registry
	list []Connector
	mu   map[string]*sync.Mutex // one apply at a time per connector

	toolsMu sync.Mutex
	tools   map[string][]string // id → tool names registered now
}

func NewManager(cfg *config.Store, reg *tools.Registry, cs ...Connector) *Manager {
	m := &Manager{cfg: cfg, reg: reg, list: cs, mu: map[string]*sync.Mutex{}, tools: map[string][]string{}}
	for _, c := range cs {
		m.mu[c.ID()] = &sync.Mutex{}
		on := false
		if d, ok := c.(DefaultOn); ok {
			on = d.DefaultOn()
		}
		cfg.Register(config.Field{Key: EnabledKey(c.ID()), Label: "Enabled", Type: config.Bool, Default: on})
		cfg.Register(c.Fields()...)
	}
	return m
}

// Start registers tools now and again whenever a connector's settings (or home.*)
// change. Each apply runs in its own goroutine: an MCP server that is slow or
// offline must not hold up the boot or a config write.
func (m *Manager) Start() {
	for _, c := range m.list {
		go m.apply(c)
	}
	m.cfg.Subscribe(func(ch config.Change) {
		for _, c := range m.list {
			if ch.Touches("connectors."+c.ID()+".") || ch.Touches("home.") {
				go m.apply(c)
			}
		}
	})
}

func (m *Manager) apply(c Connector) {
	mu := m.mu[c.ID()]
	mu.Lock()
	defer mu.Unlock()
	owner := "connector:" + c.ID()
	if !m.cfg.Bool(EnabledKey(c.ID())) {
		m.reg.Set(owner, nil)
		m.setTools(c.ID(), nil)
		if cl, ok := c.(Closer); ok {
			cl.Close()
		}
		return
	}
	ts := c.Tools(m.cfg)
	m.reg.Set(owner, ts)
	names := make([]string, len(ts))
	for i, t := range ts {
		names[i] = t.Name
	}
	m.setTools(c.ID(), names)
	slog.Info("connector on", "id", c.ID(), "tools", strings.Join(names, ","))
}

// Refresh re-applies one connector (its accounts changed outside the config).
func (m *Manager) Refresh(id string) {
	if c := m.Get(id); c != nil {
		go m.apply(c)
	}
}

func (m *Manager) setTools(id string, names []string) {
	m.toolsMu.Lock()
	m.tools[id] = names
	m.toolsMu.Unlock()
}

func (m *Manager) Status() []Status {
	out := make([]Status, 0, len(m.list))
	for _, c := range m.list {
		st := Status{ID: c.ID(), State: Disabled}
		if m.cfg.Bool(EnabledKey(c.ID())) {
			st = c.Status(m.cfg)
			m.toolsMu.Lock()
			st.Tools = m.tools[c.ID()]
			m.toolsMu.Unlock()
		}
		st.Title = c.ID()
		if d, ok := c.(Describer); ok {
			st.Title, st.Description = d.Title(), d.Description()
		}
		out = append(out, st)
	}
	return out
}

func (m *Manager) Get(id string) Connector {
	for _, c := range m.list {
		if c.ID() == id {
			return c
		}
	}
	return nil
}
