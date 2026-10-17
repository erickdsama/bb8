// Package motion is the only owner of the serial port and the gamepad/LLM arbiter.
//
// It turns move(1.0) and turn(90) into M commands with a closed loop on the
// encoders and gyro, polls the Arduino status every 50 ms and forwards the
// head ToF as D <mm>. See wiki/Protocolo.md, sections 1 and 2.
package motion

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/erickdsama/bb8/go/internal/floorplan"
	"github.com/erickdsama/bb8/go/internal/protocol"
)

// Params are the calibration values and safety limits. Hard ceilings live in
// Clamp, so a bad config can never exceed them.
type Params struct {
	WheelDiameterM float64 `json:"wheel_diameter_m"`
	TicksPerRev    float64 `json:"ticks_per_rev"`
	SlipFactor     float64 `json:"slip_factor"`
	TrackWidthM    float64 `json:"track_width_m"`
	PWMCruise      float64 `json:"pwm_cruise"`
	PWMMin         float64 `json:"pwm_min"`
	PWMTurn        float64 `json:"pwm_turn"`
	PWMTurnMin     float64 `json:"pwm_turn_min"`
	PWMManualMax   float64 `json:"pwm_manual_max"`
	MoveMaxM       float64 `json:"move_max_m"`
	TurnMaxDeg     float64 `json:"turn_max_deg"`
	HeadMaxDeg     float64 `json:"head_max_deg"`
	OrderMaxS      float64 `json:"order_max_s"`
	ManualHoldS    float64 `json:"manual_hold_s"`
}

func Defaults() Params {
	return Params{
		WheelDiameterM: 0.065, TicksPerRev: 1320, SlipFactor: 1.0, TrackWidthM: 0.17,
		PWMCruise: 140, PWMMin: 45, PWMTurn: 90, PWMTurnMin: 40, PWMManualMax: 150,
		MoveMaxM: 2.0, TurnMaxDeg: 360, HeadMaxDeg: 90, OrderMaxS: 5.0, ManualHoldS: 1.0,
	}
}

// Hard ceilings: no config value can go past these.
const (
	ceilPWM     = 160 // ~63 %
	ceilMoveM   = 3.0
	ceilOrderS  = 10.0
	PollEvery   = 50 * time.Millisecond
	ToFEvery    = 100 * time.Millisecond
	ToFFwdMaxHD = 15
)

func (p Params) Clamp() Params {
	p.PWMCruise = clamp(p.PWMCruise, 0, ceilPWM)
	p.PWMTurn = clamp(p.PWMTurn, 0, ceilPWM)
	p.PWMManualMax = clamp(p.PWMManualMax, 0, ceilPWM)
	p.PWMMin = clamp(p.PWMMin, 0, p.PWMCruise)
	p.PWMTurnMin = clamp(p.PWMTurnMin, 0, p.PWMTurn)
	p.MoveMaxM = clamp(p.MoveMaxM, 0, ceilMoveM)
	p.TurnMaxDeg = clamp(p.TurnMaxDeg, 0, 360)
	p.HeadMaxDeg = clamp(p.HeadMaxDeg, 0, 90)
	p.OrderMaxS = clamp(p.OrderMaxS, 0.5, ceilOrderS)
	p.ManualHoldS = clamp(p.ManualHoldS, 0.2, 5)
	if p.WheelDiameterM <= 0 || p.TicksPerRev <= 0 || p.SlipFactor <= 0 {
		d := Defaults()
		p.WheelDiameterM, p.TicksPerRev, p.SlipFactor = d.WheelDiameterM, d.TicksPerRev, d.SlipFactor
	}
	return p
}

func (p Params) TicksPerM() float64 {
	return p.TicksPerRev / (math.Pi * p.WheelDiameterM) * p.SlipFactor
}

// Link is the serial link (serial.Link satisfies it; tests use a fake).
type Link interface {
	Send(line string) (string, error)
	Close()
}

// Reconnecting is what serial.Resilient adds to a Link: the identification line can
// change after a reconnect and the link may be down.
type Reconnecting interface {
	Ident() string
	Connected() bool
}

type Result = map[string]any

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func round(v float64, n int) float64 {
	p := math.Pow10(n)
	return math.Round(v*p) / p
}

type hearing struct {
	text    *string
	angle   *float64
	levelDB *float64
	at      time.Time
	heading float64 // body heading when heard
	headDeg float64 // post angle when heard: the mics are on the head
}

type Service struct {
	link  Link
	ident string
	Map   *floorplan.Map

	params atomic.Pointer[Params]

	mu        sync.Mutex
	status    protocol.Status
	statusT   time.Time
	tick      chan struct{}
	x, y      float64
	traveled  float64
	prevEnc   *[2]int
	hear      *hearing
	hearNew   chan struct{}
	tofSource func() (int, bool)

	busy       chan struct{} // one order at a time
	busyNow    atomic.Bool
	cancelGen  atomic.Int64
	lastManual atomic.Int64 // unix nanos
	asleep     atomic.Bool
}

func New(link Link, ident string, m *floorplan.Map, p Params) *Service {
	s := &Service{link: link, ident: ident, Map: m, tick: make(chan struct{}), hearNew: make(chan struct{}),
		busy: make(chan struct{}, 1), status: protocol.Status{Flags: map[string]bool{}}}
	s.SetParams(p)
	return s
}

func (s *Service) Params() Params     { return *s.params.Load() }
func (s *Service) SetParams(p Params) { p = p.Clamp(); s.params.Store(&p) }

// SetToFSource sets where the forward distance comes from (the head link).
func (s *Service) SetToFSource(f func() (int, bool)) {
	s.mu.Lock()
	s.tofSource = f
	s.mu.Unlock()
}

func (s *Service) send(line string) (string, error) { return s.link.Send(line) }

// arduino returns the identification and whether the Arduino link is up.
func (s *Service) arduino() (string, bool) {
	if r, ok := s.link.(Reconnecting); ok {
		return r.Ident(), r.Connected()
	}
	return s.ident, true
}

// ArduinoUp runs after every (re)connection. A reconnect means the Uno reset: the
// encoder counters and the heading restarted from zero, and it forgot sleep and the post angle.
func (s *Service) ArduinoUp(ident string, first bool) {
	s.mu.Lock()
	s.prevEnc = nil
	s.mu.Unlock()
	s.asleep.Store(false)
	if _, err := s.send(protocol.Head(0)); err != nil {
		slog.Warn("head to 0", "err", err)
	}
	if first {
		slog.Info("arduino connected", "ident", ident)
		return
	}
	s.Map.SetUncertain("the Arduino restarted: the heading started again from 0, check where I am")
	slog.Warn("arduino reconnected", "ident", ident)
}

// Run starts the background loops until ctx ends, then stops the motors.
func (s *Service) Run(ctx context.Context) {
	s.Map.Load() // anchor (start or saved pose) before moving
	go s.tofLoop(ctx)
	s.pollLoop(ctx)
	_, _ = s.send(protocol.CmdStop)
}

func (s *Service) pollLoop(ctx context.Context) {
	t := time.NewTicker(PollEvery)
	defer t.Stop()
	failing := false
	for {
		reply, err := s.send(protocol.CmdStatus)
		if err != nil {
			if !failing { // once per outage, not every 50 ms
				slog.Warn("status poll failed", "err", err)
			}
			failing = true
		} else {
			if failing {
				slog.Info("status poll recovered")
			}
			failing = false
			st := protocol.ParseStatus(reply)
			s.mu.Lock()
			s.updateOdometry(st)
			s.status, s.statusT = st, time.Now()
			close(s.tick)
			s.tick = make(chan struct{})
			s.mu.Unlock()
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// updateOdometry runs with s.mu held.
func (s *Service) updateOdometry(st protocol.Status) {
	if s.prevEnc != nil {
		tpm := s.Params().TicksPerM()
		d := (float64(st.EncLeft-s.prevEnc[0]) + float64(st.EncRight-s.prevEnc[1])) / 2 / tpm
		yaw := st.YawDeg * math.Pi / 180
		s.x += d * math.Cos(yaw)
		s.y += d * math.Sin(yaw) // y positive to the right, like the heading
		s.traveled += math.Abs(d)
	}
	s.prevEnc = &[2]int{st.EncLeft, st.EncRight}
	if st.Has("tilt") {
		s.Map.SetUncertain("tilted too far: maybe I was picked up or pushed")
	} else if st.Has("imu") {
		s.Map.SetUncertain("the IMU does not answer: heading is not reliable")
	}
	s.Map.SavePose(floorplan.Odo{X: s.x, Y: s.y, Heading: st.YawDeg}, false)
}

func (s *Service) tofLoop(ctx context.Context) {
	t := time.NewTicker(ToFEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		s.mu.Lock()
		src, head := s.tofSource, s.status.HeadDeg
		s.mu.Unlock()
		mm := 0
		if src != nil {
			if v, ok := src(); ok {
				mm = v
			}
		}
		// Only a path distance when the head looks forward.
		if head > ToFFwdMaxHD || head < -ToFFwdMaxHD {
			mm = 0
		}
		if _, err := s.send(protocol.Distance(mm)); err != nil {
			slog.Debug("forward D failed", "err", err)
		}
	}
}

func (s *Service) Status() protocol.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// waitStatus waits for the next poll (max 500 ms).
func (s *Service) waitStatus() protocol.Status {
	s.mu.Lock()
	ch := s.tick
	s.mu.Unlock()
	select {
	case <-ch:
	case <-time.After(500 * time.Millisecond):
		slog.Warn("no new Arduino status in 500 ms")
	}
	return s.Status()
}

// --- arbiter ---------------------------------------------------------------

func (s *Service) ManualActive() bool {
	hold := time.Duration(s.Params().ManualHoldS * float64(time.Second))
	return time.Since(time.Unix(0, s.lastManual.Load())) < hold
}

func (s *Service) Control() string {
	if s.ManualActive() {
		return "manual"
	}
	if s.busyNow.Load() {
		return "llm"
	}
	return "idle"
}

func errResult(err error) Result {
	var pe *protocol.Error
	if errors.As(err, &pe) {
		return Result{"result": "error", "reason": pe.Reason}
	}
	return Result{"result": "error", "reason": err.Error()}
}

// Manual is a direct gamepad order: it always wins and cancels the LLM.
func (s *Service) Manual(left, right float64) Result {
	s.lastManual.Store(time.Now().UnixNano())
	s.cancelGen.Add(1)
	lim := s.Params().PWMManualMax
	if _, err := s.send(protocol.Motors(clamp(left, -lim, lim), clamp(right, -lim, lim))); err != nil {
		return errResult(err)
	}
	return Result{"result": "done"}
}

// --- tools -------------------------------------------------------------------

func (s *Service) Stop() Result {
	s.cancelGen.Add(1)
	if _, err := s.send(protocol.CmdStop); err != nil {
		return errResult(err)
	}
	return Result{"result": "done"}
}

func (s *Service) Move(meters float64) Result {
	lim := s.Params().MoveMaxM
	return s.run(func(gen int64) (Result, error) { return s.drive(clamp(meters, -lim, lim), gen) })
}

func (s *Service) Turn(degrees float64) Result {
	lim := s.Params().TurnMaxDeg
	return s.run(func(gen int64) (Result, error) { return s.spin(clamp(degrees, -lim, lim), gen) })
}

func (s *Service) LookAt(degrees float64) Result {
	lim := s.Params().HeadMaxDeg
	return s.run(func(gen int64) (Result, error) { return s.head(clamp(degrees, -lim, lim), gen) })
}

func (s *Service) Sleep() Result {
	s.Stop()
	if _, err := s.send(protocol.CmdSleep); err != nil {
		return errResult(err)
	}
	s.asleep.Store(true)
	return Result{"result": "done"}
}

func (s *Service) Wake() Result {
	if _, err := s.send(protocol.CmdWake); err != nil {
		return errResult(err)
	}
	s.asleep.Store(false)
	return Result{"result": "done"}
}

// PowerOff: deep sleep. The Arduino cuts the Pi in `seconds`; the caller shuts down.
func (s *Service) PowerOff(seconds float64) Result {
	s.Stop()
	sec := int(clamp(seconds, 5, 120))
	if _, err := s.send(protocol.PowerOff(sec)); err != nil {
		return errResult(err)
	}
	s.asleep.Store(true)
	return Result{"result": "done", "cut_in_s": sec}
}

func (s *Service) odo() (floorplan.Odo, float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return floorplan.Odo{X: s.x, Y: s.y, Heading: s.status.YawDeg}, s.traveled
}

func (s *Service) Pose() Result {
	s.mu.Lock()
	st, x, y, t := s.status, s.x, s.y, s.statusT
	src := s.tofSource
	s.mu.Unlock()
	var tof any
	if src != nil {
		if mm, ok := src(); ok && mm > 0 {
			tof = round(float64(mm)/1000, 3)
		}
	}
	if tof == nil && st.ToFMM > 0 {
		tof = round(float64(st.ToFMM)/1000, 3)
	}
	return Result{
		"x_m": round(x, 3), "y_m": round(y, 3), "heading_deg": round(st.YawDeg, 1),
		"tilt_deg": round(st.TiltDeg, 1), "battery_v": round(st.BatteryV, 2), "head_deg": st.HeadDeg,
		"tof_m": tof, "flags": st.FlagList(), "control": s.Control(), "asleep": s.asleep.Load(),
		"arduino": arduinoName(s), "arduino_connected": arduinoUp(s), "status_age_s": round(time.Since(t).Seconds(), 2),
	}
}

// --- floor plan --------------------------------------------------------------

func (s *Service) GetMap() Result {
	o, tr := s.odo()
	out, err := s.Map.Describe(o, tr)
	if err != nil {
		return Result{"result": "error", "reason": err.Error()}
	}
	return out
}

func (s *Service) SetPosition(x, y float64, heading *float64) Result {
	o, tr := s.odo()
	if err := s.Map.SetPosition(x, y, heading, o, tr); err != nil {
		return Result{"result": "error", "reason": err.Error()}
	}
	d, _ := s.Map.Describe(o, tr)
	return Result{"result": "done", "robot": d["robot"]}
}

func (s *Service) ChangeMap(c floorplan.Change) Result {
	if err := s.Map.Apply(c); err != nil {
		return Result{"result": "error", "reason": err.Error()}
	}
	return Result{"result": "done"}
}

// --- hearing: voice direction (ReSpeaker 2-Mic) ------------------------------

// Hear stores what the head microphone heard. angle is relative to the HEAD
// front, + right, −90…90: two mics cannot tell front from back. headDeg is the
// post angle when it was heard (nil = now).
func (s *Service) Hear(text *string, angle, levelDB, headDeg *float64) error {
	if angle != nil && (*angle < -90 || *angle > 90) {
		return errors.New("angle_deg goes from -90 to 90")
	}
	s.mu.Lock()
	hd := float64(s.status.HeadDeg)
	if headDeg != nil {
		hd = *headDeg
	}
	s.hear = &hearing{text: text, angle: angle, levelDB: levelDB, at: time.Now(), heading: s.status.YawDeg, headDeg: hd}
	close(s.hearNew)
	s.hearNew = make(chan struct{})
	s.mu.Unlock()
	return nil
}

// Listen returns the last thing heard; with wait > 0 waits up to 10 s for something new.
func (s *Service) Listen(wait float64) Result {
	if wait > 0 {
		s.mu.Lock()
		ch := s.hearNew
		s.mu.Unlock()
		select {
		case <-ch:
		case <-time.After(time.Duration(clamp(wait, 0, 10) * float64(time.Second))):
		}
	}
	s.mu.Lock()
	h, yaw := s.hear, s.status.YawDeg
	s.mu.Unlock()
	if h == nil {
		return Result{"heard": false}
	}
	turned := floorplan.Norm180(yaw - h.heading)
	out := Result{"heard": true, "text": h.text, "level_db": h.levelDB,
		"ago_s": round(time.Since(h.at).Seconds(), 1), "turned_since_deg": round(turned, 1)}
	if h.angle != nil {
		// Body frame: in front of the head (head + a) or behind it (head + 180 − a).
		a := *h.angle
		front, back := h.headDeg+a, h.headDeg+180-a
		out["angle_deg"] = round(a, 1)
		out["head_deg"] = round(h.headDeg, 1)
		out["candidates_deg"] = []float64{round(floorplan.Norm180(front-turned), 1), round(floorplan.Norm180(back-turned), 1)}
	}
	return out
}

// --- order queue -------------------------------------------------------------

func (s *Service) run(fn func(gen int64) (Result, error)) Result {
	if s.ManualActive() {
		return Result{"result": "manual_override"}
	}
	if !arduinoUp(s) {
		return Result{"result": "error", "reason": "arduino_unavailable"}
	}
	if s.asleep.Load() { // light sleep: an order wakes it instead of failing
		s.Wake()
	}
	gen := s.cancelGen.Load()
	s.busy <- struct{}{}
	s.busyNow.Store(true)
	defer func() { s.busyNow.Store(false); <-s.busy }()
	if gen != s.cancelGen.Load() {
		return Result{"result": "cancelled"}
	}
	r, err := fn(gen)
	if err != nil {
		_, _ = s.send(protocol.CmdStop)
		return errResult(err)
	}
	return r
}

func merge(a Result, b Result) Result {
	for k, v := range b {
		a[k] = v
	}
	return a
}

// interrupted checks cancel, gamepad and tilt. Returns a result if it must stop.
func (s *Service) interrupted(gen int64, out Result) Result {
	if gen != s.cancelGen.Load() {
		if s.ManualActive() {
			return merge(Result{"result": "manual_override"}, out) // the gamepad sends its own M
		}
		_, _ = s.send(protocol.CmdStop)
		return merge(Result{"result": "cancelled"}, out)
	}
	if s.Status().Has("tilt") {
		_, _ = s.send(protocol.CmdStop)
		return merge(Result{"result": "error", "reason": "tilt"}, out)
	}
	return nil
}

func (s *Service) drive(meters float64, gen int64) (Result, error) {
	p := s.Params()
	tpm := p.TicksPerM()
	st := s.waitStatus()
	e0 := float64(st.EncLeft+st.EncRight) / 2
	yaw0 := st.YawDeg
	sign := 1.0
	if meters < 0 {
		sign = -1
	}
	t0 := time.Now()
	traveled := func() float64 {
		c := s.Status()
		return (float64(c.EncLeft+c.EncRight)/2 - e0) / tpm
	}
	for {
		out := Result{"meters": round(traveled(), 3)}
		if r := s.interrupted(gen, out); r != nil {
			return r, nil
		}
		if sign > 0 && s.Status().Has("obstacle") {
			_, _ = s.send(protocol.CmdStop)
			return s.blocked(out), nil
		}
		remaining := meters - traveled()
		if remaining*sign < 0.01 {
			break
		}
		if time.Since(t0).Seconds() > p.OrderMaxS {
			_, _ = s.send(protocol.CmdStop)
			return merge(Result{"result": "timeout"}, out), nil
		}
		speed := sign * clamp(math.Abs(remaining)*400, p.PWMMin, p.PWMCruise)
		corr := 3.0 * (s.Status().YawDeg - yaw0) // hold the heading
		if _, err := s.send(protocol.Motors(speed-corr, speed+corr)); err != nil {
			var pe *protocol.Error
			if errors.As(err, &pe) && pe.Reason == "obstacle" {
				_, _ = s.send(protocol.CmdStop)
				return s.blocked(out), nil
			}
			return nil, err
		}
		s.waitStatus()
	}
	_, _ = s.send(protocol.CmdStop)
	time.Sleep(150 * time.Millisecond)
	s.waitStatus()
	return Result{"result": "done", "meters": round(traveled(), 3), "seconds": round(time.Since(t0).Seconds(), 2)}, nil
}

func (s *Service) blocked(out Result) Result {
	mm := 0
	s.mu.Lock()
	src := s.tofSource
	s.mu.Unlock()
	if src != nil {
		if v, ok := src(); ok {
			mm = v
		}
	}
	if mm == 0 {
		mm = s.Status().ToFMM
	}
	var d any
	if mm > 0 {
		d = round(float64(mm)/1000, 3)
	}
	return merge(Result{"result": "blocked", "distance_m": d, "reason": "obstacle"}, out)
}

func (s *Service) spin(degrees float64, gen int64) (Result, error) {
	p := s.Params()
	st := s.waitStatus()
	yaw0 := st.YawDeg
	target := yaw0 + degrees
	t0 := time.Now()
	for {
		yaw := s.Status().YawDeg
		out := Result{"degrees": round(yaw-yaw0, 1)}
		if r := s.interrupted(gen, out); r != nil {
			return r, nil
		}
		e := target - yaw
		if math.Abs(e) < 2.0 || e*degrees < 0 { // arrived or overshot
			break
		}
		if time.Since(t0).Seconds() > p.OrderMaxS {
			_, _ = s.send(protocol.CmdStop)
			return merge(Result{"result": "timeout"}, out), nil
		}
		v := math.Copysign(clamp(math.Abs(e)*2.0, p.PWMTurnMin, p.PWMTurn), e)
		if _, err := s.send(protocol.Motors(v, -v)); err != nil { // + right: left wheel forward
			return nil, err
		}
		s.waitStatus()
	}
	_, _ = s.send(protocol.CmdStop)
	time.Sleep(150 * time.Millisecond)
	s.waitStatus()
	return Result{"result": "done", "degrees": round(s.Status().YawDeg-yaw0, 1), "seconds": round(time.Since(t0).Seconds(), 2)}, nil
}

func (s *Service) head(degrees float64, gen int64) (Result, error) {
	if _, err := s.send(protocol.Head(degrees)); err != nil {
		return nil, err
	}
	t0 := time.Now()
	for math.Abs(float64(s.Status().HeadDeg)-math.Round(degrees)) > 1 && time.Since(t0) < 1500*time.Millisecond {
		if gen != s.cancelGen.Load() {
			return Result{"result": "cancelled", "degrees": s.Status().HeadDeg}, nil
		}
		s.waitStatus()
	}
	return Result{"result": "done", "degrees": s.Status().HeadDeg}, nil
}

func arduinoName(s *Service) string {
	id, up := s.arduino()
	if !up {
		return "disconnected"
	}
	return id
}

func arduinoUp(s *Service) bool { _, up := s.arduino(); return up }
