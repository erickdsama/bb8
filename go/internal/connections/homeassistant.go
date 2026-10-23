package connections

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/erickdsama/bb8/go/internal/mdns"
)

// Home Assistant signs apps in with its own OAuth flavor (IndieAuth): the
// client_id is the app's URL and the redirect must be on the same host, so the
// page's own address works and no bridge page is needed. Its MCP server lives
// at /api/mcp and takes the same access token (30 min, refreshed here).

// IsHomeAssistant says whether an MCP link is a Home Assistant's (…/api/mcp).
func IsHomeAssistant(u string) bool {
	return strings.HasSuffix(strings.TrimRight(u, "/"), "/api/mcp")
}

// Found is a Home Assistant seen on the network.
type Found struct {
	URL  string `json:"url"`
	Name string `json:"name,omitempty"`
}

// FindHomeAssistant looks for _home-assistant._tcp announcements.
func FindHomeAssistant(ctx context.Context) []Found {
	svcs, _ := mdns.Browse(ctx, "_home-assistant._tcp.local", 2*time.Second)
	var out []Found
	seen := map[string]bool{}
	for _, s := range svcs {
		u := s.TXT["internal_url"]
		if u == "" {
			u = s.TXT["base_url"]
		}
		if u == "" && s.Host != "" {
			u = "http://" + strings.TrimSuffix(s.Host, ".") + ":" + strconv.Itoa(s.Port)
		}
		u = strings.TrimRight(u, "/")
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, Found{URL: u, Name: s.TXT["location_name"]})
	}
	return out
}

// HABase normalizes what a person types ("homeassistant.local:8123") to a base URL.
func HABase(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		s = "homeassistant.local:8123"
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}

func HAAuthURL(base, clientID, redirect, state string) string {
	return base + "/auth/authorize?" + url.Values{"response_type": {"code"}, "client_id": {clientID},
		"redirect_uri": {redirect}, "state": {state}}.Encode()
}

// HAExchange trades the code for tokens.
func HAExchange(ctx context.Context, hc *http.Client, base, clientID, code string) (Auth, error) {
	a := Auth{Type: "homeassistant", ClientID: clientID, TokenURL: base + "/auth/token", AuthMethod: "none"}
	err := tokenRequest(ctx, hc, &a, url.Values{"grant_type": {"authorization_code"}, "code": {code}})
	return a, err
}

// HAName is the home's name from /api/config ("Casa").
func HAName(ctx context.Context, hc *http.Client, base, token string) string {
	req, _ := http.NewRequestWithContext(ctx, "GET", base+"/api/config", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r, err := hc.Do(req)
	if err != nil {
		return ""
	}
	defer r.Body.Close()
	var c struct {
		LocationName string `json:"location_name"`
	}
	if r.StatusCode != 200 || json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&c) != nil {
		return ""
	}
	return c.LocationName
}
