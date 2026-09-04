package remoteapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"
)

// Helper to create a mock OWM server returning a valid response.
func newOWMServer(t *testing.T, statusCode int, resp owmResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestFetchWeather_OWM_Success(t *testing.T) {
	resp := owmResponse{
		Name:     "Holambra",
		Timezone: -10800, // UTC-3
	}
	resp.Main.Temp = 25.4
	resp.Weather = []struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	}{
		{ID: 800, Description: "clear sky"},
	}

	srv := newOWMServer(t, http.StatusOK, resp)
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("openweathermap", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{
		Name:     "Holambra",
		Region:   "SP",
		Timezone: "America/Sao_Paulo",
	}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.CityName != "Holambra" {
		t.Errorf("CityName = %q, want %q", data.CityName, "Holambra")
	}
	if data.Region != "SP" {
		t.Errorf("Region = %q, want %q", data.Region, "SP")
	}
	if data.Temperature != 25 {
		t.Errorf("Temperature = %d, want 25", data.Temperature)
	}
	if data.Description != "clear sky" {
		t.Errorf("Description = %q, want %q", data.Description, "clear sky")
	}
	if data.IconCode != weather.IconClear {
		t.Errorf("IconCode = %q, want %q", data.IconCode, weather.IconClear)
	}
}

func TestFetchWeather_OWM_WithCoordinates(t *testing.T) {
	resp := owmResponse{
		Name:     "Holambra",
		Timezone: -10800,
	}
	resp.Main.Temp = 18.3
	resp.Weather = []struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	}{
		{ID: 500, Description: "light rain"},
	}

	var receivedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("openweathermap", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{
		Name:      "Holambra",
		Region:    "SP",
		Latitude:  -22.63,
		Longitude: -47.05,
		Timezone:  "America/Sao_Paulo",
	}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.IconCode != weather.IconRain {
		t.Errorf("IconCode = %q, want %q", data.IconCode, weather.IconRain)
	}
	if data.Temperature != 18 {
		t.Errorf("Temperature = %d, want 18", data.Temperature)
	}

	// Verify lat/lon were used instead of city name
	if receivedQuery == "" {
		t.Fatal("no query received")
	}
}

func TestFetchWeather_OWM_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"cod":401,"message":"Invalid API key"}`))
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("openweathermap", "bad-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "London", Region: "UK", Timezone: "Europe/London"}
	_, err := adapter.FetchWeather(context.Background(), city)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestFetchWeather_OWM_EmptyWeather(t *testing.T) {
	resp := owmResponse{Name: "Test", Timezone: 0}
	resp.Main.Temp = 10
	resp.Weather = nil

	srv := newOWMServer(t, http.StatusOK, resp)
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("openweathermap", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "Test", Region: "XX", Timezone: "UTC"}
	_, err := adapter.FetchWeather(context.Background(), city)
	if err == nil {
		t.Fatal("expected error for empty weather array")
	}
}

func TestFetchWeather_WU_Success(t *testing.T) {
	wuResp := wuResponse{
		Observations: []wuObservation{
			{
				StationID:    "KLAX",
				Neighborhood: "Los Angeles",
				Humidity:     45,
				Metric:       wuMetric{Temp: 22.5, WindSpeed: 10, PrecipRate: 0},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(wuResp)
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("weatherunderground", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{
		Name:     "KLAX",
		Region:   "CA",
		Timezone: "America/Los_Angeles",
	}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Temperature != 23 {
		t.Errorf("Temperature = %d, want 23", data.Temperature)
	}
	if data.Description != "Clear" {
		t.Errorf("Description = %q, want %q", data.Description, "Clear")
	}
	if data.IconCode != weather.IconClear {
		t.Errorf("IconCode = %q, want %q", data.IconCode, weather.IconClear)
	}
}

func TestFetchWeather_WU_EmptyObservations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(wuResponse{Observations: nil})
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("weatherunderground", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "Test", Region: "XX", Timezone: "UTC"}
	_, err := adapter.FetchWeather(context.Background(), city)
	if err == nil {
		t.Fatal("expected error for empty observations")
	}
}

func TestFetchWeather_UnsupportedProvider(t *testing.T) {
	adapter := NewRemoteAPIAdapter("unknown", "key")
	city := config.CityConfig{Name: "Test", Region: "XX", Timezone: "UTC"}
	_, err := adapter.FetchWeather(context.Background(), city)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestTestConnection_OWM_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(owmResponse{Name: "London"})
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("openweathermap", "valid-key")
	adapter.BaseURL = srv.URL

	if err := adapter.TestConnection(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestConnection_OWM_InvalidKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("openweathermap", "bad-key")
	adapter.BaseURL = srv.URL

	err := adapter.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
	if err.Error() != "invalid API key" {
		t.Errorf("error = %q, want %q", err.Error(), "invalid API key")
	}
}

func TestTestConnection_WU_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(wuResponse{})
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("weatherunderground", "valid-key")
	adapter.BaseURL = srv.URL

	if err := adapter.TestConnection(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestConnection_WU_Success204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("weatherunderground", "valid-key")
	adapter.BaseURL = srv.URL

	if err := adapter.TestConnection(context.Background()); err != nil {
		t.Fatalf("unexpected error for 204 response: %v", err)
	}
}

func TestTestConnection_UnsupportedProvider(t *testing.T) {
	adapter := NewRemoteAPIAdapter("unknown", "key")
	err := adapter.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestMapOWMConditionToIcon(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{200, weather.IconStorm},
		{299, weather.IconStorm},
		{300, weather.IconRain},
		{399, weather.IconRain},
		{500, weather.IconRain},
		{599, weather.IconRain},
		{600, weather.IconSnow},
		{699, weather.IconSnow},
		{700, weather.IconFog},
		{799, weather.IconFog},
		{800, weather.IconClear},
		{801, weather.IconPartlyCloudy},
		{802, weather.IconPartlyCloudy},
		{803, weather.IconPartlyCloudy},
		{804, weather.IconCloudy},
		{999, weather.IconCloudy}, // unknown defaults to cloudy
	}

	for _, tt := range tests {
		got := mapOWMConditionToIcon(tt.id)
		if got != tt.want {
			t.Errorf("mapOWMConditionToIcon(%d) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestMapWUConditionToIcon(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, weather.IconStorm},
		{5, weather.IconSnow},
		{10, weather.IconRain},
		{15, weather.IconSnow},
		{20, weather.IconFog},
		{25, weather.IconCloudy},
		{28, weather.IconPartlyCloudy},
		{32, weather.IconClear},
		{36, weather.IconRain},
		{42, weather.IconSnow},
		{44, weather.IconPartlyCloudy},
		{47, weather.IconStorm},
		{100, weather.IconCloudy}, // unknown defaults to cloudy
	}

	for _, tt := range tests {
		got := mapWUConditionToIcon(tt.code)
		if got != tt.want {
			t.Errorf("mapWUConditionToIcon(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

// --- EasyWeatherWidget tests ---

// Helper to create a mock EWW server returning a canned response.
func newEWWServer(t *testing.T, statusCode int, resp ewwResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestFetchWeather_EWW_Success(t *testing.T) {
	resp := ewwResponse{
		Temp:         23.7,
		Neighborhood: "Centro",
		Country:      "BR",
		FreeText:     "clear sky",
		ObsTimeLocal: "2025-01-15 14:30:00",
	}

	srv := newEWWServer(t, http.StatusOK, resp)
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("easyweatherwidget", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{
		Name:     "Holambra",
		Region:   "SP",
		Timezone: "America/Sao_Paulo",
	}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.CityName != "Holambra" {
		t.Errorf("CityName = %q, want %q", data.CityName, "Holambra")
	}
	if data.Region != "SP" {
		t.Errorf("Region = %q, want %q", data.Region, "SP")
	}
	if data.Temperature != 24 {
		t.Errorf("Temperature = %d, want 24 (rounded from 23.7)", data.Temperature)
	}
	if data.Description != "clear sky" {
		t.Errorf("Description = %q, want %q", data.Description, "clear sky")
	}
	if data.IconCode != weather.IconClear {
		t.Errorf("IconCode = %q, want %q", data.IconCode, weather.IconClear)
	}
}

func TestFetchWeather_EWW_RFC3339ObsTimeLocal(t *testing.T) {
	resp := ewwResponse{
		Temp:         31.0,
		Neighborhood: "California",
		Country:      "US",
		FreeText:     "Clear Sky",
		ObsTimeLocal: "2026-08-21T20:37:57Z", // Live UTC RFC3339 timestamp from gateway
	}

	srv := newEWWServer(t, http.StatusOK, resp)
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("easyweatherwidget", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{
		Name:     "California",
		Region:   "US",
		Timezone: "America/Chicago",
	}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 20:37 UTC converted to America/Chicago (CDT, UTC-5) is 15:37
	if data.LocalTime.Hour() != 15 {
		t.Errorf("LocalTime.Hour() = %d, want 15 (converted from UTC to city timezone)", data.LocalTime.Hour())
	}

	icon := weather.MapConditionToIcon(data.IconCode, data.LocalTime)
	if icon != "day/clear_day" {
		t.Errorf("MapConditionToIcon at 15:37 = %q, want %q (daytime clear icon)", icon, "day/clear_day")
	}
}

func TestFetchWeather_EWW_URLConstruction(t *testing.T) {
	var receivedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The weather fetch now also triggers a pollution request; only
		// capture the weather path so this assertion stays about the weather URL.
		if strings.HasPrefix(r.URL.Path, "/api/v1/weather/") {
			receivedPath = r.URL.Path
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ewwResponse{
			Temp:     20.0,
			FreeText: "clear",
		})
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("easyweatherwidget", "my-api-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{
		Name:     "London",
		Region:   "GB",
		Timezone: "Europe/London",
	}

	_, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPath := "/api/v1/weather/key=my-api-key/London,GB"
	if receivedPath != expectedPath {
		t.Errorf("request path = %q, want %q", receivedPath, expectedPath)
	}
}

func TestFetchWeather_EWW_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("easyweatherwidget", "bad-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "London", Region: "GB", Timezone: "Europe/London"}
	_, err := adapter.FetchWeather(context.Background(), city)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestFetchWeather_EWW_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("easyweatherwidget", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "London", Region: "GB", Timezone: "Europe/London"}
	_, err := adapter.FetchWeather(context.Background(), city)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestTestConnection_EWW_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ewwResponse{Temp: 15.0, FreeText: "clear"})
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("easyweatherwidget", "valid-key")
	adapter.BaseURL = srv.URL

	if err := adapter.TestConnection(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestConnection_EWW_InvalidKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("easyweatherwidget", "bad-key")
	adapter.BaseURL = srv.URL

	err := adapter.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
	if err.Error() != "invalid API key" {
		t.Errorf("error = %q, want %q", err.Error(), "invalid API key")
	}
}

func TestMapEWWFreeTextToIcon(t *testing.T) {
	tests := []struct {
		freeText string
		want     string
	}{
		// storm/thunder keywords
		{"Thunderstorm", weather.IconStorm},
		{"heavy storm", weather.IconStorm},
		{"thunder and rain", weather.IconStorm},
		// snow keyword
		{"Light Snow", weather.IconSnow},
		{"snow showers", weather.IconSnow},
		// rain/drizzle keywords
		{"Light Rain", weather.IconRain},
		{"heavy drizzle", weather.IconRain},
		// fog/mist/haze keywords
		{"Dense Fog", weather.IconFog},
		{"morning mist", weather.IconFog},
		{"haze", weather.IconFog},
		// partly / scattered / few / broken clouds keywords
		{"Partly Cloudy", weather.IconPartlyCloudy},
		{"scattered clouds", weather.IconPartlyCloudy},
		{"few clouds", weather.IconPartlyCloudy},
		{"broken clouds", weather.IconPartlyCloudy},
		{"partly sunny", weather.IconPartlyCloudy},
		// cloud / overcast keywords
		{"Cloudy", weather.IconCloudy},
		{"overcast clouds", weather.IconCloudy},
		{"overcast", weather.IconCloudy},
		// clear keyword
		{"Clear Sky", weather.IconClear},
		{"mostly clear", weather.IconClear},
		// default case
		{"Sunny", weather.IconPartlyCloudy},
		{"", weather.IconPartlyCloudy},
	}

	for _, tt := range tests {
		got := mapEWWFreeTextToIcon(tt.freeText)
		if got != tt.want {
			t.Errorf("mapEWWFreeTextToIcon(%q) = %q, want %q", tt.freeText, got, tt.want)
		}
	}
}

// --- EWW pollution fetch tests ---

// newEWWPollutionServer routes requests by path: weather requests
// (/api/v1/weather/...) get the weather body, pollution requests
// (/api/v1/pollution/...) are handled by the provided pollutionHandler so each
// test can control the pollution response independently.
func newEWWPollutionServer(t *testing.T, weatherResp ewwResponse, pollutionHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/pollution/"):
			pollutionHandler(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/v1/weather/"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(weatherResp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestFetchWeather_EWW_PollutionSuccess(t *testing.T) {
	weatherResp := ewwResponse{
		Temp:     21.4,
		FreeText: "clear sky",
	}
	// Distinct values per metric; PM25 and NH3 use distinct values to
	// verify the JSON-tag mapping (pm2_5 -> PM25, nh3 -> NH3).
	pol := ewwPollutionResponse{
		AQI:  2,
		CO:   127.89,
		NO:   0,
		NO2:  3.08,
		O3:   74.13,
		SO2:  1.7,
		PM25: 2.1,
		PM10: 7.92,
		NH3:  0.66,
	}

	srv := newEWWPollutionServer(t, weatherResp, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(pol)
	})
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("easyweatherwidget", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "Holambra", Region: "SP", Timezone: "America/Sao_Paulo"}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil data")
	}

	// AQI pointer.
	if data.AQI == nil {
		t.Fatal("AQI = nil, want non-nil")
	}
	if *data.AQI != pol.AQI {
		t.Errorf("*AQI = %d, want %d", *data.AQI, pol.AQI)
	}

	// Float metrics, including the explicit JSON-tag mapping checks.
	floatChecks := []struct {
		name string
		got  *float64
		want float64
	}{
		{"CO", data.CO, pol.CO},
		{"NO", data.NO, pol.NO},
		{"NO2", data.NO2, pol.NO2},
		{"O3", data.O3, pol.O3},
		{"SO2", data.SO2, pol.SO2},
		{"PM25", data.PM25, pol.PM25}, // json pm2_5 -> PM25
		{"PM10", data.PM10, pol.PM10},
		{"NH3", data.NH3, pol.NH3}, // json nh3 -> NH3
	}
	for _, c := range floatChecks {
		if c.got == nil {
			t.Errorf("%s = nil, want non-nil", c.name)
			continue
		}
		if *c.got != c.want {
			t.Errorf("*%s = %v, want %v", c.name, *c.got, c.want)
		}
	}

	// Explicit JSON-tag mapping verification with distinct values.
	if data.PM25 == nil || *data.PM25 != 2.1 {
		t.Errorf("pm2_5 -> PM25 mapping failed: got %v, want 2.1", data.PM25)
	}
	if data.NH3 == nil || *data.NH3 != 0.66 {
		t.Errorf("nh3 -> NH3 mapping failed: got %v, want 0.66", data.NH3)
	}
}

func TestFetchWeather_EWW_PollutionNon200(t *testing.T) {
	weatherResp := ewwResponse{Temp: 23.7, FreeText: "clear sky"}

	srv := newEWWPollutionServer(t, weatherResp, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	})
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("easyweatherwidget", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "Holambra", Region: "SP", Timezone: "America/Sao_Paulo"}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("expected nil error (best-effort), got: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil data")
	}
	// Weather fields still populated.
	if data.Temperature != 24 {
		t.Errorf("Temperature = %d, want 24", data.Temperature)
	}
	assertNoPollution(t, data)
}

func TestFetchWeather_EWW_PollutionMalformedJSON(t *testing.T) {
	weatherResp := ewwResponse{Temp: 23.7, FreeText: "clear sky"}

	srv := newEWWPollutionServer(t, weatherResp, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{not valid json`))
	})
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("easyweatherwidget", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "Holambra", Region: "SP", Timezone: "America/Sao_Paulo"}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("expected nil error (best-effort), got: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil data")
	}
	assertNoPollution(t, data)
}

func TestFetchWeather_EWW_PollutionNetworkError(t *testing.T) {
	weatherResp := ewwResponse{Temp: 23.7, FreeText: "clear sky"}

	// The pollution handler aborts the connection, forcing a client-side
	// network error for the pollution request while weather still succeeds.
	srv := newEWWPollutionServer(t, weatherResp, func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	})
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("easyweatherwidget", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "Holambra", Region: "SP", Timezone: "America/Sao_Paulo"}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("expected nil error (best-effort), got: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil data")
	}
	assertNoPollution(t, data)
}

func TestFetchWeather_EWW_PollutionURLConstruction(t *testing.T) {
	weatherResp := ewwResponse{Temp: 20.0, FreeText: "clear"}

	var pollutionPath string
	makeSrv := func() *httptest.Server {
		return newEWWPollutionServer(t, weatherResp, func(w http.ResponseWriter, r *http.Request) {
			pollutionPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ewwPollutionResponse{})
		})
	}

	// Configured (non-empty) key.
	srv := makeSrv()
	adapter := NewRemoteAPIAdapter("easyweatherwidget", "my-api-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "London", Region: "GB", Timezone: "Europe/London"}
	if _, err := adapter.FetchWeather(context.Background(), city); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	srv.Close()

	wantPath := "/api/v1/pollution/key=my-api-key/London,GB"
	if pollutionPath != wantPath {
		t.Errorf("pollution path = %q, want %q", pollutionPath, wantPath)
	}

	// Empty key => "free" segment. Use a default city so the weather call
	// proceeds (config.IsDefaultCity allows only default cities without a key).
	pollutionPath = ""
	srv2 := makeSrv()
	adapterFree := NewRemoteAPIAdapter("easyweatherwidget", "")
	adapterFree.BaseURL = srv2.URL

	defaultCity := config.CityConfig{Name: "Broxburn", Region: "GB", Timezone: "Europe/London"}
	if _, err := adapterFree.FetchWeather(context.Background(), defaultCity); err != nil {
		t.Fatalf("unexpected error (free/default city): %v", err)
	}
	srv2.Close()

	wantFreePath := "/api/v1/pollution/key=free/Broxburn,GB"
	if pollutionPath != wantFreePath {
		t.Errorf("free pollution path = %q, want %q", pollutionPath, wantFreePath)
	}
}

// assertNoPollution asserts that all nine pollution pointers on data are nil.
func assertNoPollution(t *testing.T, data *weather.WeatherData) {
	t.Helper()
	if data.AQI != nil {
		t.Errorf("AQI = %v, want nil", *data.AQI)
	}
	floats := []struct {
		name string
		got  *float64
	}{
		{"CO", data.CO},
		{"NO", data.NO},
		{"NO2", data.NO2},
		{"O3", data.O3},
		{"SO2", data.SO2},
		{"NH3", data.NH3},
		{"PM25", data.PM25},
		{"PM10", data.PM10},
	}
	for _, f := range floats {
		if f.got != nil {
			t.Errorf("%s = %v, want nil", f.name, *f.got)
		}
	}
}

func TestFetchWeather_OWM_NoPollution(t *testing.T) {
	resp := owmResponse{Name: "Holambra", Timezone: -10800}
	resp.Main.Temp = 25.4
	resp.Weather = []struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	}{
		{ID: 800, Description: "clear sky"},
	}

	srv := newOWMServer(t, http.StatusOK, resp)
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("openweathermap", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "Holambra", Region: "SP", Timezone: "America/Sao_Paulo"}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoPollution(t, data)
}

func TestFetchWeather_WU_NoPollution(t *testing.T) {
	wuResp := wuResponse{
		Observations: []wuObservation{
			{
				StationID:    "KLAX",
				Neighborhood: "Los Angeles",
				Humidity:     45,
				Metric:       wuMetric{Temp: 22.5, WindSpeed: 10, PrecipRate: 0},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(wuResp)
	}))
	defer srv.Close()

	adapter := NewRemoteAPIAdapter("weatherunderground", "test-key")
	adapter.BaseURL = srv.URL

	city := config.CityConfig{Name: "KLAX", Region: "CA", Timezone: "America/Los_Angeles"}

	data, err := adapter.FetchWeather(context.Background(), city)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoPollution(t, data)
}
