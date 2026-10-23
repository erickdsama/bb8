package connections

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Flows runs sign-ins and serves the page's API:
//
//	GET    /api/connections            accounts (no tokens) + status, catalog, people
//	GET    /api/connections/discover   Home Assistants on the network
//	POST   /api/connections/start      {kind, owner, url, token, origin, id?} → {auth_url} or {connected}
//	PATCH  /api/connections/{id}       {owner, strangers, confirm}
//	DELETE /api/connections/{id}
//	GET    /api/connections/callback   where sign-ins come back (no page token: the state proves it)
//
// OAuth services only accept fixed https redirects, so they return to bb8-web's
// public /oauth/ (connectors.mcp.oauth_bridge), which forwards to the callback
// down the robot's link; Home Assistant comes back directly.
type Flows struct {
	Store  *Store
	HTTP   *http.Client
	Bridge func() string            // public bridge URL
	// Client is the OAuth client the owner registered for a service that has no
	// automatic registration (Google); empty id: not set up yet.
	Client func(kind string) (id, secret string)
	Owner  func() string            // home.owner: the default owner
	People func() []string          // names for the owner picker
	Status func(id string) LiveInfo // from mcpclient

	mu      sync.Mutex
	pending map[string]*flow
}

// LiveInfo is how a connection is doing right now.
type LiveInfo struct {
	State string `json:"state"` // connecting, ok, error, off
	Error string `json:"error,omitempty"`
	Tools int    `json:"tools"`
}

type flow struct {
	draft    Connection // the account to add (or update when draft.ID is set)
	verifier string
	redirect string
	base     string // Home Assistant
	userinfo string
	at       time.Time
}

func NewFlows(s *Store, hc *http.Client) *Flows {
	return &Flows{Store: s, HTTP: hc, pending: map[string]*flow{},
		Bridge: func() string { return "" }, Client: func(string) (string, string) { return "", "" }, Owner: func() string { return "" }, People: func() []string { return nil },
		Status: func(string) LiveInfo { return LiveInfo{State: "connecting"} }}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// newState encodes where the browser must come back to, for the bridge page.
func newState(ret string) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	j, _ := json.Marshal(map[string]string{"r": ret, "n": base64.RawURLEncoding.EncodeToString(b)})
	return base64.RawURLEncoding.EncodeToString(j)
}

// Routes mounts the token-protected API.
func (f *Flows) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/connections", func(w http.ResponseWriter, r *http.Request) {
		list := []map[string]any{}
		for _, c := range f.Store.List() {
			list = append(list, map[string]any{"connection": c.View(), "live": f.Status(c.ID)})
		}
		people := f.People()
		sort.Strings(people)
		writeJSON(w, 200, map[string]any{"connections": list, "catalog": Catalog, "people": people, "owner": f.Owner()})
	})
	mux.HandleFunc("GET /api/connections/discover", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"home_assistant": FindHomeAssistant(r.Context())})
	})
	mux.HandleFunc("POST /api/connections/start", f.start)
	mux.HandleFunc("PATCH /api/connections/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Owner     *string `json:"owner"`
			Strangers *bool   `json:"strangers"`
			Confirm   *bool   `json:"confirm"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 1<<14)).Decode(&in) != nil {
			writeJSON(w, 400, map[string]any{"error": "bad json"})
			return
		}
		c, err := f.Store.Update(r.PathValue("id"), true, func(c *Connection) {
			if in.Owner != nil {
				c.Owner = strings.TrimSpace(*in.Owner)
			}
			if in.Strangers != nil {
				c.Strangers = *in.Strangers
			}
			if in.Confirm != nil {
				c.Confirm = *in.Confirm
			}
			if c.Owner != "" {
				c.Strangers = false // a person's account is never for strangers
			}
		})
		if err != nil {
			writeJSON(w, 404, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, 200, c.View())
	})
	mux.HandleFunc("DELETE /api/connections/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := f.Store.Remove(r.PathValue("id")); err != nil {
			writeJSON(w, 404, map[string]any{"error": err.Error()})
			return
		}
		w.WriteHeader(204)
	})
}

// CallbackPath is served without the page token.
const CallbackPath = "/api/connections/callback"

type startReq struct {
	Kind   string `json:"kind"`
	Owner  string `json:"owner"`
	URL    string `json:"url"`
	Token  string `json:"token"`
	Origin string `json:"origin"` // the page's own address, where the browser comes back
	ID     string `json:"id"`     // sign in again to an existing account
}

func validOrigin(o string) (string, bool) {
	u, err := url.Parse(o)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

func (f *Flows) start(w http.ResponseWriter, r *http.Request) {
	var in startReq
	if json.NewDecoder(io.LimitReader(r.Body, 1<<14)).Decode(&in) != nil {
		writeJSON(w, 400, map[string]any{"error": "bad json"})
		return
	}
	origin, ok := validOrigin(in.Origin)
	if !ok {
		writeJSON(w, 400, map[string]any{"error": "origin must be the page address"})
		return
	}
	kind, ok := Catalog[in.Kind]
	if !ok {
		writeJSON(w, 400, map[string]any{"error": "unknown service " + in.Kind})
		return
	}
	draft := Connection{Kind: in.Kind, Name: in.Kind, URL: kind.URL, Owner: strings.TrimSpace(in.Owner), Confirm: kind.Confirm}
	if in.ID != "" { // sign in again: keep who owns it and how it behaves
		old, ok := f.Store.Get(in.ID)
		if !ok {
			writeJSON(w, 404, map[string]any{"error": ErrUnknown.Error()})
			return
		}
		draft = old
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	callback := origin + CallbackPath

	if kind.SignIn == "homeassistant" {
		base, err := HABase(firstNonEmpty(in.URL, strings.TrimSuffix(draft.URL, "/api/mcp")))
		if err != nil {
			writeJSON(w, 400, map[string]any{"error": "that address does not look right"})
			return
		}
		// Make sure BB-8 itself reaches it before sending the person there.
		if msg := haProbe(ctx, f.HTTP, base); msg != "" {
			writeJSON(w, 502, map[string]any{"error": msg})
			return
		}
		draft.URL, draft.Name = base+"/api/mcp", "home"
		state := newState(callback)
		f.park(state, &flow{draft: draft, base: base, redirect: callback})
		writeJSON(w, 200, map[string]any{"auth_url": HAAuthURL(base, origin+"/", callback, state)})
		return
	}

	if kind.SignIn == "google" {
		id, secret := f.Client(in.Kind)
		if id == "" || secret == "" {
			writeJSON(w, 400, map[string]any{"error": "set up the Google client first (its ID and secret)"})
			return
		}
		bridge := f.bridge(origin)
		draft.Name, draft.URL = "google", kind.URL
		if in.Kind == KindGoogleHome {
			draft.Name = "google_home"
		}
		scope := kind.Scope
		if scope == "" {
			scope = GoogleScope
		}
		draft.Auth = Auth{Type: "oauth", ClientID: id, ClientSecret: secret, AuthMethod: "client_secret_post", TokenURL: GoogleMeta.TokenEndpoint}
		verifier, challenge := PKCE()
		state := newState(callback)
		f.park(state, &flow{draft: draft, verifier: verifier, redirect: bridge, userinfo: GoogleMeta.UserinfoEndpoint})
		writeJSON(w, 200, map[string]any{"auth_url": googleAuthURL(draft.Auth, bridge, state, challenge, scope)})
		return
	}

	if in.Kind == "custom" && in.ID == "" {
		u, err := url.Parse(strings.TrimSpace(in.URL))
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			writeJSON(w, 400, map[string]any{"error": "paste the full link, starting with https://"})
			return
		}
		draft.URL = u.String()
		draft.Name = strings.Split(strings.TrimPrefix(strings.TrimPrefix(u.Hostname(), "www."), "mcp."), ".")[0]
	}
	if tok := strings.TrimSpace(in.Token); tok != "" {
		draft.Auth = Auth{Type: "bearer", Token: tok}
		f.save(w, draft)
		return
	}
	d, err := Discover(ctx, f.HTTP, draft.URL)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": short(err)})
		return
	}
	if !d.NeedsAuth {
		draft.Auth = Auth{Type: "none"}
		f.save(w, draft)
		return
	}
	bridge := f.bridge(origin)
	a, err := Register(ctx, f.HTTP, d.Meta, bridge)
	if err != nil {
		writeJSON(w, 502, map[string]any{"error": short(err)})
		return
	}
	a.Resource = d.Resource
	draft.Auth = a
	verifier, challenge := PKCE()
	state := newState(callback)
	f.park(state, &flow{draft: draft, verifier: verifier, redirect: bridge, userinfo: d.Meta.UserinfoEndpoint})
	writeJSON(w, 200, map[string]any{"auth_url": AuthURL(d.Meta, a, bridge, state, challenge, d.Scope)})
}

func firstNonEmpty(a ...string) string {
	for _, s := range a {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// haProbe checks that Home Assistant answers and has its MCP server integration
// (without a token /api/mcp says 401; 404 means the integration is missing).
func haProbe(ctx context.Context, hc *http.Client, base string) string {
	req, _ := http.NewRequestWithContext(ctx, "POST", base+"/api/mcp", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	r, err := hc.Do(req)
	if err != nil {
		slog.Info("home assistant probe", "base", base, "err", err)
		return "BB-8 can't reach Home Assistant at " + base + ". Check the address (as you open it in the browser) and that it is on."
	}
	r.Body.Close()
	if r.StatusCode == 404 {
		return "Home Assistant answered, but the “Model Context Protocol Server” integration is not added yet (step 1)."
	}
	return ""
}

func short(err error) string {
	s := err.Error()
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "no answer (timeout)"
	}
	if len(s) > 240 {
		s = s[:240] + "…"
	}
	return s
}

func (f *Flows) park(state string, fl *flow) {
	fl.at = time.Now()
	f.mu.Lock()
	defer f.mu.Unlock()
	for k, p := range f.pending {
		if time.Since(p.at) > 15*time.Minute {
			delete(f.pending, k)
		}
	}
	f.pending[state] = fl
}

// save adds the draft (or updates the account it re-signs) and answers the page.
func (f *Flows) save(w http.ResponseWriter, draft Connection) {
	c, err := f.commit(draft)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"connected": c.View()})
}

func (f *Flows) commit(draft Connection) (Connection, error) {
	if draft.Owner != "" {
		draft.Strangers = false
	}
	if draft.ID == "" && draft.Account != "" { // the same account signing in again replaces itself
		for _, c := range f.Store.List() {
			if c.Kind == draft.Kind && c.URL == draft.URL && c.Account == draft.Account {
				draft.ID = c.ID
				break
			}
		}
	}
	if draft.ID == "" {
		return f.Store.Add(draft)
	}
	return f.Store.Update(draft.ID, true, func(c *Connection) { *c = draft })
}

var resultPage = template.Must(template.New("r").Parse(`<!doctype html><meta charset="utf-8"><title>BB-8</title>
<body style="background:#0d0706;color:#faf6f2;font:16px system-ui;display:grid;place-items:center;height:90vh">
<p>{{if .Error}}Could not connect: {{.Error}}{{else}}Connected. Going back…{{end}}</p>
<script>
try { sessionStorage.setItem("bb8-connect", {{.JSON}}); } catch (e) {}
let t = ""; try { t = localStorage.getItem("bb8-token") || ""; } catch (e) {}
location.replace("/" + (t ? "?token=" + encodeURIComponent(t) : "") + "#services");
</script>`))

// Callback finishes a sign-in. It is reached by the person's browser, coming
// back from the service; the single-use state ties it to a start the page made.
func (f *Flows) Callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f.mu.Lock()
	fl := f.pending[q.Get("state")]
	delete(f.pending, q.Get("state"))
	f.mu.Unlock()
	reply := func(res map[string]any) {
		j, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		e, _ := res["error"].(string)
		_ = resultPage.Execute(w, map[string]any{"Error": e, "JSON": string(j)})
	}
	if fl == nil {
		reply(map[string]any{"error": "this sign-in expired or was already used; try again"})
		return
	}
	if e := q.Get("error"); e != "" {
		reply(map[string]any{"error": strings.TrimSpace(e + " " + q.Get("error_description"))})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	draft := fl.draft
	if fl.base != "" {
		clientID := strings.TrimSuffix(fl.redirect, CallbackPath) + "/"
		a, err := HAExchange(ctx, f.HTTP, fl.base, clientID, q.Get("code"))
		if err != nil {
			reply(map[string]any{"error": short(err)})
			return
		}
		draft.Auth = a
		draft.Account = HAName(ctx, f.HTTP, fl.base, a.Token)
	} else {
		a := draft.Auth
		if err := Exchange(ctx, f.HTTP, &a, q.Get("code"), fl.verifier, fl.redirect); err != nil {
			reply(map[string]any{"error": short(err)})
			return
		}
		draft.Auth = a
		if acct := Userinfo(ctx, f.HTTP, fl.userinfo, a.Token); acct != "" {
			draft.Account = acct
		}
	}
	c, err := f.commit(draft)
	if err != nil {
		reply(map[string]any{"error": err.Error()})
		return
	}
	slog.Info("connection signed in", "kind", c.Kind, "owner", c.Owner, "account", c.Account)
	reply(map[string]any{"connected": c.View()})
}

// bridge is where services send sign-ins back: the setting, else bb8-web's own
// /oauth/ next to the page (it forwards to the callback through the robot's link).
func (f *Flows) bridge(origin string) string {
	if f.Bridge != nil {
		if b := strings.TrimSpace(f.Bridge()); b != "" {
			return b
		}
	}
	return strings.TrimRight(origin, "/") + "/oauth/"
}
