package calendar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
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

	mu   sync.Mutex
	cals map[string]calCache // account ID → its calendars
}

// gCal is one calendar of an account.
type gCal struct {
	ID      string
	Summary string
	Main    bool
	Write   bool
}

type calCache struct {
	at      time.Time
	list    []gCal
	limited bool // signed in before BB-8 asked for the calendar list: only the main calendar
}

func NewGoogle(c *config.Store, accts *connections.Store, owner func() string) *Google {
	if owner == nil {
		owner = func() string { return "" }
	}
	return &Google{c: c, accts: accts, owner: owner, HTTP: &http.Client{Timeout: 10 * time.Second}, cals: map[string]calCache{}}
}

func (g *Google) accounts() []connections.Connection {
	if g.accts == nil {
		return nil
	}
	return g.accts.OfKind(connections.KindGoogle)
}

// serving: every account this caller reads, oldest first (all of a person's own).
func (g *Google) serving(o tools.CallOpts) []connections.Connection {
	person := o.Speaker
	if o.SpeakerKnown && person == "" {
		person = g.owner()
	}
	return connections.PickAll(g.accounts(), o.SpeakerKnown, person, o.Admin)
}

func (g *Google) Serves(o tools.CallOpts, write bool) bool {
	if write && !o.SpeakerKnown {
		return false
	}
	return len(g.serving(o)) > 0
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
	g.mu.Lock()
	for _, a := range list {
		o := a.Owner
		if o == "" {
			o = "shared"
		}
		w := strings.TrimSpace(a.Account + " (" + o + ")")
		if g.cals[a.ID].limited {
			w += " · sign in again to see all calendars"
		}
		who = append(who, w)
	}
	g.mu.Unlock()
	return connectors.OK, strings.Join(who, ", ")
}

var errNoScope = errors.New("Google Calendar: no calendar permission (sign in again and tick the calendar box)")

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
		var e struct {
			Error struct {
				Message string `json:"message"`
				Errors  []struct {
					Reason string `json:"reason"`
				} `json:"errors"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&e)
		reason := ""
		if len(e.Error.Errors) > 0 {
			reason = e.Error.Errors[0].Reason
		}
		switch reason {
		case "accessNotConfigured":
			return errors.New("Google Calendar: the Calendar API is off in the Google Cloud project (enable it)")
		case "insufficientPermissions":
			return errNoScope
		}
		return fmt.Errorf("Google Calendar: HTTP %d %s %s", r.StatusCode, reason, e.Error.Message)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(r.Body).Decode(out)
}

func (g *Google) mainID() string {
	if id := g.c.String(keyCalendarID); id != "" {
		return id
	}
	return "primary"
}

// calendars lists an account's calendars: the ones shown in Google Calendar
// (selected, not hidden) that BB-8 may read. Cached for 10 minutes.
func (g *Google) calendars(ctx context.Context, acctID string) []gCal {
	g.mu.Lock()
	cc, ok := g.cals[acctID]
	g.mu.Unlock()
	if ok && time.Since(cc.at) < 10*time.Minute {
		return cc.list
	}
	var out struct {
		Items []struct {
			ID              string `json:"id"`
			Summary         string `json:"summary"`
			SummaryOverride string `json:"summaryOverride"`
			Primary         bool   `json:"primary"`
			Selected        bool   `json:"selected"`
			Hidden          bool   `json:"hidden"`
			AccessRole      string `json:"accessRole"`
		} `json:"items"`
	}
	main := []gCal{{ID: g.mainID(), Main: true, Write: true}}
	err := g.doAs(ctx, acctID, "GET", "/users/me/calendarList", url.Values{"maxResults": {"100"}}, nil, &out)
	cc = calCache{at: time.Now(), list: main, limited: errors.Is(err, errNoScope)}
	if err == nil {
		var list []gCal
		for _, it := range out.Items {
			if it.Hidden || it.AccessRole == "freeBusyReader" || (!it.Selected && !it.Primary) {
				continue
			}
			name := it.SummaryOverride
			if name == "" {
				name = it.Summary
			}
			c := gCal{ID: it.ID, Summary: name, Main: it.Primary, Write: it.AccessRole == "owner" || it.AccessRole == "writer"}
			if c.Main { // main first
				list = append([]gCal{c}, list...)
			} else {
				list = append(list, c)
			}
		}
		if len(list) > 0 {
			cc.list = list
		}
	} else if !cc.limited {
		cc.at = time.Now().Add(-9 * time.Minute) // a passing error: ask again in a minute
	}
	g.mu.Lock()
	g.cals[acctID] = cc
	g.mu.Unlock()
	return cc.list
}

type gTime struct {
	DateTime string `json:"dateTime,omitempty"`
	Date     string `json:"date,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type gEvent struct {
	ID       string `json:"id,omitempty"`
	ICalUID  string `json:"iCalUID,omitempty"`
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

func calPath(id string) string {
	return "/calendars/" + url.PathEscape(id) + "/events"
}

// Events reads every calendar of every account the caller has, merged by start.
// An event on two calendars (an invitation in two accounts) shows once.
func (g *Google) Events(ctx context.Context, from, to time.Time) ([]Event, error) {
	accts := g.serving(tools.CallerFrom(ctx))
	if len(accts) == 0 {
		return nil, tools.ErrNotYours
	}
	type src struct {
		acct connections.Connection
		cal  gCal
	}
	var srcs []src
	for _, a := range accts {
		for _, c := range g.calendars(ctx, a.ID) {
			srcs = append(srcs, src{a, c})
		}
	}
	loc := location(g.c)
	seen := map[string]bool{}
	var es []Event
	var firstErr error
	ok := 0
	for _, sr := range srcs {
		var out struct {
			Items []gEvent `json:"items"`
		}
		q := url.Values{"timeMin": {from.Format(time.RFC3339)}, "timeMax": {to.Format(time.RFC3339)},
			"singleEvents": {"true"}, "orderBy": {"startTime"}, "maxResults": {"50"}}
		if err := g.doAs(ctx, sr.acct.ID, "GET", calPath(sr.cal.ID), q, nil, &out); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		ok++
		for _, it := range out.Items {
			s, all := it.Start.parse(loc)
			e, _ := it.End.parse(loc)
			uid := it.ICalUID
			if uid == "" {
				uid = sr.cal.ID + "/" + it.ID
			}
			if k := uid + "|" + s.Format(time.RFC3339); seen[k] {
				continue
			} else {
				seen[k] = true
			}
			ev := Event{Title: it.Summary, Start: s, End: e, AllDay: all, Location: it.Location}
			if len(srcs) > 1 {
				ev.Calendar = sr.cal.Summary
				if ev.Calendar == "" || (len(accts) > 1 && sr.cal.Main) {
					ev.Calendar = sr.acct.Account
				}
			}
			es = append(es, ev)
		}
	}
	if ok == 0 && firstErr != nil {
		return nil, firstErr
	}
	sort.SliceStable(es, func(i, j int) bool { return es[i].Start.Before(es[j].Start) })
	return es, nil
}

// target picks where a new event goes: the calendar named in e.Calendar
// (an account's e-mail or a calendar's name, loosely), else the main calendar
// of the person's first (oldest) account.
func (g *Google) target(ctx context.Context, want string) (connections.Connection, gCal, error) {
	accts := g.serving(tools.CallerFrom(ctx))
	if len(accts) == 0 {
		return connections.Connection{}, gCal{}, tools.ErrNotYours
	}
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return accts[0], gCal{ID: g.mainID(), Main: true, Write: true}, nil
	}
	var names []string
	for _, a := range accts {
		if strings.Contains(strings.ToLower(a.Account), want) {
			return a, gCal{ID: g.mainID(), Main: true, Write: true}, nil
		}
		for _, c := range g.calendars(ctx, a.ID) {
			if !c.Write {
				continue
			}
			if c.Summary != "" && strings.Contains(strings.ToLower(c.Summary), want) {
				return a, c, nil
			}
			names = append(names, c.Summary)
		}
		names = append(names, a.Account)
	}
	return connections.Connection{}, gCal{}, fmt.Errorf("no calendar called %q; use one of: %s", want, strings.Join(names, ", "))
}

func (g *Google) Add(ctx context.Context, e Event) (Event, error) {
	a, cal, err := g.target(ctx, e.Calendar)
	if err != nil {
		return Event{}, err
	}
	tz := g.c.String("home.timezone")
	in := gEvent{Summary: e.Title, Location: e.Location,
		Start: gTime{DateTime: e.Start.Format(time.RFC3339), TimeZone: tz},
		End:   gTime{DateTime: e.End.Format(time.RFC3339), TimeZone: tz}}
	var out gEvent
	if err := g.doAs(ctx, a.ID, "POST", calPath(cal.ID), nil, in, &out); err != nil {
		return Event{}, err
	}
	s, _ := out.Start.parse(location(g.c))
	en, _ := out.End.parse(location(g.c))
	name := cal.Summary
	if cal.Main || name == "" {
		name = a.Account
	}
	return Event{Title: out.Summary, Start: s, End: en, Calendar: name}, nil
}
