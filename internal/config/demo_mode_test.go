package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestLoad_NoAPIKey_LoadsDemoCities verifies that a saved config without an
// API key (empty or whitespace-only, Pro or Free provider) starts with the
// three built-in demo cities instead of the user's custom cities.
func TestLoad_NoAPIKey_LoadsDemoCities(t *testing.T) {
	for _, tc := range []struct{ name, provider, key string }{
		{"pro empty", "easyweatherwidget", ""},
		{"pro whitespace", "easyweatherwidget", "   "},
		{"free empty", "openweathermap", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewConfigService(t.TempDir())
			cfg := DefaultConfig()
			cfg.Cities = []CityConfig{{Name: "Paris", Region: "FR", Latitude: 48.85, Longitude: 2.35, Timezone: "Europe/Paris"}}
			cfg.APIConfig = &APIConfig{Provider: tc.provider, APIKey: tc.key}
			if err := svc.Save(cfg); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			got, err := svc.Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !reflect.DeepEqual(got.Cities, DefaultCities()) {
				t.Errorf("Cities = %+v, want DefaultCities()", got.Cities)
			}
			if got.APIKeyValue() != "" || got.APIConfig.APIKey != "" {
				t.Errorf("APIKey = %q, want empty", got.APIConfig.APIKey)
			}
		})
	}
}

// TestLoad_WithAPIKey_KeepsCities ensures licensed configs are not reset.
func TestLoad_WithAPIKey_KeepsCities(t *testing.T) {
	svc := NewConfigService(t.TempDir())
	cfg := DefaultConfig()
	cities := []CityConfig{{Name: "Paris", Region: "FR", Latitude: 48.85, Longitude: 2.35, Timezone: "Europe/Paris"}}
	cfg.Cities = cities
	cfg.APIConfig = &APIConfig{Provider: "easyweatherwidget", APIKey: "f169fecc-de0c-4de1-b8c8-4ec3701b671c"}
	if err := os.MkdirAll(filepath.Dir(svc.ConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := svc.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := svc.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(got.Cities, cities) {
		t.Errorf("Cities = %+v, want %+v", got.Cities, cities)
	}
}

func TestHasLicense_WhitespaceKeyIsNotLicense(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIConfig.APIKey = " \t "
	if cfg.HasLicense() || cfg.IsPro() {
		t.Error("whitespace-only API key must not count as a license")
	}
}

