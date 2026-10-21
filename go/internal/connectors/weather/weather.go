// Package weather is the Open-Meteo connector (no API key).
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/erickdsama/bb8/go/internal/config"
	"github.com/erickdsama/bb8/go/internal/connectors"
	"github.com/erickdsama/bb8/go/internal/tools"
)

type Weather struct {
	ForecastURL string // https://api.open-meteo.com/v1/forecast
	GeocodeURL  string // https://geocoding-api.open-meteo.com/v1/search
	HTTP        *http.Client
}

func New() *Weather {
	return &Weather{
		ForecastURL: "https://api.open-meteo.com/v1/forecast",
		GeocodeURL:  "https://geocoding-api.open-meteo.com/v1/search",
		HTTP:        &http.Client{Timeout: 8 * time.Second},
	}
}

func (w *Weather) ID() string      { return "weather" }
func (w *Weather) DefaultOn() bool { return true } // Open-Meteo needs no key

func (w *Weather) Fields() []config.Field {
	return []config.Field{{Key: "connectors.weather.units", Label: "Units", Type: config.Enum,
		Options: []string{"metric", "imperial"}, Default: "metric",
		Help: "Uses home.latitude/longitude, or home.city when they are 0."}}
}

func (w *Weather) Status(c *config.Store) connectors.Status {
	if c.Float("home.latitude") == 0 && c.Float("home.longitude") == 0 && c.String("home.city") == "" {
		return connectors.Status{ID: w.ID(), State: connectors.NeedsConf, Detail: "set home.city or home.latitude/longitude"}
	}
	return connectors.Status{ID: w.ID(), State: connectors.OK}
}

func (w *Weather) Tools(c *config.Store) []tools.Tool {
	return []tools.Tool{{
		Name:        "weather",
		Description: "Weather at home. when: now, today or tomorrow. Returns temperature, sky, rain chance and wind.",
		Schema:      tools.Object(map[string]any{"when": tools.Enum("default now", "now", "today", "tomorrow")}),
		Handler: tools.Bind(func(ctx context.Context, in struct {
			When string `json:"when"`
		}) (tools.Output, error) {
			r, err := w.Get(ctx, c, in.When)
			if err != nil {
				return tools.Output{}, err
			}
			return tools.Data(r)
		}),
	}}
}

func (w *Weather) getJSON(ctx context.Context, u string, v any) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	r, err := w.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("weather service: HTTP %d", r.StatusCode)
	}
	return json.NewDecoder(r.Body).Decode(v)
}

func (w *Weather) place(ctx context.Context, c *config.Store) (lat, lon float64, name string, err error) {
	lat, lon, name = c.Float("home.latitude"), c.Float("home.longitude"), c.String("home.city")
	if lat != 0 || lon != 0 {
		return lat, lon, name, nil
	}
	if name == "" {
		return 0, 0, "", errors.New("I don't know where home is: set home.city")
	}
	var g struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := w.getJSON(ctx, w.GeocodeURL+"?count=1&language=es&name="+url.QueryEscape(name), &g); err != nil {
		return 0, 0, "", err
	}
	if len(g.Results) == 0 {
		return 0, 0, "", fmt.Errorf("city %q not found", name)
	}
	return g.Results[0].Latitude, g.Results[0].Longitude, g.Results[0].Name, nil
}

// Get returns the weather for when (now|today|tomorrow).
func (w *Weather) Get(ctx context.Context, c *config.Store, when string) (map[string]any, error) {
	lat, lon, name, err := w.place(ctx, c)
	if err != nil {
		return nil, err
	}
	q := url.Values{
		"latitude": {fmt.Sprint(lat)}, "longitude": {fmt.Sprint(lon)}, "timezone": {"auto"}, "forecast_days": {"2"},
		"current": {"temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m"},
		"daily":   {"temperature_2m_max,temperature_2m_min,precipitation_probability_max,weather_code"},
	}
	imperial := c.String("connectors.weather.units") == "imperial"
	if imperial {
		q.Set("temperature_unit", "fahrenheit")
		q.Set("wind_speed_unit", "mph")
	}
	var f struct {
		Current struct {
			Temp     float64 `json:"temperature_2m"`
			Humidity float64 `json:"relative_humidity_2m"`
			Code     int     `json:"weather_code"`
			Wind     float64 `json:"wind_speed_10m"`
		} `json:"current"`
		Daily struct {
			Max  []float64 `json:"temperature_2m_max"`
			Min  []float64 `json:"temperature_2m_min"`
			Rain []float64 `json:"precipitation_probability_max"`
			Code []int     `json:"weather_code"`
		} `json:"daily"`
	}
	if err := w.getJSON(ctx, w.ForecastURL+"?"+q.Encode(), &f); err != nil {
		return nil, err
	}
	unit, wind := "°C", "km/h"
	if imperial {
		unit, wind = "°F", "mph"
	}
	out := map[string]any{"place": name, "unit": unit}
	switch when {
	case "today", "tomorrow":
		i := 0
		if when == "tomorrow" {
			i = 1
		}
		if len(f.Daily.Max) <= i || len(f.Daily.Min) <= i || len(f.Daily.Code) <= i {
			return nil, errors.New("no forecast for " + when)
		}
		out["when"], out["max"], out["min"], out["sky"] = when, f.Daily.Max[i], f.Daily.Min[i], Sky(f.Daily.Code[i])
		if len(f.Daily.Rain) > i {
			out["rain_chance_pct"] = f.Daily.Rain[i]
		}
	default:
		out["when"], out["temperature"], out["humidity_pct"] = "now", f.Current.Temp, f.Current.Humidity
		out["sky"], out["wind"] = Sky(f.Current.Code), fmt.Sprintf("%.0f %s", f.Current.Wind, wind)
	}
	return out, nil
}

// Sky turns a WMO weather code into words.
func Sky(code int) string {
	switch {
	case code == 0:
		return "clear"
	case code <= 2:
		return "partly cloudy"
	case code == 3:
		return "overcast"
	case code == 45 || code == 48:
		return "fog"
	case code >= 51 && code <= 57:
		return "drizzle"
	case code >= 61 && code <= 67, code >= 80 && code <= 82:
		return "rain"
	case code >= 71 && code <= 77, code == 85 || code == 86:
		return "snow"
	case code >= 95:
		return "thunderstorm"
	}
	return "unknown"
}
