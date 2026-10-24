package calendar

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connections"
	"github.com/erickdsama/bb8/go/internal/connectors"
	"github.com/erickdsama/bb8/go/internal/tools"
)

func store(t *testing.T) *config.Store {
	c, _ := config.Open(t.TempDir())
	c.Register(New(nil, nil).Fields()...)
	c.Register(config.Field{Key: "home.timezone", Type: config.String, Default: "America/Mexico_City"})
	return c
}

func TestGoogleEventsAndAdd(t *testing.T) {
	var added map[string]any
	refreshes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			refreshes++
			r.ParseForm()
			if r.Form.Get("refresh_token") != "rt-1" {
				t.Errorf("refresh %q", r.Form.Get("refresh_token"))
			}
			w.Write([]byte(`{"access_token":"at-1","expires_in":3600}`))
		case r.Header.Get("Authorization") != "Bearer at-1":
			w.WriteHeader(401)
		case r.Method == "GET":
			w.Write([]byte(`{"items":[
				{"summary":"Dentista","start":{"dateTime":"2026-10-16T17:30:00-06:00"},"end":{"dateTime":"2026-10-16T18:30:00-06:00"}},
				{"summary":"Cumple","start":{"date":"2026-10-16"},"end":{"date":"2026-10-17"}}]}`))
		default:
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &added)
			w.Write(b)
		}
	}))
	defer srv.Close()
	GoogleAPI = srv.URL
	c := store(t)
	accts, _ := connections.Open("")
	cal := New(accts, func() string { return "Erick" })
	cal.Now = func() time.Time { return time.Date(2026, 10, 16, 9, 0, 0, 0, time.FixedZone("CST", -6*3600)) }
	if st := cal.Status(c); st.State != connectors.NeedsConf {
		t.Fatal(st)
	}
	c.Set(map[string]any{keyClientID: "cid", keyClientSecret: "cs"}, config.SourceWeb, nil)
	if st := cal.Status(c); st.State != connectors.NeedsAuth {
		t.Fatal(st)
	}
	accts.Add(connections.Connection{Kind: connections.KindGoogle, Name: "google", Owner: "Erick", Account: "erick@example.com",
		Auth: connections.Auth{Type: "oauth", Refresh: "rt-1", ClientID: "cid", ClientSecret: "cs", AuthMethod: "client_secret_post", TokenURL: srv.URL + "/token"}})
	if st := cal.Status(c); st.State != connectors.OK || st.Detail != "erick@example.com (Erick)" {
		t.Fatal(st)
	}
	erick := tools.CallOpts{SpeakerKnown: true, Speaker: "Erick"}
	ts := map[string]tools.Tool{}
	for _, tl := range cal.Tools(c) {
		ts[tl.Name] = tl
	}
	call := func(name, args string, o tools.CallOpts) map[string]any {
		tl := ts[name]
		if !tl.For(o) {
			t.Fatal(name, "not for", o)
		}
		out, err := tl.Handler(tools.WithCaller(context.Background(), o), json.RawMessage(args))
		if err != nil {
			t.Fatal(name, err)
		}
		return out.Data.(map[string]any)
	}
	today := call("calendar_today", `{}`, erick)
	ev := today["events"].([]map[string]any)
	if len(ev) != 2 || ev[1]["title"] != "Dentista" || ev[1]["start"] != "Fri 16 Oct 17:30" || ev[0]["all_day"] != true {
		t.Fatal(today)
	}
	// The page (no speaker) acts as home.owner.
	res := call("calendar_add", `{"title":"Pasear a BB-8","start":"2026-10-16 19:00","duration_min":30}`, tools.CallOpts{SpeakerKnown: true})
	if res["result"] != "done" || !strings.HasPrefix(added["start"].(map[string]any)["dateTime"].(string), "2026-10-16T19:00:00-06:00") {
		t.Fatal(res, added)
	}
	if refreshes != 1 {
		t.Fatalf("token should be kept, refreshed %d times", refreshes)
	}

	// Someone else's calendar is not theirs, and strangers get nothing.
	ana, stranger := tools.CallOpts{SpeakerKnown: true, Speaker: "Ana"}, tools.CallOpts{}
	if ts["calendar_today"].For(ana) || ts["calendar_today"].For(stranger) || !ts["calendar_add"].KnownOnly {
		t.Fatal("Erick's calendar leaked")
	}
	// A shared calendar serves Ana; strangers may only read it if allowed.
	accts.Add(connections.Connection{Kind: connections.KindGoogle, Name: "google", Strangers: true,
		Auth: connections.Auth{Type: "oauth", Refresh: "rt-1", TokenURL: srv.URL + "/token"}})
	if !ts["calendar_today"].For(ana) || !ts["calendar_today"].For(stranger) || ts["calendar_add"].For(stranger) {
		t.Fatal("shared calendar")
	}
}

func TestGoogleClientBuiltin(t *testing.T) {
	builtinID, builtinSecret = "built.apps", "built-secret"
	defer func() { builtinID, builtinSecret = "", "" }()
	c := store(t)
	if id, s := GoogleClient(c); id != "built.apps" || s != "built-secret" {
		t.Fatalf("builtin not used: %q %q", id, s)
	}
}

func TestGoogleManyAccountsAndCalendars(t *testing.T) {
	type post struct{ token, path string }
	var posts []post
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		switch {
		case r.URL.Path == "/token":
			r.ParseForm()
			w.Write([]byte(`{"access_token":"at-` + strings.TrimPrefix(r.Form.Get("refresh_token"), "rt-") + `","expires_in":3600}`))
		case r.URL.Path == "/users/me/calendarList" && tok == "at-1":
			w.Write([]byte(`{"items":[{"id":"erick@work.com","summary":"erick@work.com","primary":true,"accessRole":"owner"},
				{"id":"trabajo","summary":"Trabajo","selected":true,"accessRole":"writer"}]}`))
		case r.URL.Path == "/users/me/calendarList": // signed in before BB-8 asked for the list
			w.WriteHeader(403)
			w.Write([]byte(`{"error":{"errors":[{"reason":"insufficientPermissions"}],"message":"Insufficient Permission"}}`))
		case r.Method == "POST":
			posts = append(posts, post{tok, r.URL.Path})
			b, _ := io.ReadAll(r.Body)
			w.Write(b)
		case tok == "at-1" && strings.Contains(r.URL.Path, "/trabajo/"):
			w.Write([]byte(`{"items":[{"id":"j","iCalUID":"j@x","summary":"Junta","start":{"dateTime":"2026-10-16T10:00:00-06:00"},"end":{"dateTime":"2026-10-16T11:00:00-06:00"}}]}`))
		case tok == "at-1":
			w.Write([]byte(`{"items":[{"id":"i1","iCalUID":"inv@x","summary":"Revisión","start":{"dateTime":"2026-10-16T12:00:00-06:00"},"end":{"dateTime":"2026-10-16T13:00:00-06:00"}},
				{"id":"d","iCalUID":"d@x","summary":"Dentista","start":{"dateTime":"2026-10-16T17:30:00-06:00"},"end":{"dateTime":"2026-10-16T18:30:00-06:00"}}]}`))
		default: // the second account's main calendar
			w.Write([]byte(`{"items":[{"id":"i2","iCalUID":"inv@x","summary":"Revisión","start":{"dateTime":"2026-10-16T12:00:00-06:00"},"end":{"dateTime":"2026-10-16T13:00:00-06:00"}},
				{"id":"g","iCalUID":"g@x","summary":"Gym","start":{"dateTime":"2026-10-16T07:00:00-06:00"},"end":{"dateTime":"2026-10-16T08:00:00-06:00"}}]}`))
		}
	}))
	defer srv.Close()
	GoogleAPI = srv.URL
	c := store(t)
	c.Set(map[string]any{keyClientID: "cid", keyClientSecret: "cs"}, config.SourceWeb, nil)
	accts, _ := connections.Open("")
	auth := func(n string) connections.Auth {
		return connections.Auth{Type: "oauth", Refresh: "rt-" + n, ClientID: "cid", ClientSecret: "cs", AuthMethod: "client_secret_post", TokenURL: srv.URL + "/token"}
	}
	accts.Add(connections.Connection{Kind: connections.KindGoogle, Name: "google", Owner: "Erick", Account: "erick@work.com", Auth: auth("1")})
	accts.Add(connections.Connection{Kind: connections.KindGoogle, Name: "google", Owner: "Erick", Account: "erick@gmail.com", Auth: auth("2")})
	accts.Add(connections.Connection{Kind: connections.KindGoogle, Name: "google", Owner: "Ana", Account: "ana@gmail.com", Auth: auth("3")})
	cal := New(accts, func() string { return "Erick" })
	cal.Now = func() time.Time { return time.Date(2026, 10, 16, 6, 0, 0, 0, time.FixedZone("CST", -6*3600)) }
	ts := map[string]tools.Tool{}
	for _, tl := range cal.Tools(c) {
		ts[tl.Name] = tl
	}
	erick := tools.WithCaller(context.Background(), tools.CallOpts{SpeakerKnown: true, Speaker: "Erick"})
	out, err := ts["calendar_today"].Handler(erick, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(out)
	var got struct {
		Events []map[string]any `json:"events"`
	}
	var wrap struct {
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(b, &wrap) == nil && len(wrap.Data) > 0 {
		json.Unmarshal(wrap.Data, &got)
	} else {
		json.Unmarshal(b, &got)
	}
	var titles []string
	for _, e := range got.Events {
		titles = append(titles, e["title"].(string)+"@"+e["calendar"].(string))
	}
	want := "Gym@erick@gmail.com Junta@Trabajo Revisión@erick@work.com Dentista@erick@work.com"
	if strings.Join(titles, " ") != want {
		t.Fatalf("got %s\nfrom %s", strings.Join(titles, " "), b)
	}
	if st := cal.Status(c); !strings.Contains(st.Detail, "erick@gmail.com (Erick) · sign in again") || strings.Contains(st.Detail, "work.com (Erick) ·") {
		t.Fatal(st.Detail)
	}

	add := func(args string) {
		if _, err := ts["calendar_add"].Handler(erick, json.RawMessage(args)); err != nil {
			t.Fatal(args, err)
		}
	}
	add(`{"title":"A","start":"2026-10-17 10:00"}`)
	add(`{"title":"B","start":"2026-10-17 10:00","calendar":"trabajo"}`)
	add(`{"title":"C","start":"2026-10-17 10:00","calendar":"gmail"}`)
	if len(posts) != 3 || posts[0] != (post{"at-1", "/calendars/primary/events"}) || posts[1] != (post{"at-1", "/calendars/trabajo/events"}) ||
		posts[2] != (post{"at-2", "/calendars/primary/events"}) {
		t.Fatalf("%+v", posts)
	}
	if _, err := ts["calendar_add"].Handler(erick, json.RawMessage(`{"title":"D","start":"2026-10-17 10:00","calendar":"escuela"}`)); err == nil {
		t.Fatal("unknown calendar accepted")
	}
}
