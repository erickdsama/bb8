// Package floorplan holds the house plan and where the robot is on it.
//
// The plan is a JSON file (format in wiki/Protocolo.md § 2) with rooms, doors,
// walls, furniture and "start": where the robot boots. Plan axes: x east,
// y north, heading in degrees clockwise from east (0 = east, 90 = south), the
// same sign as turn.
//
// Odometry (x forward and y right of the boot pose, clockwise heading) maps to
// the plan through an anchor: the plan pose of the boot point. SetPosition moves
// the anchor to correct drift. The last pose is saved to disk so a restart does
// not lose it; booting from it marks the position "uncertain" until confirmed.
package floorplan

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	CellM      = 0.25 // text sketch resolution
	RobotR     = 0.15 // reject positions inside a wall
	SaveEveryS = 2.0
)

type Range [2]float64

type Box struct {
	X Range `json:"x"`
	Y Range `json:"y"`
}

type Room struct {
	Name string `json:"name"`
	Box
}

type Door struct {
	Between [2]string `json:"between"`
	X       float64   `json:"x"`
	Y       float64   `json:"y"`
	WidthM  float64   `json:"width_m,omitempty"`
	Closed  bool      `json:"closed,omitempty"`
}

type Furniture struct {
	Name string `json:"name"`
	Box
	HeightM float64 `json:"height_m,omitempty"`
}

type Start struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	HeadingDeg float64 `json:"heading_deg"`
}

type Plan struct {
	Name      string      `json:"name"`
	SizeM     [2]float64  `json:"size_m"`
	Start     *Start      `json:"start,omitempty"`
	Rooms     []Room      `json:"rooms"`
	Doors     []Door      `json:"doors,omitempty"`
	Walls     []Box       `json:"walls,omitempty"`
	Furniture []Furniture `json:"furniture,omitempty"`
}

// Odo is the odometry pose: x forward, y right of boot, heading clockwise (deg).
type Odo struct{ X, Y, Heading float64 }

func Norm180(deg float64) float64 {
	d := math.Mod(deg+180, 360)
	if d < 0 {
		d += 360
	}
	d -= 180
	if d == -180 {
		return 180
	}
	return d
}

func (b Box) inside(x, y, margin float64) bool {
	return b.X[0]-margin <= x && x <= b.X[1]+margin && b.Y[0]-margin <= y && y <= b.Y[1]+margin
}

func (b Box) center() (float64, float64) { return (b.X[0] + b.X[1]) / 2, (b.Y[0] + b.Y[1]) / 2 }

// crosses reports whether segment (x0,y0)→(x1,y1) crosses the box (slab test).
func (b Box) crosses(x0, y0, x1, y1 float64) bool {
	dx, dy := x1-x0, y1-y0
	t0, t1 := 0.0, 1.0
	for _, s := range [2][4]float64{{x0, dx, b.X[0], b.X[1]}, {y0, dy, b.Y[0], b.Y[1]}} {
		p, d, lo, hi := s[0], s[1], s[2], s[3]
		if math.Abs(d) < 1e-9 {
			if p < lo || p > hi {
				return false
			}
			continue
		}
		a, c := (lo-p)/d, (hi-p)/d
		if a > c {
			a, c = c, a
		}
		t0, t1 = math.Max(t0, a), math.Min(t1, c)
		if t0 > t1 {
			return false
		}
	}
	return true
}

func (d Door) width() float64 {
	if d.WidthM == 0 {
		return 0.9
	}
	return d.WidthM
}

func (d Door) name() string { return d.Between[0] + "↔" + d.Between[1] }

func round(v float64, n int) float64 {
	p := math.Pow10(n)
	return math.Round(v*p) / p
}

type Map struct {
	path, statePath string

	mu        sync.Mutex
	plan      *Plan
	err       string
	mtime     time.Time
	anchor    *[3]float64
	uncertain string
	odoAnchor float64
	savedAt   time.Time
}

func New(path, statePath string) *Map { return &Map{path: path, statePath: statePath} }

// SetUncertain marks the pose as not trustworthy (tilt, imu failure).
func (m *Map) SetUncertain(reason string) {
	m.mu.Lock()
	m.uncertain = reason
	m.mu.Unlock()
}

// Load (re)reads the plan if it changed on disk. A broken JSON keeps the old one.
func (m *Map) Load() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.load()
}

func (m *Map) load() bool {
	if m.path == "" {
		return m.plan != nil
	}
	fi, err := os.Stat(m.path)
	if err != nil {
		return m.plan != nil
	}
	if fi.ModTime().Equal(m.mtime) {
		return m.plan != nil
	}
	m.mtime = fi.ModTime()
	var p Plan
	data, err := os.ReadFile(m.path)
	if err == nil {
		err = json.Unmarshal(data, &p)
	}
	if err == nil && (p.SizeM[0] == 0 || len(p.Rooms) == 0) {
		err = errors.New("missing 'size_m' or 'rooms'")
	}
	if err != nil {
		m.err = fmt.Sprintf("%s: %v", m.path, err)
		slog.Error("invalid plan, keeping the previous one", "err", m.err)
		return m.plan != nil
	}
	m.plan, m.err = &p, ""
	slog.Info("plan loaded", "name", p.Name, "path", m.path)
	if m.anchor == nil {
		m.anchorAtBoot()
	}
	return true
}

func (m *Map) anchorAtBoot() {
	s := Start{}
	if m.plan.Start != nil {
		s = *m.plan.Start
	}
	m.anchor = &[3]float64{s.X, s.Y, s.HeadingDeg}
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		return
	}
	var e struct {
		Plan       string   `json:"plan"`
		X          *float64 `json:"x"`
		Y          *float64 `json:"y"`
		HeadingDeg *float64 `json:"heading_deg"`
	}
	if json.Unmarshal(data, &e) == nil && e.Plan == m.plan.Name && e.X != nil && e.Y != nil && e.HeadingDeg != nil {
		m.anchor = &[3]float64{*e.X, *e.Y, *e.HeadingDeg}
		m.uncertain = "booted from the last saved position; I may have been moved while off"
		slog.Info("saved pose", "x", *e.X, "y", *e.Y, "heading", *e.HeadingDeg)
	}
}

func writeAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (m *Map) writePlan() error {
	if _, err := os.Stat(m.path); err == nil {
		_ = os.Rename(m.path, m.path+".bak")
	}
	if err := writeAtomic(m.path, m.plan); err != nil {
		return err
	}
	if fi, err := os.Stat(m.path); err == nil {
		m.mtime = fi.ModTime()
	}
	return nil
}

// SavePose stores the pose every few seconds to survive a restart.
func (m *Map) SavePose(o Odo, force bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.savePose(o, force)
}

func (m *Map) savePose(o Odo, force bool) {
	if m.statePath == "" || m.plan == nil || m.anchor == nil {
		return
	}
	if !force && time.Since(m.savedAt).Seconds() < SaveEveryS {
		return
	}
	m.savedAt = time.Now()
	x, y, h := m.toPlan(o)
	err := writeAtomic(m.statePath, map[string]any{"plan": m.plan.Name, "x": round(x, 3), "y": round(y, 3), "heading_deg": round(h, 1)})
	if err != nil {
		slog.Warn("could not save pose", "err", err)
	}
}

func (m *Map) toPlan(o Odo) (float64, float64, float64) {
	ax, ay, ah := m.anchor[0], m.anchor[1], m.anchor[2]
	r := ah * math.Pi / 180
	// forward of the boot pose = (cos r, −sin r); right = (−sin r, −cos r)
	return ax + o.X*math.Cos(r) - o.Y*math.Sin(r), ay - o.X*math.Sin(r) - o.Y*math.Cos(r), Norm180(ah + o.Heading)
}

func (m *Map) noPlan() error {
	if m.err != "" {
		return errors.New(m.err)
	}
	return errors.New("no floor plan: set BB8_MAP to the JSON path")
}

// SetPosition moves the anchor so the current odometry lands on (x, y, heading).
func (m *Map) SetPosition(x, y float64, heading *float64, o Odo, traveled float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.load() {
		return m.noPlan()
	}
	w, h := m.plan.SizeM[0], m.plan.SizeM[1]
	if x < 0 || x > w || y < 0 || y > h {
		return fmt.Errorf("(%g, %g) is outside the house (0…%g, 0…%g)", x, y, w, h)
	}
	for _, wall := range m.plan.Walls {
		if wall.inside(x, y, RobotR-0.05) {
			return fmt.Errorf("(%g, %g) is inside a wall (%v, %v)", x, y, wall.X, wall.Y)
		}
	}
	hd := 0.0
	if heading == nil {
		_, _, hd = m.toPlan(o)
	} else {
		hd = *heading
	}
	ar := hd - o.Heading
	r := ar * math.Pi / 180
	m.anchor = &[3]float64{x - o.X*math.Cos(r) + o.Y*math.Sin(r), y + o.X*math.Sin(r) + o.Y*math.Cos(r), Norm180(ar)}
	m.uncertain, m.odoAnchor = "", traveled
	m.savePose(o, true)
	return nil
}

// Change is an update_map request.
type Change struct {
	Action  string   `json:"action"`
	Name    string   `json:"name"`
	X       *float64 `json:"x,omitempty"`
	Y       *float64 `json:"y,omitempty"`
	WidthM  *float64 `json:"width_m,omitempty"`
	LengthM *float64 `json:"length_m,omitempty"`
	HeightM *float64 `json:"height_m,omitempty"`
}

var Actions = []string{"add_furniture", "move_furniture", "remove_furniture", "close_door", "open_door"}

func or(p *float64, d float64) float64 {
	if p == nil || *p == 0 {
		return d
	}
	return *p
}

func (m *Map) Apply(c Change) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.load() {
		return m.noPlan()
	}
	p := m.plan
	w, h := p.SizeM[0], p.SizeM[1]
	if c.X != nil && c.Y != nil && (*c.X < 0 || *c.X > w || *c.Y < 0 || *c.Y > h) {
		return fmt.Errorf("(%g, %g) is outside the house (0…%g, 0…%g)", *c.X, *c.Y, w, h)
	}
	name := strings.TrimSpace(c.Name)
	if name == "" {
		return errors.New("missing 'name'")
	}
	idx := -1
	for i, f := range p.Furniture {
		if f.Name == name {
			idx = i
		}
	}
	names := func() string {
		var n []string
		for _, f := range p.Furniture {
			n = append(n, f.Name)
		}
		return strings.Join(n, ", ")
	}
	switch c.Action {
	case "add_furniture":
		if idx >= 0 {
			return fmt.Errorf("there is already furniture %q; use move_furniture", name)
		}
		if c.X == nil || c.Y == nil {
			return errors.New("add_furniture needs x, y (center)")
		}
		wd, ln := or(c.WidthM, 0.4), or(c.LengthM, 0.4)
		p.Furniture = append(p.Furniture, Furniture{Name: name,
			Box:     Box{X: Range{round(*c.X-wd/2, 2), round(*c.X+wd/2, 2)}, Y: Range{round(*c.Y-ln/2, 2), round(*c.Y+ln/2, 2)}},
			HeightM: or(c.HeightM, 0.5)})
	case "move_furniture":
		if idx < 0 {
			return fmt.Errorf("no furniture %q; there is: %s", name, names())
		}
		if c.X == nil || c.Y == nil {
			return errors.New("move_furniture needs x, y (new center)")
		}
		f := &p.Furniture[idx]
		cx, cy := f.center()
		for i := range 2 {
			f.X[i] = round(f.X[i]+*c.X-cx, 2)
			f.Y[i] = round(f.Y[i]+*c.Y-cy, 2)
		}
	case "remove_furniture":
		if idx < 0 {
			return fmt.Errorf("no furniture %q; there is: %s", name, names())
		}
		p.Furniture = append(p.Furniture[:idx], p.Furniture[idx+1:]...)
	case "close_door", "open_door":
		sides := map[string]bool{}
		for _, s := range strings.Split(strings.ReplaceAll(name, "↔", "-"), "-") {
			sides[strings.TrimSpace(s)] = true
		}
		var door *Door
		var all []string
		for i := range p.Doors {
			d := &p.Doors[i]
			all = append(all, d.name())
			if len(sides) == 2 && sides[d.Between[0]] && sides[d.Between[1]] {
				door = d
			}
		}
		if door == nil {
			return fmt.Errorf("no door %q; there is: %s", name, strings.Join(all, ", "))
		}
		door.Closed = c.Action == "close_door"
	default:
		return fmt.Errorf("action is one of %s", strings.Join(Actions, ", "))
	}
	if err := m.writePlan(); err != nil {
		return err
	}
	slog.Info("plan changed", "action", c.Action, "name", name)
	return nil
}

func (m *Map) blockers() []Box {
	out := append([]Box(nil), m.plan.Walls...)
	for _, d := range m.plan.Doors {
		if d.Closed {
			r := d.width() / 2
			out = append(out, Box{X: Range{d.X - r, d.X + r}, Y: Range{d.Y - r, d.Y + r}})
		}
	}
	return out
}

func (m *Map) room(x, y float64) (string, string) {
	for _, r := range m.plan.Rooms {
		if r.inside(x, y, 0) {
			return r.Name, ""
		}
	}
	for _, d := range m.plan.Doors {
		if math.Hypot(x-d.X, y-d.Y) <= d.width()/2+0.1 {
			return "", d.name()
		}
	}
	return "", ""
}

// route returns the open doors to cross from one room to another (BFS); nil if unreachable.
func (m *Map) route(from, to string) []string {
	if from == "" {
		return nil
	}
	type prev struct {
		room string
		door *Door
	}
	seen := map[string]*prev{from: nil}
	queue := []string{from}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == to {
			r := []string{}
			for seen[c] != nil {
				r = append([]string{seen[c].door.name()}, r...)
				c = seen[c].room
			}
			return r
		}
		for i := range m.plan.Doors {
			d := &m.plan.Doors[i]
			if d.Closed || (d.Between[0] != c && d.Between[1] != c) {
				continue
			}
			other := d.Between[0]
			if other == c {
				other = d.Between[1]
			}
			if _, ok := seen[other]; !ok {
				seen[other] = &prev{c, d}
				queue = append(queue, other)
			}
		}
	}
	return nil
}

// Where is the pose on the plan (meters, degrees clockwise from east) and the room
// or door there. ok is false without a plan.
func (m *Map) Where(o Odo) (x, y, heading float64, place string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.load() || m.anchor == nil {
		return 0, 0, 0, "", false
	}
	x, y, heading = m.toPlan(o)
	room, door := m.room(x, y)
	if room == "" {
		room = door
	}
	return round(x, 2), round(y, 2), round(heading, 1), room, true
}

// Waypoint is the next point to drive to on the way to a room or a piece of furniture.
type Waypoint struct {
	Target    string  `json:"target"`
	Kind      string  `json:"kind"` // room, furniture, door (a door on the way)
	Name      string  `json:"name"`
	TurnDeg   float64 `json:"turn_deg"` // + right, from the current heading
	DistanceM float64 `json:"distance_m"`
	Direct    bool    `json:"direct"`   // no wall or furniture in the straight line
	ArriveM   float64 `json:"arrive_m"` // close enough at this distance
	Final     bool    `json:"final"`    // the target itself, not a door on the way
	Room      string  `json:"room"`     // where the robot is now ("" = in a doorway)
	Uncertain bool    `json:"uncertain"`
}

// ErrNoTarget: the name is not a room or furniture of the plan.
var ErrNoTarget = errors.New("not a room or furniture of the plan")

// Waypoint plans one hop toward target: the target itself if it is in this room,
// or else the first open door of the route.
func (m *Map) Waypoint(o Odo, target string) (Waypoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.load() {
		return Waypoint{}, m.noPlan()
	}
	x, y, hd := m.toPlan(o)
	room, _ := m.room(x, y)
	w := Waypoint{Target: target, Room: room, Uncertain: m.uncertain != ""}
	var tx, ty float64
	var skip *Box
	found := false
	for i, f := range m.plan.Furniture {
		if strings.EqualFold(f.Name, target) {
			tx, ty = f.center()
			skip = &m.plan.Furniture[i].Box
			w.Kind, w.Name, w.Final = "furniture", f.Name, true
			// stop in front of it: half its size plus a little margin
			w.ArriveM = math.Max(f.X[1]-f.X[0], f.Y[1]-f.Y[0])/2 + 0.3
			found = true
			for _, r := range m.plan.Rooms {
				if r.inside(tx, ty, 0) {
					target = r.Name // route by its room
				}
			}
			break
		}
	}
	isRoom := false
	for _, r := range m.plan.Rooms {
		if strings.EqualFold(r.Name, target) {
			target, isRoom = r.Name, true
			if !found {
				tx, ty = r.center()
				w.Kind, w.Name, w.Final, w.ArriveM = "room", r.Name, true, 0.35
				found = true
			}
		}
	}
	if !found {
		var names []string
		for _, r := range m.plan.Rooms {
			names = append(names, r.Name)
		}
		for _, f := range m.plan.Furniture {
			names = append(names, f.Name)
		}
		return Waypoint{}, fmt.Errorf("%w: %q (use one of: %s)", ErrNoTarget, target, strings.Join(names, ", "))
	}
	if isRoom && room != "" && room != target {
		rt := m.route(room, target)
		if rt == nil {
			return Waypoint{}, errors.New("no path through open doors from " + room + " to " + target)
		}
		for _, d := range m.plan.Doors {
			if d.name() == rt[0] {
				tx, ty, skip = d.X, d.Y, nil
				w.Kind, w.Name, w.Final, w.ArriveM = "door", d.name(), false, 0.2
			}
		}
	}
	w.DistanceM = round(math.Hypot(tx-x, ty-y), 2)
	w.TurnDeg = math.Round(Norm180(-math.Atan2(ty-y, tx-x)*180/math.Pi - hd))
	w.Direct = true
	for _, b := range m.blockers() {
		if skip != nil && b == *skip {
			continue
		}
		if b.crosses(x, y, tx, ty) {
			w.Direct = false
			break
		}
	}
	return w, nil
}

// Describe is what get_map returns.
func (m *Map) Describe(o Odo, traveled float64) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.load() {
		return nil, m.noPlan()
	}
	p := m.plan
	x, y, hd := m.toPlan(o)
	bl := m.blockers()
	room, atDoor := m.room(x, y)
	toward := func(out map[string]any, tx, ty float64) map[string]any {
		target := -math.Atan2(ty-y, tx-x) * 180 / math.Pi // clockwise from east
		direct := true
		for _, b := range bl {
			if b.crosses(x, y, tx, ty) {
				direct = false
				break
			}
		}
		out["distance_m"] = round(math.Hypot(tx-x, ty-y), 2)
		out["turn_deg"] = math.Round(Norm180(target - hd))
		out["direct"] = direct
		return out
	}
	rooms := []any{}
	for _, r := range p.Rooms {
		cx, cy := r.center()
		ri := toward(map[string]any{"name": r.Name, "x": r.X, "y": r.Y}, cx, cy)
		if r.Name != room {
			if rt := m.route(room, r.Name); rt != nil {
				ri["route"] = rt
			} else {
				ri["route"] = "no path through open doors"
			}
		}
		rooms = append(rooms, ri)
	}
	doors := []any{}
	for _, d := range p.Doors {
		di := map[string]any{"between": d.Between, "x": d.X, "y": d.Y, "width_m": d.width()}
		if d.Closed {
			di["closed"] = true
		}
		doors = append(doors, toward(di, d.X, d.Y))
	}
	furn := []any{}
	for _, f := range p.Furniture {
		cx, cy := f.center()
		furn = append(furn, toward(map[string]any{"name": f.Name, "x": f.X, "y": f.Y}, cx, cy))
	}
	robot := map[string]any{"x": round(x, 2), "y": round(y, 2), "heading_deg": round(hd, 1), "room": nilIfEmpty(room)}
	if atDoor != "" {
		robot["at_door"] = atDoor
	}
	pos := map[string]any{"uncertain": m.uncertain != "", "traveled_since_fix_m": round(traveled-m.odoAnchor, 1)}
	if m.uncertain != "" {
		pos["reason"] = m.uncertain
	}
	out := map[string]any{"plan": p.Name, "size_m": p.SizeM, "robot": robot, "position": pos,
		"rooms": rooms, "doors": doors, "furniture": furn, "sketch": m.sketch(x, y, hd)}
	if m.err != "" {
		out["warning"] = "the plan on disk has an error, using the previous version: " + m.err
	}
	return out, nil
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// sketch: north up, one char per 25 cm: # wall, m furniture, . floor, = closed door, arrow robot.
func (m *Map) sketch(x, y, hd float64) []string {
	w, h := m.plan.SizeM[0], m.plan.SizeM[1]
	cols, rows := int(math.Ceil(w/CellM)), int(math.Ceil(h/CellM))
	g := make([][]rune, rows)
	for i := range g {
		g[i] = []rune(strings.Repeat(".", cols))
	}
	cell := func(cx, cy float64) (int, int) { return int(cx / CellM), int((h - cy) / CellM) }
	paint := func(b Box, ch rune, margin float64) {
		for f := range rows {
			for c := range cols {
				if b.inside((float64(c)+0.5)*CellM, h-(float64(f)+0.5)*CellM, margin) {
					g[f][c] = ch
				}
			}
		}
	}
	for _, f := range m.plan.Furniture {
		paint(f.Box, 'm', 0)
	}
	for _, wl := range m.plan.Walls {
		paint(wl, '#', 0.07)
	}
	in := func(c, f int) bool { return c >= 0 && c < cols && f >= 0 && f < rows }
	for _, d := range m.plan.Doors {
		if !d.Closed {
			continue
		}
		c, f := cell(d.X, d.Y)
		horizontal := false
		for _, k := range []int{-3, -2, 2, 3} {
			if in(c+k, f) && g[f][c+k] == '#' {
				horizontal = true
			}
		}
		r := int(d.width()/2/CellM + 0.5)
		for k := -r; k <= r; k++ {
			cc, ff := c, f+k
			if horizontal {
				cc, ff = c+k, f
			}
			if in(cc, ff) && g[ff][cc] != '#' {
				g[ff][cc] = '='
			}
		}
	}
	if c, f := cell(x, y); in(c, f) {
		arrows := []rune("→↘↓↙←↖↑↗")
		a := math.Mod(hd, 360)
		if a < 0 {
			a += 360
		}
		g[f][c] = arrows[int((a+22.5)/45)%8]
	}
	border := strings.Repeat("#", cols+2)
	out := []string{border}
	for _, r := range g {
		out = append(out, "#"+string(r)+"#")
	}
	return append(out, border)
}
