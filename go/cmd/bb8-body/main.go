// bb8-body runs on the main Pi: serial owner, motion, floor plan, MCP server,
// local API and the secure link the head dials into.
//
//	bb8-body                       # Arduino on /dev/ttyACM0
//	bb8-body --serial socket://127.0.0.1:5555   # simulated Arduino
//	bb8-body pair --dir /etc/bb8/pki [--host 192.168.1.20]
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/erickdsama/bb8/go/internal/agent"
	"github.com/erickdsama/bb8/go/internal/body"
	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connections"
	"github.com/erickdsama/bb8/go/internal/connectors"
	"github.com/erickdsama/bb8/go/internal/connectors/calendar"
	"github.com/erickdsama/bb8/go/internal/connectors/mcpclient"
	"github.com/erickdsama/bb8/go/internal/connectors/nest"
	"github.com/erickdsama/bb8/go/internal/connectors/weather"
	"github.com/erickdsama/bb8/go/internal/connectors/websearch"
	"github.com/erickdsama/bb8/go/internal/events"
	"github.com/erickdsama/bb8/go/internal/floorplan"
	"github.com/erickdsama/bb8/go/internal/gamepad"
	"github.com/erickdsama/bb8/go/internal/homeassist"
	"github.com/erickdsama/bb8/go/internal/link"
	"github.com/erickdsama/bb8/go/internal/mcpserver"
	"github.com/erickdsama/bb8/go/internal/motion"
	"github.com/erickdsama/bb8/go/internal/navigate"
	"github.com/erickdsama/bb8/go/internal/people"
	"github.com/erickdsama/bb8/go/internal/persona"
	"github.com/erickdsama/bb8/go/internal/relay"
	"github.com/erickdsama/bb8/go/internal/serial"
	"github.com/erickdsama/bb8/go/internal/tools"
	"github.com/erickdsama/bb8/go/internal/tts"
)

func env(keys []string, def string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return def
}

func home(p string) string {
	if h, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, "~/") {
		return filepath.Join(h, p[2:])
	}
	return p
}

func defaultMCP() string {
	if p := os.Getenv("BB8_MCP_PORT"); p != "" {
		return "127.0.0.1:" + p
	}
	return env([]string{"BB8_MCP_ADDR"}, "127.0.0.1:8765")
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "pair" {
		pair(os.Args[2:])
		return
	}
	serialURL := flag.String("serial", env([]string{"BB8_SERIAL"}, "/dev/ttyACM0"), "Arduino port or socket://host:port")
	mapPath := flag.String("map", env([]string{"BB8_MAP", "BB8_MAPA"}, ""), "house plan JSON")
	mapState := flag.String("map-state", env([]string{"BB8_MAP_STATE", "BB8_MAPA_ESTADO"}, "~/.bb8/pose.json"), "saved pose")
	apiAddr := flag.String("api", env([]string{"BB8_API_ADDR"}, "127.0.0.1:8770"), "local API (keep it on loopback)")
	mcpAddr := flag.String("mcp", defaultMCP(), "HTTP listen address for MCP (/mcp) and the config web UI (/); LAN clients need the token")
	dataDir := flag.String("data", env([]string{"BB8_DATA"}, "~/.bb8"), "config.json and secrets.json")
	linkAddr := flag.String("link", env([]string{"BB8_LINK_ADDR"}, ":8772"), "secure head link listen address")
	pki := flag.String("pki", env([]string{"BB8_PKI"}, "~/.bb8/pki"), "certs from 'bb8-body pair'")
	tokenFile := flag.String("token-file", env([]string{"BB8_TOKEN_FILE"}, "~/.bb8/token"), "bearer token for LAN clients")
	webURL := flag.String("web", env([]string{"BB8_WEB"}, ""), "bb8-web to dial out to, e.g. ws://debian:8780/robot (empty: none)")
	webTokenFile := flag.String("web-token-file", env([]string{"BB8_WEB_TOKEN_FILE"}, "~/.bb8/web-token"), "device token for bb8-web")
	pad := flag.Bool("gamepad", os.Getenv("BB8_GAMEPAD") == "1", "read a Bluetooth gamepad (evdev); it wins over the agent")
	verbose := flag.Bool("v", false, "debug logs")
	flag.Parse()

	lvl := slog.LevelInfo
	if *verbose {
		lvl = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The Arduino may not be ready at boot or may reset later: the link connects
	// lazily and reconnects on its own, so the service never dies because of it.
	ser := serial.NewResilient(*serialURL, 115200)
	defer ser.Close()

	cfg, err := config.Open(home(*dataDir))
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	cfg.Register(body.RobotFields()...)
	cfg.Register(agent.Fields()...)
	cfg.Register(navigate.Fields()...)
	cfg.Register(tts.Fields()...)

	svc := motion.New(ser, "", floorplan.New(home(*mapPath), home(*mapState)), motion.Defaults())
	ser.OnConnect = svc.ArduinoUp

	// Secure head link.
	srv := &link.Server{}
	head := body.NewHead(srv)
	gemini := tts.NewGemini(cfg)
	head.Voice = gemini.Speak
	svc.SetToFSource(head.ToF)
	head.HeadDeg = func() float64 { return float64(svc.Status().HeadDeg) }
	head.OnSound = func(text *string, angle, level, headDeg *float64) {
		if err := svc.Hear(text, angle, level, headDeg); err != nil {
			slog.Warn("sound_dir", "err", err)
		}
	}
	tlsCfg, err := link.ServerTLS(home(*pki))
	if err != nil {
		slog.Error("head link needs certs: run 'bb8-body pair'", "dir", *pki, "err", err)
		os.Exit(1)
	}
	ln, err := tls.Listen("tcp", *linkAddr, tlsCfg)
	if err != nil {
		slog.Error("link listen", "err", err)
		os.Exit(1)
	}
	go srv.Serve(ctx, ln)

	body.Wire(cfg, svc, head)

	bus := events.New()
	cfg.Subscribe(func(ch config.Change) {
		keys := make([]string, 0, len(ch.Values))
		for k := range ch.Values {
			keys = append(keys, k)
		}
		bus.Publish("config", map[string]any{"rev": ch.Rev, "source": ch.Source, "keys": keys})
	})
	folks, err := people.Open(home(*dataDir))
	if err != nil {
		slog.Error("people.json", "err", err)
		os.Exit(1)
	}
	folks.Subscribe(func(p people.Person, what string) {
		bus.Publish("person", map[string]any{"name": p.Name, "role": p.Role, "what": what})
	})
	go body.Watch(ctx, bus, svc, head)

	accounts, err := connections.Open(home(*dataDir))
	if err != nil {
		slog.Error("connections.json", "err", err)
		os.Exit(1)
	}
	apps := mcpclient.New(accounts)
	apps.Owner = func() string { return cfg.String("home.owner") }
	reg := tools.NewRegistry()
	conns := connectors.NewManager(cfg, reg, weather.New(), calendar.New(accounts, apps.Owner), nest.New(accounts), websearch.New(), apps)
	accounts.Subscribe(func() {
		conns.Refresh("mcp")
		conns.Refresh("calendar") // Google accounts live in the same store
		conns.Refresh("nest")
		bus.Publish("connections", map[string]any{"count": len(accounts.List())})
	})
	reg.Set("robot", body.RobotTools(svc, head))
	reg.Set("settings", body.SettingTools(cfg))
	reg.Set("people", folks.Tools())
	nav := navigate.New(cfg, svc)
	nav.Publish = bus.Publish
	reg.Set("navigation", nav.Tools())
	conns.Start()
	brain := agent.New(cfg, reg, persona.Instructions)
	brain.People, brain.Publish = folks, bus.Publish

	token := func() string {
		b, _ := os.ReadFile(home(*tokenFile))
		return strings.TrimSpace(string(b))
	}
	mcpSrv := mcpserver.New(reg, persona.Instructions())
	var webUp atomic.Bool // the link to bb8-web
	live := func() map[string]any {
		return map[string]any{"head": head.State(), "connectors": conns.Status(), "agent_ready": brain.Ready() == nil,
			"simulated": strings.HasPrefix(*serialURL, "socket://"), "web_relay": webUp.Load()} // the dummy Arduino: the pages say so
	}
	web := http.NewServeMux()
	web.Handle("/", config.Web(cfg, live))
	alog := body.NewAudioLog(func() bool { return cfg.Bool("voice.audio_log") })
	alog.Routes(web, "/api/audio")
	feed := body.NewVoiceFeed()
	go feed.RunLevels(ctx, head.Audio)
	web.HandleFunc("GET /api/voice/events", feed.SSEHandler)
	brain.Routes(web, "/api/agent")
	folks.Routes(web)
	web.HandleFunc("GET /api/events", bus.Handler)
	// The page plays a Gemini voice before you pick it (WAV, in the browser).
	web.HandleFunc("GET /api/voice/preview", func(w http.ResponseWriter, r *http.Request) {
		wav, err := gemini.Preview(r.Context(), r.URL.Query().Get("voice"), "¡Hola! Soy Bebocho, tu robot de casa. ¿En qué te ayudo?")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(wav)
	})
	signin := connections.NewFlows(accounts, apps.HTTP)
	signin.Bridge = func() string { return mcpclient.Bridge(cfg) }
	signin.Client = func(kind string) (string, string) {
		if kind == connections.KindGoogle || kind == connections.KindGoogleHome { // one Google client for both
			return calendar.GoogleClient(cfg)
		}
		return "", ""
	}
	signin.NestProject = func() string { return cfg.String("connectors.nest.project_id") }
	signin.Owner = apps.Owner
	signin.People = func() []string {
		var names []string
		for _, p := range folks.List() {
			names = append(names, p.Name)
		}
		return names
	}
	signin.Status = func(id string) connections.LiveInfo {
		if c, ok := accounts.Get(id); ok && c.Kind == connections.KindGoogle { // not an MCP server: signed in is ready
			return connections.LiveInfo{State: "ok", Tools: 3}
		}
		return apps.Live(id)
	}
	signin.Routes(web)
	// "Add devices": the page drives Home Assistant's setup through the robot,
	// which holds its token; new devices are exposed to Bebocho every few minutes.
	house := homeassist.New(accounts, apps.HTTP)
	house.Routes(web)
	go house.Run(ctx, 5*time.Minute)
	web.HandleFunc("GET /api/connectors", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(conns.Status())
	})
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpserver.Handler(mcpSrv, token))
	// Sign-ins come back here from the person's browser without the page token;
	// the single-use state ties them to a start the page made.
	mux.HandleFunc("GET "+connections.CallbackPath, signin.Callback)
	// Calendar notices: someone else adds, moves or cancels an event, and reminders.
	// Google's "something changed" comes through bb8-web; its channel token is the check.
	watch := calendar.NewWatcher(calendar.NewGoogle(cfg, accounts, apps.Owner), func(ctx context.Context, text string) {
		slog.Info("calendar notice", "text", text)
		bus.Publish("calendar", map[string]any{"text": text})
		head.Say(ctx, text, "")
	})
	go watch.Run(ctx)
	mux.HandleFunc("POST "+calendar.HookPath, watch.Hook)
	// The pages live off the robot (bb8-web): the robot dials out to it and
	// answers their requests through that link. It opens no port for them.
	if *webURL != "" {
		rc := &relay.Client{URL: *webURL, Handler: mux, OnState: webUp.Store, Token: func() string {
			b, _ := os.ReadFile(home(*webTokenFile))
			return strings.TrimSpace(string(b))
		}}
		go rc.Run(ctx)
	}
	mux.Handle("/", mcpserver.RequireToken(web, token))
	serve(ctx, *mcpAddr, mux)

	local := body.LocalAPI(svc, head, reg)
	brain.Routes(local, "/agent")
	local.HandleFunc("POST /voice/event", feed.PostHandler)                               // voz: live state for the /voice page
	local.HandleFunc("POST /audio/phrase", func(w http.ResponseWriter, r *http.Request) { // voz: PCM body, ?text=
		pcm, err := io.ReadAll(io.LimitReader(r.Body, 32000*30))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		alog.Add(pcm, r.URL.Query().Get("text"), r.URL.Query().Get("reply"))
		w.WriteHeader(http.StatusNoContent)
	})
	serve(ctx, *apiAddr, body.Logged(local))

	slog.Info("bb8-body ready", "mcp", "http://"+*mcpAddr+"/mcp", "web_relay", *webURL, "api", *apiAddr, "link", *linkAddr)
	if *pad {
		go gamepad.Run(ctx, svc, func() float64 { return svc.Params().PWMManualMax })
	}
	svc.Run(ctx)
}

func serve(ctx context.Context, addr string, h http.Handler) {
	s := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("listen", "addr", addr, "err", err)
		os.Exit(1)
	}
	go func() { _ = s.Serve(ln) }()
	go func() { <-ctx.Done(); _ = s.Close() }()
}

func pair(args []string) {
	fs := flag.NewFlagSet("pair", flag.ExitOnError)
	dir := fs.String("dir", env([]string{"BB8_PKI"}, "~/.bb8/pki"), "where to write the CA and certs")
	var hosts multi
	fs.Var(&hosts, "host", "extra hostname or IP for the body cert (repeatable)")
	_ = fs.Parse(args)
	d := home(*dir)
	if err := link.Pair(d, hosts); err != nil {
		fmt.Fprintln(os.Stderr, "pair:", err)
		os.Exit(1)
	}
	fmt.Printf("Certs in %s.\nCopy to the head: scp %s/{ca.crt,bb8-head.crt,bb8-head.key} bb8-head.local:~/.bb8/pki/\n", d, d)
}

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }
