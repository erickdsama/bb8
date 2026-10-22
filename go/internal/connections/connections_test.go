package connections

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// fakeService is an MCP server behind OAuth: 401 with resource metadata, an
// authorization server with registration, PKCE, refresh and userinfo.
type fakeService struct {
	srv                 *httptest.Server
	challenge, verifier string
	refreshes           int
	access              string
}

func newFake(t *testing.T) *fakeService {
	f := &fakeService{access: "tok1"}
	mux := http.NewServeMux()
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	base := f.srv.URL
	mux.HandleFunc("POST /mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.access {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+base+`/.well-known/oauth-protected-resource/mcp", scope="read write"`)
			http.Error(w, "no", 401)
			return
		}
		w.Write([]byte(`{}`))
	})
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"resource": base + "/mcp", "authorization_servers": []string{base}})
	})
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"issuer": base, "authorization_endpoint": base + "/authorize", "token_endpoint": base + "/token",
			"registration_endpoint": base + "/register", "userinfo_endpoint": base + "/userinfo"})
	})
	mux.HandleFunc("POST /register", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		if in["redirect_uris"].([]any)[0] != "https://bridge.example/oauth/" {
			t.Errorf("redirect %v", in["redirect_uris"])
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"client_id":"bb8-client"}`))
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			sum := pkceS256(r.Form.Get("code_verifier"))
			if r.Form.Get("code") != "the-code" || sum != f.challenge || r.Form.Get("client_id") != "bb8-client" || r.Form.Get("resource") != base+"/mcp" {
				http.Error(w, `{"error":"invalid_grant"}`, 400)
				return
			}
			w.Write([]byte(`{"access_token":"tok1","refresh_token":"ref1","expires_in":3600}`))
		case "refresh_token":
			f.refreshes++
			f.access = "tok2"
			w.Write([]byte(`{"access_token":"tok2","expires_in":3600}`))
		}
	})
	mux.HandleFunc("GET /userinfo", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"email":"erick@example.com"}`))
	})
	return f
}

func pkceS256(v string) string { _, c := pkceFor(v); return c }

func pkceFor(v string) (string, string) {
	// same as PKCE() but for a given verifier
	h := sha256Sum(v)
	return v, base64.RawURLEncoding.EncodeToString(h)
}

func TestOAuthSignIn(t *testing.T) {
	fake := newFake(t)
	st, _ := Open("")
	fl := NewFlows(st, http.DefaultClient)
	fl.Bridge = func() string { return "https://bridge.example/oauth/" }
	mux := http.NewServeMux()
	fl.Routes(mux)
	mux.HandleFunc("GET "+CallbackPath, fl.Callback)
	api := httptest.NewServer(mux)
	defer api.Close()

	body, _ := json.Marshal(map[string]any{"kind": "custom", "url": fake.srv.URL + "/mcp", "owner": "Erick", "origin": "http://bb8.local:8765"})
	r, err := http.Post(api.URL+"/api/connections/start", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	json.NewDecoder(r.Body).Decode(&out)
	au, _ := url.Parse(out["auth_url"].(string))
	q := au.Query()
	if !strings.HasPrefix(out["auth_url"].(string), fake.srv.URL+"/authorize?") || q.Get("client_id") != "bb8-client" ||
		q.Get("redirect_uri") != "https://bridge.example/oauth/" || q.Get("scope") != "read write" || q.Get("code_challenge_method") != "S256" {
		t.Fatal(out)
	}
	fake.challenge = q.Get("code_challenge")
	// The state tells the bridge page where to send the browser back.
	raw, _ := base64.RawURLEncoding.DecodeString(q.Get("state"))
	var st8 map[string]string
	json.Unmarshal(raw, &st8)
	if st8["r"] != "http://bb8.local:8765"+CallbackPath {
		t.Fatal(st8)
	}

	// The browser comes back with the code (through the bridge).
	res, err := http.Get(api.URL + CallbackPath + "?code=the-code&state=" + url.QueryEscape(q.Get("state")))
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(page), "connected") || strings.Contains(string(page), "Could not") {
		t.Fatalf("%s", page)
	}
	list := st.List()
	if len(list) != 1 || list[0].Owner != "Erick" || list[0].Account != "erick@example.com" || list[0].Auth.Token != "tok1" || list[0].Auth.Refresh != "ref1" {
		t.Fatalf("%+v", list)
	}
	// A state works once.
	res, _ = http.Get(api.URL + CallbackPath + "?code=the-code&state=" + url.QueryEscape(q.Get("state")))
	page, _ = io.ReadAll(res.Body)
	if !strings.Contains(string(page), "expired") {
		t.Fatalf("%s", page)
	}

	// Signed requests refresh a token about to expire.
	id := list[0].ID
	st.Update(id, false, func(c *Connection) { c.Auth.Expiry = time.Now().Add(10 * time.Second) })
	hc := st.Client(id, http.DefaultClient)
	resp, err := hc.Post(fake.srv.URL+"/mcp", "application/json", strings.NewReader("{}"))
	if err != nil || resp.StatusCode != 200 || fake.refreshes != 1 {
		t.Fatal(err, resp.StatusCode, fake.refreshes)
	}
	if c, _ := st.Get(id); c.Auth.Token != "tok2" || c.Auth.Refresh != "ref1" {
		t.Fatalf("%+v", c.Auth)
	}
	// The page never sees tokens.
	r, _ = http.Get(api.URL + "/api/connections")
	all, _ := io.ReadAll(r.Body)
	if strings.Contains(string(all), "tok2") || strings.Contains(string(all), "ref1") || !strings.Contains(string(all), `"owner":"Erick"`) {
		t.Fatalf("%s", all)
	}
}

func TestOpenServerAndOwnerRules(t *testing.T) {
	open := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }))
	defer open.Close()
	st, _ := Open(t.TempDir())
	fl := NewFlows(st, http.DefaultClient)
	mux := http.NewServeMux()
	fl.Routes(mux)
	api := httptest.NewServer(mux)
	defer api.Close()
	body, _ := json.Marshal(map[string]any{"kind": "custom", "url": open.URL + "/mcp", "origin": "http://bb8.local:8765"})
	r, _ := http.Post(api.URL+"/api/connections/start", "application/json", bytes.NewReader(body))
	var out map[string]map[string]any
	json.NewDecoder(r.Body).Decode(&out)
	if out["connected"]["sign_in"] != "none" || out["connected"]["name"] != "127" {
		t.Fatal(out)
	}
	id := out["connected"]["id"].(string)
	// Shared + strangers, then given to a person: strangers turns off.
	req, _ := http.NewRequest("PATCH", api.URL+"/api/connections/"+id, strings.NewReader(`{"strangers":true}`))
	http.DefaultClient.Do(req)
	req, _ = http.NewRequest("PATCH", api.URL+"/api/connections/"+id, strings.NewReader(`{"owner":"Ana"}`))
	http.DefaultClient.Do(req)
	if c, _ := st.Get(id); c.Owner != "Ana" || c.Strangers {
		t.Fatalf("%+v", c)
	}
	// Survives a restart.
	st2, _ := Open(st.path[:len(st.path)-len("/connections.json")])
	if c, ok := st2.Get(id); !ok || c.Owner != "Ana" {
		t.Fatal("not persisted")
	}
}

func TestHABase(t *testing.T) {
	for in, want := range map[string]string{"": "http://homeassistant.local:8123", "192.168.1.5:8123": "http://192.168.1.5:8123",
		"https://ha.example.com/": "https://ha.example.com"} {
		if got, _ := HABase(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}

func sha256Sum(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

func TestGoogleSignIn(t *testing.T) {
	var form url.Values
	g := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			r.ParseForm()
			form = r.PostForm
			w.Write([]byte(`{"access_token":"g-at","refresh_token":"g-rt","expires_in":3599}`))
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer g-at" {
				w.WriteHeader(401)
				return
			}
			w.Write([]byte(`{"email":"ana@gmail.com"}`))
		}
	}))
	defer g.Close()
	old := GoogleMeta
	GoogleMeta = ASMeta{AuthorizationEndpoint: g.URL + "/auth", TokenEndpoint: g.URL + "/token", UserinfoEndpoint: g.URL + "/userinfo"}
	defer func() { GoogleMeta = old }()

	st, _ := Open("")
	fl := NewFlows(st, http.DefaultClient)
	fl.Bridge = func() string { return "https://bridge.example/oauth/" }
	mux := http.NewServeMux()
	fl.Routes(mux)
	mux.HandleFunc("GET "+CallbackPath, fl.Callback)
	api := httptest.NewServer(mux)
	defer api.Close()
	start := func() (int, map[string]any) {
		body, _ := json.Marshal(map[string]any{"kind": KindGoogle, "owner": "Ana", "origin": "http://debian.local:8780"})
		r, _ := http.Post(api.URL+"/api/connections/start", "application/json", bytes.NewReader(body))
		var out map[string]any
		json.NewDecoder(r.Body).Decode(&out)
		return r.StatusCode, out
	}

	// No Google client yet: a clear message, no redirect.
	if code, out := start(); code != 400 || !strings.Contains(out["error"].(string), "Google client") {
		t.Fatal(code, out)
	}
	fl.Client = func(kind string) (string, string) { return "gid.apps.googleusercontent.com", "gsecret" }
	_, out := start()
	au, _ := url.Parse(out["auth_url"].(string))
	q := au.Query()
	if !strings.HasPrefix(au.String(), g.URL+"/auth?") || q.Get("client_id") != "gid.apps.googleusercontent.com" ||
		q.Get("redirect_uri") != "https://bridge.example/oauth/" || q.Get("access_type") != "offline" || q.Get("prompt") != "consent" ||
		!strings.Contains(q.Get("scope"), "calendar.events") || q.Get("code_challenge") == "" {
		t.Fatal(au)
	}
	res, _ := http.Get(api.URL + CallbackPath + "?code=g-code&state=" + url.QueryEscape(q.Get("state")))
	page, _ := io.ReadAll(res.Body)
	if strings.Contains(string(page), "Could not") {
		t.Fatalf("%s", page)
	}
	if form.Get("client_secret") != "gsecret" || form.Get("code_verifier") == "" || form.Get("redirect_uri") != "https://bridge.example/oauth/" {
		t.Fatal(form)
	}
	list := st.OfKind(KindGoogle)
	if len(list) != 1 || list[0].Owner != "Ana" || list[0].Account != "ana@gmail.com" || list[0].Auth.Refresh != "g-rt" || list[0].Auth.TokenURL != g.URL+"/token" {
		t.Fatalf("%+v", list)
	}
}
