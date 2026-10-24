package connections

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

// The MCP authorization flow (spec 2025-06-18): the server answers 401 and points
// to its protected-resource metadata (RFC 9728), which names the authorization
// server (RFC 8414). BB-8 registers itself there (RFC 7591), sends the person to
// sign in with PKCE (RFC 7636) and keeps the refresh token.

// ASMeta is the part of the authorization server metadata BB-8 uses.
type ASMeta struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint"`
	UserinfoEndpoint      string   `json:"userinfo_endpoint"`
	ScopesSupported       []string `json:"scopes_supported"`
}

// Discovery is what probing an MCP server found.
type Discovery struct {
	NeedsAuth bool
	Meta      ASMeta
	Scope     string
	Resource  string
}

var wwwParam = regexp.MustCompile(`(\w+)="([^"]*)"`)

func getJSON(ctx context.Context, hc *http.Client, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	r, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("%s: HTTP %d", u, r.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}

// wellKnown inserts /.well-known/<name> between the origin and the path (RFC 8414 §3).
func wellKnown(base, name string) []string {
	u, err := url.Parse(base)
	if err != nil {
		return nil
	}
	origin := u.Scheme + "://" + u.Host
	p := strings.TrimRight(u.Path, "/")
	out := []string{}
	if p != "" {
		out = append(out, origin+"/.well-known/"+name+p)
	}
	return append(out, origin+"/.well-known/"+name)
}

// Discover probes an MCP endpoint without credentials.
func Discover(ctx context.Context, hc *http.Client, server string) (Discovery, error) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"bb8","version":"0.5"}}}`)
	req, err := http.NewRequestWithContext(ctx, "POST", server, bytes.NewReader(body))
	if err != nil {
		return Discovery{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	r, err := hc.Do(req)
	if err != nil {
		return Discovery{}, err
	}
	io.Copy(io.Discard, io.LimitReader(r.Body, 1<<16))
	r.Body.Close()
	d := Discovery{Resource: server}
	switch {
	case r.StatusCode < 300:
		return d, nil // open server: no sign-in
	case r.StatusCode != 401 && r.StatusCode != 403:
		return d, fmt.Errorf("the service answered HTTP %d", r.StatusCode)
	}
	d.NeedsAuth = true
	params := map[string]string{}
	for _, m := range wwwParam.FindAllStringSubmatch(r.Header.Get("WWW-Authenticate"), -1) {
		params[strings.ToLower(m[1])] = m[2]
	}
	d.Scope = params["scope"]

	// Protected resource metadata → its authorization server.
	var prm struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
	}
	cands := wellKnown(server, "oauth-protected-resource")
	if u := params["resource_metadata"]; u != "" {
		cands = append([]string{u}, cands...)
	}
	asURL := ""
	for _, u := range cands {
		if getJSON(ctx, hc, u, &prm) == nil && len(prm.AuthorizationServers) > 0 {
			asURL = prm.AuthorizationServers[0]
			if prm.Resource != "" {
				d.Resource = prm.Resource
			}
			if d.Scope == "" {
				d.Scope = strings.Join(prm.ScopesSupported, " ")
			}
			break
		}
	}
	if asURL == "" { // no metadata (Zapier): the server's origin is the issuer
		u, _ := url.Parse(server)
		asURL = u.Scheme + "://" + u.Host
	}
	ok := false
	for _, u := range append(wellKnown(asURL, "oauth-authorization-server"), wellKnown(asURL, "openid-configuration")...) {
		if getJSON(ctx, hc, u, &d.Meta) == nil && d.Meta.AuthorizationEndpoint != "" && d.Meta.TokenEndpoint != "" {
			ok = true
			break
		}
	}
	if !ok {
		return d, errors.New("this service needs a sign-in BB-8 cannot find; paste a token instead")
	}
	if d.Scope == "" && slices.Contains(d.Meta.ScopesSupported, "openid") && slices.Contains(d.Meta.ScopesSupported, "email") {
		d.Scope = "openid email" // lets BB-8 show which account signed in
		if slices.Contains(d.Meta.ScopesSupported, "profile") {
			d.Scope += " profile"
		}
	}
	return d, nil
}

// Register adds BB-8 as a client of the authorization server.
func Register(ctx context.Context, hc *http.Client, m ASMeta, redirect string) (Auth, error) {
	if m.RegistrationEndpoint == "" {
		return Auth{}, errors.New("this service does not let apps register by themselves; paste a token instead")
	}
	body, _ := json.Marshal(map[string]any{
		"client_name": "BB-8", "redirect_uris": []string{redirect}, "grant_types": []string{"authorization_code", "refresh_token"},
		"response_types": []string{"code"}, "token_endpoint_auth_method": "none",
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", m.RegistrationEndpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	r, err := hc.Do(req)
	if err != nil {
		return Auth{}, err
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if r.StatusCode >= 300 {
		return Auth{}, fmt.Errorf("registering BB-8: HTTP %d %.200s", r.StatusCode, b)
	}
	var out struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		AuthMethod   string `json:"token_endpoint_auth_method"`
	}
	if err := json.Unmarshal(b, &out); err != nil || out.ClientID == "" {
		return Auth{}, fmt.Errorf("registering BB-8: no client_id in %.200s", b)
	}
	if out.AuthMethod == "" {
		out.AuthMethod = "none"
		if out.ClientSecret != "" {
			out.AuthMethod = "client_secret_basic"
		}
	}
	return Auth{Type: "oauth", ClientID: out.ClientID, ClientSecret: out.ClientSecret, AuthMethod: out.AuthMethod, TokenURL: m.TokenEndpoint}, nil
}

// PKCE returns a verifier and its S256 challenge.
func PKCE() (verifier, challenge string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func AuthURL(m ASMeta, a Auth, redirect, state, challenge, scope string) string {
	q := url.Values{"response_type": {"code"}, "client_id": {a.ClientID}, "redirect_uri": {redirect}, "state": {state},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}}
	if a.Resource != "" {
		q.Set("resource", a.Resource)
	}
	if scope != "" {
		q.Set("scope", scope)
	}
	sep := "?"
	if strings.Contains(m.AuthorizationEndpoint, "?") {
		sep = "&"
	}
	return m.AuthorizationEndpoint + sep + q.Encode()
}

// tokenRequest posts a token-endpoint form and stores the answer in a.
func tokenRequest(ctx context.Context, hc *http.Client, a *Auth, form url.Values) error {
	if a.Resource != "" && a.Type == "oauth" {
		form.Set("resource", a.Resource)
	}
	if a.AuthMethod == "client_secret_post" {
		form.Set("client_id", a.ClientID)
		form.Set("client_secret", a.ClientSecret)
	} else if a.AuthMethod != "client_secret_basic" {
		form.Set("client_id", a.ClientID)
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", a.TokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if a.AuthMethod == "client_secret_basic" {
		req.SetBasicAuth(url.QueryEscape(a.ClientID), url.QueryEscape(a.ClientSecret))
	}
	r, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	var out struct {
		AccessToken  string  `json:"access_token"`
		RefreshToken string  `json:"refresh_token"`
		ExpiresIn    float64 `json:"expires_in"`
		Error        string  `json:"error"`
		Desc         string  `json:"error_description"`
	}
	_ = json.Unmarshal(b, &out)
	if r.StatusCode >= 300 || out.AccessToken == "" {
		if out.Error != "" {
			return fmt.Errorf("sign-in: %s %s", out.Error, out.Desc)
		}
		return fmt.Errorf("sign-in: HTTP %d %.200s", r.StatusCode, b)
	}
	a.Token = out.AccessToken
	if out.RefreshToken != "" {
		a.Refresh = out.RefreshToken
	}
	a.Expiry = time.Time{}
	if out.ExpiresIn > 0 {
		a.Expiry = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	}
	return nil
}

// Exchange trades the authorization code for tokens.
func Exchange(ctx context.Context, hc *http.Client, a *Auth, code, verifier, redirect string) error {
	f := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirect}}
	if verifier != "" {
		f.Set("code_verifier", verifier)
	}
	return tokenRequest(ctx, hc, a, f)
}

// RefreshToken gets a new access token.
func RefreshToken(ctx context.Context, hc *http.Client, a *Auth) error {
	if a.Refresh == "" {
		return errors.New("signed out: sign in again")
	}
	return tokenRequest(ctx, hc, a, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {a.Refresh}})
}

// Userinfo returns the e-mail (or name) of who signed in, if the server says.
func Userinfo(ctx context.Context, hc *http.Client, endpoint, token string) string {
	if endpoint == "" {
		return ""
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r, err := hc.Do(req)
	if err != nil {
		return ""
	}
	defer r.Body.Close()
	var u struct{ Email, Name string }
	if r.StatusCode != 200 || json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&u) != nil {
		return ""
	}
	if u.Email != "" {
		return u.Email
	}
	return u.Name
}
