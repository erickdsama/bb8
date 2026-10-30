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
// calendars, moves it or cancels it, and reminds them before it starts. Only
// accounts with an owner speak up; a shared calendar stays quiet. It watches
// every calendar of every account (Google.calendars), and an event seen in
// two places (an invitation in two accounts) is said once.
//
// Google pushes "something changed" to a public HTTPS address (bb8-web's
// /hooks/google, see HookPath), which passes it down the robot's link to Hook;
// the watcher then asks Google for what changed. Without a public address, or
// if the push stops, it asks every few minutes anyway.
type Watcher struct {
	G   *Google
	Say func(ctx context.Context, text string)
	Now func() time.Time

	mu   sync.Mutex
	cals map[string]*calState // by account ID + "|" + calendar ID
	said map[string]time.Time // notices and reminders already given (by person and event)
	held []string             // notices kept through quiet hours
	kick chan string
}

// HookPath is where Google's change notices arrive (relayed by bb8-web).
const HookPath = "/hooks/google"

const (
	keyNotify     = "connectors.calendar.notify"
	keyNotifyOwn  = "connectors.calendar.notify_own"
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
		{Key: keyNotifyOwn, Label: "Also the ones I add myself", Type: config.Bool, Default: false},
		{Key: keyRemindMin, Label: "Remind before events (minutes, 0 = off)", Type: config.Number, Default: 10.0, Min: config.F(0), Max: config.F(120)},
		{Key: keyQuietFrom, Label: "Quiet from", Type: config.String, Default: "22:00", Help: "HH:MM. Notices wait until quiet ends."},
		{Key: keyQuietUntil, Label: "Quiet until", Type: config.String, Default: "08:00"},
		{Key: keyPushURL, Label: "Public address of the pages", Type: config.String, Default: "", Advanced: true,
			Help: "For instant notices, e.g. https://bb8.zerock.name. Empty = check every 2 minutes."},
	}
}

// seenEvent is what the watcher remembers of an event, to tell a move or a cancel.
type seenEvent struct {
	start time.Time
	title string
	uid   string
}

// calState is one calendar of one account.
type calState struct {
	since    time.Time            // changes are asked for from here
	starts   map[string]seenEvent // known events by id
	done     map[string]bool      // changes already handled: id|updated
	chID     string               // push channel
	chToken  string
	chRes    string
	chExp    time.Time
	retryAt  time.Time // after a failed subscribe
	lastPoll time.Time
}

func NewWatcher(g *Google, say func(ctx context.Context, text string)) *Watcher {
	return &Watcher{G: g, Say: say, Now: time.Now, cals: map[string]*calState{}, said: map[string]time.Time{}, kick: make(chan string, 8)}
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
		case key := <-w.kick:
			w.tick(ctx, key)
		case <-t.C:
			w.tick(ctx, "")
		}
	}
}

// Hook takes Google's change notices. Google sends no event data, only which
// channel changed; the channel token says it's ours and which calendar.
func (w *Watcher) Hook(rw http.ResponseWriter, r *http.Request) {
	tok, state := r.Header.Get("X-Goog-Channel-Token"), r.Header.Get("X-Goog-Resource-State")
	w.mu.Lock()
	key := ""
	for k, st := range w.cals {
		if st.chToken != "" && subtle.ConstantTimeCompare([]byte(tok), []byte(st.chToken)) == 1 {
			key = k
		}
	}
	w.mu.Unlock()
	if key != "" && state != "sync" { // "sync" only confirms a new channel
		select {
		case w.kick <- key:
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
		for _, cal := range w.G.calendars(ctx, a.ID) {
			key := a.ID + "|" + cal.ID
			live[key] = true
			if only != "" && key != only {
				continue
			}
			w.check(ctx, a, cal, w.state(key), now, only != "")
		}
	}
	w.mu.Lock()
	for k := range w.cals {
		if !live[k] {
			delete(w.cals, k)
		}
	}
	for k, t := range w.said {
		if now.Sub(t) > 3*24*time.Hour {
			delete(w.said, k)
		}
	}
	w.mu.Unlock()
}

func (w *Watcher) state(key string) *calState {
	w.mu.Lock()
	defer w.mu.Unlock()
	st := w.cals[key]
	if st == nil {
		st = &calState{starts: map[string]seenEvent{}, done: map[string]bool{}}
		w.cals[key] = st
	}
	return st
}

// once says whether a notice is new (and marks it given).
func (w *Watcher) once(key string, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.said[key]; ok {
		return false
	}
	w.said[key] = now
	return true
}

type wPerson struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Self        bool   `json:"self"`
}

type wEvent struct {
	ID        string    `json:"id"`
	ICalUID   string    `json:"iCalUID"`
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

func (e wEvent) uid() string {
	if e.ICalUID != "" {
		return e.ICalUID
	}
	return e.ID
}

func (w *Watcher) check(ctx context.Context, a connections.Connection, cal gCal, st *calState, now time.Time, pushed bool) {
	loc := location(w.G.c)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if st.since.IsZero() { // first look: learn what's there, say nothing
		var out struct {
			Items []wEvent `json:"items"`
		}
		q := url.Values{"timeMin": {now.Format(time.RFC3339)}, "timeMax": {now.AddDate(0, 0, 30).Format(time.RFC3339)},
			"singleEvents": {"true"}, "maxResults": {"250"}}
		if err := w.G.doAs(ctx, a.ID, "GET", calPath(cal.ID), q, nil, &out); err != nil {
			slog.Warn("calendar watch", "account", a.Account, "calendar", cal.Summary, "err", err)
			return
		}
		for _, e := range out.Items {
			s, _ := e.Start.parse(loc)
			st.starts[e.ID] = seenEvent{start: s, title: e.Summary, uid: e.uid()}
		}
		st.since, st.lastPoll = now, now
	}

	push := strings.TrimRight(strings.TrimSpace(w.G.c.String(keyPushURL)), "/")
	if push != "" && now.After(st.retryAt) && (st.chID == "" || st.chExp.Sub(now) < 24*time.Hour) {
		w.subscribe(ctx, a, cal, st, push, now)
	}
	every := pollEvery
	if push != "" && st.chID != "" {
		every = pollPushEvery
	}

	if w.G.c.Bool(keyNotify) && (pushed || now.Sub(st.lastPoll) >= every-5*time.Second) {
		w.changes(ctx, a, cal, st, now, loc)
	}
	if m := w.G.c.Float(keyRemindMin); m > 0 && !w.quiet(now) {
		w.remind(ctx, a, cal, now, time.Duration(m)*time.Minute, loc)
	}
}

func (w *Watcher) changes(ctx context.Context, a connections.Connection, cal gCal, st *calState, now time.Time, loc *time.Location) {
	var out struct {
		Items []wEvent `json:"items"`
	}
	q := url.Values{"updatedMin": {st.since.Add(-time.Minute).Format(time.RFC3339)}, "showDeleted": {"true"},
		"singleEvents": {"true"}, "maxResults": {"100"}}
	if err := w.G.doAs(ctx, a.ID, "GET", calPath(cal.ID), q, nil, &out); err != nil {
		slog.Warn("calendar watch", "account", a.Account, "calendar", cal.Summary, "err", err)
		return
	}
	st.since, st.lastPoll = now, now
	own := w.G.c.Bool(keyNotifyOwn)
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
			st.starts[e.ID] = seenEvent{start: s, title: e.Summary, uid: e.uid()}
		}
		mine := e.Creator.Self && e.Organizer.Self // the person did it: they know
		if (mine && !own) || (!s.IsZero() && s.Before(now)) {
			continue
		}
		who := e.Organizer
		if who.Self {
			who = e.Creator
		}
		title := e.Summary
		if title == "" {
			title = old.title
		}
		if title == "" {
			title = "un evento"
		} else {
			title = "«" + title + "»"
		}
		uid := e.uid()
		if uid == e.ID && old.uid != "" {
			uid = old.uid
		}
		var msg, kind string
		switch {
		case e.Status == "cancelled":
			if !known {
				continue
			}
			msg, kind = fmt.Sprintf("se canceló %s %s", title, when(old.start, false, now, loc)), "x|"+old.start.Format(time.RFC3339)
		case !known && mine:
			msg, kind = fmt.Sprintf("agregaste %s %s", title, when(s, allDay, now, loc)), "n|"+s.Format(time.RFC3339)
		case !known:
			msg, kind = fmt.Sprintf("%s te agendó %s %s", who.name(), title, when(s, allDay, now, loc)), "n|"+s.Format(time.RFC3339)
		case !old.start.Equal(s):
			verb := who.name() + " cambió"
			if mine {
				verb = "cambiaste"
			}
			msg, kind = fmt.Sprintf("%s %s: ahora es %s", verb, title, when(s, allDay, now, loc)), "m|"+s.Format(time.RFC3339)
		default:
			continue
		}
		// One notice per person and event, wherever it shows up (two accounts, two calendars).
		if !w.once(strings.ToLower(a.Owner)+"|"+uid+"|"+kind, now) {
			continue
		}
		w.notify(ctx, a.Owner+", "+msg+".", now)
	}
	if len(st.done) > 2000 { // keep it small; old keys are outside the window anyway
		st.done = map[string]bool{}
	}
}

func (w *Watcher) remind(ctx context.Context, a connections.Connection, cal gCal, now time.Time, before time.Duration, loc *time.Location) {
	var out struct {
		Items []wEvent `json:"items"`
	}
	q := url.Values{"timeMin": {now.Format(time.RFC3339)}, "timeMax": {now.Add(before + time.Minute).Format(time.RFC3339)},
		"singleEvents": {"true"}, "maxResults": {"10"}}
	if err := w.G.doAs(ctx, a.ID, "GET", calPath(cal.ID), q, nil, &out); err != nil {
		return
	}
	for _, e := range out.Items {
		s, allDay := e.Start.parse(loc)
		if allDay || e.Status == "cancelled" || s.Sub(now) > before {
			continue
		}
		if !w.once(strings.ToLower(a.Owner)+"|r|"+e.uid()+"|"+s.Format(time.RFC3339), now) {
			continue
		}
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

// subscribe opens (or renews) the push channel for one calendar.
func (w *Watcher) subscribe(ctx context.Context, a connections.Connection, cal gCal, st *calState, push string, now time.Time) {
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
	if err := w.G.doAs(ctx, a.ID, "POST", calPath(cal.ID)+"/watch", nil, body, &out); err != nil {
		slog.Warn("calendar push: falling back to checking every 2 minutes", "account", a.Account, "calendar", cal.Summary, "err", err)
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
	slog.Info("calendar push on", "account", a.Account, "calendar", cal.Summary, "until", st.chExp.Format(time.DateTime))
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
