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

// WindSpeedUnit represents the display unit for wind speed values.
type WindSpeedUnit string

const (
	WindSpeedUnitKmh   WindSpeedUnit = "kmh"
	WindSpeedUnitMph   WindSpeedUnit = "mph"
	WindSpeedUnitKnots WindSpeedUnit = "knots"
)

// NormalizeWindSpeedUnit returns the unit unchanged if it is a known value,
// otherwise returns WindSpeedUnitKmh as the safe default.
func NormalizeWindSpeedUnit(u WindSpeedUnit) WindSpeedUnit {
	switch u {
	case WindSpeedUnitKmh, WindSpeedUnitMph, WindSpeedUnitKnots:
		return u
	default:
		return WindSpeedUnitKmh
	}
}

// IconTheme represents the icon set/style to display for weather conditions.
type IconTheme string

const (
	IconThemeNew      IconTheme = "new"      // Modern day/night icons from assets/icons/day and assets/icons/night
	IconThemeOriginal IconTheme = "original" // Original icons from assets/icons/original
)

// NormalizeIconTheme returns the theme unchanged if it is a known value,
// otherwise returns IconThemeNew as the safe default.
func NormalizeIconTheme(t IconTheme) IconTheme {
	switch t {
	case IconThemeNew, IconThemeOriginal:
		return t
	default:
		return IconThemeNew
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
	Opacity         int             `json:"opacity"`                // 25, 50, 75, or 100 (percent)
	NoBackground    bool            `json:"noBackground,omitempty"` // remove panel background (GTK Linux)
	NoBorder        bool            `json:"noBorder,omitempty"`     // remove window decorations (GTK Linux)
	Locale          string          `json:"locale"`
	TemperatureUnit TemperatureUnit `json:"temperatureUnit,omitempty"`
	WindSpeedUnit   WindSpeedUnit   `json:"windSpeedUnit,omitempty"`
	IconTheme       IconTheme       `json:"iconTheme,omitempty"`
	DisplayFields   *DisplayFields   `json:"displayFields,omitempty"`
	PollutionFields *PollutionFields `json:"pollutionFields,omitempty"`
	APIConfig       *APIConfig       `json:"apiConfig,omitempty"`
	DatabaseConfig  *DatabaseConfig  `json:"databaseConfig,omitempty"`

	// FontSizeCityTime controls the font size (px) for the city name and time row.
	// Defaults to 14px (city) / 16px (time) when 0.
	FontSizeCityTime int `json:"fontSizeCityTime,omitempty"`
	// FontSizeTempIcon controls the font size (px) for the temperature value.
	// Defaults to 32px when 0.
	FontSizeTempIcon int `json:"fontSizeTempIcon,omitempty"`
	// FontSizeConditions controls the font size (px) for weather conditions
	// (description, humidity, wind, and all rows below temperature).
	// Defaults to 10px when 0.
	FontSizeConditions int `json:"fontSizeConditions,omitempty"`
}

// DisplayFields controls which elements are visible on each city panel.
// All fields default to true (show everything).
type DisplayFields struct {
	ShowCity     bool `json:"showCity"`
	ShowIcon     bool `json:"showIcon"`
	ShowTemp     bool `json:"showTemp"`
	ShowDesc     bool `json:"showDesc"`
	ShowHumidity bool `json:"showHumidity"`
	ShowWind     bool `json:"showWind"` // wind speed + wind direction together
	ShowTime     bool `json:"showTime"`
	ShowDate     bool `json:"showDate"`
	ShowWindGust bool `json:"showWindGust"`
	ShowDewPoint bool `json:"showDewPoint"`
	ShowPressure bool `json:"showPressure"`
	ShowUVIndex  bool `json:"showUVIndex"`
}

// DefaultDisplayFields returns a DisplayFields with all elements visible.
func DefaultDisplayFields() *DisplayFields {
	return &DisplayFields{
		ShowCity:     true,
		ShowIcon:     true,
		ShowTemp:     true,
		ShowDesc:     true,
		ShowHumidity: true,
		ShowWind:     true,
		ShowTime:     true,
		ShowDate:     true,
		ShowWindGust: false,
		ShowDewPoint: false,
		ShowPressure: false,
		ShowUVIndex:  false,
	}
}

// GetDisplayFields returns the config's DisplayFields, or defaults if nil.
func (c *Config) GetDisplayFields() *DisplayFields {
	if c.DisplayFields != nil {
		return c.DisplayFields
	}
	return DefaultDisplayFields()
}

// PollutionFields controls which air quality and pollution metrics are visible.
type PollutionFields struct {
	ShowCO   bool `json:"showCO"`
	ShowNO   bool `json:"showNO"`
	ShowNO2  bool `json:"showNO2"`
	ShowO3   bool `json:"showO3"`
	ShowSO2  bool `json:"showSO2"`
	ShowNH3  bool `json:"showNH3"`
	ShowPM25 bool `json:"showPM25"`
	ShowPM10 bool `json:"showPM10"`
	ShowAQI  bool `json:"showAQI,omitempty"`
}

// DefaultPollutionFields returns a PollutionFields with all metrics initially off.
func DefaultPollutionFields() *PollutionFields {
	return &PollutionFields{
		ShowCO:   false,
		ShowNO:   false,
		ShowNO2:  false,
		ShowO3:   false,
		ShowSO2:  false,
		ShowNH3:  false,
		ShowPM25: false,
		ShowPM10: false,
		ShowAQI:  false,
	}
}

// GetPollutionFields returns the config's PollutionFields, or defaults if nil.
func (c *Config) GetPollutionFields() *PollutionFields {
	if c.PollutionFields != nil {
		return c.PollutionFields
	}
	return DefaultPollutionFields()
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

const (
	// MaxCitiesFree is the maximum number of cities allowed in Free mode (OpenWeatherMap / default).
	MaxCitiesFree = 3
	// MaxCitiesPro is the maximum number of cities allowed in Pro mode (EasyWeatherWidget gateway).
	MaxCitiesPro = 5
)

// IsPro returns true if the config uses EasyWeatherWidget Pro with a valid API key.
func (c *Config) IsPro() bool {
	return c.APIConfig != nil && c.APIConfig.Provider == "easyweatherwidget" && c.APIConfig.APIKey != ""
}

// MaxCities returns the maximum number of cities allowed based on the active tier.
func (c *Config) MaxCities() int {
	if c.IsPro() {
		return MaxCitiesPro
	}
	return MaxCitiesFree
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
		DataSource:         DataSourceRemoteAPI,
		Cities:             DefaultCities(),
		RefreshInterval:    120,
		CornerPosition:     "bottom-right",
		Opacity:            100,
		Locale:             "en-GB",
		TemperatureUnit:    TemperatureUnitCelsius,
		WindSpeedUnit:      WindSpeedUnitKmh,
		IconTheme:          IconThemeNew,
		FontSizeCityTime:   14,
		FontSizeTempIcon:   32,
		FontSizeConditions: 10,
		APIConfig: &APIConfig{
			Provider: "easyweatherwidget",
		},
	}
}

// GetFontSizeCityTime returns the city/time font size, falling back to the
// default 14px when the stored value is zero (e.g. older config files).
func (c *Config) GetFontSizeCityTime() int {
	if c.FontSizeCityTime <= 0 {
		return 14
	}
	return c.FontSizeCityTime
}

// GetFontSizeTempIcon returns the temperature font size, defaulting to 32px.
func (c *Config) GetFontSizeTempIcon() int {
	if c.FontSizeTempIcon <= 0 {
		return 32
	}
	return c.FontSizeTempIcon
}

// GetFontSizeConditions returns the conditions font size, defaulting to 10px.
func (c *Config) GetFontSizeConditions() int {
	if c.FontSizeConditions <= 0 {
		return 10
	}
	return c.FontSizeConditions
}
