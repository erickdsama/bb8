// Package mcpclient lets BB-8's agent use tools from remote MCP servers
// (Zapier, Composio, Home Assistant, n8n…) over streamable HTTP.
//
// Each configured server's tools are registered as "<name>_<tool>". They never
// move the robot, but they act on the outside world, so they are KnownOnly:
// an unknown voice cannot call them unless the slot says otherwise.
package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connectors"
	"github.com/erickdsama/bb8/go/internal/tools"
)

// Slots is how many servers the config form offers.
const Slots = 3

func key(i int, f string) string { return fmt.Sprintf("connectors.mcp.%d.%s", i+1, f) }

type server struct {
	name, url string
	sess      *mcp.ClientSession
	err       error
	tools     int
}

type Client struct {
	HTTP    *http.Client
	Timeout time.Duration

	mu      sync.Mutex
	servers []*server
}

func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 60 * time.Second}, Timeout: 10 * time.Second}
}

func (c *Client) ID() string { return "mcp" }

func (c *Client) Fields() []config.Field {
	var fs []config.Field
	for i := 0; i < Slots; i++ {
		n := fmt.Sprint(i + 1)
		fs = append(fs,
			config.Field{Key: key(i, "name"), Label: "Server " + n + " name", Type: config.String, Default: "",
				Help: "Short prefix for its tools, e.g. zapier → zapier_gmail_send. Empty = slot off."},
			config.Field{Key: key(i, "url"), Label: "Server " + n + " URL", Type: config.String, Default: "",
				Help: "Streamable HTTP endpoint, e.g. https://mcp.zapier.com/api/mcp/s/…/mcp"},
			config.Field{Key: key(i, "token"), Label: "Server " + n + " bearer token", Type: config.Secret,
				Help: "Sent as Authorization: Bearer …. Leave empty if the URL already carries the key."},
			config.Field{Key: key(i, "unknown_voices"), Label: "Server " + n + ": unknown voices may use it", Type: config.Bool, Default: false,
				Help: "Off: only known voices (and the web page) can call these tools."},
		)
	}
	return fs
}

func (c *Client) Status(s *config.Store) connectors.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.servers) == 0 {
		return connectors.Status{ID: c.ID(), State: connectors.NeedsConf, Detail: "add a server name and URL"}
	}
	var parts []string
	state := connectors.OK
	for _, sv := range c.servers {
		if sv.err != nil {
			state = connectors.Error
			parts = append(parts, sv.name+": "+sv.err.Error())
		} else {
			parts = append(parts, fmt.Sprintf("%s: %d tools", sv.name, sv.tools))
		}
	}
	return connectors.Status{ID: c.ID(), State: state, Detail: strings.Join(parts, "; ")}
}

// Close drops every session.
func (c *Client) Close() {
	c.mu.Lock()
	old := c.servers
	c.servers = nil
	c.mu.Unlock()
	for _, sv := range old {
		if sv.sess != nil {
			_ = sv.sess.Close()
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

// bearer adds the slot's token to every request.
type bearer struct {
	token string
	base  http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r)
}

func (c *Client) connect(ctx context.Context, url, token string) (*mcp.ClientSession, error) {
	hc := c.HTTP
	if token != "" {
		cp := *c.HTTP
		base := cp.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		cp.Transport = bearer{token, base}
		hc = &cp
	}
	cl := mcp.NewClient(&mcp.Implementation{Name: "bb8", Version: "0.5"}, nil)
	return cl.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url, HTTPClient: hc, DisableStandaloneSSE: true}, nil)
}

// Tools connects to every configured server (replacing old sessions) and wraps its tools.
func (c *Client) Tools(s *config.Store) []tools.Tool {
	c.Close()
	var out []tools.Tool
	var servers []*server
	for i := 0; i < Slots; i++ {
		name := strings.TrimSpace(s.String(key(i, "name")))
		url := strings.TrimSpace(s.String(key(i, "url")))
		if name == "" || url == "" {
			continue
		}
		sv := &server{name: name, url: url}
		servers = append(servers, sv)
		ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
		ts, err := c.load(ctx, sv, s.Secret(key(i, "token")), !s.Bool(key(i, "unknown_voices")))
		cancel()
		if err != nil {
			sv.err = err
			slog.Warn("mcp server", "name", name, "err", err)
			continue
		}
		sv.tools = len(ts)
		out = append(out, ts...)
	}
	c.mu.Lock()
	c.servers = servers
	c.mu.Unlock()
	return out
}

func (c *Client) load(ctx context.Context, sv *server, token string, knownOnly bool) ([]tools.Tool, error) {
	sess, err := c.connect(ctx, sv.url, token)
	if err != nil {
		return nil, err
	}
	sv.sess = sess
	var out []tools.Tool
	for t, err := range sess.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		schema, _ := toMap(t.InputSchema)
		if schema == nil {
			schema = tools.Object(map[string]any{})
		}
		desc := t.Description
		if len(desc) > 1000 { // long descriptions cost tokens on every turn
			desc = desc[:1000] + "…"
		}
		remote := t.Name
		out = append(out, tools.Tool{
			Name:        toolName(sv.name, remote),
			Description: "[" + sv.name + "] " + desc,
			Schema:      schema,
			KnownOnly:   knownOnly,
			Handler: func(ctx context.Context, args json.RawMessage) (tools.Output, error) {
				return c.call(ctx, sv, remote, args)
			},
		})
	}
	return out, nil
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

func (c *Client) call(ctx context.Context, sv *server, name string, args json.RawMessage) (tools.Output, error) {
	if sv.sess == nil {
		return tools.Output{}, errors.New(sv.name + " is not connected")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	res, err := sv.sess.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return tools.Data(map[string]any{"result": "error", "reason": sv.name + ": " + err.Error()})
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
	if res.StructuredContent != nil && joined == "" {
		out["data"] = res.StructuredContent
	}
	return tools.Output{Data: out, Image: img}, nil
}
