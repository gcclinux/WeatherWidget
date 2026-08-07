package weather

import (
	"context"
	"time"

	"weatherwidget/internal/config"
)

// Icon code constants representing weather conditions.
// Each maps to an embedded icon asset in assets/icons/.
const (
	IconClear        = "clear"
	IconMoon         = "moon"
	IconPartlyCloudy = "partly_cloudy"
	IconCloudy       = "cloudy"
	IconCloudyMoon   = "cloudy_moon"
	IconRain         = "rain"
	IconHeavyRain    = "heavy_rain"
	IconSnow         = "snow"
	IconStorm        = "storm"
	IconFog          = "fog"
)

// AllIconCodes contains all valid icon code values.
var AllIconCodes = []string{
	IconClear,
	IconMoon,
	IconPartlyCloudy,
	IconCloudy,
	IconCloudyMoon,
	IconRain,
	IconHeavyRain,
	IconSnow,
	IconStorm,
	IconFog,
}

// WeatherData holds the current weather information for a single city.
type WeatherData struct {
	CityName      string    `json:"cityName"`
	Region        string    `json:"region"`
	Temperature   int       `json:"temperature"` // Celsius
	Description   string    `json:"description"` // e.g. "Partial Sunny"
	IconCode      string    `json:"iconCode"`    // Maps to embedded icon asset
	LocalTime     time.Time `json:"localTime"`   // City's local timezone
	FetchedAt     time.Time `json:"fetchedAt"`
	Humidity      int       `json:"humidity"`      // Percentage (0–100)
	WindSpeed     float64   `json:"windSpeed"`     // km/h
	WindDirection int       `json:"windDirection"` // Degrees (0–360)
	WindGust      float64   `json:"windGust"`      // km/h; 0 if not available
	DewPoint      float64   `json:"dewPoint"`      // Celsius; 0 if not available
	Pressure      float64   `json:"pressure"`      // hPa; 0 if not available
	UVIndex       float64   `json:"uvIndex"`       // 0–11+; 0 if not available
}

// WeatherProvider defines the interface for fetching weather data
// from different sources (remote API, local database, etc.).
type WeatherProvider interface {
	FetchWeather(ctx context.Context, city config.CityConfig) (*WeatherData, error)
	TestConnection(ctx context.Context) error
}
