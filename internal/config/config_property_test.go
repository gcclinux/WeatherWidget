package config

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"pgregory.net/rapid"
)

// **Feature: windows-weather-widget, Property 1: Configuration serialization round-trip**
// **Validates: Requirements 3.4, 8.1, 8.2**

// validIANATimezones is a set of known valid IANA timezone strings for generation.
var validIANATimezones = []string{
	"UTC",
	"America/New_York",
	"Europe/London",
	"Asia/Tokyo",
	"America/Sao_Paulo",
	"Australia/Sydney",
	"Europe/Berlin",
	"Asia/Kolkata",
	"America/Los_Angeles",
	"Pacific/Auckland",
	"Africa/Cairo",
	"Asia/Shanghai",
}

// cornerPositions is the set of allowed corner positions.
var cornerPositions = []string{
	"top-left",
	"top-right",
	"bottom-left",
	"bottom-right",
}

// providers is the set of allowed API providers.
var providers = []string{
	"openweathermap",
	"easywetherwidget",
}

// genCityConfig generates a random valid CityConfig.
func genCityConfig(t *rapid.T, label string) CityConfig {
	name := rapid.StringMatching(`[A-Za-z][A-Za-z ]{0,19}`).Draw(t, label+"_name")
	region := rapid.StringMatching(`[A-Z]{1,5}`).Draw(t, label+"_region")
	tzIdx := rapid.IntRange(0, len(validIANATimezones)-1).Draw(t, label+"_tzIdx")
	return CityConfig{
		Name:     name,
		Region:   region,
		Timezone: validIANATimezones[tzIdx],
	}
}

// genConfig generates a random valid Config struct.
func genConfig(t *rapid.T) *Config {
	// Pick data source
	dsIdx := rapid.IntRange(0, 1).Draw(t, "dataSourceIdx")
	var ds DataSourceType
	if dsIdx == 0 {
		ds = DataSourceRemoteAPI
	} else {
		ds = DataSourceLocalDatabase
	}

	// Generate 1-5 cities
	cityCount := rapid.IntRange(1, 5).Draw(t, "cityCount")
	cities := make([]CityConfig, cityCount)
	for i := 0; i < cityCount; i++ {
		cities[i] = genCityConfig(t, "city"+string(rune('0'+i)))
	}

	// Corner position
	cpIdx := rapid.IntRange(0, len(cornerPositions)-1).Draw(t, "cornerPosIdx")
	cornerPos := cornerPositions[cpIdx]

	// Generate source-specific config and provider-appropriate refresh interval
	var refreshInterval int
	var apiCfg *APIConfig
	var dbCfg *DatabaseConfig

	if ds == DataSourceRemoteAPI {
		provIdx := rapid.IntRange(0, len(providers)-1).Draw(t, "providerIdx")
		provider := providers[provIdx]
		apiKey := rapid.StringMatching(`[a-zA-Z0-9]{8,32}`).Draw(t, "apiKey")
		apiCfg = &APIConfig{
			Provider: provider,
			APIKey:   apiKey,
		}
		// Provider-dependent refresh intervals:
		// OWM: exactly 120 (min=120, max=120)
		// EWW: 30–120
		switch provider {
		case "openweathermap":
			refreshInterval = 120
		case "easywetherwidget":
			refreshInterval = rapid.IntRange(30, 120).Draw(t, "refreshInterval")
		}
	} else {
		// Non-remote_api: 1–60
		refreshInterval = rapid.IntRange(1, 60).Draw(t, "refreshInterval")
	}

	cfg := &Config{
		DataSource:      ds,
		Cities:          cities,
		RefreshInterval: refreshInterval,
		CornerPosition:  cornerPos,
		APIConfig:       apiCfg,
		DatabaseConfig:  dbCfg,
	}

	if ds == DataSourceLocalDatabase {
		host := rapid.StringMatching(`[a-z][a-z0-9.]{0,29}`).Draw(t, "dbHost")
		port := rapid.IntRange(1, 65535).Draw(t, "dbPort")
		dbName := rapid.StringMatching(`[a-z][a-z0-9_]{0,19}`).Draw(t, "dbName")
		username := rapid.StringMatching(`[a-z][a-z0-9_]{0,14}`).Draw(t, "dbUsername")
		password := rapid.StringMatching(`[a-zA-Z0-9!@#]{0,20}`).Draw(t, "dbPassword")
		query := rapid.StringMatching(`SELECT .{1,30}`).Draw(t, "dbQuery")
		cfg.DatabaseConfig = &DatabaseConfig{
			Host:     host,
			Port:     port,
			DBName:   dbName,
			Username: username,
			Password: password,
			Query:    query,
		}
	}

	return cfg
}

func TestProperty1_ConfigSerializationRoundTrip(t *testing.T) {
	// Use a counter to give each iteration its own subdirectory
	iteration := 0
	baseDir := t.TempDir()

	rapid.Check(t, func(rt *rapid.T) {
		original := genConfig(rt)

		// Each iteration gets its own subdirectory to avoid conflicts
		tmpDir := filepath.Join(baseDir, fmt.Sprintf("iter_%d", iteration))
		iteration++

		svc := NewConfigService(tmpDir)

		// Save the config
		if err := svc.Save(original); err != nil {
			rt.Fatalf("Save() error: %v", err)
		}

		// Load it back
		loaded, err := svc.Load()
		if err != nil {
			rt.Fatalf("Load() error: %v", err)
		}

		// Assert deep equality
		if !reflect.DeepEqual(original, loaded) {
			rt.Fatalf("Round-trip mismatch.\nOriginal: %+v\nLoaded:   %+v", original, loaded)
		}
	})
}
