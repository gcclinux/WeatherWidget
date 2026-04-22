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
	IconSnow,
	IconStorm,
	IconFog,
}

// WeatherData holds the current weather information for a single city.
type WeatherData struct {
	CityName    string    `json:"cityName"`
	Region      string    `json:"region"`
	Temperature int       `json:"temperature"` // Celsius
	Description string    `json:"description"` // e.g. "Partial Sunny"
	IconCode    string    `json:"iconCode"`    // Maps to embedded icon asset
	LocalTime   time.Time `json:"localTime"`   // City's local timezone
	FetchedAt   time.Time `json:"fetchedAt"`
}

// WeatherProvider defines the interface for fetching weather data
// from different sources (remote API, local database, etc.).
type WeatherProvider interface {
	FetchWeather(ctx context.Context, city config.CityConfig) (*WeatherData, error)
	TestConnection(ctx context.Context) error
}
