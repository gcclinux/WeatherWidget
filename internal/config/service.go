package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigService manages loading, saving, and locating the configuration file.
type ConfigService struct {
	configPath string
}

// NewConfigService creates a ConfigService that stores config at
// {appDataDir}/WeatherWidget/config.json.
func NewConfigService(appDataDir string) *ConfigService {
	return &ConfigService{
		configPath: filepath.Join(appDataDir, "WeatherWidget", "config.json"),
	}
}

// ConfigPath returns the full path to the configuration file.
func (s *ConfigService) ConfigPath() string {
	return s.configPath
}

// Load reads the configuration file and unmarshals it into a Config struct.
// On any error (missing file, corrupt JSON, invalid schema) it returns
// DefaultConfig() without panicking.
func (s *ConfigService) Load() (*Config, error) {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return DefaultConfig(), nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), nil
	}

	// Validate schema conformance: a valid config must have at least one city,
	// a valid refresh interval, and a non-empty corner position.
	if len(cfg.Cities) == 0 || cfg.RefreshInterval == 0 || cfg.CornerPosition == "" {
		return DefaultConfig(), nil
	}

	// Ensure remote_api configs always have an APIConfig with a valid provider
	// so the saved provider choice is never silently dropped.
	if cfg.DataSource == DataSourceRemoteAPI && cfg.APIConfig == nil {
		cfg.APIConfig = &APIConfig{Provider: "openweathermap"}
	}

	// Default locale to en-GB when missing or empty.
	if cfg.Locale == "" {
		cfg.Locale = "en-GB"
	}

	return &cfg, nil
}

// Save marshals the config to indented JSON and writes it atomically by
// writing to a temporary file in the same directory and then renaming.
func (s *ConfigService) Save(cfg *Config) error {
	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	if err := os.Rename(tmpName, s.configPath); err != nil {
		os.Remove(tmpName)
		return err
	}

	return nil
}
