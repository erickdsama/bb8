// bb8-head runs on the Pi Zero W. It opens no ports: it dials the body over
// mutual TLS and keeps that one connection.
//
//	bb8-head --body bb8.local:8772
//	bb8-head --body 127.0.0.1:8772 --sim http://127.0.0.1:8080   # PC, against the dummy
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/erickdsama/bb8/go/internal/head"
	"github.com/erickdsama/bb8/go/internal/link"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	body := flag.String("body", env("BB8_BODY_ADDR", "bb8.local:8772"), "body link address")
	pki := flag.String("pki", env("BB8_PKI", "~/.bb8/pki"), "dir with ca.crt, bb8-head.crt, bb8-head.key")
	sim := flag.String("sim", "", "forward to the simulated head at this URL instead of hardware")
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	dir := *pki
	if h, err := os.UserHomeDir(); err == nil && strings.HasPrefix(dir, "~/") {
		dir = filepath.Join(h, dir[2:])
	}
	cfg, err := link.ClientTLS(dir)
	if err != nil {
		slog.Error("certs missing: copy them from 'bb8-body pair'", "dir", dir, "err", err)
		os.Exit(1)
	}
	var b head.Backend
	if *sim != "" {
		b = head.NewSim(*sim)
	} else {
		b = head.NewReal()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	slog.Info("bb8-head", "body", *body, "sim", *sim)
	(&head.Head{B: b}).Run(ctx, *body, cfg)
}
