package config

import (
	"fmt"
	"sort"
	"testing"

	"pgregory.net/rapid"
)

// **Feature: windows-weather-widget, Property 2: Configuration validation correctness**
// **Validates: Requirements 1.4, 4.2, 4.3, 5.2, 5.3, 6.2**

// TestProperty2_ValidConfigsProduceNoErrors verifies that for any valid Config
// (all constraints satisfied), Validate returns no errors.
func TestProperty2_ValidConfigsProduceNoErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cfg := genConfig(rt)

		errs := Validate(cfg, nil)
		if len(errs) != 0 {
			var fields []string
			for _, e := range errs {
				fields = append(fields, fmt.Sprintf("%s: %s", e.Field, e.Message))
			}
			rt.Fatalf("expected no validation errors for valid config, got %v", fields)
		}
	})
}

// TestProperty2_InvalidConfigsProduceExactErrors verifies that for any Config
// with specific known violations, Validate returns errors for exactly those fields.
func TestProperty2_InvalidConfigsProduceExactErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cfg, expectedFields := genMaybeInvalidConfig(rt)

		errs := Validate(cfg, nil)

		// Collect actual error fields
		actualFields := make(map[string]bool)
		for _, e := range errs {
			actualFields[e.Field] = true
		}

		// Check every expected field has an error
		for field := range expectedFields {
			if !actualFields[field] {
				rt.Fatalf("expected error for field %q but got none.\nConfig: %+v\nExpected fields: %v\nActual errors: %v",
					field, cfg, sortedKeys(expectedFields), errs)
			}
		}

		// Check no unexpected errors
		for field := range actualFields {
			if !expectedFields[field] {
				rt.Fatalf("unexpected error for field %q.\nConfig: %+v\nExpected fields: %v\nActual errors: %v",
					field, cfg, sortedKeys(expectedFields), errs)
			}
		}
	})
}

// genMaybeInvalidConfig generates a Config that may have violations in any field.
// It returns the config and a set of field names that are expected to have errors.
func genMaybeInvalidConfig(t *rapid.T) (*Config, map[string]bool) {
	expectedErrors := make(map[string]bool)

	// --- Cities count: sometimes invalid ---
	invalidCitiesCount := rapid.Bool().Draw(t, "invalidCitiesCount")
	var cityCount int
	if invalidCitiesCount {
		// Pick 0 or 6-10 cities
		if rapid.Bool().Draw(t, "zeroCities") {
			cityCount = 0
		} else {
			cityCount = rapid.IntRange(6, 10).Draw(t, "tooManyCities")
		}
		expectedErrors["cities"] = true
	} else {
		cityCount = rapid.IntRange(1, 5).Draw(t, "validCityCount")
	}

	// --- Generate cities (each may have violations) ---
	cities := make([]CityConfig, cityCount)
	for i := range cities {
		label := fmt.Sprintf("city%d", i)
		city, cityErrs := genMaybeInvalidCity(t, label, i)
		cities[i] = city
		for k, v := range cityErrs {
			expectedErrors[k] = v
		}
	}

	// --- Corner position: sometimes invalid ---
	invalidCorner := rapid.Bool().Draw(t, "invalidCorner")
	var cornerPos string
	if invalidCorner {
		cornerPos = rapid.StringMatching(`[a-z\-]{1,15}`).Draw(t, "badCorner")
		// Make sure it's not accidentally valid
		if cornerPos == "top-left" || cornerPos == "top-right" || cornerPos == "bottom-left" || cornerPos == "bottom-right" {
			cornerPos = "invalid-corner"
		}
		expectedErrors["cornerPosition"] = true
	} else {
		cpIdx := rapid.IntRange(0, len(cornerPositions)-1).Draw(t, "validCornerIdx")
		cornerPos = cornerPositions[cpIdx]
	}

	// --- Data source and source-specific config ---
	// Generate the data source and its config BEFORE the refresh interval,
	// because for remote_api the valid interval range depends on the provider.
	dsIdx := rapid.IntRange(0, 1).Draw(t, "dsIdx")
	var ds DataSourceType
	var apiCfg *APIConfig
	var dbCfg *DatabaseConfig

	if dsIdx == 0 {
		ds = DataSourceRemoteAPI
		apiCfg, expectedErrors = genMaybeInvalidAPIConfig(t, expectedErrors)
	} else {
		ds = DataSourceLocalDatabase
		dbCfg, expectedErrors = genMaybeInvalidDBConfig(t, expectedErrors)
	}

	// --- Refresh interval: sometimes invalid ---
	// The valid range depends on the data source and provider:
	//   remote_api + non-nil APIConfig + OWM:     exactly 120
	//   remote_api + non-nil APIConfig + EWW:     30–120
	//   remote_api + non-nil APIConfig + invalid: only >120 is checked (no min from switch)
	//   remote_api + nil APIConfig:               falls to else branch → 1–60
	//   local_database:                           1–60
	invalidRefresh := rapid.Bool().Draw(t, "invalidRefresh")
	var refreshInterval int

	isRemoteWithAPI := ds == DataSourceRemoteAPI && apiCfg != nil
	if isRemoteWithAPI {
		provider := apiCfg.Provider
		if invalidRefresh {
			// Generate an interval that violates the provider-specific rules
			switch provider {
			case "openweathermap":
				// Valid is exactly 120; invalid is anything != 120 within a reasonable range
				// Could be too low (0–119) or too high (121+)
				if rapid.Bool().Draw(t, "refreshTooLow") {
					refreshInterval = rapid.IntRange(-100, 119).Draw(t, "lowRefresh")
				} else {
					refreshInterval = rapid.IntRange(121, 300).Draw(t, "highRefresh")
				}
			case "easyweatherwidget":
				// Valid is 30–120; invalid is <30 or >120
				if rapid.Bool().Draw(t, "refreshTooLow") {
					refreshInterval = rapid.IntRange(-100, 29).Draw(t, "lowRefresh")
				} else {
					refreshInterval = rapid.IntRange(121, 300).Draw(t, "highRefresh")
				}
			default:
				// Invalid provider: the switch in Validate won't match any case,
				// so only the >120 check applies. Generate >120 to trigger that.
				refreshInterval = rapid.IntRange(121, 300).Draw(t, "highRefresh")
			}
			expectedErrors["refreshInterval"] = true
		} else {
			// Generate a valid interval for the provider
			switch provider {
			case "openweathermap":
				refreshInterval = 120
			case "easyweatherwidget":
				refreshInterval = rapid.IntRange(30, 120).Draw(t, "validRefresh")
			default:
				// Invalid provider: no min from switch, only >120 check.
				// Valid means <= 120 and >= some reasonable value.
				refreshInterval = rapid.IntRange(1, 120).Draw(t, "validRefresh")
			}
		}
	} else {
		// local_database or remote_api with nil APIConfig → 1–60 range
		if invalidRefresh {
			if rapid.Bool().Draw(t, "refreshTooLow") {
				refreshInterval = rapid.IntRange(-100, 0).Draw(t, "lowRefresh")
			} else {
				refreshInterval = rapid.IntRange(61, 200).Draw(t, "highRefresh")
			}
			expectedErrors["refreshInterval"] = true
		} else {
			refreshInterval = rapid.IntRange(1, 60).Draw(t, "validRefresh")
		}
	}

	cfg := &Config{
		DataSource:      ds,
		Cities:          cities,
		RefreshInterval: refreshInterval,
		CornerPosition:  cornerPos,
		Locale:          validLocales[rapid.IntRange(0, len(validLocales)-1).Draw(t, "localeIdx")],
		APIConfig:       apiCfg,
		DatabaseConfig:  dbCfg,
	}

	return cfg, expectedErrors
}

// genMaybeInvalidCity generates a CityConfig that may have violations.
func genMaybeInvalidCity(t *rapid.T, label string, index int) (CityConfig, map[string]bool) {
	errs := make(map[string]bool)
	prefix := fmt.Sprintf("cities[%d]", index)

	// Name: sometimes empty
	invalidName := rapid.Bool().Draw(t, label+"_invalidName")
	var name string
	if invalidName {
		name = ""
		errs[prefix+".name"] = true
	} else {
		name = rapid.StringMatching(`[A-Za-z][A-Za-z ]{0,19}`).Draw(t, label+"_name")
	}

	region := rapid.StringMatching(`[A-Z]{1,5}`).Draw(t, label+"_region")

	// Coordinates: sometimes provide invalid ones
	provideCoords := rapid.Bool().Draw(t, label+"_provideCoords")
	var lat, lon float64
	if provideCoords {
		invalidLat := rapid.Bool().Draw(t, label+"_invalidLat")
		if invalidLat {
			// Outside -90..90 but not zero
			if rapid.Bool().Draw(t, label+"_latHigh") {
				lat = rapid.Float64Range(90.01, 500).Draw(t, label+"_highLat")
			} else {
				lat = rapid.Float64Range(-500, -90.01).Draw(t, label+"_lowLat")
			}
			errs[prefix+".latitude"] = true
		} else {
			lat = rapid.Float64Range(-90, 90).Draw(t, label+"_validLat")
		}

		invalidLon := rapid.Bool().Draw(t, label+"_invalidLon")
		if invalidLon {
			if rapid.Bool().Draw(t, label+"_lonHigh") {
				lon = rapid.Float64Range(180.01, 500).Draw(t, label+"_highLon")
			} else {
				lon = rapid.Float64Range(-500, -180.01).Draw(t, label+"_lowLon")
			}
			errs[prefix+".longitude"] = true
		} else {
			lon = rapid.Float64Range(-180, 180).Draw(t, label+"_validLon")
		}

		// If both lat and lon ended up as 0, coordinates won't be validated.
		// This is unlikely with float ranges but handle it: if lat==0 && lon==0,
		// remove any coordinate errors since validation skips them.
		if lat == 0 && lon == 0 {
			delete(errs, prefix+".latitude")
			delete(errs, prefix+".longitude")
		}
	}

	return CityConfig{
		Name:      name,
		Region:    region,
		Latitude:  lat,
		Longitude: lon,
	}, errs
}

// genMaybeInvalidAPIConfig generates an APIConfig that may have violations.
func genMaybeInvalidAPIConfig(t *rapid.T, errs map[string]bool) (*APIConfig, map[string]bool) {
	nilAPI := rapid.Bool().Draw(t, "nilAPI")
	if nilAPI {
		errs["apiConfig"] = true
		return nil, errs
	}

	// API key: sometimes empty
	invalidKey := rapid.Bool().Draw(t, "invalidAPIKey")
	var apiKey string
	if invalidKey {
		apiKey = ""
		errs["apiConfig.apiKey"] = true
	} else {
		apiKey = rapid.StringMatching(`[a-zA-Z0-9]{8,32}`).Draw(t, "validAPIKey")
	}

	// Provider: sometimes invalid
	invalidProvider := rapid.Bool().Draw(t, "invalidProvider")
	var provider string
	if invalidProvider {
		provider = rapid.StringMatching(`[a-z]{3,15}`).Draw(t, "badProvider")
		// Ensure the generated string is not accidentally a valid provider
		if provider == "openweathermap" || provider == "easyweatherwidget" {
			provider = "invalidprovider"
		}
		errs["apiConfig.provider"] = true
	} else {
		provIdx := rapid.IntRange(0, len(providers)-1).Draw(t, "validProvIdx")
		provider = providers[provIdx]
	}

	return &APIConfig{
		Provider: provider,
		APIKey:   apiKey,
	}, errs
}

// genMaybeInvalidDBConfig generates a DatabaseConfig that may have violations.
func genMaybeInvalidDBConfig(t *rapid.T, errs map[string]bool) (*DatabaseConfig, map[string]bool) {
	nilDB := rapid.Bool().Draw(t, "nilDB")
	if nilDB {
		errs["databaseConfig"] = true
		return nil, errs
	}

	// Host: sometimes empty
	invalidHost := rapid.Bool().Draw(t, "invalidHost")
	var host string
	if invalidHost {
		host = ""
		errs["databaseConfig.host"] = true
	} else {
		host = rapid.StringMatching(`[a-z][a-z0-9.]{0,29}`).Draw(t, "validHost")
	}

	// Port: sometimes invalid
	invalidPort := rapid.Bool().Draw(t, "invalidPort")
	var port int
	if invalidPort {
		if rapid.Bool().Draw(t, "portTooLow") {
			port = rapid.IntRange(-100, 0).Draw(t, "lowPort")
		} else {
			port = rapid.IntRange(65536, 100000).Draw(t, "highPort")
		}
		errs["databaseConfig.port"] = true
	} else {
		port = rapid.IntRange(1, 65535).Draw(t, "validPort")
	}

	// DBName: sometimes empty
	invalidDBName := rapid.Bool().Draw(t, "invalidDBName")
	var dbName string
	if invalidDBName {
		dbName = ""
		errs["databaseConfig.dbName"] = true
	} else {
		dbName = rapid.StringMatching(`[a-z][a-z0-9_]{0,19}`).Draw(t, "validDBName")
	}

	// Username: sometimes empty
	invalidUsername := rapid.Bool().Draw(t, "invalidUsername")
	var username string
	if invalidUsername {
		username = ""
		errs["databaseConfig.username"] = true
	} else {
		username = rapid.StringMatching(`[a-z][a-z0-9_]{0,14}`).Draw(t, "validUsername")
	}

	password := rapid.StringMatching(`[a-zA-Z0-9]{0,20}`).Draw(t, "dbPassword")
	query := rapid.StringMatching(`SELECT .{1,20}`).Draw(t, "dbQuery")

	return &DatabaseConfig{
		Host:     host,
		Port:     port,
		DBName:   dbName,
		Username: username,
		Password: password,
		Query:    query,
	}, errs
}

// sortedKeys returns sorted keys from a map for deterministic output.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// **Feature: easy-wether-widget-provider, Property 5: Provider-Dependent Interval Validation**
// **Validates: Requirements 6.6, 6.7**

// TestProperty5_OWM_IntervalBelow120_ReturnsError verifies that for any config
// with DataSource=remote_api, Provider="openweathermap", and RefreshInterval < 120,
// Validate returns a refreshInterval error.
func TestProperty5_OWM_IntervalBelow120_ReturnsError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cfg := genValidRemoteAPIConfig(rt, "openweathermap")
		// Override interval to be below 120 (range: 1–119)
		cfg.RefreshInterval = rapid.IntRange(1, 119).Draw(rt, "owmLowInterval")

		errs := Validate(cfg, nil)

		if !hasFieldError(errs, "refreshInterval") {
			rt.Fatalf("expected refreshInterval error for OWM with interval %d, got errors: %v",
				cfg.RefreshInterval, errs)
		}
	})
}

// TestProperty5_EWW_IntervalBelow30_ReturnsError verifies that for any config
// with DataSource=remote_api, Provider="easyweatherwidget", and RefreshInterval < 30,
// Validate returns a refreshInterval error.
func TestProperty5_EWW_IntervalBelow30_ReturnsError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cfg := genValidRemoteAPIConfig(rt, "easyweatherwidget")
		// Override interval to be below 30 (range: 1–29)
		cfg.RefreshInterval = rapid.IntRange(1, 29).Draw(rt, "ewwLowInterval")

		errs := Validate(cfg, nil)

		if !hasFieldError(errs, "refreshInterval") {
			rt.Fatalf("expected refreshInterval error for EWW with interval %d, got errors: %v",
				cfg.RefreshInterval, errs)
		}
	})
}

// TestProperty5_EWW_Interval30to120_NoRefreshError verifies that for any config
// with DataSource=remote_api, Provider="easyweatherwidget", and RefreshInterval
// between 30 and 120 (inclusive), Validate does NOT return a refreshInterval error.
func TestProperty5_EWW_Interval30to120_NoRefreshError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cfg := genValidRemoteAPIConfig(rt, "easyweatherwidget")
		// Override interval to be in the valid range 30–120
		cfg.RefreshInterval = rapid.IntRange(30, 120).Draw(rt, "ewwValidInterval")

		errs := Validate(cfg, nil)

		if hasFieldError(errs, "refreshInterval") {
			rt.Fatalf("unexpected refreshInterval error for EWW with interval %d, got errors: %v",
				cfg.RefreshInterval, errs)
		}
	})
}

// genValidRemoteAPIConfig generates a Config that is fully valid except the
// RefreshInterval, which the caller is expected to override. The config uses
// DataSource=remote_api with the given provider.
func genValidRemoteAPIConfig(rt *rapid.T, provider string) *Config {
	// Generate 1–5 valid cities
	cityCount := rapid.IntRange(1, 5).Draw(rt, "cityCount")
	cities := make([]CityConfig, cityCount)
	for i := 0; i < cityCount; i++ {
		cities[i] = genCityConfig(rt, fmt.Sprintf("city%d", i))
	}

	// Valid corner position
	cpIdx := rapid.IntRange(0, len(cornerPositions)-1).Draw(rt, "cornerPosIdx")

	// Valid API key
	apiKey := rapid.StringMatching(`[a-zA-Z0-9]{8,32}`).Draw(rt, "apiKey")

	// Set a provider-appropriate default interval (will be overridden by caller)
	var refreshInterval int
	switch provider {
	case "openweathermap":
		refreshInterval = 120
	case "easyweatherwidget":
		refreshInterval = 30
	}

	return &Config{
		DataSource:      DataSourceRemoteAPI,
		Cities:          cities,
		RefreshInterval: refreshInterval,
		CornerPosition:  cornerPositions[cpIdx],
		Opacity:         100,
		Locale:          "en-GB",
		APIConfig: &APIConfig{
			Provider: provider,
			APIKey:   apiKey,
		},
	}
}

// hasFieldError checks whether any ValidationError in the slice has the given field name.
func hasFieldError(errs []ValidationError, field string) bool {
	for _, e := range errs {
		if e.Field == field {
			return true
		}
	}
	return false
}

// **Feature: i18n-localization, Property 3: Locale validation accepts valid and rejects invalid**
// **Validates: Requirements 3.4, 3.5**

// TestProperty3_LocaleValidation_AcceptsValid verifies that for any available locale code,
// ValidateLocale returns no errors.
func TestProperty3_LocaleValidation_AcceptsValid(t *testing.T) {
	validCodes := availableLocaleCodes()

	// Convert to slice for rapid sampling
	codeSlice := make([]string, 0, len(validCodes))
	for code := range validCodes {
		codeSlice = append(codeSlice, code)
	}

	rapid.Check(t, func(rt *rapid.T) {
		idx := rapid.IntRange(0, len(codeSlice)-1).Draw(rt, "localeIdx")
		locale := codeSlice[idx]

		errs := ValidateLocale(locale, validCodes, nil)
		if len(errs) != 0 {
			rt.Fatalf("expected no errors for valid locale %q, got %v", locale, errs)
		}
	})
}

// TestProperty3_LocaleValidation_RejectsInvalid verifies that for any string that is NOT
// one of the available locale codes, ValidateLocale returns a validation error.
func TestProperty3_LocaleValidation_RejectsInvalid(t *testing.T) {
	validCodes := availableLocaleCodes()

	rapid.Check(t, func(rt *rapid.T) {
		locale := rapid.StringMatching(`[a-zA-Z\-]{0,10}`).Draw(rt, "randomLocale")

		// Skip if accidentally generated a valid locale
		if validCodes[locale] {
			return
		}

		errs := ValidateLocale(locale, validCodes, nil)
		if len(errs) == 0 {
			rt.Fatalf("expected validation error for invalid locale %q, got none", locale)
		}
		if errs[0].Field != "locale" {
			rt.Fatalf("expected error field %q, got %q", "locale", errs[0].Field)
		}
	})
}

// **Feature: i18n-localization, Property 7: Translated validation errors**
// **Validates: Requirements 7.1**

// TestProperty7_ValidationErrorsAreHumanReadable verifies that for any invalid configuration,
// the validation engine produces error messages that are non-empty, human-readable strings
// (not raw message keys).
func TestProperty7_ValidationErrorsAreHumanReadable(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cfg, expectedFields := genMaybeInvalidConfig(rt)

		// Only test configs that actually have expected errors
		if len(expectedFields) == 0 {
			return
		}

		errs := Validate(cfg, nil)

		for _, e := range errs {
			// Error message must be non-empty
			if e.Message == "" {
				rt.Fatalf("validation error for field %q has empty message", e.Field)
			}

			// Error message must not look like a raw message key
			// (message keys follow the pattern "section.subsection.key")
			if isMessageKey(e.Message) {
				rt.Fatalf("validation error for field %q appears to be a raw message key: %q",
					e.Field, e.Message)
			}

			// Error message must be a human-readable string (contains spaces or descriptive text)
			if len(e.Message) < 3 {
				rt.Fatalf("validation error for field %q has suspiciously short message: %q",
					e.Field, e.Message)
			}
		}
	})
}

// isMessageKey checks if a string looks like a raw i18n message key
// (e.g. "validation.locale.invalid" rather than "must be a supported locale").
func isMessageKey(s string) bool {
	// Message keys are dot-separated identifiers with no spaces
	if len(s) == 0 {
		return false
	}
	// If it contains spaces, it's likely a human-readable message
	for _, c := range s {
		if c == ' ' {
			return false
		}
	}
	// Check if it looks like a dotted key path (at least one dot)
	dotCount := 0
	for _, c := range s {
		if c == '.' {
			dotCount++
		}
	}
	return dotCount >= 2
}
