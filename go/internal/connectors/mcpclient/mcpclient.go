// Package mcpclient lets BB-8's agent use tools from the accounts in the
// connections store (Zapier, Notion, Home Assistant, any MCP server) over
// streamable HTTP.
//
// Each account's tools are registered as "<name>_<tool>". Two people can
// connect the same service: the tool is registered once, and each call goes to
// the account of whoever is speaking (their own first, then the shared one).
// Tools that send, create or delete ask for a yes first (tools.Tool.Confirm).
package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connections"
	"github.com/erickdsama/bb8/go/internal/connectors"
	"github.com/erickdsama/bb8/go/internal/tools"
)

const keyBridge = "connectors.mcp.oauth_bridge"

// DefaultBridge is the public page that sends sign-ins back to the robot
// (docs/oauth/index.html, published with GitHub Pages).
const DefaultBridge = "https://erickdsama.github.io/bb8/oauth/"

type live struct {
	conn  connections.Connection
	sess  *mcp.ClientSession
	err   error
	tools int
}

type Client struct {
	Store   *connections.Store
	HTTP    *http.Client
	Timeout time.Duration

	// Owner is home.owner: whose accounts the page, MCP clients and a robot
	// without voice ID use.
	Owner func() string

	mu   sync.Mutex
	live map[string]*live // connection id → session
	busy bool             // connecting right now
}

func New(store *connections.Store) *Client {
	return &Client{Store: store, HTTP: connections.NewHTTP(60 * time.Second), Timeout: 12 * time.Second, live: map[string]*live{},
		Owner: func() string { return "" }}
}

func (c *Client) ID() string    { return "mcp" }
func (c *Client) Title() string { return "Apps and smart home" }
func (c *Client) Description() string {
	return "Gmail, Google Calendar, Notion, automations or the lights at home: connect Zapier, Home Assistant and more."
}
func (c *Client) DefaultOn() bool { return true }

func (c *Client) Fields() []config.Field {
	return []config.Field{{Key: keyBridge, Label: "Sign-in bridge page", Type: config.String, Default: DefaultBridge, Advanced: true,
		Help: "Public page that sends sign-ins back to the robot (services only accept https addresses)."}}
}

// Bridge is the configured bridge URL.
func Bridge(s *config.Store) string { return strings.TrimSpace(s.String(keyBridge)) }

// Live reports one connection for the page.
func (c *Client) Live(id string) connections.LiveInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	l, ok := c.live[id]
	switch {
	case !ok && c.busy:
		return connections.LiveInfo{State: "connecting"}
	case !ok:
		return connections.LiveInfo{State: "off"}
	case l.err != nil:
		return connections.LiveInfo{State: "error", Error: l.err.Error()}
	}
	return connections.LiveInfo{State: "ok", Tools: l.tools}
}

// mcpAccounts are the accounts that are MCP servers (Google ones belong to the calendar).
func (c *Client) mcpAccounts() []connections.Connection {
	var out []connections.Connection
	for _, conn := range c.Store.List() {
		if conn.Kind != connections.KindGoogle {
			out = append(out, conn)
		}
	}
	return out
}

func (c *Client) Status(s *config.Store) connectors.Status {
	list := c.mcpAccounts()
	if len(list) == 0 {
		return connectors.Status{ID: c.ID(), State: connectors.NeedsConf, Detail: "nothing connected yet"}
	}
	var parts []string
	state := connectors.OK
	for _, conn := range list {
		l := c.Live(conn.ID)
		who := conn.Owner
		if who == "" {
			who = "shared"
		}
		switch l.State {
		case "error":
			state = connectors.Error
			parts = append(parts, fmt.Sprintf("%s (%s): %s", conn.Name, who, l.Error))
		case "ok":
			parts = append(parts, fmt.Sprintf("%s (%s): %d tools", conn.Name, who, l.Tools))
		default:
			parts = append(parts, fmt.Sprintf("%s (%s): connecting", conn.Name, who))
		}
	}
	return connectors.Status{ID: c.ID(), State: state, Detail: strings.Join(parts, "; ")}
}

// Close drops every session.
func (c *Client) Close() {
	c.mu.Lock()
	old := c.live
	c.live = map[string]*live{}
	c.mu.Unlock()
	for _, l := range old {
		if l.sess != nil {
			_ = l.sess.Close()
		}
	}
}

var badName = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// toolName keeps within the OpenAI function-name rules (^[a-zA-Z0-9_-]{1,64}$).
func toolName(prefix, name string) string {
	n := badName.ReplaceAllString(prefix+"_"+name, "_")
	if len(n) > 64 {
		n = n[:64]
	}
	return n
}

// readOnly: the server says so, or the name says it only looks things up.
var lookName = regexp.MustCompile(`(?i)(^|[_.-])(get|list|find|search|read|query|fetch|lookup|describe|show|retrieve|check)([_.-]|$)|^Get[A-Z]`)

func readOnly(t *mcp.Tool) bool {
	if t.Annotations != nil && t.Annotations.ReadOnlyHint {
		return true
	}
	return lookName.MatchString(t.Name)
}

func (c *Client) connect(ctx context.Context, conn connections.Connection) (*mcp.ClientSession, error) {
	hc := c.Store.Client(conn.ID, c.HTTP)
	cl := mcp.NewClient(&mcp.Implementation{Name: "bb8", Version: "0.6"}, nil)
	if conn.Kind == connections.KindGoogleHome { // Google's server speaks the older SSE transport
		sse := *hc
		sse.Timeout = 0 // the event stream stays open
		return cl.Connect(ctx, &mcp.SSEClientTransport{Endpoint: conn.URL, HTTPClient: &sse}, nil)
	}
	return cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: conn.URL, HTTPClient: hc, DisableStandaloneSSE: true}, nil)
}

// entry is one registered tool name and the accounts that have it.
type entry struct {
	tool     mcp.Tool
	readOnly bool
	confirm  bool
	accounts []*live
}

// Tools connects every account (in parallel, replacing old sessions) and
// registers the union of their tools.
func (c *Client) Tools(s *config.Store) []tools.Tool {
	c.Close()
	conns := c.mcpAccounts()
	c.mu.Lock()
	c.busy = true
	c.mu.Unlock()
	lives := make([]*live, len(conns))
	lists := make([][]*mcp.Tool, len(conns))
	var wg sync.WaitGroup
	for i, conn := range conns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l := &live{conn: conn}
			lives[i] = l
			ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
			defer cancel()
			sess, err := c.connect(ctx, conn)
			if err == nil {
				l.sess = sess
				for t, e := range sess.Tools(ctx, nil) {
					if e != nil {
						err = e
						break
					}
					lists[i] = append(lists[i], t)
				}
			}
			if err != nil {
				l.err = err
				slog.Warn("connection", "name", conn.Name, "owner", conn.Owner, "err", err)
				return
			}
			l.tools = len(lists[i])
		}()
	}
	wg.Wait()
	c.mu.Lock()
	for _, l := range lives {
		c.live[l.conn.ID] = l
	}
	c.busy = false
	c.mu.Unlock()

	byName := map[string]*entry{}
	var names []string
	for i, l := range lives {
		if l.err != nil {
			continue
		}
		for _, t := range lists[i] {
			n := toolName(l.conn.Name, t.Name)
			e, ok := byName[n]
			if !ok {
				e = &entry{tool: *t, readOnly: readOnly(t)}
				byName[n] = e
				names = append(names, n)
			}
			e.accounts = append(e.accounts, l)
			if l.conn.Confirm && !e.readOnly {
				e.confirm = true
			}
		}
	}
	sort.Strings(names)
	var out []tools.Tool
	for _, n := range names {
		e := byName[n]
		schema, _ := toMap(e.tool.InputSchema)
		if schema == nil {
			schema = tools.Object(map[string]any{})
		}
		desc := e.tool.Description
		if len(desc) > 1000 { // long descriptions cost tokens on every turn
			desc = desc[:1000] + "…"
		}
		out = append(out, tools.Tool{
			Name:        n,
			Description: "[" + e.accounts[0].conn.Name + "] " + desc,
			Schema:      schema,
			Confirm:     e.confirm,
			For:         func(o tools.CallOpts) bool { return c.pick(o, e) != nil },
			Handler: func(ctx context.Context, args json.RawMessage) (tools.Output, error) {
				l := c.pick(tools.CallerFrom(ctx), e)
				if l == nil {
					return tools.Output{}, tools.ErrNotYours
				}
				return c.call(ctx, l, e.tool.Name, args)
			},
		})
	}
	return out
}

// pick chooses whose account a call uses: the speaker's own, else a shared one.
// The page, MCP clients and a robot without voice ID act as home.owner.
// Unknown voices (and guests) only get shared accounts that allow strangers,
// and only their read-only tools (tools.Registry refuses the rest).
func (c *Client) pick(o tools.CallOpts, e *entry) *live {
	if !o.SpeakerKnown {
		for _, l := range e.accounts {
			if l.conn.Owner == "" && l.conn.Strangers && e.readOnly {
				return l
			}
		}
		return nil
	}
	person := o.Speaker
	if person == "" {
		person = c.Owner()
	}
	for _, l := range e.accounts {
		if l.conn.Owns(person) {
			return l
		}
	}
	for _, l := range e.accounts {
		if l.conn.Owner == "" {
			return l
		}
	}
	if person == "" && o.Admin { // no owner set yet: an MCP client may use any
		return e.accounts[0]
	}
	return nil
}

func toMap(v any) (map[string]any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	err = json.Unmarshal(b, &m)
	if m != nil && m["type"] == nil {
		m["type"] = "object"
	}
	return m, err
}

func (c *Client) call(ctx context.Context, l *live, name string, args json.RawMessage) (tools.Output, error) {
	if l.sess == nil {
		return tools.Output{}, errors.New(l.conn.Name + " is not connected")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	res, err := l.sess.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return tools.Data(map[string]any{"result": "error", "reason": l.conn.Name + ": " + err.Error()})
	}
	var text []string
	var img []byte
	for _, ct := range res.Content {
		switch v := ct.(type) {
		case *mcp.TextContent:
			text = append(text, v.Text)
		case *mcp.ImageContent:
			if img == nil && strings.Contains(v.MIMEType, "jpeg") {
				img = v.Data
			}
		}
	}
	joined := strings.Join(text, "\n")
	if len(joined) > 8000 { // keep a huge reply from filling the context
		joined = joined[:8000] + "…(cut)"
	}
	if res.IsError {
		return tools.Data(map[string]any{"result": "error", "reason": joined})
	}
	out := map[string]any{"result": "done", "text": joined}
	if l.conn.Owner != "" {
		out["account"] = l.conn.Owner
	}
	if res.StructuredContent != nil && joined == "" {
		out["data"] = res.StructuredContent
	}
	return tools.Output{Data: out, Image: img}, nil
}
