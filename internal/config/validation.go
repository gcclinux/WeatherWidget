package config

import (
	"fmt"

	"weatherwidget/internal/i18n"
)

// allowedCornerPositions defines the valid corner positions for the widget.
var allowedCornerPositions = map[string]bool{
	"top-left":     true,
	"top-right":    true,
	"bottom-left":  true,
	"bottom-right": true,
}

// allowedProviders defines the valid remote API providers.
var allowedProviders = map[string]bool{
	"openweathermap":    true,
	"easyweatherwidget": true,
}

// availableLocaleCodes returns the set of valid locale codes from the embedded locale files.
func availableLocaleCodes() map[string]bool {
	codes := make(map[string]bool)
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		// If we can't load locales, at least allow the default.
		codes["en-GB"] = true
		return codes
	}
	for _, loc := range lm.AvailableLocales() {
		codes[loc.Code] = true
	}
	return codes
}

// Validate checks the given Config and returns a slice of ValidationError
// for each field that violates its constraint. Returns nil when all
// constraints are satisfied.
// The t parameter is an optional translation function; pass nil to use default English messages.
func Validate(cfg *Config, t TranslateFunc) []ValidationError {
	var errs []ValidationError

	// Cities length 1–5
	if len(cfg.Cities) < 1 || len(cfg.Cities) > 5 {
		tmpl := translate(t, "validation.cities.count", "must contain 1 to 5 cities, got %d")
		errs = append(errs, ValidationError{
			Field:   "cities",
			Message: fmt.Sprintf(tmpl, len(cfg.Cities)),
		})
	}

	// Refresh interval — provider-dependent for remote_api, 1–60 otherwise
	if cfg.DataSource == DataSourceRemoteAPI && cfg.APIConfig != nil {
		switch cfg.APIConfig.Provider {
		case "openweathermap":
			if cfg.RefreshInterval < 120 {
				errs = append(errs, ValidationError{
					Field:   "refreshInterval",
					Message: translate(t, "validation.refreshInterval.min.owm", "must be at least 120 for openweathermap"),
				})
			}
		case "easyweatherwidget":
			if cfg.RefreshInterval < 10 {
				errs = append(errs, ValidationError{
					Field:   "refreshInterval",
					Message: translate(t, "validation.refreshInterval.min.eww", "must be at least 10 for easyweatherwidget"),
				})
			}
		}
		if cfg.RefreshInterval > 120 {
			errs = append(errs, ValidationError{
				Field:   "refreshInterval",
				Message: translate(t, "validation.refreshInterval.max", "must be at most 120"),
			})
		}
	} else {
		if cfg.RefreshInterval < 1 || cfg.RefreshInterval > 60 {
			tmpl := translate(t, "validation.refreshInterval.range", "must be between 1 and 60, got %d")
			errs = append(errs, ValidationError{
				Field:   "refreshInterval",
				Message: fmt.Sprintf(tmpl, cfg.RefreshInterval),
			})
		}
	}

	// Corner position
	if !allowedCornerPositions[cfg.CornerPosition] {
		tmpl := translate(t, "validation.cornerPosition.invalid", "must be one of top-left, top-right, bottom-left, bottom-right, got %q")
		errs = append(errs, ValidationError{
			Field:   "cornerPosition",
			Message: fmt.Sprintf(tmpl, cfg.CornerPosition),
		})
	}

	// Locale validation
	errs = append(errs, ValidateLocale(cfg.Locale, availableLocaleCodes(), t)...)

	// Data-source-specific validation
	switch cfg.DataSource {
	case DataSourceRemoteAPI:
		errs = append(errs, validateAPIConfig(cfg.APIConfig, t)...)
	case DataSourceLocalDatabase:
		errs = append(errs, validateDatabaseConfig(cfg.DatabaseConfig, t)...)
	}

	// Per-city validation
	for i, city := range cfg.Cities {
		errs = append(errs, validateCity(city, i, t)...)
	}

	return errs
}

func validateAPIConfig(api *APIConfig, t TranslateFunc) []ValidationError {
	var errs []ValidationError
	if api == nil {
		errs = append(errs, ValidationError{
			Field:   "apiConfig",
			Message: translate(t, "validation.apiConfig.required", "required when dataSource is remote_api"),
		})
		return errs
	}
	if api.APIKey == "" {
		errs = append(errs, ValidationError{
			Field:   "apiConfig.apiKey",
			Message: translate(t, "validation.apiConfig.apiKey.empty", "must not be empty"),
		})
	}
	if !allowedProviders[api.Provider] {
		tmpl := translate(t, "validation.apiConfig.provider.invalid", "must be openweathermap or easyweatherwidget, got %q")
		errs = append(errs, ValidationError{
			Field:   "apiConfig.provider",
			Message: fmt.Sprintf(tmpl, api.Provider),
		})
	}
	return errs
}

func validateDatabaseConfig(db *DatabaseConfig, t TranslateFunc) []ValidationError {
	var errs []ValidationError
	if db == nil {
		errs = append(errs, ValidationError{
			Field:   "databaseConfig",
			Message: translate(t, "validation.dbConfig.required", "required when dataSource is local_database"),
		})
		return errs
	}
	if db.Host == "" {
		errs = append(errs, ValidationError{
			Field:   "databaseConfig.host",
			Message: translate(t, "validation.dbConfig.host.empty", "must not be empty"),
		})
	}
	if db.Port < 1 || db.Port > 65535 {
		tmpl := translate(t, "validation.dbConfig.port.range", "must be between 1 and 65535, got %d")
		errs = append(errs, ValidationError{
			Field:   "databaseConfig.port",
			Message: fmt.Sprintf(tmpl, db.Port),
		})
	}
	if db.DBName == "" {
		errs = append(errs, ValidationError{
			Field:   "databaseConfig.dbName",
			Message: translate(t, "validation.dbConfig.dbName.empty", "must not be empty"),
		})
	}
	if db.Username == "" {
		errs = append(errs, ValidationError{
			Field:   "databaseConfig.username",
			Message: translate(t, "validation.dbConfig.username.empty", "must not be empty"),
		})
	}
	return errs
}

// ValidateLocale checks that the given locale string is one of the valid locale codes.
// Returns a ValidationError slice (empty if valid).
// The t parameter is an optional translation function; pass nil to use default English messages.
func ValidateLocale(locale string, validCodes map[string]bool, t TranslateFunc) []ValidationError {
	if !validCodes[locale] {
		tmpl := translate(t, "validation.locale.invalid", "must be a supported locale, got %q")
		return []ValidationError{{
			Field:   "locale",
			Message: fmt.Sprintf(tmpl, locale),
		}}
	}
	return nil
}

func validateCity(city CityConfig, index int, t TranslateFunc) []ValidationError {
	var errs []ValidationError
	prefix := fmt.Sprintf("cities[%d]", index)

	if city.Name == "" {
		errs = append(errs, ValidationError{
			Field:   prefix + ".name",
			Message: translate(t, "validation.city.name.empty", "must not be empty"),
		})
	}

	// Coordinates are optional — validate only when provided (lat != 0 or lon != 0).
	coordsProvided := city.Latitude != 0 || city.Longitude != 0
	if coordsProvided {
		if city.Latitude < -90 || city.Latitude > 90 {
			tmpl := translate(t, "validation.city.lat.range", "must be between -90 and 90, got %v")
			errs = append(errs, ValidationError{
				Field:   prefix + ".latitude",
				Message: fmt.Sprintf(tmpl, city.Latitude),
			})
		}
		if city.Longitude < -180 || city.Longitude > 180 {
			tmpl := translate(t, "validation.city.lon.range", "must be between -180 and 180, got %v")
			errs = append(errs, ValidationError{
				Field:   prefix + ".longitude",
				Message: fmt.Sprintf(tmpl, city.Longitude),
			})
		}
	}

	return errs
}
