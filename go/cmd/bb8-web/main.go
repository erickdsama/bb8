// bb8-web serves BB-8's web pages off the robot. The robot dials out to it
// (bb8-body --web ws://host:8780/robot) and every page request reaches the robot
// through that link (go/internal/relay). Runs on any computer: for now the home
// PC, later a public host.
//
//	bb8-web --addr :8780 --data ~/.bb8-web
//
// <data> holds device-token (copy it to the robot's --web-token-file), password
// (the owner's login; created on the first run and printed once) and the
// session key.
package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/erickdsama/bb8/go/internal/relay"
	"github.com/erickdsama/bb8/go/internal/webui"
)

func env(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func home(p string) string {
	if strings.HasPrefix(p, "~/") {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, p[2:])
	}
	return p
}

// secret reads <dir>/<name>, creating it with n random bytes (hex) the first time.
func secret(dir, name string, n int) (string, bool, error) {
	p := filepath.Join(dir, name)
	if b, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		return strings.TrimSpace(string(b)), false, nil
	} else if !errors.Is(err, os.ErrNotExist) && err != nil {
		return "", false, err
	}
	b := make([]byte, n)
	_, _ = rand.Read(b)
	v := hex.EncodeToString(b)
	return v, true, os.WriteFile(p, []byte(v+"\n"), 0o600)
}

const cookieName = "bb8_session"

type auth struct {
	key      []byte
	password string
	off      bool // --no-login (loopback only)
}

func (a *auth) sign(exp int64) string {
	m := hmac.New(sha256.New, a.key)
	fmt.Fprint(m, exp)
	return strconv.FormatInt(exp, 10) + "." + base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (a *auth) ok(r *http.Request) bool {
	if a.off {
		return true
	}
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	exp, _, found := strings.Cut(c.Value, ".")
	t, err := strconv.ParseInt(exp, 10, 64)
	if !found || err != nil || time.Now().Unix() > t {
		return false
	}
	return hmac.Equal([]byte(c.Value), []byte(a.sign(t)))
}

var loginPage = template.Must(template.New("l").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1"><title>BB-8 · Sign in</title>
<style>
body{margin:0;min-height:100vh;display:grid;place-items:center;color:#faf6f2;font:15px/1.5 system-ui,sans-serif;
background:#040305;background-image:linear-gradient(135deg,#040305,#0d0706 50%,#2b0e04 90%,#521905)}
form{width:min(340px,calc(100vw - 32px));display:grid;gap:12px;padding:28px;border:1px solid rgba(255,120,40,.3);border-radius:18px;background:rgba(16,10,9,.85)}
.logo{width:30px;height:30px;border-radius:50%;background:radial-gradient(circle at 32% 32%,#ff8533,#ff5a00);box-shadow:0 0 18px rgba(255,90,0,.5)}
h1{margin:0;font-size:20px} p{margin:0;color:#a8968b;font-size:14px}
input{font:inherit;color:inherit;background:rgba(255,255,255,.05);border:1px solid rgba(255,120,40,.25);border-radius:10px;padding:10px 12px}
button{font:inherit;font-weight:600;border:0;border-radius:10px;padding:10px;background:#ff5a00;color:#fff;cursor:pointer}
.err{color:#fca5a5}
</style></head><body><form method="post" action="/login">
<div class="logo"></div><h1>BB-8</h1><p>Sign in to talk to your robot.</p>
{{if .Err}}<p class="err">{{.Err}}</p>{{end}}
<input type="password" name="password" placeholder="Password" autofocus autocomplete="current-password">
<input type="hidden" name="next" value="{{.Next}}"><button>Sign in</button></form></body></html>`))

func main() {
	addr := flag.String("addr", env("BB8_WEB_ADDR", ":8780"), "listen address")
	dataDir := flag.String("data", env("BB8_WEB_DATA", "~/.bb8-web"), "device-token, password and session key")
	noLogin := flag.Bool("no-login", false, "development only: no password (refused unless --addr is loopback)")
	flag.Parse()
	dir := home(*dataDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		slog.Error("data dir", "err", err)
		os.Exit(1)
	}
	device, created, err := secret(dir, "device-token", 24)
	if err != nil {
		slog.Error("device token", "err", err)
		os.Exit(1)
	}
	if created {
		slog.Info("new device token: copy it to the robot", "file", filepath.Join(dir, "device-token"))
	}
	key, _, err := secret(dir, "session-key", 32)
	if err != nil {
		slog.Error("session key", "err", err)
		os.Exit(1)
	}
	pw, created, err := secret(dir, "password", 6)
	if err != nil {
		slog.Error("password", "err", err)
		os.Exit(1)
	}
	if created {
		fmt.Fprintf(os.Stderr, "\nFirst run: the BB-8 web password is %s\n(change it in %s)\n\n", pw, filepath.Join(dir, "password"))
	}
	a := &auth{key: []byte(key), password: pw}
	if *noLogin {
		host, _, _ := net.SplitHostPort(*addr)
		if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
			slog.Error("--no-login only with a loopback --addr, e.g. 127.0.0.1:8780")
			os.Exit(1)
		}
		a.off = true
	}

	rl := &relay.Server{Token: func() string { return device }}
	pages := http.NewServeMux()
	webui.Routes(pages)

	mux := http.NewServeMux()
	mux.Handle("GET /robot", rl.RobotHandler())
	// Sign-ins (Zapier, Home Assistant, Google) come back here without a session;
	// the robot checks their single-use state.
	mux.Handle("GET /api/connections/callback", rl)
	// Google Calendar's change notices (no data, only "changed"); the robot checks their channel token.
	mux.Handle("POST /hooks/google", rl)
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		_ = loginPage.Execute(w, map[string]string{"Next": r.URL.Query().Get("next")})
	})
	mux.HandleFunc("POST /login", func(w http.ResponseWriter, r *http.Request) {
		next := r.FormValue("next")
		if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
			next = "/"
		}
		if subtle.ConstantTimeCompare([]byte(r.FormValue("password")), []byte(a.password)) != 1 {
			time.Sleep(700 * time.Millisecond) // slow down guessing
			w.WriteHeader(http.StatusUnauthorized)
			_ = loginPage.Execute(w, map[string]string{"Next": next, "Err": "Wrong password."})
			return
		}
		exp := time.Now().Add(30 * 24 * time.Hour)
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: a.sign(exp.Unix()), Path: "/", Expires: exp,
			HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode})
		http.Redirect(w, r, next, http.StatusSeeOther)
	})
	mux.HandleFunc("GET /logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /api/web", func(w http.ResponseWriter, r *http.Request) {
		if !a.ok(r) {
			http.Error(w, "sign in", http.StatusUnauthorized)
			return
		}
		up, since := rl.Connected()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"robot": up, "since": since})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		api := strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/mcp"
		if !a.ok(r) {
			if api {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"sign in","login":"/login"}`))
				return
			}
			http.Redirect(w, r, "/login?next="+r.URL.RequestURI(), http.StatusSeeOther)
			return
		}
		if api {
			rl.ServeHTTP(w, r)
			return
		}
		pages.ServeHTTP(w, r)
	})

	slog.Info("bb8-web ready", "addr", *addr, "robot_dials", "ws://<this host>"+*addr+"/robot", "login", !a.off)
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}
}
