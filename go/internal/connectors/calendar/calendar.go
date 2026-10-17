// Package calendar is the calendar connector. Providers plug in behind Provider;
// Google Calendar is the first, CalDAV or Outlook can be added the same way.
package calendar

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connectors"
	"github.com/erickdsama/bb8/go/internal/tools"
)

type Event struct {
	Title    string    `json:"title"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	AllDay   bool      `json:"all_day,omitempty"`
	Location string    `json:"location,omitempty"`
}

// Provider is one calendar backend.
type Provider interface {
	Events(ctx context.Context, from, to time.Time) ([]Event, error)
	Add(ctx context.Context, e Event) (Event, error)
	Ready() (connectors.State, string) // ok, needs_auth or needs_config
}

// Factory builds a provider from the current config.
type Factory func(c *config.Store) Provider

type Calendar struct {
	Providers map[string]Factory
	Now       func() time.Time
}

func New() *Calendar {
	return &Calendar{Providers: map[string]Factory{"google": func(c *config.Store) Provider { return NewGoogle(c) }}, Now: time.Now}
}

func (c *Calendar) ID() string { return "calendar" }

func (c *Calendar) names() []string {
	var n []string
	for k := range c.Providers {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}

func (c *Calendar) Fields() []config.Field {
	return append([]config.Field{
		{Key: "connectors.calendar.provider", Label: "Provider", Type: config.Enum, Options: c.names(), Default: "google"},
	}, GoogleFields()...)
}

func (c *Calendar) provider(s *config.Store) (Provider, error) {
	f := c.Providers[s.String("connectors.calendar.provider")]
	if f == nil {
		return nil, errors.New("no calendar provider configured")
	}
	return f(s), nil
}

func (c *Calendar) Status(s *config.Store) connectors.Status {
	p, err := c.provider(s)
	if err != nil {
		return connectors.Status{ID: c.ID(), State: connectors.NeedsConf, Detail: err.Error()}
	}
	st, d := p.Ready()
	return connectors.Status{ID: c.ID(), State: st, Detail: d}
}

func location(s *config.Store) *time.Location {
	if l, err := time.LoadLocation(s.String("home.timezone")); err == nil {
		return l
	}
	return time.Local
}

func view(es []Event, loc *time.Location) []map[string]any {
	out := make([]map[string]any, 0, len(es))
	for _, e := range es {
		m := map[string]any{"title": e.Title, "start": e.Start.In(loc).Format("Mon 2 Jan 15:04"),
			"end": e.End.In(loc).Format("15:04")}
		if e.AllDay {
			m["start"], m["all_day"] = e.Start.In(loc).Format("Mon 2 Jan"), true
			delete(m, "end")
		}
		if e.Location != "" {
			m["location"] = e.Location
		}
		out = append(out, m)
	}
	return out
}

func (c *Calendar) Tools(s *config.Store) []tools.Tool {
	return []tools.Tool{
		{Name: "calendar_today", Description: "Events for today (or tomorrow with day=tomorrow), in home time.",
			Schema: tools.Object(map[string]any{"day": tools.Enum("default today", "today", "tomorrow")}),
			Handler: tools.Bind(func(ctx context.Context, in struct {
				Day string `json:"day"`
			}) (tools.Output, error) {
				p, err := c.provider(s)
				if err != nil {
					return tools.Output{}, err
				}
				loc := location(s)
				now := c.Now().In(loc)
				from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
				if in.Day == "tomorrow" {
					from = from.AddDate(0, 0, 1)
				}
				es, err := p.Events(ctx, from, from.AddDate(0, 0, 1))
				if err != nil {
					return tools.Output{}, err
				}
				return tools.Data(map[string]any{"day": from.Format("Mon 2 Jan"), "events": view(es, loc)})
			})},
		{Name: "calendar_next", Description: "The next events from now (up to 7 days ahead). count: 1 to 10, default 3.",
			Schema: tools.Object(map[string]any{"count": tools.Num("how many, 1 to 10")}),
			Handler: tools.Bind(func(ctx context.Context, in struct {
				Count int `json:"count"`
			}) (tools.Output, error) {
				p, err := c.provider(s)
				if err != nil {
					return tools.Output{}, err
				}
				n := min(max(in.Count, 1), 10)
				if in.Count == 0 {
					n = 3
				}
				now := c.Now()
				es, err := p.Events(ctx, now, now.AddDate(0, 0, 7))
				if err != nil {
					return tools.Output{}, err
				}
				if len(es) > n {
					es = es[:n]
				}
				return tools.Data(map[string]any{"events": view(es, location(s))})
			})},
		{Name: "calendar_add",
			Description: "Add an event when someone asks. start: \"2026-10-16 17:30\" in home time. duration_min default 60. " +
				"Repeat the title and time back before calling it.",
			Schema: tools.Object(map[string]any{"title": tools.Str("what"), "start": tools.Str("YYYY-MM-DD HH:MM, home time"),
				"duration_min": tools.Num("minutes, default 60")}, "title", "start"),
			Handler: tools.Bind(func(ctx context.Context, in struct {
				Title    string  `json:"title"`
				Start    string  `json:"start"`
				Duration float64 `json:"duration_min"`
			}) (tools.Output, error) {
				p, err := c.provider(s)
				if err != nil {
					return tools.Output{}, err
				}
				loc := location(s)
				st, err := time.ParseInLocation("2006-01-02 15:04", in.Start, loc)
				if err != nil {
					return tools.Output{}, fmt.Errorf("start must look like 2026-10-16 17:30: %w", err)
				}
				if in.Title == "" {
					return tools.Output{}, errors.New("missing title")
				}
				d := in.Duration
				if d <= 0 {
					d = 60
				}
				e, err := p.Add(ctx, Event{Title: in.Title, Start: st, End: st.Add(time.Duration(d) * time.Minute)})
				if err != nil {
					return tools.Output{}, err
				}
				return tools.Data(map[string]any{"result": "done", "event": view([]Event{e}, loc)[0]})
			})},
	}
}
