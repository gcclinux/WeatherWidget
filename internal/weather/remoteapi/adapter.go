// Package remoteapi implements the WeatherProvider interface for remote weather APIs.
package remoteapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"
)

const (
	defaultOWMBaseURL = "https://api.openweathermap.org"
	defaultWUBaseURL  = "https://api.weather.com"
)

// RemoteAPIAdapter implements weather.WeatherProvider for remote weather APIs.
// It supports OpenWeatherMap and Weather Underground providers.
type RemoteAPIAdapter struct {
	client   *http.Client
	provider string // "openweathermap" | "weatherunderground"
	apiKey   string
	BaseURL  string // Exported so tests can point to a mock server
}

// NewRemoteAPIAdapter creates a new RemoteAPIAdapter for the given provider and API key.
// The HTTP client is configured with a 10-second timeout.
func NewRemoteAPIAdapter(provider, apiKey string) *RemoteAPIAdapter {
	baseURL := defaultOWMBaseURL
	if provider == "weatherunderground" {
		baseURL = defaultWUBaseURL
	}
	return &RemoteAPIAdapter{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		provider: provider,
		apiKey:   apiKey,
		BaseURL:  baseURL,
	}
}

// FetchWeather retrieves current weather data for the given city from the configured provider.
func (r *RemoteAPIAdapter) FetchWeather(ctx context.Context, city config.CityConfig) (*weather.WeatherData, error) {
	switch r.provider {
	case "openweathermap":
		return r.fetchOWM(ctx, city)
	case "weatherunderground":
		return r.fetchWU(ctx, city)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", r.provider)
	}
}

// TestConnection performs a lightweight API call to verify the credentials are valid.
func (r *RemoteAPIAdapter) TestConnection(ctx context.Context) error {
	switch r.provider {
	case "openweathermap":
		return r.testOWM(ctx)
	case "weatherunderground":
		return r.testWU(ctx)
	default:
		return fmt.Errorf("unsupported provider: %s", r.provider)
	}
}

// --- OpenWeatherMap implementation ---

// owmResponse represents the relevant fields from the OpenWeatherMap current weather API.
type owmResponse struct {
	Name     string `json:"name"`
	Timezone int    `json:"timezone"` // offset in seconds from UTC
	Main     struct {
		Temp float64 `json:"temp"`
	} `json:"main"`
	Weather []struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	} `json:"weather"`
	Cod     interface{} `json:"cod"` // can be int or string
	Message string      `json:"message"`
}

func (r *RemoteAPIAdapter) fetchOWM(ctx context.Context, city config.CityConfig) (*weather.WeatherData, error) {
	u, err := url.Parse(r.BaseURL + "/data/2.5/weather")
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}

	q := u.Query()
	if city.Latitude != 0 || city.Longitude != 0 {
		q.Set("lat", fmt.Sprintf("%f", city.Latitude))
		q.Set("lon", fmt.Sprintf("%f", city.Longitude))
	} else {
		q.Set("q", city.Name+","+city.Region)
	}
	q.Set("appid", r.apiKey)
	q.Set("units", "metric")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	log.Printf("fetching weather from OWM for %s (%f, %f)...", city.Name, city.Latitude, city.Longitude)
	resp, err := r.client.Do(req)
	if err != nil {
		log.Printf("OWM network error for %s: %v", city.Name, err)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("OWM API error for %s: status %d, body: %s", city.Name, resp.StatusCode, string(body))
		return nil, fmt.Errorf("OWM API error (status %d): %s", resp.StatusCode, string(body))
	}

	var owm owmResponse
	if err := json.Unmarshal(body, &owm); err != nil {
		return nil, fmt.Errorf("parse OWM response: %w", err)
	}

	if len(owm.Weather) == 0 {
		return nil, fmt.Errorf("OWM response missing weather data")
	}

	now := time.Now().UTC()
	localTime := now.Add(time.Duration(owm.Timezone) * time.Second)

	temp := int(math.Round(owm.Main.Temp))
	log.Printf("successfully fetched weather for %s from OWM: %d°C, %s", city.Name, temp, owm.Weather[0].Description)

	return &weather.WeatherData{
		CityName:    city.Name,
		Region:      city.Region,
		Temperature: temp,
		Description: owm.Weather[0].Description,
		IconCode:    mapOWMConditionToIcon(owm.Weather[0].ID),
		LocalTime:   localTime,
		FetchedAt:   now,
	}, nil
}

func (r *RemoteAPIAdapter) testOWM(ctx context.Context) error {
	// Use a lightweight call with a known city to verify the API key.
	u, err := url.Parse(r.BaseURL + "/data/2.5/weather")
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	q := u.Query()
	q.Set("q", "London")
	q.Set("appid", r.apiKey)
	q.Set("units", "metric")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API key")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API test failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// mapOWMConditionToIcon maps OpenWeatherMap condition IDs to internal icon codes.
// See: https://openweathermap.org/weather-conditions
func mapOWMConditionToIcon(id int) string {
	switch {
	case id >= 200 && id <= 299:
		return weather.IconStorm
	case id >= 300 && id <= 399:
		return weather.IconRain
	case id >= 500 && id <= 599:
		return weather.IconRain
	case id >= 600 && id <= 699:
		return weather.IconSnow
	case id >= 700 && id <= 799:
		return weather.IconFog
	case id == 800:
		return weather.IconClear
	case id == 801 || id == 802:
		return weather.IconPartlyCloudy
	case id == 803 || id == 804:
		return weather.IconCloudy
	default:
		return weather.IconCloudy
	}
}

// --- Weather Underground implementation ---

// wuResponse represents the relevant fields from the Weather Underground API.
type wuResponse struct {
	Observations []wuObservation `json:"observations"`
}

type wuObservation struct {
	StationID    string   `json:"stationID"`
	Neighborhood string   `json:"neighborhood"`
	Metric       wuMetric `json:"metric"`
	Condition    string   `json:"condition"`
	IconCode     int      `json:"iconCode"`
}

type wuMetric struct {
	Temp float64 `json:"temp"`
}

func (r *RemoteAPIAdapter) fetchWU(ctx context.Context, city config.CityConfig) (*weather.WeatherData, error) {
	u, err := url.Parse(r.BaseURL + "/v2/pws/observations/current")
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}

	q := u.Query()
	q.Set("stationId", city.Name)
	q.Set("format", "json")
	q.Set("units", "m")
	q.Set("apiKey", r.apiKey)
	if city.Latitude != 0 || city.Longitude != 0 {
		q.Set("geocode", fmt.Sprintf("%f,%f", city.Latitude, city.Longitude))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("WU API error (status %d): %s", resp.StatusCode, string(body))
	}

	var wu wuResponse
	if err := json.Unmarshal(body, &wu); err != nil {
		return nil, fmt.Errorf("parse WU response: %w", err)
	}

	if len(wu.Observations) == 0 {
		return nil, fmt.Errorf("WU response missing observation data")
	}

	obs := wu.Observations[0]
	now := time.Now().UTC()

	// Load the city's timezone for local time calculation.
	loc, err := time.LoadLocation(city.Timezone)
	if err != nil {
		loc = time.UTC
	}

	temp := int(math.Round(obs.Metric.Temp))
	return &weather.WeatherData{
		CityName:    city.Name,
		Region:      city.Region,
		Temperature: temp,
		Description: obs.Condition,
		IconCode:    mapWUConditionToIcon(obs.IconCode),
		LocalTime:   now.In(loc),
		FetchedAt:   now,
	}, nil
}

func (r *RemoteAPIAdapter) testWU(ctx context.Context) error {
	u, err := url.Parse(r.BaseURL + "/v2/pws/observations/current")
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	q := u.Query()
	q.Set("stationId", "KLAX") // well-known station for testing
	q.Set("format", "json")
	q.Set("units", "m")
	q.Set("apiKey", r.apiKey)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API key")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API test failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// mapWUConditionToIcon maps Weather Underground icon codes to internal icon codes.
// WU uses numeric icon codes; we map common ranges to our internal set.
func mapWUConditionToIcon(code int) string {
	switch {
	case code == 0 || code == 1 || code == 2:
		return weather.IconStorm
	case code >= 3 && code <= 4:
		return weather.IconStorm
	case code >= 5 && code <= 8:
		return weather.IconSnow
	case code >= 9 && code <= 12:
		return weather.IconRain
	case code >= 13 && code <= 18:
		return weather.IconSnow
	case code >= 19 && code <= 22:
		return weather.IconFog
	case code >= 23 && code <= 24:
		return weather.IconCloudy // windy
	case code >= 25 && code <= 26:
		return weather.IconCloudy
	case code >= 27 && code <= 30:
		return weather.IconPartlyCloudy
	case code >= 31 && code <= 34:
		return weather.IconClear
	case code >= 35 && code <= 40:
		return weather.IconRain
	case code >= 41 && code <= 43:
		return weather.IconSnow
	case code == 44:
		return weather.IconPartlyCloudy
	case code >= 45 && code <= 47:
		return weather.IconStorm
	default:
		return weather.IconCloudy
	}
}
