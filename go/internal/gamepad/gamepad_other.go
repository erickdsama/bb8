//go:build !linux

package gamepad

import (
	"context"
	"log/slog"
)

type Driver interface {
	Manual(left, right float64) map[string]any
	Stop() map[string]any
}

// Run is a no-op outside Linux (evdev only exists there).
func Run(ctx context.Context, d Driver, limit func() float64) {
	slog.Warn("gamepad: evdev only on Linux")
}
