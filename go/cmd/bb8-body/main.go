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
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/erickdsama/bb8/go/internal/agent"
	"github.com/erickdsama/bb8/go/internal/body"
	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connectors"
	"github.com/erickdsama/bb8/go/internal/connectors/calendar"
	"github.com/erickdsama/bb8/go/internal/connectors/weather"
	"github.com/erickdsama/bb8/go/internal/floorplan"
	"github.com/erickdsama/bb8/go/internal/gamepad"
	"github.com/erickdsama/bb8/go/internal/link"
	"github.com/erickdsama/bb8/go/internal/mcpserver"
	"github.com/erickdsama/bb8/go/internal/motion"
	"github.com/erickdsama/bb8/go/internal/persona"
	"github.com/erickdsama/bb8/go/internal/serial"
	"github.com/erickdsama/bb8/go/internal/tools"
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

	ser, err := serial.Open(*serialURL, 115200)
	if err != nil {
		slog.Error("arduino", "err", err)
		os.Exit(1)
	}
	defer ser.Close()

	cfg, err := config.Open(home(*dataDir))
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	cfg.Register(body.RobotFields()...)
	cfg.Register(agent.Fields()...)

	svc := motion.New(ser, ser.Ident, floorplan.New(home(*mapPath), home(*mapState)), motion.Defaults())

	// Secure head link.
	srv := &link.Server{}
	head := body.NewHead(srv)
	svc.SetToFSource(head.ToF)
	head.OnSound = func(text *string, angle, level *float64) {
		if err := svc.Hear(text, angle, level); err != nil {
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

	reg := tools.NewRegistry()
	conns := connectors.NewManager(cfg, reg, weather.New(), calendar.New())
	reg.Set("robot", body.RobotTools(svc, head))
	reg.Set("settings", body.SettingTools(cfg))
	conns.Start()
	brain := agent.New(cfg, reg, persona.Instructions)

	token := func() string {
		b, _ := os.ReadFile(home(*tokenFile))
		return strings.TrimSpace(string(b))
	}
	mcpSrv := mcpserver.New(reg, persona.Instructions())
	live := func() map[string]any {
		return map[string]any{"head": head.State(), "connectors": conns.Status(), "agent_ready": brain.Ready() == nil}
	}
	web := http.NewServeMux()
	web.Handle("/", config.Web(cfg, live))
	brain.Routes(web, "/api/agent")
	calendar.AuthHandlers(cfg, web)
	web.HandleFunc("GET /api/connectors", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(conns.Status())
	})
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpserver.Handler(mcpSrv, token))
	mux.Handle("/", mcpserver.RequireToken(web, token))
	serve(ctx, *mcpAddr, mux)

	local := body.LocalAPI(svc, head, reg)
	brain.Routes(local, "/agent")
	serve(ctx, *apiAddr, body.Logged(local))

	slog.Info("bb8-body ready", "mcp", "http://"+*mcpAddr+"/mcp", "web", "http://"+*mcpAddr+"/", "api", *apiAddr, "link", *linkAddr)
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
