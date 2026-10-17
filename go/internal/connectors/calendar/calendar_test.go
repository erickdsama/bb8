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
	"github.com/erickdsama/bb8/go/internal/connectors"
)

func store(t *testing.T) *config.Store {
	c, _ := config.Open(t.TempDir())
	c.Register(New().Fields()...)
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
	GoogleAPI, GoogleToken = srv.URL, srv.URL+"/token"

	c := store(t)
	cal := New()
	cal.Now = func() time.Time { return time.Date(2026, 10, 16, 9, 0, 0, 0, time.FixedZone("CST", -6*3600)) }
	if st := cal.Status(c); st.State != connectors.NeedsConf {
		t.Fatal(st)
	}
	c.Set(map[string]any{keyClientID: "cid", keyClientSecret: "cs"}, config.SourceWeb, nil)
	if st := cal.Status(c); st.State != connectors.NeedsAuth {
		t.Fatal(st)
	}
	c.Set(map[string]any{keyRefresh: "rt-1"}, config.SourceWeb, nil)
	if st := cal.Status(c); st.State != connectors.OK {
		t.Fatal(st)
	}
	ts := map[string]func(string) map[string]any{}
	for _, tl := range cal.Tools(c) {
		tl := tl
		ts[tl.Name] = func(args string) map[string]any {
			out, err := tl.Handler(context.Background(), json.RawMessage(args))
			if err != nil {
				t.Fatal(tl.Name, err)
			}
			return out.Data.(map[string]any)
		}
	}
	today := ts["calendar_today"](`{}`)
	ev := today["events"].([]map[string]any)
	if len(ev) != 2 || ev[1]["title"] != "Dentista" || ev[1]["start"] != "Fri 16 Oct 17:30" || ev[0]["all_day"] != true {
		t.Fatal(today)
	}
	res := ts["calendar_add"](`{"title":"Pasear a BB-8","start":"2026-10-16 19:00","duration_min":30}`)
	if res["result"] != "done" || !strings.HasPrefix(added["start"].(map[string]any)["dateTime"].(string), "2026-10-16T19:00:00-06:00") {
		t.Fatal(res, added)
	}
	if refreshes != 1 {
		t.Fatalf("token should be cached, refreshed %d times", refreshes)
	}
}
