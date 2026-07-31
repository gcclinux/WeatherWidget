package config

// TemperatureUnit represents the display unit for temperature values.
type TemperatureUnit string

const (
	TemperatureUnitCelsius    TemperatureUnit = "celsius"
	TemperatureUnitFahrenheit TemperatureUnit = "fahrenheit"
)

// NormalizeTemperatureUnit returns the unit unchanged if it is a known value,
// otherwise returns TemperatureUnitCelsius as the safe default.
func NormalizeTemperatureUnit(u TemperatureUnit) TemperatureUnit {
	switch u {
	case TemperatureUnitCelsius, TemperatureUnitFahrenheit:
		return u
	default:
		return TemperatureUnitCelsius
	}
}

// DataSourceType represents the type of weather data source.
type DataSourceType string

const (
	DataSourceRemoteAPI     DataSourceType = "remote_api"
	DataSourceLocalDatabase DataSourceType = "local_database"
)

// Config holds the complete application configuration.
type Config struct {
	DataSource      DataSourceType  `json:"dataSource"`
	Cities          []CityConfig    `json:"cities"`
	RefreshInterval int             `json:"refreshInterval"` // minutes, 1–60, default 10
	CornerPosition  string          `json:"cornerPosition"`  // "top-left"|"top-right"|"bottom-left"|"bottom-right"
	MonitorIndex    int             `json:"monitorIndex"`    // 0-based monitor index; 0 = primary
	CustomX         *int            `json:"customX,omitempty"`
	CustomY         *int            `json:"customY,omitempty"`
	Opacity         int             `json:"opacity"` // 25, 50, 75, or 100 (percent)
	Locale          string          `json:"locale"`
	TemperatureUnit TemperatureUnit `json:"temperatureUnit,omitempty"`
	DisplayFields   *DisplayFields  `json:"displayFields,omitempty"`
	APIConfig       *APIConfig      `json:"apiConfig,omitempty"`
	DatabaseConfig  *DatabaseConfig `json:"databaseConfig,omitempty"`
}

// DisplayFields controls which elements are visible on each city panel.
// All fields default to true (show everything).
type DisplayFields struct {
	ShowCity      bool `json:"showCity"`
	ShowIcon      bool `json:"showIcon"`
	ShowTemp      bool `json:"showTemp"`
	ShowDesc      bool `json:"showDesc"`
	ShowHumidWind bool `json:"showHumidWind"`
	ShowTime      bool `json:"showTime"`
	ShowDate      bool `json:"showDate"`
}

// DefaultDisplayFields returns a DisplayFields with all elements visible.
func DefaultDisplayFields() *DisplayFields {
	return &DisplayFields{
		ShowCity:      true,
		ShowIcon:      true,
		ShowTemp:      true,
		ShowDesc:      true,
		ShowHumidWind: true,
		ShowTime:      true,
		ShowDate:      true,
	}
}

// GetDisplayFields returns the config's DisplayFields, or defaults if nil.
func (c *Config) GetDisplayFields() *DisplayFields {
	if c.DisplayFields != nil {
		return c.DisplayFields
	}
	return DefaultDisplayFields()
}

// CityConfig holds the configuration for a single city.
type CityConfig struct {
	Name      string  `json:"name"`
	Region    string  `json:"region"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
	Timezone  string  `json:"timezone"`
}

// APIConfig holds the configuration for a remote weather API.
type APIConfig struct {
	Provider string `json:"provider"` // "openweathermap" | "easyweatherwidget"
	APIKey   string `json:"apiKey"`
}

// DatabaseConfig holds the configuration for a local PostgreSQL database.
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	DBName   string `json:"dbName"`
	Username string `json:"username"`
	Password string `json:"password"`
	Query    string `json:"query"`
}

// HasLicense returns true if the config has a valid API key or database config.
// When false, only the default cities will work.
func (c *Config) HasLicense() bool {
	hasAPIKey := c.APIConfig != nil && c.APIConfig.APIKey != ""
	hasDBConfig := c.DatabaseConfig != nil && c.DatabaseConfig.Host != ""
	return hasAPIKey || hasDBConfig
}

// ValidationError represents a single validation failure for a config field.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error implements the error interface for ValidationError.
func (v ValidationError) Error() string {
	return v.Field + ": " + v.Message
}

// DefaultCities returns the built-in cities that work without a license key.
// These cities are always available for free, even without an API key configured.
func DefaultCities() []CityConfig {
	return []CityConfig{
		{
			Name:      "Broxburn",
			Region:    "GB",
			Latitude:  55.934392,
			Longitude: -3.469387,
			Timezone:  "Europe/London",
		},
		{
			Name:      "Holambra",
			Region:    "BR",
			Latitude:  -22.633203,
			Longitude: -47.05453,
			Timezone:  "America/Sao_Paulo",
		},
		{
			Name:      "Ryjewo",
			Region:    "PL",
			Latitude:  53.843497,
			Longitude: 18.96275,
			Timezone:  "Europe/Warsaw",
		},
	}
}

// IsDefaultCity checks whether a city matches one of the built-in default cities.
func IsDefaultCity(city CityConfig) bool {
	for _, dc := range DefaultCities() {
		if dc.Name == city.Name && dc.Region == city.Region {
			return true
		}
	}
	return false
}

// DefaultConfig returns a Config with sensible defaults:
// three default cities (Broxburn, Holambra, Ryjewo), 120-minute refresh,
// bottom-right corner, remote_api data source, and EasyWeatherWidget as the
// default provider (works without a key for the default cities).
func DefaultConfig() *Config {
	return &Config{
		DataSource:      DataSourceRemoteAPI,
		Cities:          DefaultCities(),
		RefreshInterval: 120,
		CornerPosition:  "bottom-right",
		Opacity:         100,
		Locale:          "en-GB",
		TemperatureUnit: TemperatureUnitCelsius,
		APIConfig: &APIConfig{
			Provider: "easyweatherwidget",
		},
	}
}
