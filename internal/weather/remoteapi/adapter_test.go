package remoteapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	resp.Main.Temp = 25.7
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
		{803, weather.IconCloudy},
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
