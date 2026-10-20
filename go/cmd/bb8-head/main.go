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
	mic := flag.String("mic", env("BB8_HEAD_MIC", "plughw:CARD=wm8960soundcard"), `ReSpeaker ALSA device ("" = no microphone)`)
	spacing := flag.Float64("mic-spacing", 0.058, "distance between the two capsules, m (measure it)")
	swap := flag.Bool("mic-swap", os.Getenv("BB8_MIC_SWAP") == "1", "the card reports the right capsule first")
	eye := flag.String("eye", env("BB8_EYE", ""), `eye: ws2812-pwm (WS2812 on GPIO12 via bb8-eye), ws2812 (WS2812 on SPI0, no HAT), apa102 (the HAT's LEDs) or none; default ws2812-pwm with a mic, else ws2812`)
	eyeSock := flag.String("eye-socket", env("BB8_EYE_SOCKET", "/run/bb8-eye.sock"), "bb8-eye helper socket (ws2812-pwm)")
	tts := flag.String("tts", env("BB8_TTS_WYOMING", ""), "Wyoming text-to-speech server host:port (Piper); empty = espeak-ng")
	leds := flag.Int("eye-leds", 16, "LEDs in the eye (16 on the ring; 3 for --eye apa102)")
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var b head.Backend
	if *sim != "" {
		b = head.NewSim(*sim)
	} else {
		e := *eye
		if e == "" {
			e = "ws2812"
			if *mic != "" {
				e = "ws2812-pwm"
			}
		}
		b = head.NewReal(ctx, head.RealConfig{Eye: e, EyeLEDs: *leds, EyeSocket: *eyeSock, MicDevice: *mic, MicSpacing: *spacing, MicSwap: *swap, TTS: *tts})
	}
	slog.Info("bb8-head", "body", *body, "sim", *sim)
	(&head.Head{B: b}).Run(ctx, *body, cfg)
}
