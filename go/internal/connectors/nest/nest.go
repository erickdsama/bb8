// Package nest is the Nest thermostat connector, through Google's Smart Device
// Management (SDM) API. It uses the Google accounts people sign in with on the
// Services page (the sign-in also asks for sdm.service); the owner registers
// once in Google's Device Access console (a one-time US$5 fee) and links their
// Nest account there, which gives the project ID set here.
package nest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connections"
	"github.com/erickdsama/bb8/go/internal/connectors"
	"github.com/erickdsama/bb8/go/internal/tools"
)

const keyProject = "connectors.nest.project_id"

// SDMAPI is the SDM endpoint; tests point it at a fake server.
var SDMAPI = "https://smartdevicemanagement.googleapis.com/v1"

// Setpoints BB-8 may ask for, whatever the model says.
const minC, maxC = 9.0, 32.0

type Nest struct {
	accts *connections.Store
	HTTP  *http.Client

	mu     sync.Mutex
	acct   string // the account that reached the thermostats last time
	detail string // what the last call found, for the page
}

func New(accts *connections.Store) *Nest {
	return &Nest{accts: accts, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

func (n *Nest) ID() string    { return "nest" }
func (n *Nest) Title() string { return "Nest thermostat" }
func (n *Nest) Description() string {
	return "Reads and sets the Nest thermostat."
}

func (n *Nest) Fields() []config.Field {
	return []config.Field{{Key: keyProject, Label: "Device Access project ID", Type: config.String,
		Help: "From console.nest.google.com/device-access."}}
}

func (n *Nest) accounts() []connections.Connection {
	if n.accts == nil {
		return nil
	}
	return n.accts.OfKind(connections.KindGoogle)
}

func (n *Nest) Status(c *config.Store) connectors.Status {
	st := connectors.Status{ID: n.ID()}
	switch {
	case strings.TrimSpace(c.String(keyProject)) == "":
		st.State, st.Detail = connectors.NeedsConf, "set the Device Access project ID"
	case len(n.accounts()) == 0:
		st.State, st.Detail = connectors.NeedsAuth, "connect Google on the Services page"
	default:
		n.mu.Lock()
		st.State, st.Detail = connectors.OK, n.detail
		n.mu.Unlock()
	}
	return st
}

// apiError explains SDM's refusals in words the agent can repeat.
type apiError struct {
	code int
	msg  string
}

func (e *apiError) Error() string { return e.msg }

func explain(code int, status, msg string) error {
	switch {
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return &apiError{code, "Google did not allow the thermostat: connect Google again on the Services page (it asks for the Nest permission) " +
			"and make sure that account is linked to the Device Access project"}
	case code == http.StatusNotFound || strings.Contains(strings.ToLower(msg), "enterprise"):
		return &apiError{code, "the Device Access project ID is wrong (connectors.nest.project_id)"}
	case status == "FAILED_PRECONDITION":
		return &apiError{code, "the thermostat refused: " + msg}
	}
	return &apiError{code, fmt.Sprintf("Nest: HTTP %d %s", code, msg)}
}

func (n *Nest) call(ctx context.Context, acctID, method, path string, body, out any) error {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequestWithContext(ctx, method, SDMAPI+path, bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", "application/json")
	r, err := n.accts.Client(acctID, n.HTTP).Do(req) // signs and refreshes the token
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 300 {
		var e struct {
			Error struct {
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&e)
		return explain(r.StatusCode, e.Error.Status, e.Error.Message)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(r.Body).Decode(out)
}

type device struct {
	Name    string                     `json:"name"`
	Type    string                     `json:"type"`
	Traits  map[string]json.RawMessage `json:"traits"`
	Parents []struct {
		DisplayName string `json:"displayName"`
	} `json:"parentRelations"`
}

func (d device) trait(name string, v any) bool {
	raw, ok := d.Traits["sdm.devices.traits."+name]
	return ok && json.Unmarshal(raw, v) == nil
}

// thermostat is what BB-8 reports about one.
type thermostat struct {
	dev      device
	Name     string   `json:"name"`
	Room     string   `json:"room,omitempty"`
	AmbientC *float64 `json:"ambient_c,omitempty"`
	Humidity *float64 `json:"humidity_pct,omitempty"`
	Mode     string   `json:"mode"`           // heat, cool, heatcool, off
	HVAC     string   `json:"hvac,omitempty"` // heating, cooling, off
	HeatC    *float64 `json:"heat_setpoint_c,omitempty"`
	CoolC    *float64 `json:"cool_setpoint_c,omitempty"`
	Eco      bool     `json:"eco"`
	Modes    []string `json:"available_modes,omitempty"`
}

func one(v float64) *float64 { r := math.Round(v*10) / 10; return &r }

func read(d device) thermostat {
	t := thermostat{dev: d}
	var info struct {
		CustomName string `json:"customName"`
	}
	d.trait("Info", &info)
	if len(d.Parents) > 0 {
		t.Room = d.Parents[0].DisplayName
	}
	t.Name = info.CustomName
	if t.Name == "" {
		t.Name = t.Room
	}
	if t.Name == "" {
		t.Name = "Nest"
	}
	var temp struct {
		C *float64 `json:"ambientTemperatureCelsius"`
	}
	if d.trait("Temperature", &temp) && temp.C != nil {
		t.AmbientC = one(*temp.C)
	}
	var hum struct {
		P *float64 `json:"ambientHumidityPercent"`
	}
	if d.trait("Humidity", &hum) && hum.P != nil {
		t.Humidity = one(*hum.P)
	}
	var mode struct {
		Mode      string   `json:"mode"`
		Available []string `json:"availableModes"`
	}
	d.trait("ThermostatMode", &mode)
	t.Mode = strings.ToLower(mode.Mode)
	for _, m := range mode.Available {
		t.Modes = append(t.Modes, strings.ToLower(m))
	}
	var hvac struct {
		Status string `json:"status"`
	}
	d.trait("ThermostatHvac", &hvac)
	t.HVAC = strings.ToLower(hvac.Status)
	var sp struct {
		Heat *float64 `json:"heatCelsius"`
		Cool *float64 `json:"coolCelsius"`
	}
	if d.trait("ThermostatTemperatureSetpoint", &sp) {
		if sp.Heat != nil {
			t.HeatC = one(*sp.Heat)
		}
		if sp.Cool != nil {
			t.CoolC = one(*sp.Cool)
		}
	}
	var eco struct {
		Mode string `json:"mode"`
	}
	d.trait("ThermostatEco", &eco)
	t.Eco = eco.Mode != "" && eco.Mode != "OFF"
	return t
}

// thermostats lists them with the first Google account that reaches the
// project, remembering it for next time.
func (n *Nest) thermostats(ctx context.Context, project string) ([]thermostat, string, error) {
	list := n.accounts()
	if len(list) == 0 {
		return nil, "", errors.New("nobody has connected Google yet (Services page)")
	}
	n.mu.Lock()
	last := n.acct
	n.mu.Unlock()
	for i, a := range list { // the one that worked last time goes first
		if a.ID == last {
			list[0], list[i] = list[i], list[0]
		}
	}
	var firstErr error
	for _, a := range list {
		var out struct {
			Devices []device `json:"devices"`
		}
		if err := n.call(ctx, a.ID, "GET", "/enterprises/"+project+"/devices", nil, &out); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			var ae *apiError
			if errors.As(err, &ae) && ae.code == http.StatusNotFound {
				break // the project itself is wrong: no account will fix it
			}
			continue
		}
		var ts []thermostat
		for _, d := range out.Devices {
			if d.Type == "sdm.devices.types.THERMOSTAT" {
				ts = append(ts, read(d))
			}
		}
		n.mu.Lock()
		n.acct = a.ID
		n.detail = fmt.Sprintf("%d thermostat(s) · %s", len(ts), a.Account)
		n.mu.Unlock()
		return ts, a.ID, nil
	}
	n.mu.Lock()
	n.detail = firstErr.Error()
	n.mu.Unlock()
	return nil, "", firstErr
}

func pick(ts []thermostat, name string) (thermostat, bool) {
	if len(ts) == 1 && name == "" {
		return ts[0], true
	}
	name = strings.ToLower(strings.TrimSpace(name))
	for _, t := range ts {
		if name != "" && (strings.Contains(strings.ToLower(t.Name), name) || strings.Contains(strings.ToLower(t.Room), name)) {
			return t, true
		}
	}
	if name == "" && len(ts) > 0 {
		return ts[0], true
	}
	return thermostat{}, false
}

func clamp(c float64) float64 { return math.Max(minC, math.Min(maxC, c)) }

func (n *Nest) command(ctx context.Context, acct string, t thermostat, cmd string, params map[string]any) error {
	return n.call(ctx, acct, "POST", "/"+t.dev.Name+":executeCommand",
		map[string]any{"command": "sdm.devices.commands." + cmd, "params": params}, nil)
}

func fail(why string) (tools.Output, error) {
	return tools.Data(map[string]any{"result": "error", "reason": why})
}

func (n *Nest) Tools(c *config.Store) []tools.Tool {
	project := strings.TrimSpace(c.String(keyProject))
	if project == "" {
		return nil
	}
	return []tools.Tool{
		{Name: "thermostat_status",
			Description: "The Nest thermostat(s) at home: name, room, temperature inside (°C), humidity, mode (heat, cool, heatcool, off), " +
				"hvac (heating, cooling, off), setpoints (°C) and eco. result: done or error with a reason to tell the person.",
			Schema: tools.Object(map[string]any{}),
			Handler: tools.Bind(func(ctx context.Context, _ struct{}) (tools.Output, error) {
				ts, _, err := n.thermostats(ctx, project)
				if err != nil {
					return fail(err.Error())
				}
				if len(ts) == 0 {
					return fail("no thermostat in this Device Access project")
				}
				return tools.Data(map[string]any{"result": "done", "thermostats": ts})
			})},
		{Name: "thermostat_set", KnownOnly: true,
			Description: "Set the Nest thermostat when someone asks. temperature_c: 9 to 32 °C. mode (optional): heat, cool, heatcool or off; " +
				"without it the current mode stays (if it is off, ask whether to heat or cool). name: only when there are several. " +
				"Turns eco off so the setpoint holds. result: done (with the new state) or error with a reason.",
			Schema: tools.Object(map[string]any{"temperature_c": tools.Num("°C, 9 to 32"),
				"mode": tools.Enum("optional", "heat", "cool", "heatcool", "off"), "name": tools.Str("which thermostat, if several")}),
			Handler: tools.Bind(func(ctx context.Context, in struct {
				Temp float64 `json:"temperature_c"`
				Mode string  `json:"mode"`
				Name string  `json:"name"`
			}) (tools.Output, error) {
				ts, acct, err := n.thermostats(ctx, project)
				if err != nil {
					return fail(err.Error())
				}
				t, ok := pick(ts, in.Name)
				if !ok {
					var names []string
					for _, x := range ts {
						names = append(names, x.Name)
					}
					return fail("no thermostat called that; there are: " + strings.Join(names, ", "))
				}
				mode := strings.ToLower(in.Mode)
				if mode != "" && mode != t.Mode {
					if err := n.command(ctx, acct, t, "ThermostatMode.SetMode", map[string]any{"mode": strings.ToUpper(mode)}); err != nil {
						return fail(err.Error())
					}
					t.Mode = mode
				}
				if t.Mode == "off" {
					if mode == "off" {
						return tools.Data(map[string]any{"result": "done", "mode": "off"})
					}
					return fail("the thermostat is off: ask whether to heat or cool")
				}
				if in.Temp == 0 {
					return tools.Data(map[string]any{"result": "done", "mode": t.Mode})
				}
				temp := clamp(in.Temp)
				if t.Eco {
					if err := n.command(ctx, acct, t, "ThermostatEco.SetMode", map[string]any{"mode": "OFF"}); err != nil {
						return fail(err.Error())
					}
				}
				switch t.Mode {
				case "heat":
					err = n.command(ctx, acct, t, "ThermostatTemperatureSetpoint.SetHeat", map[string]any{"heatCelsius": temp})
				case "cool":
					err = n.command(ctx, acct, t, "ThermostatTemperatureSetpoint.SetCool", map[string]any{"coolCelsius": temp})
				default: // heatcool: a 2 °C band around it
					err = n.command(ctx, acct, t, "ThermostatTemperatureSetpoint.SetRange",
						map[string]any{"heatCelsius": clamp(temp - 1), "coolCelsius": clamp(temp + 1)})
				}
				if err != nil {
					return fail(err.Error())
				}
				out := map[string]any{"result": "done", "name": t.Name, "mode": t.Mode, "temperature_c": temp}
				if temp != in.Temp {
					out["note"] = fmt.Sprintf("kept within %.0f–%.0f °C", minC, maxC)
				}
				return tools.Data(out)
			})},
	}
}
