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

		errs := Validate(cfg)
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

		errs := Validate(cfg)

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

	// --- Refresh interval: sometimes invalid ---
	invalidRefresh := rapid.Bool().Draw(t, "invalidRefresh")
	var refreshInterval int
	if invalidRefresh {
		// Pick from outside 1-60
		if rapid.Bool().Draw(t, "refreshTooLow") {
			refreshInterval = rapid.IntRange(-100, 0).Draw(t, "lowRefresh")
		} else {
			refreshInterval = rapid.IntRange(61, 200).Draw(t, "highRefresh")
		}
		expectedErrors["refreshInterval"] = true
	} else {
		refreshInterval = rapid.IntRange(1, 60).Draw(t, "validRefresh")
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

	cfg := &Config{
		DataSource:      ds,
		Cities:          cities,
		RefreshInterval: refreshInterval,
		CornerPosition:  cornerPos,
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
		if provider == "openweathermap" {
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
