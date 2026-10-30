package connections

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/erickdsama/bb8/go/internal/mdns"
)

// NewHTTP is the client for talking to services: ".local" names resolve over
// mDNS (Home Assistant at homeassistant.local) even in the static build.
func NewHTTP(timeout time.Duration) *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = mdns.Dial
	return &http.Client{Timeout: timeout, Transport: t}
}

// signer adds the connection's token to each request, refreshing it shortly
// before it expires and once more if the server answers 401.
type signer struct {
	store *Store
	id    string
	base  http.RoundTripper
	hc    *http.Client // for refreshes
	mu    *sync.Mutex
}

var refreshLocks sync.Map // id → *sync.Mutex

// Client returns an HTTP client that signs requests as connection id.
func (s *Store) Client(id string, base *http.Client) *http.Client {
	mu, _ := refreshLocks.LoadOrStore(id, &sync.Mutex{})
	rt := base.Transport
	if rt == nil {
		rt = http.DefaultTransport
	}
	cp := *base
	cp.Transport = &signer{store: s, id: id, base: rt, hc: base, mu: mu.(*sync.Mutex)}
	return &cp
}

// Token is connection id's current access token (refreshed if needed), for
// clients that can't take an *http.Client, such as a WebSocket.
func (s *Store) Token(ctx context.Context, id string, base *http.Client) (string, error) {
	mu, _ := refreshLocks.LoadOrStore(id, &sync.Mutex{})
	return (&signer{store: s, id: id, hc: base, mu: mu.(*sync.Mutex)}).token(ctx, false)
}

// token returns a valid access token, refreshing it if it expires within a minute
// (or always when force is set).
func (g *signer) token(ctx context.Context, force bool) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	c, ok := g.store.Get(g.id)
	if !ok {
		return "", ErrUnknown
	}
	a := c.Auth
	if a.Type != "oauth" && a.Type != "homeassistant" {
		return a.Token, nil
	}
	if !force && a.Token != "" && (a.Expiry.IsZero() || time.Until(a.Expiry) > time.Minute) {
		return a.Token, nil
	}
	if err := RefreshToken(ctx, g.hc, &a); err != nil {
		return "", err
	}
	if _, err := g.store.Update(g.id, false, func(c *Connection) { c.Auth = a }); err != nil {
		return "", err
	}
	return a.Token, nil
}

func (g *signer) RoundTrip(r *http.Request) (*http.Response, error) {
	tok, err := g.token(r.Context(), false)
	if err != nil {
		return nil, err
	}
	send := func(tok string) (*http.Response, error) {
		req := r.Clone(r.Context())
		if r.GetBody != nil {
			req.Body, _ = r.GetBody()
		}
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		return g.base.RoundTrip(req)
	}
	res, err := send(tok)
	if err != nil || res.StatusCode != http.StatusUnauthorized || (r.Body != nil && r.GetBody == nil) {
		return res, err
	}
	c, _ := g.store.Get(g.id)
	if c.Auth.Refresh == "" {
		return res, nil
	}
	res.Body.Close()
	if tok, err = g.token(r.Context(), true); err != nil {
		return nil, err
	}
	return send(tok)
}
