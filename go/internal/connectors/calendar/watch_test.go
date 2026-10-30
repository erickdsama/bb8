package calendar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connections"
)

func TestWatcherNoticesAndPush(t *testing.T) {
	cst := time.FixedZone("CST", -6*3600)
	now := time.Date(2026, 10, 23, 9, 0, 0, 0, cst)
	var mu sync.Mutex
	changes := map[string]string{"main": `[]`, "fam": `[]`}
	watched := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		q := r.URL.Query()
		cal := "main"
		if strings.Contains(r.URL.Path, "/calendars/fam/") {
			cal = "fam"
		}
		switch {
		case r.URL.Path == "/token":
			w.Write([]byte(`{"access_token":"at-1","expires_in":3600}`))
		case r.URL.Path == "/users/me/calendarList":
			w.Write([]byte(`{"items":[{"id":"erick@example.com","summary":"erick@example.com","primary":true,"accessRole":"owner"},
				{"id":"fam","summary":"Familia","selected":true,"accessRole":"writer"},
				{"id":"es.mexican#holiday","summary":"Festivos","selected":true,"hidden":true,"accessRole":"reader"},
				{"id":"busy","summary":"Ocupado","selected":true,"accessRole":"freeBusyReader"}]}`))
		case strings.HasSuffix(r.URL.Path, "/watch"):
			var b map[string]any
			json.NewDecoder(r.Body).Decode(&b)
			watched[cal] = b
			w.Write([]byte(`{"resourceId":"res-1","expiration":"1793000000000"}`))
		case q.Get("updatedMin") != "":
			w.Write([]byte(`{"items":` + changes[cal] + `}`))
		case q.Get("maxResults") == "10": // reminders: the call is on both calendars
			w.Write([]byte(`{"items":[{"id":"e2-` + cal + `","iCalUID":"call@x","summary":"Llamada con Ana","start":{"dateTime":"2026-10-23T09:08:00-06:00"}}]}`))
		case cal == "main": // first look
			w.Write([]byte(`{"items":[{"id":"e2","iCalUID":"call@x","summary":"Llamada con Ana","start":{"dateTime":"2026-10-23T09:08:00-06:00"}},
				{"id":"e3","iCalUID":"junta@x","summary":"Junta","start":{"dateTime":"2026-10-24T16:00:00-06:00"}}]}`))
		default:
			w.Write([]byte(`{"items":[]}`))
		}
	}))
	defer srv.Close()
	GoogleAPI = srv.URL
	c := store(t)
	c.Register(config.Field{Key: "connectors.calendar.enabled", Type: config.Bool})
	c.Set(map[string]any{"connectors.calendar.enabled": true, keyPushURL: "https://bb8.example.com/"}, config.SourceWeb, nil)
	accts, _ := connections.Open("")
	acct := connections.Connection{Kind: connections.KindGoogle, Name: "google", Owner: "Erick", Account: "erick@example.com",
		Auth: connections.Auth{Type: "oauth", Refresh: "rt", ClientID: "cid", ClientSecret: "cs", AuthMethod: "client_secret_post", TokenURL: srv.URL + "/token"}}
	accts.Add(acct)
	accts.Add(connections.Connection{Kind: connections.KindGoogle, Name: "google", Account: "casa@example.com", Auth: acct.Auth}) // shared: quiet

	var said []string
	w := NewWatcher(NewGoogle(c, accts, nil), func(_ context.Context, s string) { said = append(said, s) })
	w.Now = func() time.Time { return now }
	ctx := context.Background()

	w.tick(ctx, "") // first look, a push channel per calendar, one reminder for the call on two calendars
	if len(watched) != 2 || watched["main"]["address"] != "https://bb8.example.com/hooks/google" || watched["fam"]["token"] == watched["main"]["token"] {
		t.Fatalf("watch requests %v", watched)
	}
	if len(w.cals) != 2 {
		t.Fatalf("watching %d calendars, want 2 (hidden and free/busy skipped)", len(w.cals))
	}
	if len(said) != 1 || said[0] != "Erick, en 8 minutos tienes Llamada con Ana." {
		t.Fatalf("reminder %q", said)
	}

	// Ana invites Erick (it lands on both calendars) and moves tomorrow's meeting; Erick's own event stays quiet.
	invite := `{"id":"e9","iCalUID":"rev@x","status":"confirmed","summary":"Revisión","start":{"dateTime":"2026-10-24T10:00:00-06:00"},"updated":"2026-10-23T15:00:30Z",
	  "creator":{"email":"ana@example.com"},"organizer":{"email":"ana@example.com","displayName":"Ana"}}`
	mu.Lock()
	changes["main"] = `[` + invite + `,
	 {"id":"e3","iCalUID":"junta@x","status":"confirmed","summary":"Junta","start":{"dateTime":"2026-10-24T17:00:00-06:00"},"updated":"2026-10-23T15:00:31Z",
	  "creator":{"email":"ana@example.com"},"organizer":{"email":"ana@example.com","displayName":"Ana"}},
	 {"id":"e10","status":"confirmed","summary":"Gym","start":{"dateTime":"2026-10-24T07:00:00-06:00"},"updated":"2026-10-23T15:00:32Z",
	  "creator":{"self":true},"organizer":{"self":true}}]`
	changes["fam"] = `[` + strings.Replace(invite, `"id":"e9"`, `"id":"e9-copy"`, 1) + `]`
	mu.Unlock()
	kick := func(cal string) {
		tok, _ := watched[cal]["token"].(string)
		req := httptest.NewRequest("POST", HookPath, nil)
		req.Header.Set("X-Goog-Channel-Token", tok)
		req.Header.Set("X-Goog-Resource-State", "exists")
		w.Hook(httptest.NewRecorder(), req)
		select {
		case key := <-w.kick:
			if !strings.HasSuffix(key, "|"+map[string]string{"main": "erick@example.com", "fam": "fam"}[cal]) {
				t.Fatal("kicked", key)
			}
			w.tick(ctx, key)
		default:
			t.Fatal("hook did not kick", cal)
		}
	}
	kick("main")
	kick("fam")
	want := []string{"Erick, Ana te agendó «Revisión» mañana a las 10:00.", "Erick, Ana cambió «Junta»: ahora es mañana a las 5:00."}
	if len(said) != 3 || said[1] != want[0] || said[2] != want[1] {
		t.Fatalf("notices %q", said[1:])
	}

	// A wrong token does nothing.
	bad := httptest.NewRequest("POST", HookPath, nil)
	bad.Header.Set("X-Goog-Channel-Token", "nope")
	w.Hook(httptest.NewRecorder(), bad)
	if len(w.kick) != 0 {
		t.Fatal("kicked by a stranger")
	}

	// With "also mine" on, Erick's own new event is said too.
	c.Set(map[string]any{keyNotifyOwn: true}, config.SourceWeb, nil)
	mu.Lock()
	changes["main"] = `[{"id":"e11","status":"confirmed","summary":"Dentista","start":{"dateTime":"2026-10-24T12:00:00-06:00"},"updated":"2026-10-23T15:01:00Z",
	  "creator":{"self":true},"organizer":{"self":true}}]`
	changes["fam"] = `[]`
	mu.Unlock()
	kick("main")
	if said[len(said)-1] != "Erick, agregaste «Dentista» mañana a las 12:00." {
		t.Fatalf("own notice %q", said[len(said)-1])
	}

	// Quiet hours hold notices until they end.
	n := len(said)
	c.Set(map[string]any{keyQuietFrom: "08:00", keyQuietUntil: "09:30"}, config.SourceWeb, nil)
	w.notify(ctx, "x", now)
	if len(said) != n {
		t.Fatal("spoke in quiet hours")
	}
	now = now.Add(31 * time.Minute)
	w.tick(ctx, "")
	if said[len(said)-1] != "x" {
		t.Fatalf("held notice not said: %q", said)
	}
}

func TestWhen(t *testing.T) {
	loc := time.FixedZone("CST", -6*3600)
	now := time.Date(2026, 10, 23, 9, 0, 0, 0, loc) // Friday
	for _, c := range []struct {
		t    time.Time
		want string
	}{
		{time.Date(2026, 10, 23, 13, 5, 0, 0, loc), "hoy a la 1:05"},
		{time.Date(2026, 10, 24, 0, 30, 0, 0, loc), "mañana a las 12:30"},
		{time.Date(2026, 10, 27, 10, 0, 0, 0, loc), "el martes 27 a las 10:00"},
	} {
		if got := when(c.t, false, now, loc); got != c.want {
			t.Errorf("%v: %q", c.t, got)
		}
	}
}
