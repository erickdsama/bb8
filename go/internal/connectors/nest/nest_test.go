package nest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connections"
	"github.com/erickdsama/bb8/go/internal/connectors"
	"github.com/erickdsama/bb8/go/internal/tools"
)

func TestThermostat(t *testing.T) {
	var cmds []map[string]any
	eco, mode := "MANUAL_ECO", "HEAT"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			w.Write([]byte(`{"access_token":"at","expires_in":3600}`))
		case r.Header.Get("Authorization") != "Bearer at":
			w.WriteHeader(401)
		case r.URL.Path == "/enterprises/bad/devices":
			w.WriteHeader(404)
			w.Write([]byte(`{"error":{"code":404,"message":"Enterprise enterprises/bad not found.","status":"NOT_FOUND"}}`))
		case r.Method == "GET" && r.URL.Path == "/enterprises/p1/devices":
			w.Write([]byte(`{"devices":[{"name":"enterprises/p1/devices/d1","type":"sdm.devices.types.THERMOSTAT",
			 "traits":{"sdm.devices.traits.Info":{"customName":""},
			  "sdm.devices.traits.Temperature":{"ambientTemperatureCelsius":21.46},
			  "sdm.devices.traits.Humidity":{"ambientHumidityPercent":38},
			  "sdm.devices.traits.ThermostatMode":{"mode":"` + mode + `","availableModes":["HEAT","COOL","HEATCOOL","OFF"]},
			  "sdm.devices.traits.ThermostatEco":{"mode":"` + eco + `"},
			  "sdm.devices.traits.ThermostatHvac":{"status":"HEATING"},
			  "sdm.devices.traits.ThermostatTemperatureSetpoint":{"heatCelsius":19.5}},
			 "parentRelations":[{"parent":"x","displayName":"Sala"}]},
			 {"name":"enterprises/p1/devices/cam","type":"sdm.devices.types.CAMERA","traits":{}}]}`))
		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, ":executeCommand"):
			var c map[string]any
			json.NewDecoder(r.Body).Decode(&c)
			cmds = append(cmds, c)
			if c["command"] == "sdm.devices.commands.ThermostatEco.SetMode" {
				eco = "OFF"
			}
			w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	SDMAPI = srv.URL

	c, _ := config.Open(t.TempDir())
	n := New(nil)
	c.Register(n.Fields()...)
	if st := n.Status(c); st.State != connectors.NeedsConf {
		t.Fatal(st)
	}
	if len(n.Tools(c)) != 0 {
		t.Fatal("tools without a project")
	}
	accts, _ := connections.Open("")
	n.accts = accts
	accts.Add(connections.Connection{Kind: connections.KindGoogle, Name: "google", Owner: "Erick", Account: "erick@example.com",
		Auth: connections.Auth{Type: "oauth", Refresh: "rt", ClientID: "cid", ClientSecret: "cs", AuthMethod: "client_secret_post", TokenURL: srv.URL + "/token"}})
	c.Set(map[string]any{keyProject: "p1"}, config.SourceWeb, nil)

	ts := map[string]tools.Tool{}
	for _, tl := range n.Tools(c) {
		ts[tl.Name] = tl
	}
	call := func(name, args string) map[string]any {
		out, err := ts[name].Handler(context.Background(), json.RawMessage(args))
		if err != nil {
			t.Fatal(name, err)
		}
		var m map[string]any
		b, _ := json.Marshal(out.Data)
		json.Unmarshal(b, &m)
		return m
	}

	st := call("thermostat_status", `{}`)
	th := st["thermostats"].([]any)
	if len(th) != 1 {
		t.Fatal(st)
	}
	got := th[0].(map[string]any)
	if got["name"] != "Sala" || got["ambient_c"] != 21.5 || got["mode"] != "heat" || got["hvac"] != "heating" || got["eco"] != true || got["heat_setpoint_c"] != 19.5 {
		t.Fatal(got)
	}
	if s := n.Status(c); s.State != connectors.OK || !strings.Contains(s.Detail, "1 thermostat") {
		t.Fatal(s)
	}
	if !ts["thermostat_set"].KnownOnly || ts["thermostat_set"].Moves {
		t.Fatal("thermostat_set: known voices only, does not move the robot")
	}

	// 40 °C: eco off first, then the heat setpoint, kept at 32.
	r := call("thermostat_set", `{"temperature_c":40}`)
	if r["result"] != "done" || r["temperature_c"] != 32.0 || len(cmds) != 2 ||
		cmds[0]["command"] != "sdm.devices.commands.ThermostatEco.SetMode" ||
		cmds[1]["command"] != "sdm.devices.commands.ThermostatTemperatureSetpoint.SetHeat" || cmds[1]["params"].(map[string]any)["heatCelsius"] != 32.0 {
		t.Fatal(r, cmds)
	}

	// Cool at 24: mode first, then the cool setpoint.
	cmds = nil
	r = call("thermostat_set", `{"temperature_c":24,"mode":"cool"}`)
	if r["result"] != "done" || len(cmds) != 2 || cmds[0]["params"].(map[string]any)["mode"] != "COOL" ||
		cmds[1]["command"] != "sdm.devices.commands.ThermostatTemperatureSetpoint.SetCool" {
		t.Fatal(r, cmds)
	}

	// Off and no mode: ask, change nothing.
	mode, cmds = "OFF", nil
	if r = call("thermostat_set", `{"temperature_c":22}`); r["result"] != "error" || len(cmds) != 0 {
		t.Fatal(r, cmds)
	}

	// A wrong project says so.
	c.Set(map[string]any{keyProject: "bad"}, config.SourceWeb, nil)
	for _, tl := range n.Tools(c) {
		ts[tl.Name] = tl
	}
	if r = call("thermostat_status", `{}`); r["result"] != "error" || !strings.Contains(r["reason"].(string), "project ID") {
		t.Fatal(r)
	}
}
