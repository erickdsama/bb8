// Package mcpserver exposes the tool registry over MCP (streamable HTTP).
package mcpserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/erickdsama/bb8/go/internal/tools"
)

// New builds the MCP server and keeps it in sync with the registry.
// MCP clients (Claude) are treated as a known speaker.
func New(reg *tools.Registry, instructions string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "bb8", Version: "0.4"}, &mcp.ServerOptions{Instructions: instructions})
	var (
		mu    sync.Mutex
		names []string
	)
	// Connectors register their tools from goroutines: subscribe first, then sync.
	resync := func() {
		mu.Lock()
		defer mu.Unlock()
		s.RemoveTools(names...)
		names = names[:0]
		for _, t := range reg.List() {
			names = append(names, t.Name)
			s.AddTool(&mcp.Tool{Name: t.Name, Description: t.Description, InputSchema: t.Schema}, handler(reg, t.Name))
		}
	}
	reg.OnChange(resync)
	resync()
	return s
}

func handler(reg *tools.Registry, name string) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		out, err := reg.Call(ctx, name, req.Params.Arguments, tools.CallOpts{SpeakerKnown: true, Admin: true})
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil
		}
		res := &mcp.CallToolResult{}
		if out.Image != nil {
			res.Content = append(res.Content, &mcp.ImageContent{Data: out.Image, MIMEType: "image/jpeg"})
		}
		if out.Data != nil {
			b, _ := json.Marshal(out.Data)
			res.Content = append(res.Content, &mcp.TextContent{Text: string(b)})
			if _, isObj := out.Data.(map[string]any); isObj {
				res.StructuredContent = out.Data
			}
		}
		if len(res.Content) == 0 {
			res.Content = []mcp.Content{&mcp.TextContent{Text: "{}"}}
		}
		return res, nil
	}
}

// Handler serves /mcp. Requests from loopback pass; others need the bearer token.
func Handler(s *mcp.Server, token func() string) http.Handler {
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, nil)
	return RequireToken(h, token)
}

// RequireToken lets loopback through and asks everyone else for "Bearer <token>".
func RequireToken(h http.Handler, token func() string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsLoopback(r) {
			want := token()
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if q := r.URL.Query().Get("token"); got == "" && q != "" {
				got = q
			}
			if want == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

func IsLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
