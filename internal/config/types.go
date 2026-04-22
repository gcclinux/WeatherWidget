package config

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
	CustomX         *int            `json:"customX,omitempty"`
	CustomY         *int            `json:"customY,omitempty"`
	Opacity         int             `json:"opacity"` // 25, 50, 75, or 100 (percent)
	APIConfig       *APIConfig      `json:"apiConfig,omitempty"`
	DatabaseConfig  *DatabaseConfig `json:"databaseConfig,omitempty"`
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

// ValidationError represents a single validation failure for a config field.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error implements the error interface for ValidationError.
func (v ValidationError) Error() string {
	return v.Field + ": " + v.Message
}

// DefaultConfig returns a Config with sensible defaults:
// one city (Holambra, SP), 120-minute refresh, bottom-right corner, remote_api data source,
// and OpenWeatherMap as the default provider.
func DefaultConfig() *Config {
	return &Config{
		DataSource: DataSourceRemoteAPI,
		Cities: []CityConfig{
			{
				Name:     "Holambra",
				Region:   "SP",
				Timezone: "America/Sao_Paulo",
			},
		},
		RefreshInterval: 120,
		CornerPosition:  "bottom-right",
		Opacity:         100,
		APIConfig: &APIConfig{
			Provider: "openweathermap",
		},
	}
}
