package calendar

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connectors"
)

// Google Calendar over REST with an OAuth refresh token (no SDK: keeps the binary small).
const (
	keyClientID     = "connectors.calendar.google_client_id"
	keyClientSecret = "connectors.calendar.google_client_secret"
	keyRefresh      = "connectors.calendar.google_refresh_token"
	keyCalendarID   = "connectors.calendar.google_calendar_id"
	keyRedirect     = "connectors.calendar.google_redirect_url"
	scope           = "https://www.googleapis.com/auth/calendar.events"
)

// Endpoints; tests point them at a fake server.
var (
	GoogleAPI   = "https://www.googleapis.com/calendar/v3"
	GoogleToken = "https://oauth2.googleapis.com/token"
	GoogleAuth  = "https://accounts.google.com/o/oauth2/v2/auth"
)

func GoogleFields() []config.Field {
	return []config.Field{
		{Key: keyClientID, Label: "Google OAuth client ID", Type: config.String,
			Help: "Google Cloud console → APIs → Credentials → OAuth client (Web application)."},
		{Key: keyClientSecret, Label: "Google OAuth client secret", Type: config.Secret},
		{Key: keyRedirect, Label: "OAuth redirect URL", Type: config.String,
			Default: "http://localhost:8765/api/connectors/calendar/callback",
			Help:    "Register it in the OAuth client. Open the config page through an SSH tunnel to use localhost."},
		{Key: keyRefresh, Label: "Google refresh token", Type: config.Secret, Help: "Filled by \"Connect Google\"."},
		{Key: keyCalendarID, Label: "Calendar ID", Type: config.String, Default: "primary"},
	}
}

var tokens = struct {
	sync.Mutex
	m map[string]cachedToken
}{m: map[string]cachedToken{}}

type cachedToken struct {
	access string
	exp    time.Time
}

type Google struct {
	c    *config.Store
	HTTP *http.Client
}

func NewGoogle(c *config.Store) *Google {
	return &Google{c: c, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func (g *Google) Ready() (connectors.State, string) {
	if g.c.String(keyClientID) == "" || g.c.Secret(keyClientSecret) == "" {
		return connectors.NeedsConf, "set the Google OAuth client ID and secret"
	}
	if g.c.Secret(keyRefresh) == "" {
		return connectors.NeedsAuth, "open /api/connectors/calendar/auth to connect Google"
	}
	return connectors.OK, ""
}

func (g *Google) token(ctx context.Context) (string, error) {
	refresh := g.c.Secret(keyRefresh)
	if refresh == "" {
		return "", errors.New("Google calendar not connected yet")
	}
	tokens.Lock()
	t := tokens.m[refresh]
	tokens.Unlock()
	if t.access != "" && time.Until(t.exp) > time.Minute {
		return t.access, nil
	}
	var out struct {
		Access    string `json:"access_token"`
		ExpiresIn int    `json:"expires_in"`
		Error     string `json:"error"`
	}
	if err := postForm(ctx, g.HTTP, GoogleToken, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh},
		"client_id": {g.c.String(keyClientID)}, "client_secret": {g.c.Secret(keyClientSecret)}}, &out); err != nil {
		return "", err
	}
	if out.Access == "" {
		return "", fmt.Errorf("Google token refresh failed: %s", out.Error)
	}
	tokens.Lock()
	tokens.m[refresh] = cachedToken{out.Access, time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)}
	tokens.Unlock()
	return out.Access, nil
}

func postForm(ctx context.Context, h *http.Client, u string, form url.Values, v any) error {
	req, _ := http.NewRequestWithContext(ctx, "POST", u, bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r, err := h.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func (g *Google) do(ctx context.Context, method, path string, q url.Values, body any, out any) error {
	tok, err := g.token(ctx)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	u := GoogleAPI + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, _ := http.NewRequestWithContext(ctx, method, u, &buf)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r, err := g.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 300 {
		return fmt.Errorf("Google Calendar: HTTP %d", r.StatusCode)
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

// --- OAuth (web) -------------------------------------------------------------

var oauthState = struct {
	sync.Mutex
	v string
}{}

// AuthHandlers serve /api/connectors/calendar/auth and /callback. The callback
// stores the refresh token as a secret through the config store.
func AuthHandlers(c *config.Store, mux *http.ServeMux) {
	mux.HandleFunc("GET /api/connectors/calendar/auth", func(w http.ResponseWriter, r *http.Request) {
		if c.String(keyClientID) == "" {
			http.Error(w, "set the Google OAuth client ID and secret first", 400)
			return
		}
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		oauthState.Lock()
		oauthState.v = hex.EncodeToString(b)
		st := oauthState.v
		oauthState.Unlock()
		q := url.Values{"client_id": {c.String(keyClientID)}, "redirect_uri": {c.String(keyRedirect)},
			"response_type": {"code"}, "scope": {scope}, "access_type": {"offline"}, "prompt": {"consent"}, "state": {st}}
		http.Redirect(w, r, GoogleAuth+"?"+q.Encode(), http.StatusFound)
	})
	mux.HandleFunc("GET /api/connectors/calendar/callback", func(w http.ResponseWriter, r *http.Request) {
		oauthState.Lock()
		want := oauthState.v
		oauthState.v = ""
		oauthState.Unlock()
		if want == "" || r.URL.Query().Get("state") != want {
			http.Error(w, "bad state; start again", 400)
			return
		}
		var out struct {
			Refresh string `json:"refresh_token"`
			Error   string `json:"error"`
		}
		err := postForm(r.Context(), http.DefaultClient, GoogleToken, url.Values{"grant_type": {"authorization_code"},
			"code": {r.URL.Query().Get("code")}, "redirect_uri": {c.String(keyRedirect)},
			"client_id": {c.String(keyClientID)}, "client_secret": {c.Secret(keyClientSecret)}}, &out)
		if err != nil || out.Refresh == "" {
			http.Error(w, fmt.Sprintf("Google did not return a refresh token: %v %s", err, out.Error), 502)
			return
		}
		if _, err := c.Set(map[string]any{keyRefresh: out.Refresh}, config.SourceWeb, nil); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	})
}
