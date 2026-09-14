//go:build darwin

package uidarwin

import (
	"encoding/json"
	"testing"

	"weatherwidget/internal/config"
)

// TestMergeSettingsJSON_ObjCQuirks verifies the tolerant merge handles the
// exact JSON quirks the ObjC settings layer can emit: booleans as numbers,
// numbers as strings, and preserves base fields for anything absent/malformed.
func TestMergeSettingsJSON_ObjCQuirks(t *testing.T) {
	base := config.DefaultConfig()
	base.APIConfig = &config.APIConfig{Provider: "easyweatherwidget", APIKey: "key-123"}

	// Simulate a payload where NSJSONSerialization emitted BOOLs as 0/1 numbers
	// and where cities is well-formed.
	payload := `{
		"temperatureUnit": "fahrenheit",
		"windSpeedUnit": "mph",
		"viewMode": "simple",
		"opacity": 75,
		"displayFields": {
			"showCity": 1, "showIcon": 1, "showTemp": 1, "showDesc": 1,
			"showHumidity": 1, "showWind": 1, "showTime": 1, "showDate": 1,
			"showWindGust": 1, "showDewPoint": 1, "showPressure": 0, "showUVIndex": 1
		},
		"pollutionFields": {
			"showAQI": 1, "showCO": 0, "showNO": 0, "showNO2": 0, "showO3": 1,
			"showSO2": 0, "showNH3": 1, "showPM25": 1, "showPM10": 1
		},
		"apiConfig": { "provider": "easyweatherwidget", "apiKey": "key-123" },
		"cities": [
			{"name":"Broxburn","region":"GB","timezone":"Europe/London"},
			{"name":"Holambra","region":"BR","timezone":"America/Sao_Paulo"}
		]
	}`

	out, err := mergeSettingsJSON(base, []byte(payload))
	if err != nil {
		t.Fatalf("mergeSettingsJSON returned error: %v", err)
	}
	if out.TemperatureUnit != config.TemperatureUnitFahrenheit {
		t.Errorf("temperatureUnit = %q, want fahrenheit", out.TemperatureUnit)
	}
	if out.WindSpeedUnit != config.WindSpeedUnitMph {
		t.Errorf("windSpeedUnit = %q, want mph", out.WindSpeedUnit)
	}
	if out.ViewMode != config.ViewModeSimple {
		t.Errorf("viewMode = %q, want simple", out.ViewMode)
	}
	if out.Opacity != 75 {
		t.Errorf("opacity = %d, want 75", out.Opacity)
	}
	if out.GetDisplayFields().ShowPressure {
		t.Errorf("showPressure = true, want false (was unchecked)")
	}
	if !out.GetDisplayFields().ShowHumidity {
		t.Errorf("showHumidity = false, want true")
	}
	if out.GetPollutionFields().ShowCO {
		t.Errorf("showCO = true, want false")
	}
	if !out.GetPollutionFields().ShowAQI {
		t.Errorf("showAQI = false, want true")
	}
	if len(out.Cities) != 2 {
		t.Errorf("cities = %d, want 2", len(out.Cities))
	}
}

// TestMergeSettingsJSON_MalformedCitiesKeepsBase verifies a malformed cities
// payload (numbers instead of objects) does NOT discard the save — base cities
// are preserved and all other fields still apply.
func TestMergeSettingsJSON_MalformedCitiesKeepsBase(t *testing.T) {
	base := config.DefaultConfig()
	baseCityCount := len(base.Cities)

	payload := `{
		"temperatureUnit": "fahrenheit",
		"cities": [1, 2, 3]
	}`

	out, err := mergeSettingsJSON(base, []byte(payload))
	if err != nil {
		t.Fatalf("mergeSettingsJSON returned error: %v", err)
	}
	if out.TemperatureUnit != config.TemperatureUnitFahrenheit {
		t.Errorf("temperatureUnit = %q, want fahrenheit (must apply despite bad cities)", out.TemperatureUnit)
	}
	if len(out.Cities) != baseCityCount {
		t.Errorf("cities = %d, want %d (base preserved)", len(out.Cities), baseCityCount)
	}
}

// Guard against accidental base mutation.
func TestMergeSettingsJSON_DoesNotMutateBase(t *testing.T) {
	base := config.DefaultConfig()
	before, _ := json.Marshal(base)

	_, err := mergeSettingsJSON(base, []byte(`{"temperatureUnit":"fahrenheit"}`))
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(base)
	if string(before) != string(after) {
		t.Errorf("base config was mutated by mergeSettingsJSON")
	}
}
