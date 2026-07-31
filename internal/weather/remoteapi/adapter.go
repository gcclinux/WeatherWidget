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
	"strings"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"
)

const (
	defaultOWMBaseURL = "https://api.openweathermap.org"
	defaultWUBaseURL  = "https://api.weather.com"
	defaultEWWBaseURL = "https://weather-gateway-ricardo.web.app"
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
	switch provider {
	case "weatherunderground":
		baseURL = defaultWUBaseURL
	case "easyweatherwidget":
		baseURL = defaultEWWBaseURL
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
	case "easyweatherwidget":
		return r.fetchEWW(ctx, city)
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
	case "easyweatherwidget":
		return r.testEWW(ctx)
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
		Temp     float64 `json:"temp"`
		Humidity int     `json:"humidity"`
	} `json:"main"`
	Wind struct {
		Speed float64 `json:"speed"` // m/s
		Deg   int     `json:"deg"`   // degrees
	} `json:"wind"`
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

	// OWM returns wind speed in m/s; convert to km/h.
	windSpeedKmh := owm.Wind.Speed * 3.6

	return &weather.WeatherData{
		CityName:      city.Name,
		Region:        city.Region,
		Temperature:   temp,
		Description:   owm.Weather[0].Description,
		IconCode:      mapOWMConditionToIcon(owm.Weather[0].ID),
		LocalTime:     localTime,
		FetchedAt:     now,
		Humidity:      owm.Main.Humidity,
		WindSpeed:     windSpeedKmh,
		WindDirection: owm.Wind.Deg,
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
	case id == 502 || id == 503 || id == 504:
		return weather.IconHeavyRain
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

// wuResponse represents the relevant fields from the Weather Underground PWS API.
type wuResponse struct {
	Observations []wuObservation `json:"observations"`
}

type wuObservation struct {
	StationID    string   `json:"stationID"`
	Neighborhood string   `json:"neighborhood"`
	Humidity     float64  `json:"humidity"`
	WindDir      int      `json:"winddir"`
	Metric       wuMetric `json:"metric"`
}

type wuMetric struct {
	Temp       float64 `json:"temp"`
	WindSpeed  float64 `json:"windSpeed"`
	WindGust   float64 `json:"windGust"`
	PrecipRate float64 `json:"precipRate"`
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

	if resp.StatusCode == http.StatusNoContent {
		return nil, fmt.Errorf("WU station %q not found or has no recent data (status 204) — ensure city Name is a valid PWS station ID (e.g. KLAX, IWARSAW123)", city.Name)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("WU API error (status %d): %s", resp.StatusCode, string(body))
	}

	log.Printf("WU raw response for %s: %s", city.Name, string(body))

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
	icon, description := deriveWUCondition(obs)
	return &weather.WeatherData{
		CityName:      city.Name,
		Region:        city.Region,
		Temperature:   temp,
		Description:   description,
		IconCode:      icon,
		LocalTime:     now.In(loc),
		FetchedAt:     now,
		Humidity:      int(obs.Humidity),
		WindSpeed:     obs.Metric.WindSpeed,
		WindDirection: obs.WindDir,
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
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API test failed (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// deriveWUCondition infers a human-readable description and icon from PWS sensor
// readings, since the PWS current-conditions endpoint does not return a condition
// string or icon code.
func deriveWUCondition(obs wuObservation) (icon, description string) {
	switch {
	case obs.Metric.PrecipRate > 5:
		return weather.IconHeavyRain, "Heavy Rain"
	case obs.Metric.PrecipRate > 0:
		return weather.IconRain, "Rain"
	case obs.Metric.WindSpeed > 50:
		return weather.IconCloudy, "Very Windy"
	case obs.Metric.WindSpeed > 30:
		return weather.IconCloudy, "Windy"
	case obs.Humidity > 90:
		return weather.IconFog, "Foggy"
	case obs.Humidity > 70:
		return weather.IconCloudy, "Cloudy"
	case obs.Humidity > 50:
		return weather.IconPartlyCloudy, "Partly Cloudy"
	default:
		return weather.IconClear, "Clear"
	}
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

// --- EasyWeatherWidget implementation ---

// ewwResponse represents the relevant fields from the EasyWeatherWidget API.
type ewwResponse struct {
	Temp         float64 `json:"Temp"`
	Neighborhood string  `json:"Neighborhood"`
	Country      string  `json:"Country"`
	FreeText     string  `json:"FreeText"`
	ObsTimeLocal string  `json:"ObsTimeLocal"`
	Humidity     int     `json:"Humidity"`
	WindSpeed    float64 `json:"WindSpeed"`
	WindDir      int     `json:"WindDir"`
}

func (r *RemoteAPIAdapter) fetchEWW(ctx context.Context, city config.CityConfig) (*weather.WeatherData, error) {
	// When no API key is set, only default cities are allowed.
	if r.apiKey == "" {
		if !config.IsDefaultCity(city) {
			return nil, fmt.Errorf("no API key configured — only default cities are available without a license")
		}
	}

	key := r.apiKey
	if key == "" {
		key = "free"
	}
	reqURL := fmt.Sprintf("%s/api/v1/weather/key=%s/%s,%s", r.BaseURL, key, city.Name, city.Region)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	log.Printf("fetching weather from EWW for %s,%s...", city.Name, city.Region)
	resp, err := r.client.Do(req)
	if err != nil {
		log.Printf("EWW network error for %s: %v", city.Name, err)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("EWW API error for %s: status %d, body: %s", city.Name, resp.StatusCode, string(body))
		return nil, fmt.Errorf("EWW API error (status %d): %s", resp.StatusCode, string(body))
	}

	var eww ewwResponse
	if err := json.Unmarshal(body, &eww); err != nil {
		return nil, fmt.Errorf("parse EWW response: %w", err)
	}

	now := time.Now().UTC()

	// Parse ObsTimeLocal as the local observation time.
	localTime := now
	if eww.ObsTimeLocal != "" {
		if parsed, err := time.Parse("2006-01-02 15:04:05", eww.ObsTimeLocal); err == nil {
			localTime = parsed
		}
	}

	temp := int(math.Round(eww.Temp))
	log.Printf("successfully fetched weather for %s from EWW: %d°C, %s", city.Name, temp, eww.FreeText)

	return &weather.WeatherData{
		CityName:      city.Name,
		Region:        city.Region,
		Temperature:   temp,
		Description:   eww.FreeText,
		IconCode:      mapEWWFreeTextToIcon(eww.FreeText),
		LocalTime:     localTime,
		FetchedAt:     now,
		Humidity:      eww.Humidity,
		WindSpeed:     eww.WindSpeed,
		WindDirection: eww.WindDir,
	}, nil
}

func (r *RemoteAPIAdapter) testEWW(ctx context.Context) error {
	key := r.apiKey
	if key == "" {
		// In free mode (no key), use "free" as the key and test with a default city.
		key = "free"
	}
	reqURL := fmt.Sprintf("%s/api/v1/weather/key=%s/London,GB", r.BaseURL, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
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

// mapEWWFreeTextToIcon maps EasyWeatherWidget FreeText descriptions to internal icon codes.
// It performs case-insensitive keyword matching with the following priority order:
// storm/thunder → snow → rain/drizzle → fog/mist/haze → cloud → clear → default partly_cloudy.
func mapEWWFreeTextToIcon(freeText string) string {
	lower := strings.ToLower(freeText)
	switch {
	case strings.Contains(lower, "storm"), strings.Contains(lower, "thunder"):
		return weather.IconStorm
	case strings.Contains(lower, "snow"):
		return weather.IconSnow
	case strings.Contains(lower, "heavy") && strings.Contains(lower, "rain"):
		return weather.IconHeavyRain
	case strings.Contains(lower, "rain"), strings.Contains(lower, "drizzle"):
		return weather.IconRain
	case strings.Contains(lower, "fog"), strings.Contains(lower, "mist"), strings.Contains(lower, "haze"):
		return weather.IconFog
	case strings.Contains(lower, "cloud"):
		return weather.IconCloudy
	case strings.Contains(lower, "clear"):
		return weather.IconClear
	default:
		return weather.IconPartlyCloudy
	}
}
