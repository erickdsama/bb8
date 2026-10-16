//go:build linux

// Package gamepad reads a Bluetooth gamepad straight from /dev/input (evdev,
// no library). It has priority over the LLM.
//
// Left stick: Y forward/back, X turn. Arcade mix left = y + x, right = y − x.
// South button (X/A) = stop. Resends M every 100 ms while the stick is outside
// the dead zone, to feed the Arduino watchdog.
package gamepad

import (
	"context"
	"encoding/binary"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	evKey     = 0x01
	evAbs     = 0x03
	absX      = 0x00
	absY      = 0x01
	btnSouth  = 0x130
	keepalive = 100 * time.Millisecond
	deadzone  = 10 // PWM units
)

// Driver is what the gamepad drives (motion.Service).
type Driver interface {
	Manual(left, right float64) map[string]any
	Stop() map[string]any
}

func ioc(nr, size uintptr) uintptr { return 2<<30 | size<<16 | 'E'<<8 | nr }

type absInfo struct{ Value, Min, Max, Fuzz, Flat, Res int32 }

func hasXY(fd int) bool {
	bits := make([]byte, 8)
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), ioc(0x20+evAbs, uintptr(len(bits))), uintptr(unsafe.Pointer(&bits[0]))); e != 0 {
		return false
	}
	return bits[0]&0b11 == 0b11 // ABS_X and ABS_Y
}

func absinfo(fd int, code uintptr) (absInfo, error) {
	var a absInfo
	if _, _, e := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), ioc(0x40+code, unsafe.Sizeof(a)), uintptr(unsafe.Pointer(&a))); e != 0 {
		return a, e
	}
	return a, nil
}

func find() (*os.File, string) {
	paths, _ := filepath.Glob("/dev/input/event*")
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		if hasXY(int(f.Fd())) {
			return f, p
		}
		f.Close()
	}
	return nil, ""
}

// Mix turns x, y in −1..1 (y + forward) into left/right PWM with dead zone and limit.
func Mix(x, y, lim float64) (float64, float64) {
	f := func(v float64) float64 {
		v *= lim
		if math.Abs(v) < deadzone {
			return 0
		}
		return math.Max(-lim, math.Min(lim, v))
	}
	return f(y + x), f(y - x)
}

type event struct {
	typ, code uint16
	value     int32
}

// Run looks for a gamepad every 5 s and drives d until ctx ends.
func Run(ctx context.Context, d Driver, limit func() float64) {
	for ctx.Err() == nil {
		f, path := find()
		if f == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		slog.Info("gamepad", "device", path)
		serve(ctx, f, d, limit)
		f.Close()
	}
}

func serve(ctx context.Context, f *os.File, d Driver, limit func() float64) {
	fd := int(f.Fd())
	ax, _ := absinfo(fd, absX)
	ay, _ := absinfo(fd, absY)
	norm := func(a absInfo, v int32) float64 {
		mid, half := float64(a.Max+a.Min)/2, float64(a.Max-a.Min)/2
		if half == 0 {
			return 0
		}
		return (float64(v) - mid) / half
	}
	evs := make(chan event, 64)
	go func() { // input_event: timeval, u16 type, u16 code, s32 value
		defer close(evs)
		tv := int(unsafe.Sizeof(unix.Timeval{}))
		buf := make([]byte, tv+8)
		for {
			if _, err := f.Read(buf); err != nil {
				return
			}
			evs <- event{binary.LittleEndian.Uint16(buf[tv:]), binary.LittleEndian.Uint16(buf[tv+2:]),
				int32(binary.LittleEndian.Uint32(buf[tv+4:]))}
		}
	}()
	go func() { <-ctx.Done(); f.Close() }()
	x, y := 0.0, 0.0
	var last [2]float64
	lastT := time.Time{}
	tick := time.NewTicker(keepalive)
	defer tick.Stop()
	for {
		select {
		case e, ok := <-evs:
			if !ok {
				slog.Warn("gamepad disconnected")
				d.Stop()
				return
			}
			switch {
			case e.typ == evAbs && e.code == absX:
				x = norm(ax, e.value)
			case e.typ == evAbs && e.code == absY:
				y = norm(ay, e.value)
			case e.typ == evKey && e.code == btnSouth && e.value == 1:
				d.Stop()
				continue
			default:
				continue
			}
		case <-tick.C:
		}
		l, r := Mix(x, -y, limit())
		cmd := [2]float64{l, r}
		moving := cmd != [2]float64{}
		if cmd != last || (moving && time.Since(lastT) >= keepalive) {
			d.Manual(l, r)
			last, lastT = cmd, time.Now()
		}
	}
}
