package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/erickdsama/bb8/go/internal/config"
)

func TestWeatherByCity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/geo") {
			if r.URL.Query().Get("name") != "Monterrey" {
				t.Errorf("city %q", r.URL.Query().Get("name"))
			}
			w.Write([]byte(`{"results":[{"name":"Monterrey","latitude":25.67,"longitude":-100.31}]}`))
			return
		}
		if r.URL.Query().Get("latitude") != "25.67" {
			t.Errorf("lat %q", r.URL.Query().Get("latitude"))
		}
		w.Write([]byte(`{"current":{"temperature_2m":31.2,"relative_humidity_2m":40,"weather_code":2,"wind_speed_10m":12},
			"daily":{"temperature_2m_max":[33,29],"temperature_2m_min":[22,20],"precipitation_probability_max":[10,70],"weather_code":[1,63]}}`))
	}))
	defer srv.Close()
	w := New()
	w.ForecastURL, w.GeocodeURL = srv.URL+"/forecast", srv.URL+"/geo"
	c, _ := config.Open("")
	c.Register(w.Fields()...)
	c.Register(config.Field{Key: "home.city", Type: config.String}, config.Field{Key: "home.latitude", Type: config.Number, Default: 0.0},
		config.Field{Key: "home.longitude", Type: config.Number, Default: 0.0})
	if _, err := w.Get(context.Background(), c, "now"); err == nil {
		t.Fatal("no city should fail")
	}
	c.Set(map[string]any{"home.city": "Monterrey"}, config.SourceWeb, nil)
	now, err := w.Get(context.Background(), c, "now")
	if err != nil || now["temperature"] != 31.2 || now["sky"] != "partly cloudy" {
		t.Fatal(now, err)
	}
	tm, _ := w.Get(context.Background(), c, "tomorrow")
	if tm["max"] != 29.0 || tm["sky"] != "rain" || tm["rain_chance_pct"] != 70.0 {
		t.Fatal(tm)
	}
}
