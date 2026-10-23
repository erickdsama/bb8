// Package homeassist lets the page add devices to the house without opening
// Home Assistant: bb8-body drives Home Assistant's own setup flows (the same
// ones its settings use) with the token of the "home" connection, and turns
// each step into something the page can draw: fields, a QR to scan, a link to
// sign in. New devices are exposed to Bebocho so its tools see them at once.
//
// The token never leaves the robot; the page only sees steps.
package homeassist

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/net/websocket"

	"github.com/erickdsama/bb8/go/internal/connections"
)

// Brand is one tile in "Add devices": a Home Assistant integration.
type Brand struct {
	Handler string `json:"handler"`
	Label   string `json:"label"`
	What    string `json:"what"`
	Linked  bool   `json:"linked"`
}

// Brands are the ones most homes have, in this order.
var Brands = []Brand{
	{Handler: "tuya", Label: "Smart Life", What: "Focos, contactos y apagadores (Tuya)"},
	{Handler: "alexa_devices", Label: "Alexa", What: "Tus Echo"},
	{Handler: "smartthings", Label: "SmartThings", What: "Samsung: TV, lavadora, refri"},
	{Handler: "lg_thinq", Label: "LG ThinQ", What: "LG: TV, lavadora, refri"},
	{Handler: "hue", Label: "Philips Hue", What: "Focos Hue"},
	{Handler: "tplink", Label: "TP-Link", What: "Kasa y Tapo"},
	{Handler: "samsungtv", Label: "TV Samsung", What: "Control directo de la TV"},
	{Handler: "cast", Label: "Google Cast", What: "Chromecast y bocinas Nest"},
}

// Domains whose entities Bebocho may use. Locks and alarms stay out: Bebocho
// does not open doors.
var Domains = map[string]bool{"light": true, "switch": true, "climate": true, "fan": true, "cover": true,
	"media_player": true, "vacuum": true, "humidifier": true, "water_heater": true}

// Bridge talks to the Home Assistant of the house's "home" connection.
type Bridge struct {
	Store *connections.Store
	HTTP  *http.Client

	mu    sync.Mutex
	trans map[string]map[string]string // handler → translation key → Spanish text
}

func New(store *connections.Store, hc *http.Client) *Bridge {
	return &Bridge{Store: store, HTTP: hc, trans: map[string]map[string]string{}}
}

var ErrNoHome = errors.New("Home Assistant is not connected")

// home is the connection and its base URL (without /api/mcp).
func (b *Bridge) home() (connections.Connection, string, bool) {
	for _, c := range b.Store.List() {
		if connections.IsHomeAssistant(c.URL) {
			return c, strings.TrimSuffix(strings.TrimRight(c.URL, "/"), "/api/mcp"), true
		}
	}
	return connections.Connection{}, "", false
}

// rest calls Home Assistant's REST API as the home connection.
func (b *Bridge) rest(ctx context.Context, method, path string, in, out any) error {
	c, base, ok := b.home()
	if !ok {
		return ErrNoHome
	}
	var body io.Reader
	if in != nil {
		buf, _ := json.Marshal(in)
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := b.Store.Client(c.ID, b.HTTP).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &e)
		return &httpError{res.StatusCode, firstNonEmpty(e.Message, strings.TrimSpace(string(raw)))}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

type httpError struct {
	Code int
	Msg  string
}

func (e *httpError) Error() string { return fmt.Sprintf("Home Assistant: HTTP %d %s", e.Code, e.Msg) }

// ws runs commands on one authenticated WebSocket session and returns each result.
func (b *Bridge) ws(ctx context.Context, cmds ...map[string]any) ([]json.RawMessage, error) {
	c, base, ok := b.home()
	if !ok {
		return nil, ErrNoHome
	}
	tok, err := b.Store.Token(ctx, c.ID, b.HTTP)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(base + "/api/websocket")
	if err != nil {
		return nil, err
	}
	u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)
	cfg, err := websocket.NewConfig(u.String(), base)
	if err != nil {
		return nil, err
	}
	if t, ok := b.HTTP.Transport.(*http.Transport); ok && t.DialContext != nil { // .local names over mDNS
		cfg.Dialer = nil
		conn, err := t.DialContext(ctx, "tcp", hostPort(u))
		if err != nil {
			return nil, err
		}
		if u.Scheme == "wss" {
			conn = tls.Client(conn, &tls.Config{ServerName: u.Hostname()})
		}
		ws, err := websocket.NewClient(cfg, conn)
		if err != nil {
			conn.Close()
			return nil, err
		}
		return session(ctx, ws, tok, cmds)
	}
	ws, err := websocket.DialConfig(cfg)
	if err != nil {
		return nil, err
	}
	return session(ctx, ws, tok, cmds)
}

func hostPort(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	if u.Scheme == "wss" {
		return u.Host + ":443"
	}
	return u.Host + ":80"
}

func session(ctx context.Context, ws *websocket.Conn, tok string, cmds []map[string]any) ([]json.RawMessage, error) {
	defer ws.Close()
	deadline := time.Now().Add(20 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = ws.SetDeadline(deadline)
	var m struct {
		Type    string          `json:"type"`
		ID      int             `json:"id"`
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := websocket.JSON.Receive(ws, &m); err != nil { // auth_required
		return nil, err
	}
	if err := websocket.JSON.Send(ws, map[string]any{"type": "auth", "access_token": tok}); err != nil {
		return nil, err
	}
	if err := websocket.JSON.Receive(ws, &m); err != nil {
		return nil, err
	}
	if m.Type != "auth_ok" {
		return nil, errors.New("Home Assistant refused the token")
	}
	out := make([]json.RawMessage, len(cmds))
	for i, c := range cmds {
		c["id"] = i + 1
		if err := websocket.JSON.Send(ws, c); err != nil {
			return nil, err
		}
		for {
			m.Result, m.Error = nil, nil
			if err := websocket.JSON.Receive(ws, &m); err != nil {
				return nil, err
			}
			if m.Type == "result" && m.ID == i+1 {
				break
			}
		}
		if !m.Success {
			msg := "failed"
			if m.Error != nil {
				msg = m.Error.Message
			}
			return nil, fmt.Errorf("Home Assistant: %s: %s", c["type"], msg)
		}
		out[i] = m.Result
	}
	return out, nil
}

// ---------- the house: brands, what was found, how many devices ----------

type entry struct {
	Domain string `json:"domain"`
	Title  string `json:"title"`
	State  string `json:"state"`
}

type flowInfo struct {
	FlowID  string `json:"flow_id"`
	Handler string `json:"handler"`
	StepID  string `json:"step_id"`
	Context struct {
		Source       string            `json:"source"`
		Placeholders map[string]string `json:"title_placeholders"`
	} `json:"context"`
}

// Found is a device Home Assistant discovered on the network, one tap to add.
type Found struct {
	FlowID  string `json:"flow_id"`
	Handler string `json:"handler"`
	Label   string `json:"label"`
}

// House is what the page shows.
type House struct {
	Connected bool    `json:"connected"`
	Devices   int     `json:"devices"`
	Brands    []Brand `json:"brands"`
	Found     []Found `json:"found"`
	Error     string  `json:"error,omitempty"`
}

func (b *Bridge) House(ctx context.Context) House {
	h := House{Brands: append([]Brand(nil), Brands...), Found: []Found{}}
	if _, _, ok := b.home(); !ok {
		return h
	}
	h.Connected = true
	var entries []entry
	if err := b.rest(ctx, "GET", "/api/config/config_entries/entry", nil, &entries); err != nil {
		h.Error = err.Error()
		return h
	}
	linked := map[string]bool{}
	for _, e := range entries {
		linked[e.Domain] = true
	}
	for i := range h.Brands {
		h.Brands[i].Linked = linked[h.Brands[i].Handler]
	}
	res, err := b.ws(ctx, map[string]any{"type": "config_entries/flow/progress"}, map[string]any{"type": "get_states"})
	if err != nil {
		h.Error = err.Error()
		return h
	}
	var flows []flowInfo
	_ = json.Unmarshal(res[0], &flows)
	for _, f := range flows {
		if f.Context.Source == "user" || f.Context.Source == "reauth" || f.Context.Source == "" {
			continue // flows someone is in the middle of, not discoveries
		}
		h.Found = append(h.Found, Found{FlowID: f.FlowID, Handler: f.Handler, Label: foundLabel(f)})
	}
	sort.Slice(h.Found, func(i, j int) bool { return h.Found[i].Label < h.Found[j].Label })
	h.Devices = len(usable(res[1]))
	return h
}

func foundLabel(f flowInfo) string {
	for _, k := range []string{"name", "device", "model", "host"} {
		if v := f.Context.Placeholders[k]; v != "" {
			return v
		}
	}
	for _, br := range Brands {
		if br.Handler == f.Handler {
			return br.Label
		}
	}
	return f.Handler
}

// usable are the entity ids Bebocho may use, from a get_states result.
func usable(states json.RawMessage) []string {
	var st []struct {
		EntityID string `json:"entity_id"`
	}
	_ = json.Unmarshal(states, &st)
	var ids []string
	for _, s := range st {
		d, _, _ := strings.Cut(s.EntityID, ".")
		if Domains[d] {
			ids = append(ids, s.EntityID)
		}
	}
	return ids
}

// Expose makes every usable device visible to Bebocho (Home Assistant's
// "Expose" list for its assistant, which its MCP server serves). It returns how many.
func (b *Bridge) Expose(ctx context.Context) (int, error) {
	res, err := b.ws(ctx, map[string]any{"type": "get_states"})
	if err != nil {
		return 0, err
	}
	ids := usable(res[0])
	if len(ids) == 0 {
		return 0, nil
	}
	_, err = b.ws(ctx, map[string]any{"type": "homeassistant/expose_entity", "assistants": []string{"conversation"},
		"entity_ids": ids, "should_expose": true})
	return len(ids), err
}

// Run exposes new devices now and then (they also appear when a brand adds
// one in its own app), until ctx ends.
func (b *Bridge) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if _, _, ok := b.home(); ok {
			c, cancel := context.WithTimeout(ctx, 30*time.Second)
			if _, err := b.Expose(c); err != nil {
				slog.Warn("home assistant expose", "err", err)
			}
			cancel()
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// ---------- setup flows → steps the page draws ----------

// Field is one input of a form step.
type Field struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"` // text, password, number, bool, select
	Label    string   `json:"label"`
	Required bool     `json:"required,omitempty"`
	Default  any      `json:"default,omitempty"`
	Options  []Option `json:"options,omitempty"`
}

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Step is one screen of adding a brand.
type Step struct {
	FlowID  string  `json:"flow_id,omitempty"`
	Handler string  `json:"handler"`
	Type    string  `json:"type"` // form, external, progress, done, abort
	Title   string  `json:"title,omitempty"`
	Text    string  `json:"text,omitempty"`
	Fields  []Field `json:"fields,omitempty"`
	QR      string  `json:"qr,omitempty"`  // data:image/png for a code to scan
	URL     string  `json:"url,omitempty"` // external: open it to sign in, then wait
	Error   string  `json:"error,omitempty"`
	Devices int     `json:"devices,omitempty"` // done: how many Bebocho can use now
}

type flowResult struct {
	Type         string            `json:"type"`
	FlowID       string            `json:"flow_id"`
	Handler      string            `json:"handler"`
	StepID       string            `json:"step_id"`
	DataSchema   []map[string]any  `json:"data_schema"`
	Errors       map[string]string `json:"errors"`
	Placeholders map[string]any    `json:"description_placeholders"`
	URL          string            `json:"url"`
	Reason       string            `json:"reason"`
	MenuOptions  json.RawMessage   `json:"menu_options"`
	Title        string            `json:"title"`
}

// Start begins adding a brand (or continues a discovered device with its flow id).
func (b *Bridge) Start(ctx context.Context, handler, flowID string) (Step, error) {
	var r flowResult
	if flowID != "" {
		if err := b.rest(ctx, "GET", "/api/config/config_entries/flow/"+url.PathEscape(flowID), nil, &r); err != nil {
			return Step{}, err
		}
	} else {
		if handler == "" {
			return Step{}, errors.New("which brand?")
		}
		if err := b.rest(ctx, "POST", "/api/config/config_entries/flow", map[string]any{"handler": handler, "show_advanced_options": false}, &r); err != nil {
			return Step{}, err
		}
	}
	return b.step(ctx, r)
}

// Next sends a form's answers (or nothing, to check on a sign-in or a wait).
func (b *Bridge) Next(ctx context.Context, flowID string, input map[string]any) (Step, error) {
	var r flowResult
	if input == nil {
		if err := b.rest(ctx, "GET", "/api/config/config_entries/flow/"+url.PathEscape(flowID), nil, &r); err != nil {
			return Step{}, err
		}
		if r.Type == "external_done" || r.Type == "progress_done" { // the sign-in finished: move on
			if err := b.rest(ctx, "POST", "/api/config/config_entries/flow/"+url.PathEscape(flowID), map[string]any{}, &r); err != nil {
				return Step{}, err
			}
		}
		return b.step(ctx, r)
	}
	if err := b.rest(ctx, "POST", "/api/config/config_entries/flow/"+url.PathEscape(flowID), input, &r); err != nil {
		return Step{}, err
	}
	return b.step(ctx, r)
}

// Cancel drops a flow the person closed.
func (b *Bridge) Cancel(ctx context.Context, flowID string) error {
	return b.rest(ctx, "DELETE", "/api/config/config_entries/flow/"+url.PathEscape(flowID), nil, nil)
}

func (b *Bridge) step(ctx context.Context, r flowResult) (Step, error) {
	tr := b.translations(ctx, r.Handler)
	key := "component." + r.Handler + ".config."
	s := Step{FlowID: r.FlowID, Handler: r.Handler}
	ph := map[string]string{}
	for k, v := range r.Placeholders {
		ph[k] = fmt.Sprint(v)
	}
	s.Title = fill(tr[key+"step."+r.StepID+".title"], ph)
	s.Text = shortText(fill(tr[key+"step."+r.StepID+".description"], ph))
	switch r.Type {
	case "form":
		s.Type = "form"
		for _, f := range r.DataSchema {
			name, _ := f["name"].(string)
			if sel, ok := f["selector"].(map[string]any); ok {
				if qr, ok := sel["qr_code"].(map[string]any); ok {
					if data, _ := qr["data"].(string); data != "" {
						s.QR = qrPNG(data)
					}
					continue
				}
			}
			fl := field(f)
			fl.Label = firstNonEmpty(tr[key+"step."+r.StepID+".data."+name], humanize(name))
			for i, o := range fl.Options {
				fl.Options[i].Label = firstNonEmpty(tr[key+"step."+r.StepID+".data."+name+"."+o.Value], o.Label)
			}
			s.Fields = append(s.Fields, fl)
		}
		for _, e := range r.Errors {
			s.Error = firstNonEmpty(tr[key+"error."+e], humanize(e))
			break
		}
	case "menu":
		s.Type = "form"
		f := Field{Name: "next_step_id", Type: "select", Label: "Elige", Required: true}
		var opts []string
		if json.Unmarshal(r.MenuOptions, &opts) != nil {
			var m map[string]string
			_ = json.Unmarshal(r.MenuOptions, &m)
			for k := range m {
				opts = append(opts, k)
			}
			sort.Strings(opts)
		}
		for _, o := range opts {
			f.Options = append(f.Options, Option{Value: o, Label: firstNonEmpty(tr[key+"step."+r.StepID+".menu_options."+o], humanize(o))})
		}
		s.Fields = []Field{f}
	case "external":
		s.Type, s.URL = "external", r.URL
	case "external_done", "progress", "show_progress", "progress_done":
		s.Type = "progress"
	case "create_entry":
		s.Type = "done"
		s.Title = r.Title
		n, err := b.Expose(ctx)
		if err != nil {
			slog.Warn("home assistant expose", "err", err)
		}
		s.Devices = n
	case "abort":
		s.Type = "abort"
		s.Error = firstNonEmpty(tr[key+"abort."+r.Reason], humanize(r.Reason))
	default:
		s.Type = "abort"
		s.Error = r.Type
	}
	return s, nil
}

// field reads one serialized voluptuous field (old style "type" or a selector).
func field(f map[string]any) Field {
	name, _ := f["name"].(string)
	fl := Field{Name: name, Type: "text", Default: f["default"]}
	fl.Required, _ = f["required"].(bool)
	if sel, ok := f["selector"].(map[string]any); ok {
		for k, v := range sel {
			cfg, _ := v.(map[string]any)
			switch k {
			case "boolean":
				fl.Type = "bool"
			case "number":
				fl.Type = "number"
			case "select":
				fl.Type = "select"
				opts, _ := cfg["options"].([]any)
				for _, o := range opts {
					switch o := o.(type) {
					case string:
						fl.Options = append(fl.Options, Option{Value: o, Label: o})
					case map[string]any:
						fl.Options = append(fl.Options, Option{Value: fmt.Sprint(o["value"]), Label: fmt.Sprint(o["label"])})
					}
				}
			case "text":
				if t, _ := cfg["type"].(string); t == "password" {
					fl.Type = "password"
				}
			}
		}
		return fl
	}
	switch f["type"] {
	case "boolean":
		fl.Type = "bool"
	case "integer", "float":
		fl.Type = "number"
	case "select":
		fl.Type = "select"
		switch opts := f["options"].(type) {
		case []any:
			for _, o := range opts {
				if pair, ok := o.([]any); ok && len(pair) == 2 {
					fl.Options = append(fl.Options, Option{Value: fmt.Sprint(pair[0]), Label: fmt.Sprint(pair[1])})
				} else {
					fl.Options = append(fl.Options, Option{Value: fmt.Sprint(o), Label: fmt.Sprint(o)})
				}
			}
		case map[string]any:
			for k, v := range opts {
				fl.Options = append(fl.Options, Option{Value: k, Label: fmt.Sprint(v)})
			}
			sort.Slice(fl.Options, func(i, j int) bool { return fl.Options[i].Label < fl.Options[j].Label })
		}
	}
	if strings.Contains(name, "password") || strings.Contains(name, "secret") || strings.Contains(name, "token") {
		fl.Type = "password"
	}
	return fl
}

// translations are Home Assistant's own Spanish texts for a brand's setup.
func (b *Bridge) translations(ctx context.Context, handler string) map[string]string {
	b.mu.Lock()
	t := b.trans[handler]
	b.mu.Unlock()
	if t != nil || handler == "" {
		return t
	}
	res, err := b.ws(ctx, map[string]any{"type": "frontend/get_translations", "language": "es", "category": "config", "integration": []string{handler}})
	if err != nil {
		return map[string]string{}
	}
	var out struct {
		Resources map[string]string `json:"resources"`
	}
	_ = json.Unmarshal(res[0], &out)
	if out.Resources == nil {
		out.Resources = map[string]string{}
	}
	b.mu.Lock()
	b.trans[handler] = out.Resources
	b.mu.Unlock()
	return out.Resources
}

var (
	placeholder = regexp.MustCompile(`\{(\w+)\}`)
	mdLink      = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
)

func fill(s string, ph map[string]string) string {
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := ph[m[1:len(m)-1]]; ok {
			return v
		}
		return ""
	})
}

// shortText keeps the first sentence or two: the page stays short.
func shortText(s string) string {
	s = strings.TrimSpace(mdLink.ReplaceAllString(s, "$1"))
	s = strings.NewReplacer("**", "", "\n\n", " ", "\n", " ").Replace(s)
	if len(s) > 220 {
		if i := strings.Index(s[120:], ". "); i >= 0 {
			s = s[:120+i+1]
		} else {
			s = s[:220] + "…"
		}
	}
	return s
}

func humanize(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func qrPNG(data string) string {
	png, err := qrcode.Encode(data, qrcode.High, 320)
	if err != nil {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

// ---------- HTTP for the page ----------

// Routes: GET /api/home, POST /api/home/link {handler | flow_id}, GET and POST
// /api/home/link/{flow} (POST with the form's answers), DELETE to cancel.
func (b *Bridge) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/home", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		writeJSON(w, 200, b.House(ctx))
	})
	mux.HandleFunc("POST /api/home/link", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Handler string `json:"handler"`
			FlowID  string `json:"flow_id"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<12)).Decode(&in)
		ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
		defer cancel()
		b.reply(w, func() (Step, error) { return b.Start(ctx, in.Handler, in.FlowID) })
	})
	mux.HandleFunc("GET /api/home/link/{flow}", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
		defer cancel()
		b.reply(w, func() (Step, error) { return b.Next(ctx, r.PathValue("flow"), nil) })
	})
	mux.HandleFunc("POST /api/home/link/{flow}", func(w http.ResponseWriter, r *http.Request) {
		in := map[string]any{}
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<14)).Decode(&in)
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		b.reply(w, func() (Step, error) { return b.Next(ctx, r.PathValue("flow"), in) })
	})
	mux.HandleFunc("DELETE /api/home/link/{flow}", func(w http.ResponseWriter, r *http.Request) {
		_ = b.Cancel(r.Context(), r.PathValue("flow"))
		writeJSON(w, 200, map[string]any{"ok": true})
	})
}

func (b *Bridge) reply(w http.ResponseWriter, f func() (Step, error)) {
	s, err := f()
	if err != nil {
		code := 502
		var he *httpError
		if errors.Is(err, ErrNoHome) {
			code = 409
		} else if errors.As(err, &he) && he.Code == 404 {
			code, err = 404, errors.New("that step expired: start again")
		}
		writeJSON(w, code, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, s)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
