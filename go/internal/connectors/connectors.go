// Package connectors holds optional integrations (weather, calendar, later Nest…).
// Each connector declares its settings and tools; the Manager registers the tools
// of enabled connectors and re-registers them live when their settings change.
package connectors

import (
	"log/slog"
	"strings"

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
	ID     string `json:"id"`
	State  State  `json:"state"`
	Detail string `json:"detail,omitempty"`
}

type Connector interface {
	ID() string
	Fields() []config.Field             // shown in the web form; keys start with "connectors.<id>."
	Tools(c *config.Store) []tools.Tool // added to the registry → agent and MCP
	Status(c *config.Store) Status
}

// EnabledKey is the on/off switch every connector has.
func EnabledKey(id string) string { return "connectors." + id + ".enabled" }

type Manager struct {
	cfg  *config.Store
	reg  *tools.Registry
	list []Connector
}

func NewManager(cfg *config.Store, reg *tools.Registry, cs ...Connector) *Manager {
	m := &Manager{cfg: cfg, reg: reg, list: cs}
	for _, c := range cs {
		cfg.Register(config.Field{Key: EnabledKey(c.ID()), Label: "Enabled", Type: config.Bool, Default: false})
		cfg.Register(c.Fields()...)
	}
	return m
}

// Start registers tools now and again whenever a connector's settings (or home.*) change.
func (m *Manager) Start() {
	for _, c := range m.list {
		m.apply(c)
	}
	m.cfg.Subscribe(func(ch config.Change) {
		for _, c := range m.list {
			if ch.Touches("connectors."+c.ID()+".") || ch.Touches("home.") {
				m.apply(c)
			}
		}
	})
}

func (m *Manager) apply(c Connector) {
	owner := "connector:" + c.ID()
	if !m.cfg.Bool(EnabledKey(c.ID())) {
		m.reg.Set(owner, nil)
		return
	}
	ts := c.Tools(m.cfg)
	m.reg.Set(owner, ts)
	names := make([]string, len(ts))
	for i, t := range ts {
		names[i] = t.Name
	}
	slog.Info("connector on", "id", c.ID(), "tools", strings.Join(names, ","))
}

func (m *Manager) Status() []Status {
	out := make([]Status, 0, len(m.list))
	for _, c := range m.list {
		if !m.cfg.Bool(EnabledKey(c.ID())) {
			out = append(out, Status{ID: c.ID(), State: Disabled})
			continue
		}
		out = append(out, c.Status(m.cfg))
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
