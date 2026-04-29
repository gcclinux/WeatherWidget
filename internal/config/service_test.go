package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNewConfigService_SetsPath(t *testing.T) {
	svc := NewConfigService("/tmp/appdata")
	want := filepath.Join("/tmp/appdata", "WeatherWidget", "config.json")
	if got := svc.ConfigPath(); got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestLoad_MissingFile_ReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	svc := NewConfigService(dir)

	cfg, err := svc.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(cfg, DefaultConfig()) {
		t.Errorf("Load() on missing file did not return DefaultConfig()")
	}
}

func TestLoad_CorruptJSON_ReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	svc := NewConfigService(dir)

	// Create the directory and write garbage
	configDir := filepath.Dir(svc.ConfigPath())
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(svc.ConfigPath(), []byte("{not valid json!!!"), 0o644)

	cfg, err := svc.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(cfg, DefaultConfig()) {
		t.Errorf("Load() on corrupt JSON did not return DefaultConfig()")
	}
}

func TestLoad_EmptyFile_ReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	svc := NewConfigService(dir)

	configDir := filepath.Dir(svc.ConfigPath())
	os.MkdirAll(configDir, 0o755)
	os.WriteFile(svc.ConfigPath(), []byte(""), 0o644)

	cfg, err := svc.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Empty file is not valid JSON, should return default
	if !reflect.DeepEqual(cfg, DefaultConfig()) {
		t.Errorf("Load() on empty file did not return DefaultConfig()")
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	svc := NewConfigService(dir)

	original := &Config{
		DataSource: DataSourceLocalDatabase,
		Cities: []CityConfig{
			{Name: "TestCity", Region: "TC", Timezone: "UTC"},
			{Name: "Other", Region: "OT", Latitude: 10.5, Longitude: -20.3, Timezone: "America/New_York"},
		},
		RefreshInterval: 30,
		CornerPosition:  "top-left",
		Locale:          "en-GB",
		DatabaseConfig: &DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			DBName:   "weather",
			Username: "user",
			Password: "pass",
			Query:    "SELECT * FROM weather WHERE city = $1",
		},
	}

	if err := svc.Save(original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := svc.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(loaded, original) {
		t.Errorf("Round-trip mismatch.\nGot:  %+v\nWant: %+v", loaded, original)
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	// Use a nested path that doesn't exist yet
	svc := NewConfigService(filepath.Join(dir, "nested", "deep"))

	cfg := DefaultConfig()
	if err := svc.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(svc.ConfigPath()); os.IsNotExist(err) {
		t.Error("Save() did not create the config file")
	}
}

func TestSave_WritesValidJSON(t *testing.T) {
	dir := t.TempDir()
	svc := NewConfigService(dir)

	cfg := DefaultConfig()
	if err := svc.Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(svc.ConfigPath())
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}

	var parsed Config
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Saved file is not valid JSON: %v", err)
	}
}

func TestSave_AtomicWrite_NoPartialFile(t *testing.T) {
	dir := t.TempDir()
	svc := NewConfigService(dir)

	// Save initial config
	initial := DefaultConfig()
	if err := svc.Save(initial); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Save again — the original file should be fully replaced
	updated := &Config{
		DataSource:      DataSourceRemoteAPI,
		Cities:          []CityConfig{{Name: "NewCity", Region: "NC", Timezone: "UTC"}},
		RefreshInterval: 120,
		CornerPosition:  "top-right",
		Locale:          "en-GB",
		APIConfig: &APIConfig{
			Provider: "easyweatherwidget",
			APIKey:   "test-key-123",
		},
	}
	if err := svc.Save(updated); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := svc.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(loaded, updated) {
		t.Errorf("After second save, loaded config doesn't match updated config.\nGot:  %+v\nWant: %+v", loaded, updated)
	}
	// Verify the provider survived the round-trip.
	if loaded.APIConfig == nil || loaded.APIConfig.Provider != "easyweatherwidget" {
		t.Errorf("Provider not persisted: got %v, want easyweatherwidget", loaded.APIConfig)
	}
}

func TestSave_NoTempFileLeftBehind(t *testing.T) {
	dir := t.TempDir()
	svc := NewConfigService(dir)

	if err := svc.Save(DefaultConfig()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	configDir := filepath.Dir(svc.ConfigPath())
	entries, err := os.ReadDir(configDir)
	if err != nil {
		t.Fatalf("ReadDir error = %v", err)
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("Temp file left behind: %s", e.Name())
		}
	}
}
