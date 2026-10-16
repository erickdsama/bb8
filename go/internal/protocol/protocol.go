// Package protocol builds and parses the Pi ↔ Arduino serial lines
// (wiki/Protocolo.md, section 1). It never opens a port.
package protocol

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	PWMLimit   = 255
	HeadLimit  = 90
	ToFLimitMM = 4000
	MaxLine    = 63
)

const (
	CmdStop   = "S"
	CmdStatus = "?"
	CmdSleep  = "Z"
	CmdWake   = "W"
	CmdIdent  = "I"
)

// Error is an "ERR <reason>" reply from the Arduino.
type Error struct {
	Reason string
	Line   string
}

func (e *Error) Error() string { return e.Reason }

func clampInt(v float64, lim int) int {
	r := int(math.Round(v))
	if r > lim {
		return lim
	}
	if r < -lim {
		return -lim
	}
	return r
}

func Motors(left, right float64) string {
	return fmt.Sprintf("M %d %d", clampInt(left, PWMLimit), clampInt(right, PWMLimit))
}

func Head(deg float64) string { return fmt.Sprintf("H %d", clampInt(deg, HeadLimit)) }

func Distance(mm int) string {
	return fmt.Sprintf("D %d", max(0, min(ToFLimitMM, mm)))
}

// PowerOff asks the Arduino to cut the Pi in s seconds and sleep. 0 cancels.
func PowerOff(s int) string { return fmt.Sprintf("P %d", s) }

// ParseReply returns the text after OK, or an *Error for ERR or garbage.
func ParseReply(line string) (string, error) {
	line = strings.TrimSpace(line)
	if line == "OK" {
		return "", nil
	}
	if strings.HasPrefix(line, "OK ") {
		return line[3:], nil
	}
	if strings.HasPrefix(line, "ERR") {
		r := strings.TrimSpace(strings.TrimPrefix(line, "ERR"))
		if r == "" {
			r = "unknown"
		}
		return "", &Error{Reason: r, Line: line}
	}
	return "", &Error{Reason: "garbage", Line: line}
}

// Status is the "?" reply.
type Status struct {
	BatteryV float64
	TiltDeg  float64
	YawDeg   float64
	EncLeft  int
	EncRight int
	HeadDeg  int
	ToFMM    int
	Flags    map[string]bool
	Raw      string
}

func (s Status) Has(flag string) bool { return s.Flags[flag] }

func (s Status) FlagList() []string {
	out := make([]string, 0, len(s.Flags))
	for f := range s.Flags {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// ParseStatus reads "V=11.62 T=3.1 Y=-12.4 E=1420,1398 H=0 D=1830 F=-".
// Unknown keys and bad values are ignored.
func ParseStatus(payload string) Status {
	st := Status{Raw: payload, Flags: map[string]bool{}}
	for _, tok := range strings.Fields(payload) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			continue
		}
		f := func() (float64, bool) {
			x, err := strconv.ParseFloat(v, 64)
			return x, err == nil
		}
		switch k {
		case "V":
			if x, ok := f(); ok {
				st.BatteryV = x
			}
		case "T":
			if x, ok := f(); ok {
				st.TiltDeg = x
			}
		case "Y":
			if x, ok := f(); ok {
				st.YawDeg = x
			}
		case "E":
			l, r, ok := strings.Cut(v, ",")
			li, e1 := strconv.Atoi(l)
			ri, e2 := strconv.Atoi(r)
			if ok && e1 == nil && e2 == nil {
				st.EncLeft, st.EncRight = li, ri
			}
		case "H":
			if x, ok := f(); ok {
				st.HeadDeg = int(x)
			}
		case "D":
			if x, ok := f(); ok {
				st.ToFMM = int(x)
			}
		case "F":
			if v != "-" {
				for _, fl := range strings.Split(v, ",") {
					st.Flags[fl] = true
				}
			}
		}
	}
	return st
}
