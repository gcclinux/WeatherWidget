package config

import "fmt"

// allowedCornerPositions defines the valid corner positions for the widget.
var allowedCornerPositions = map[string]bool{
	"top-left":     true,
	"top-right":    true,
	"bottom-left":  true,
	"bottom-right": true,
}

// allowedProviders defines the valid remote API providers.
var allowedProviders = map[string]bool{
	"openweathermap":   true,
	"easywetherwidget": true,
}

// Validate checks the given Config and returns a slice of ValidationError
// for each field that violates its constraint. Returns nil when all
// constraints are satisfied.
func Validate(cfg *Config) []ValidationError {
	var errs []ValidationError

	// Cities length 1–5
	if len(cfg.Cities) < 1 || len(cfg.Cities) > 5 {
		errs = append(errs, ValidationError{
			Field:   "cities",
			Message: fmt.Sprintf("must contain 1 to 5 cities, got %d", len(cfg.Cities)),
		})
	}

	// Refresh interval — provider-dependent for remote_api, 1–60 otherwise
	if cfg.DataSource == DataSourceRemoteAPI && cfg.APIConfig != nil {
		switch cfg.APIConfig.Provider {
		case "openweathermap":
			if cfg.RefreshInterval < 120 {
				errs = append(errs, ValidationError{
					Field:   "refreshInterval",
					Message: "must be at least 120 for openweathermap",
				})
			}
		case "easywetherwidget":
			if cfg.RefreshInterval < 30 {
				errs = append(errs, ValidationError{
					Field:   "refreshInterval",
					Message: "must be at least 30 for easywetherwidget",
				})
			}
		}
		if cfg.RefreshInterval > 120 {
			errs = append(errs, ValidationError{
				Field:   "refreshInterval",
				Message: "must be at most 120",
			})
		}
	} else {
		if cfg.RefreshInterval < 1 || cfg.RefreshInterval > 60 {
			errs = append(errs, ValidationError{
				Field:   "refreshInterval",
				Message: fmt.Sprintf("must be between 1 and 60, got %d", cfg.RefreshInterval),
			})
		}
	}

	// Corner position
	if !allowedCornerPositions[cfg.CornerPosition] {
		errs = append(errs, ValidationError{
			Field:   "cornerPosition",
			Message: fmt.Sprintf("must be one of top-left, top-right, bottom-left, bottom-right, got %q", cfg.CornerPosition),
		})
	}

	// Data-source-specific validation
	switch cfg.DataSource {
	case DataSourceRemoteAPI:
		errs = append(errs, validateAPIConfig(cfg.APIConfig)...)
	case DataSourceLocalDatabase:
		errs = append(errs, validateDatabaseConfig(cfg.DatabaseConfig)...)
	}

	// Per-city validation
	for i, city := range cfg.Cities {
		errs = append(errs, validateCity(city, i)...)
	}

	return errs
}

func validateAPIConfig(api *APIConfig) []ValidationError {
	var errs []ValidationError
	if api == nil {
		errs = append(errs, ValidationError{
			Field:   "apiConfig",
			Message: "required when dataSource is remote_api",
		})
		return errs
	}
	if api.APIKey == "" {
		errs = append(errs, ValidationError{
			Field:   "apiConfig.apiKey",
			Message: "must not be empty",
		})
	}
	if !allowedProviders[api.Provider] {
		errs = append(errs, ValidationError{
			Field:   "apiConfig.provider",
			Message: fmt.Sprintf("must be openweathermap or easywetherwidget, got %q", api.Provider),
		})
	}
	return errs
}

func validateDatabaseConfig(db *DatabaseConfig) []ValidationError {
	var errs []ValidationError
	if db == nil {
		errs = append(errs, ValidationError{
			Field:   "databaseConfig",
			Message: "required when dataSource is local_database",
		})
		return errs
	}
	if db.Host == "" {
		errs = append(errs, ValidationError{
			Field:   "databaseConfig.host",
			Message: "must not be empty",
		})
	}
	if db.Port < 1 || db.Port > 65535 {
		errs = append(errs, ValidationError{
			Field:   "databaseConfig.port",
			Message: fmt.Sprintf("must be between 1 and 65535, got %d", db.Port),
		})
	}
	if db.DBName == "" {
		errs = append(errs, ValidationError{
			Field:   "databaseConfig.dbName",
			Message: "must not be empty",
		})
	}
	if db.Username == "" {
		errs = append(errs, ValidationError{
			Field:   "databaseConfig.username",
			Message: "must not be empty",
		})
	}
	return errs
}

func validateCity(city CityConfig, index int) []ValidationError {
	var errs []ValidationError
	prefix := fmt.Sprintf("cities[%d]", index)

	if city.Name == "" {
		errs = append(errs, ValidationError{
			Field:   prefix + ".name",
			Message: "must not be empty",
		})
	}

	// Coordinates are optional — validate only when provided (lat != 0 or lon != 0).
	coordsProvided := city.Latitude != 0 || city.Longitude != 0
	if coordsProvided {
		if city.Latitude < -90 || city.Latitude > 90 {
			errs = append(errs, ValidationError{
				Field:   prefix + ".latitude",
				Message: fmt.Sprintf("must be between -90 and 90, got %v", city.Latitude),
			})
		}
		if city.Longitude < -180 || city.Longitude > 180 {
			errs = append(errs, ValidationError{
				Field:   prefix + ".longitude",
				Message: fmt.Sprintf("must be between -180 and 180, got %v", city.Longitude),
			})
		}
	}

	return errs
}
