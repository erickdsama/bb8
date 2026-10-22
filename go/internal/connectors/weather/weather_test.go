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

func TestGeocodeFreeForm(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("name")
		asked = append(asked, q)
		if q == "Ciudad Juarez" {
			w.Write([]byte(`{"results":[{"name":"Ciudad Juárez","latitude":22.3,"longitude":-98.4,"admin1":"Tamaulipas","country":"México"},
				{"name":"Ciudad Juárez","latitude":31.72,"longitude":-106.46,"admin1":"Estado de Chihuahua","country":"México"}]}`))
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	w := New()
	w.GeocodeURL = srv.URL
	lat, _, name, err := w.geocode(context.Background(), "Cd. Juarez Chihuahua")
	if err != nil || lat != 31.72 || name != "Ciudad Juárez" {
		t.Fatal(lat, name, err, asked)
	}
	if asked[0] != "Ciudad Juarez Chihuahua" || len(asked) != 2 {
		t.Fatal(asked)
	}
	w.geocode(context.Background(), "Cd. Juarez Chihuahua") // cached
	if len(asked) != 2 {
		t.Fatal("not cached", asked)
	}
	if _, _, _, err := w.geocode(context.Background(), "Nowhere"); err == nil {
		t.Fatal("want not found")
	}
}
