package app

import (
	"testing"

	"weatherwidget/internal/config"
)

// baseConfig returns a minimal valid config for use in routing tests.
func baseConfig() *config.Config {
	return &config.Config{
		DataSource: config.DataSourceRemoteAPI,
		Cities: []config.CityConfig{
			{Name: "London", Region: "England", Timezone: "Europe/London"},
		},
		RefreshInterval: 10,
		CornerPosition:  "bottom-right",
		TemperatureUnit: config.TemperatureUnitCelsius,
		APIConfig:       &config.APIConfig{Provider: "openweathermap", APIKey: "key"},
	}
}

// TestDetermineSettingsSaveAction_UnitOnlyChange verifies that when only the
// TemperatureUnit changes (no city or provider change), the action is Rerender
// (not Fetch). This satisfies Requirements 2.4 and 4.4: a unit-only save must
// re-render panels from cache without triggering a new weather fetch.
func TestDetermineSettingsSaveAction_UnitOnlyChange(t *testing.T) {
	oldCfg := baseConfig()
	newCfg := baseConfig()
	newCfg.TemperatureUnit = config.TemperatureUnitFahrenheit

	action := determineSettingsSaveAction(oldCfg, newCfg, false)

	if action != settingsSaveActionRerender {
		t.Errorf("expected settingsSaveActionRerender for unit-only change, got %v", action)
	}
}

// TestDetermineSettingsSaveAction_UnitOnlyChange_CelsiusToFahrenheit is a
// symmetric check: switching from Fahrenheit back to Celsius also triggers
// Rerender, not Fetch.
func TestDetermineSettingsSaveAction_UnitOnlyChange_FahrenheitToCelsius(t *testing.T) {
	oldCfg := baseConfig()
	oldCfg.TemperatureUnit = config.TemperatureUnitFahrenheit
	newCfg := baseConfig()
	newCfg.TemperatureUnit = config.TemperatureUnitCelsius

	action := determineSettingsSaveAction(oldCfg, newCfg, false)

	if action != settingsSaveActionRerender {
		t.Errorf("expected settingsSaveActionRerender for unit-only change (F→C), got %v", action)
	}
}

// TestDetermineSettingsSaveAction_CityListChange verifies that when the city
// list changes (regardless of unit), the action is Fetch. This satisfies
// Requirement 2.4: city changes must trigger a fresh weather fetch.
func TestDetermineSettingsSaveAction_CityListChange(t *testing.T) {
	oldCfg := baseConfig()
	newCfg := baseConfig()
	newCfg.Cities = []config.CityConfig{
		{Name: "Paris", Region: "Île-de-France", Timezone: "Europe/Paris"},
	}

	action := determineSettingsSaveAction(oldCfg, newCfg, false)

	if action != settingsSaveActionFetch {
		t.Errorf("expected settingsSaveActionFetch for city change, got %v", action)
	}
}

// TestDetermineSettingsSaveAction_CityListChangeWithUnitChange verifies that
// when both the city list and the unit change, the action is still Fetch
// (city change takes precedence over unit-only fast path).
func TestDetermineSettingsSaveAction_CityListChangeWithUnitChange(t *testing.T) {
	oldCfg := baseConfig()
	newCfg := baseConfig()
	newCfg.TemperatureUnit = config.TemperatureUnitFahrenheit
	newCfg.Cities = []config.CityConfig{
		{Name: "Tokyo", Region: "Tokyo", Timezone: "Asia/Tokyo"},
	}

	action := determineSettingsSaveAction(oldCfg, newCfg, false)

	if action != settingsSaveActionFetch {
		t.Errorf("expected settingsSaveActionFetch when both city and unit change, got %v", action)
	}
}

// TestDetermineSettingsSaveAction_ProviderChange verifies that when the
// provider changes (even with a unit change), the action is Fetch.
func TestDetermineSettingsSaveAction_ProviderChange(t *testing.T) {
	oldCfg := baseConfig()
	newCfg := baseConfig()
	newCfg.TemperatureUnit = config.TemperatureUnitFahrenheit

	// providerChanged = true simulates a data source or credential change.
	action := determineSettingsSaveAction(oldCfg, newCfg, true)

	if action != settingsSaveActionFetch {
		t.Errorf("expected settingsSaveActionFetch when provider changed, got %v", action)
	}
}

// TestDetermineSettingsSaveAction_NoChange verifies that when nothing changes,
// the action is Fetch (the normal path; no unit change means no fast path).
func TestDetermineSettingsSaveAction_NoChange(t *testing.T) {
	oldCfg := baseConfig()
	newCfg := baseConfig()

	action := determineSettingsSaveAction(oldCfg, newCfg, false)

	if action != settingsSaveActionFetch {
		t.Errorf("expected settingsSaveActionFetch when nothing changed, got %v", action)
	}
}

// TestDetermineSettingsSaveAction_CityCountChange verifies that adding a city
// triggers Fetch even when the unit also changes.
func TestDetermineSettingsSaveAction_CityCountChange(t *testing.T) {
	oldCfg := baseConfig()
	newCfg := baseConfig()
	newCfg.TemperatureUnit = config.TemperatureUnitFahrenheit
	newCfg.Cities = append(newCfg.Cities, config.CityConfig{
		Name: "Berlin", Region: "Berlin", Timezone: "Europe/Berlin",
	})

	action := determineSettingsSaveAction(oldCfg, newCfg, false)

	if action != settingsSaveActionFetch {
		t.Errorf("expected settingsSaveActionFetch when city count changes, got %v", action)
	}
}

// TestDetermineSettingsSaveAction_WindSpeedUnitChange verifies that when only the
// WindSpeedUnit changes (no city or provider change), the action is Rerender.
func TestDetermineSettingsSaveAction_WindSpeedUnitChange(t *testing.T) {
	oldCfg := baseConfig()
	newCfg := baseConfig()
	newCfg.WindSpeedUnit = config.WindSpeedUnitMph

	action := determineSettingsSaveAction(oldCfg, newCfg, false)

	if action != settingsSaveActionRerender {
		t.Errorf("expected settingsSaveActionRerender for wind unit-only change, got %v", action)
	}
}

// TestDetermineSettingsSaveAction_IconThemeChange verifies that when only the
// IconTheme changes (no city or provider change), the action is Rerender.
func TestDetermineSettingsSaveAction_IconThemeChange(t *testing.T) {
	oldCfg := baseConfig()
	oldCfg.IconTheme = config.IconThemeNew
	newCfg := baseConfig()
	newCfg.IconTheme = config.IconThemeOriginal

	action := determineSettingsSaveAction(oldCfg, newCfg, false)

	if action != settingsSaveActionRerender {
		t.Errorf("expected settingsSaveActionRerender for icon theme-only change, got %v", action)
	}
}
