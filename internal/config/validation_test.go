package config

import (
	"testing"
)

// helper to check if a specific field appears in the errors slice.
func hasError(errs []ValidationError, field string) bool {
	for _, e := range errs {
		if e.Field == field {
			return true
		}
	}
	return false
}

func validRemoteConfig() *Config {
	return &Config{
		DataSource:      DataSourceRemoteAPI,
		Cities:          []CityConfig{{Name: "Berlin", Region: "BE", Timezone: "Europe/Berlin"}},
		RefreshInterval: 10,
		CornerPosition:  "bottom-right",
		APIConfig:       &APIConfig{Provider: "openweathermap", APIKey: "key123"},
	}
}

func validDatabaseConfig() *Config {
	return &Config{
		DataSource:      DataSourceLocalDatabase,
		Cities:          []CityConfig{{Name: "Berlin", Region: "BE", Timezone: "Europe/Berlin"}},
		RefreshInterval: 10,
		CornerPosition:  "bottom-right",
		DatabaseConfig:  &DatabaseConfig{Host: "localhost", Port: 5432, DBName: "weather", Username: "user"},
	}
}

func TestValidate_ValidRemoteConfig(t *testing.T) {
	errs := Validate(validRemoteConfig())
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid remote config, got %v", errs)
	}
}

func TestValidate_ValidDatabaseConfig(t *testing.T) {
	errs := Validate(validDatabaseConfig())
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid database config, got %v", errs)
	}
}

func TestValidate_CitiesLength(t *testing.T) {
	// Zero cities
	cfg := validRemoteConfig()
	cfg.Cities = nil
	errs := Validate(cfg)
	if !hasError(errs, "cities") {
		t.Error("expected error for empty cities")
	}

	// Six cities
	cfg.Cities = []CityConfig{
		{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}, {Name: "E"}, {Name: "F"},
	}
	errs = Validate(cfg)
	if !hasError(errs, "cities") {
		t.Error("expected error for 6 cities")
	}
}

func TestValidate_RefreshInterval(t *testing.T) {
	cfg := validRemoteConfig()

	cfg.RefreshInterval = 0
	errs := Validate(cfg)
	if !hasError(errs, "refreshInterval") {
		t.Error("expected error for refreshInterval=0")
	}

	cfg.RefreshInterval = 61
	errs = Validate(cfg)
	if !hasError(errs, "refreshInterval") {
		t.Error("expected error for refreshInterval=61")
	}

	cfg.RefreshInterval = 1
	errs = Validate(cfg)
	if hasError(errs, "refreshInterval") {
		t.Error("unexpected error for refreshInterval=1")
	}

	cfg.RefreshInterval = 60
	errs = Validate(cfg)
	if hasError(errs, "refreshInterval") {
		t.Error("unexpected error for refreshInterval=60")
	}
}

func TestValidate_CornerPosition(t *testing.T) {
	cfg := validRemoteConfig()

	for _, pos := range []string{"top-left", "top-right", "bottom-left", "bottom-right"} {
		cfg.CornerPosition = pos
		errs := Validate(cfg)
		if hasError(errs, "cornerPosition") {
			t.Errorf("unexpected error for cornerPosition=%q", pos)
		}
	}

	cfg.CornerPosition = "center"
	errs := Validate(cfg)
	if !hasError(errs, "cornerPosition") {
		t.Error("expected error for invalid cornerPosition")
	}
}

func TestValidate_APIConfig_EmptyKey(t *testing.T) {
	cfg := validRemoteConfig()
	cfg.APIConfig.APIKey = ""
	errs := Validate(cfg)
	if !hasError(errs, "apiConfig.apiKey") {
		t.Error("expected error for empty API key")
	}
}

func TestValidate_APIConfig_InvalidProvider(t *testing.T) {
	cfg := validRemoteConfig()
	cfg.APIConfig.Provider = "invalid"
	errs := Validate(cfg)
	if !hasError(errs, "apiConfig.provider") {
		t.Error("expected error for invalid provider")
	}
}

func TestValidate_APIConfig_Nil(t *testing.T) {
	cfg := validRemoteConfig()
	cfg.APIConfig = nil
	errs := Validate(cfg)
	if !hasError(errs, "apiConfig") {
		t.Error("expected error for nil apiConfig when dataSource is remote_api")
	}
}

func TestValidate_DatabaseConfig_EmptyFields(t *testing.T) {
	cfg := validDatabaseConfig()
	cfg.DatabaseConfig.Host = ""
	cfg.DatabaseConfig.DBName = ""
	cfg.DatabaseConfig.Username = ""
	errs := Validate(cfg)
	if !hasError(errs, "databaseConfig.host") {
		t.Error("expected error for empty host")
	}
	if !hasError(errs, "databaseConfig.dbName") {
		t.Error("expected error for empty dbName")
	}
	if !hasError(errs, "databaseConfig.username") {
		t.Error("expected error for empty username")
	}
}

func TestValidate_DatabaseConfig_InvalidPort(t *testing.T) {
	cfg := validDatabaseConfig()

	cfg.DatabaseConfig.Port = 0
	errs := Validate(cfg)
	if !hasError(errs, "databaseConfig.port") {
		t.Error("expected error for port=0")
	}

	cfg.DatabaseConfig.Port = 65536
	errs = Validate(cfg)
	if !hasError(errs, "databaseConfig.port") {
		t.Error("expected error for port=65536")
	}
}

func TestValidate_DatabaseConfig_Nil(t *testing.T) {
	cfg := validDatabaseConfig()
	cfg.DatabaseConfig = nil
	errs := Validate(cfg)
	if !hasError(errs, "databaseConfig") {
		t.Error("expected error for nil databaseConfig when dataSource is local_database")
	}
}

func TestValidate_CityEmptyName(t *testing.T) {
	cfg := validRemoteConfig()
	cfg.Cities = []CityConfig{{Name: "", Region: "X"}}
	errs := Validate(cfg)
	if !hasError(errs, "cities[0].name") {
		t.Error("expected error for empty city name")
	}
}

func TestValidate_CityInvalidCoordinates(t *testing.T) {
	cfg := validRemoteConfig()
	cfg.Cities = []CityConfig{{Name: "Test", Latitude: 91, Longitude: 0}}
	errs := Validate(cfg)
	if !hasError(errs, "cities[0].latitude") {
		t.Error("expected error for latitude=91")
	}

	cfg.Cities = []CityConfig{{Name: "Test", Latitude: 0, Longitude: 181}}
	errs = Validate(cfg)
	if !hasError(errs, "cities[0].longitude") {
		t.Error("expected error for longitude=181")
	}
}

func TestValidate_CityValidCoordinates(t *testing.T) {
	cfg := validRemoteConfig()
	cfg.Cities = []CityConfig{{Name: "Test", Latitude: -22.63, Longitude: -47.05}}
	errs := Validate(cfg)
	if hasError(errs, "cities[0].latitude") || hasError(errs, "cities[0].longitude") {
		t.Error("unexpected coordinate errors for valid coordinates")
	}
}

func TestValidate_CityZeroCoordinatesSkipped(t *testing.T) {
	cfg := validRemoteConfig()
	cfg.Cities = []CityConfig{{Name: "Test", Latitude: 0, Longitude: 0}}
	errs := Validate(cfg)
	if hasError(errs, "cities[0].latitude") || hasError(errs, "cities[0].longitude") {
		t.Error("should not validate coordinates when both are zero")
	}
}
