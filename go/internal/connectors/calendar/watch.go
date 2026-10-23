package calendar

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connections"
)

// Watcher tells people out loud when someone else puts something on their
// calendar, moves it or cancels it, and reminds them before it starts. Only
// accounts with an owner speak up; a shared calendar stays quiet.
//
// Google pushes "something changed" to a public HTTPS address (bb8-web's
// /hooks/google, see HookPath), which passes it down the robot's link to Hook;
// the watcher then asks Google for what changed. Without a public address, or
// if the push stops, it asks every few minutes anyway.
type Watcher struct {
	G   *Google
	Say func(ctx context.Context, text string)
	Now func() time.Time

	mu    sync.Mutex
	accts map[string]*watchState // by account ID
	held  []string               // notices kept through quiet hours
	kick  chan string
}

// HookPath is where Google's change notices arrive (relayed by bb8-web).
const HookPath = "/hooks/google"

const (
	keyNotify     = "connectors.calendar.notify"
	keyRemindMin  = "connectors.calendar.remind_min"
	keyQuietFrom  = "connectors.calendar.quiet_from"
	keyQuietUntil = "connectors.calendar.quiet_until"
	keyPushURL    = "connectors.calendar.push_url"

	pollEvery     = 2 * time.Minute  // without push
	pollPushEvery = 15 * time.Minute // with push, in case a notice got lost
	channelTTL    = 7 * 24 * time.Hour
)

func WatchFields() []config.Field {
	return []config.Field{
		{Key: keyNotify, Label: "Tell me about new, moved or canceled events", Type: config.Bool, Default: true},
		{Key: keyRemindMin, Label: "Remind before events (minutes, 0 = off)", Type: config.Number, Default: 10.0, Min: config.F(0), Max: config.F(120)},
		{Key: keyQuietFrom, Label: "Quiet from", Type: config.String, Default: "22:00", Help: "HH:MM. Notices wait until quiet ends."},
		{Key: keyQuietUntil, Label: "Quiet until", Type: config.String, Default: "08:00"},
		{Key: keyPushURL, Label: "Public address of the pages", Type: config.String, Default: "", Advanced: true,
			Help: "For instant notices, e.g. https://bb8.zerock.name. Empty = check every 2 minutes."},
	}
}

type watchState struct {
	since    time.Time            // changes are asked for from here
	starts   map[string]time.Time // known events: id → start
	done     map[string]bool      // notices already given: id|updated, and reminders: r|id|start
	chID     string               // push channel
	chToken  string
	chRes    string
	chExp    time.Time
	retryAt  time.Time // after a failed subscribe
	lastPoll time.Time
}

func NewWatcher(g *Google, say func(ctx context.Context, text string)) *Watcher {
	return &Watcher{G: g, Say: say, Now: time.Now, accts: map[string]*watchState{}, kick: make(chan string, 8)}
}

// Run checks every minute until ctx ends, and at once when Google pushes.
func (w *Watcher) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	w.tick(ctx, "")
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-w.kick:
			w.tick(ctx, id)
		case <-t.C:
			w.tick(ctx, "")
		}
	}
}

// Hook takes Google's change notices. Google sends no event data, only which
// channel changed; the channel token says it's ours.
func (w *Watcher) Hook(rw http.ResponseWriter, r *http.Request) {
	tok, state := r.Header.Get("X-Goog-Channel-Token"), r.Header.Get("X-Goog-Resource-State")
	w.mu.Lock()
	id := ""
	for aid, st := range w.accts {
		if st.chToken != "" && subtle.ConstantTimeCompare([]byte(tok), []byte(st.chToken)) == 1 {
			id = aid
		}
	}
	w.mu.Unlock()
	if id != "" && state != "sync" { // "sync" only confirms a new channel
		select {
		case w.kick <- id:
		default:
		}
	}
	rw.WriteHeader(http.StatusOK) // anything else makes Google retry
}

func (w *Watcher) enabled() bool {
	c := w.G.c
	return c.Bool("connectors.calendar.enabled") && (c.Bool(keyNotify) || c.Float(keyRemindMin) > 0)
}

func (w *Watcher) tick(ctx context.Context, only string) {
	now := w.Now()
	if !w.enabled() {
		return
	}
	if !w.quiet(now) {
		w.mu.Lock()
		held := w.held
		w.held = nil
		w.mu.Unlock()
		for _, t := range held {
			w.Say(ctx, t)
		}
	}
	live := map[string]bool{}
	for _, a := range w.G.accounts() {
		if a.Owner == "" {
			continue
		}
		live[a.ID] = true
		if only != "" && a.ID != only {
			continue
		}
		w.check(ctx, a, now, only != "")
	}
	w.mu.Lock()
	for id := range w.accts {
		if !live[id] {
			delete(w.accts, id)
		}
	}
	w.mu.Unlock()
}

func (w *Watcher) state(id string) *watchState {
	w.mu.Lock()
	defer w.mu.Unlock()
	st := w.accts[id]
	if st == nil {
		st = &watchState{starts: map[string]time.Time{}, done: map[string]bool{}}
		w.accts[id] = st
	}
	return st
}

type wPerson struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Self        bool   `json:"self"`
}

type wEvent struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Summary   string    `json:"summary"`
	Start     gTime     `json:"start"`
	Updated   time.Time `json:"updated"`
	Created   time.Time `json:"created"`
	Creator   wPerson   `json:"creator"`
	Organizer wPerson   `json:"organizer"`
}

func (p wPerson) name() string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	n, _, _ := strings.Cut(p.Email, "@")
	return n
}

func (w *Watcher) check(ctx context.Context, a connections.Connection, now time.Time, pushed bool) {
	st := w.state(a.ID)
	loc := location(w.G.c)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if st.since.IsZero() { // first look: learn what's there, say nothing
		var out struct {
			Items []wEvent `json:"items"`
		}
		q := url.Values{"timeMin": {now.Format(time.RFC3339)}, "timeMax": {now.AddDate(0, 0, 30).Format(time.RFC3339)},
			"singleEvents": {"true"}, "maxResults": {"250"}}
		if err := w.G.doAs(ctx, a.ID, "GET", w.G.calPath(), q, nil, &out); err != nil {
			slog.Warn("calendar watch", "account", a.Account, "err", err)
			return
		}
		for _, e := range out.Items {
			s, _ := e.Start.parse(loc)
			st.starts[e.ID] = s
		}
		st.since, st.lastPoll = now, now
	}

	push := strings.TrimRight(strings.TrimSpace(w.G.c.String(keyPushURL)), "/")
	if push != "" && now.After(st.retryAt) && (st.chID == "" || st.chExp.Sub(now) < 24*time.Hour) {
		w.subscribe(ctx, a, st, push, now)
	}
	every := pollEvery
	if push != "" && st.chID != "" {
		every = pollPushEvery
	}

	if w.G.c.Bool(keyNotify) && (pushed || now.Sub(st.lastPoll) >= every-5*time.Second) {
		w.changes(ctx, a, st, now, loc)
	}
	if m := w.G.c.Float(keyRemindMin); m > 0 && !w.quiet(now) {
		w.remind(ctx, a, st, now, time.Duration(m)*time.Minute, loc)
	}
}

func (w *Watcher) changes(ctx context.Context, a connections.Connection, st *watchState, now time.Time, loc *time.Location) {
	var out struct {
		Items []wEvent `json:"items"`
	}
	q := url.Values{"updatedMin": {st.since.Add(-time.Minute).Format(time.RFC3339)}, "showDeleted": {"true"},
		"singleEvents": {"true"}, "maxResults": {"100"}}
	if err := w.G.doAs(ctx, a.ID, "GET", w.G.calPath(), q, nil, &out); err != nil {
		slog.Warn("calendar watch", "account", a.Account, "err", err)
		return
	}
	st.since, st.lastPoll = now, now
	for _, e := range out.Items {
		key := e.ID + "|" + e.Updated.Format(time.RFC3339Nano)
		if st.done[key] {
			continue
		}
		st.done[key] = true
		s, allDay := e.Start.parse(loc)
		old, known := st.starts[e.ID]
		if e.Status == "cancelled" {
			delete(st.starts, e.ID)
		} else {
			st.starts[e.ID] = s
		}
		mine := e.Creator.Self && e.Organizer.Self // the person did it: they know
		if mine || (!s.IsZero() && s.Before(now)) {
			continue
		}
		who := e.Organizer
		if who.Self {
			who = e.Creator
		}
		title := e.Summary
		if title == "" {
			title = "un evento"
		} else {
			title = "«" + title + "»"
		}
		var msg string
		switch {
		case e.Status == "cancelled":
			if !known {
				continue
			}
			msg = fmt.Sprintf("se canceló %s %s", title, when(old, false, now, loc))
		case !known:
			msg = fmt.Sprintf("%s te agendó %s %s", who.name(), title, when(s, allDay, now, loc))
		case !old.Equal(s):
			msg = fmt.Sprintf("%s cambió %s: ahora es %s", who.name(), title, when(s, allDay, now, loc))
		default:
			continue
		}
		w.notify(ctx, a.Owner+", "+msg+".", now)
	}
	if len(st.done) > 2000 { // keep it small; old keys are outside the window anyway
		st.done = map[string]bool{}
	}
}

func (w *Watcher) remind(ctx context.Context, a connections.Connection, st *watchState, now time.Time, before time.Duration, loc *time.Location) {
	var out struct {
		Items []wEvent `json:"items"`
	}
	q := url.Values{"timeMin": {now.Format(time.RFC3339)}, "timeMax": {now.Add(before + time.Minute).Format(time.RFC3339)},
		"singleEvents": {"true"}, "maxResults": {"10"}}
	if err := w.G.doAs(ctx, a.ID, "GET", w.G.calPath(), q, nil, &out); err != nil {
		return
	}
	for _, e := range out.Items {
		s, allDay := e.Start.parse(loc)
		if allDay || e.Status == "cancelled" || s.Sub(now) > before {
			continue
		}
		key := "r|" + e.ID + "|" + s.Format(time.RFC3339)
		if st.done[key] {
			continue
		}
		st.done[key] = true
		mins := int(s.Sub(now).Round(time.Minute) / time.Minute)
		title := e.Summary
		if title == "" {
			title = "un evento"
		}
		if mins <= 1 {
			w.Say(ctx, fmt.Sprintf("%s, ya empieza %s.", a.Owner, title))
		} else {
			w.Say(ctx, fmt.Sprintf("%s, en %d minutos tienes %s.", a.Owner, mins, title))
		}
	}
}

// notify speaks now, or keeps it for when quiet hours end.
func (w *Watcher) notify(ctx context.Context, text string, now time.Time) {
	if w.quiet(now) {
		w.mu.Lock()
		w.held = append(w.held, text)
		w.mu.Unlock()
		return
	}
	w.Say(ctx, text)
}

func (w *Watcher) quiet(now time.Time) bool {
	from, ok1 := hhmm(w.G.c.String(keyQuietFrom))
	until, ok2 := hhmm(w.G.c.String(keyQuietUntil))
	if !ok1 || !ok2 || from == until {
		return false
	}
	t := now.In(location(w.G.c))
	m := t.Hour()*60 + t.Minute()
	if from < until {
		return m >= from && m < until
	}
	return m >= from || m < until // crosses midnight
}

func hhmm(s string) (int, bool) {
	h, m, ok := strings.Cut(strings.TrimSpace(s), ":")
	hi, err1 := strconv.Atoi(h)
	mi, err2 := strconv.Atoi(m)
	if !ok || err1 != nil || err2 != nil || hi < 0 || hi > 23 || mi < 0 || mi > 59 {
		return 0, false
	}
	return hi*60 + mi, true
}

var dias = []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}

// when says a time the way BB-8 speaks: "hoy a las 5:30", "mañana a la 1:00", "el martes 28 a las 10:00".
func when(t time.Time, allDay bool, now time.Time, loc *time.Location) string {
	t, now = t.In(loc), now.In(loc)
	d0 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	var day string
	switch int(d.Sub(d0).Hours() / 24) {
	case 0:
		day = "hoy"
	case 1:
		day = "mañana"
	default:
		day = fmt.Sprintf("el %s %d", dias[t.Weekday()], t.Day())
	}
	if allDay {
		return day
	}
	h := t.Hour() % 12
	if h == 0 {
		h = 12
	}
	art := "a las"
	if h == 1 {
		art = "a la"
	}
	return fmt.Sprintf("%s %s %d:%02d", day, art, h, t.Minute())
}

// subscribe opens (or renews) the push channel for one account.
func (w *Watcher) subscribe(ctx context.Context, a connections.Connection, st *watchState, push string, now time.Time) {
	id, tok := randHex(16), randHex(24)
	body := map[string]any{"id": id, "type": "web_hook", "address": push + HookPath, "token": tok,
		"params": map[string]string{"ttl": strconv.Itoa(int(channelTTL / time.Second))}}
	var out struct {
		ResourceID string `json:"resourceId"`
		Expiration string `json:"expiration"` // ms since epoch
	}
	// The hook may fire before this returns: know the token first.
	w.mu.Lock()
	prevID, prevRes, prevTok := st.chID, st.chRes, st.chToken
	st.chToken = tok
	w.mu.Unlock()
	if err := w.G.doAs(ctx, a.ID, "POST", w.G.calPath()+"/watch", nil, body, &out); err != nil {
		slog.Warn("calendar push: falling back to checking every 2 minutes", "account", a.Account, "err", err)
		w.mu.Lock()
		st.chToken = prevTok
		w.mu.Unlock()
		st.retryAt = now.Add(time.Hour) // don't retry every minute
		return
	}
	ms, _ := strconv.ParseInt(out.Expiration, 10, 64)
	w.mu.Lock()
	st.chID, st.chRes, st.chExp = id, out.ResourceID, time.UnixMilli(ms)
	if ms == 0 {
		st.chExp = now.Add(channelTTL)
	}
	w.mu.Unlock()
	if prevID != "" { // the old one would still fire until it expires
		_ = w.G.doAs(ctx, a.ID, "POST", "/channels/stop", nil, map[string]string{"id": prevID, "resourceId": prevRes}, nil)
	}
	slog.Info("calendar push on", "account", a.Account, "until", st.chExp.Format(time.DateTime))
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
