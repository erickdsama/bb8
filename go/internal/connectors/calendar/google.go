package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connections"
	"github.com/erickdsama/bb8/go/internal/connectors"
	"github.com/erickdsama/bb8/go/internal/tools"
)

// Google Calendar over REST (no SDK: keeps the binary small). Each person signs
// in with their own Google account on the Services page (a "google" account in
// the connections store); a call uses the account of whoever is speaking, else
// a shared one. All accounts sign in with one OAuth client that the owner
// registers in Google Cloud: its ID and secret are the settings below.
const (
	keyClientID     = "connectors.calendar.google_client_id"
	keyClientSecret = "connectors.calendar.google_client_secret"
	keyCalendarID   = "connectors.calendar.google_calendar_id"
)

// GoogleAPI is the Calendar endpoint; tests point it at a fake server.
var GoogleAPI = "https://www.googleapis.com/calendar/v3"

func GoogleFields() []config.Field {
	return []config.Field{
		{Key: keyClientID, Label: "Client ID", Type: config.String,
			Help: "Empty = the one built in (google_client.env)."},
		{Key: keyClientSecret, Label: "Client secret", Type: config.Secret},
		{Key: keyCalendarID, Label: "Calendar ID", Type: config.String, Default: "primary", Advanced: true},
	}
}

// The built-in Google client, baked in at build time from the gitignored
// google_client.env (see the Makefile), so nobody has to paste it in the page.
var builtinID, builtinSecret string

// GoogleClient is the OAuth client the sign-in uses (connections.Flows.Client):
// the settings when both are set, else the built-in one.
func GoogleClient(c *config.Store) (id, secret string) {
	id, secret = strings.TrimSpace(c.String(keyClientID)), strings.TrimSpace(c.Secret(keyClientSecret))
	if id == "" || secret == "" {
		return builtinID, builtinSecret
	}
	return id, secret
}

type Google struct {
	c     *config.Store
	accts *connections.Store
	owner func() string // home.owner: who the page, MCP clients and no voice ID act as
	HTTP  *http.Client
}

func NewGoogle(c *config.Store, accts *connections.Store, owner func() string) *Google {
	if owner == nil {
		owner = func() string { return "" }
	}
	return &Google{c: c, accts: accts, owner: owner, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func (g *Google) accounts() []connections.Connection {
	if g.accts == nil {
		return nil
	}
	return g.accts.OfKind(connections.KindGoogle)
}

func (g *Google) pick(o tools.CallOpts, write bool) (connections.Connection, bool) {
	person := o.Speaker
	if o.SpeakerKnown && person == "" {
		person = g.owner()
	}
	return connections.Pick(g.accounts(), o.SpeakerKnown, person, o.Admin, write)
}

func (g *Google) Serves(o tools.CallOpts, write bool) bool {
	_, ok := g.pick(o, write)
	return ok
}

// Confirm: ask before adding events if any account says so.
func (g *Google) Confirm() bool {
	for _, a := range g.accounts() {
		if a.Confirm {
			return true
		}
	}
	return false
}

func (g *Google) Ready() (connectors.State, string) {
	if id, secret := GoogleClient(g.c); id == "" || secret == "" {
		return connectors.NeedsConf, "set up the Google client (ID and secret)"
	}
	list := g.accounts()
	if len(list) == 0 {
		return connectors.NeedsAuth, "sign in with Google on the Services page"
	}
	var who []string
	for _, a := range list {
		o := a.Owner
		if o == "" {
			o = "shared"
		}
		who = append(who, strings.TrimSpace(a.Account+" ("+o+")"))
	}
	return connectors.OK, strings.Join(who, ", ")
}

func (g *Google) do(ctx context.Context, method, path string, q url.Values, body any, out any) error {
	acct, ok := g.pick(tools.CallerFrom(ctx), method != "GET")
	if !ok {
		return tools.ErrNotYours
	}
	return g.doAs(ctx, acct.ID, method, path, q, body, out)
}

// doAs calls the Calendar API with one account's token.
func (g *Google) doAs(ctx context.Context, acctID, method, path string, q url.Values, body any, out any) error {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	u := GoogleAPI + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, _ := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", "application/json")
	r, err := g.accts.Client(acctID, g.HTTP).Do(req) // signs and refreshes the token
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 300 {
		return fmt.Errorf("Google Calendar: HTTP %d", r.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(r.Body).Decode(out)
}

type gTime struct {
	DateTime string `json:"dateTime,omitempty"`
	Date     string `json:"date,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type gEvent struct {
	Summary  string `json:"summary"`
	Location string `json:"location,omitempty"`
	Start    gTime  `json:"start"`
	End      gTime  `json:"end"`
}

func (t gTime) parse(loc *time.Location) (time.Time, bool) {
	if t.DateTime != "" {
		v, _ := time.Parse(time.RFC3339, t.DateTime)
		return v, false
	}
	v, _ := time.ParseInLocation("2006-01-02", t.Date, loc)
	return v, true
}

func (g *Google) calPath() string {
	id := g.c.String(keyCalendarID)
	if id == "" {
		id = "primary"
	}
	return "/calendars/" + url.PathEscape(id) + "/events"
}

func (g *Google) Events(ctx context.Context, from, to time.Time) ([]Event, error) {
	var out struct {
		Items []gEvent `json:"items"`
	}
	q := url.Values{"timeMin": {from.Format(time.RFC3339)}, "timeMax": {to.Format(time.RFC3339)},
		"singleEvents": {"true"}, "orderBy": {"startTime"}, "maxResults": {"50"}}
	if err := g.do(ctx, "GET", g.calPath(), q, nil, &out); err != nil {
		return nil, err
	}
	loc := location(g.c)
	es := make([]Event, 0, len(out.Items))
	for _, it := range out.Items {
		s, all := it.Start.parse(loc)
		e, _ := it.End.parse(loc)
		es = append(es, Event{Title: it.Summary, Start: s, End: e, AllDay: all, Location: it.Location})
	}
	sort.Slice(es, func(i, j int) bool { return es[i].Start.Before(es[j].Start) })
	return es, nil
}

func (g *Google) Add(ctx context.Context, e Event) (Event, error) {
	tz := g.c.String("home.timezone")
	in := gEvent{Summary: e.Title, Location: e.Location,
		Start: gTime{DateTime: e.Start.Format(time.RFC3339), TimeZone: tz},
		End:   gTime{DateTime: e.End.Format(time.RFC3339), TimeZone: tz}}
	var out gEvent
	if err := g.do(ctx, "POST", g.calPath(), nil, in, &out); err != nil {
		return Event{}, err
	}
	s, _ := out.Start.parse(location(g.c))
	en, _ := out.End.parse(location(g.c))
	return Event{Title: out.Summary, Start: s, End: en}, nil
}
