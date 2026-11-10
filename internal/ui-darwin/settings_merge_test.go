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
			"showSO2": 0, "showNH3": 1, "showPM25": 1, "showPM10": 1,
			"tickerProfile": "sensitive"
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
	if out.GetPollutionFields().TickerProfile != config.TickerProfileSensitive {
		t.Errorf("tickerProfile = %q, want sensitive", out.GetPollutionFields().TickerProfile)
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

// TestMergeSettingsJSON_ProfilesAndTickerFields verifies the 3 weather profiles,
// extended display fields, health profile flags, news feed flags, and per-pollutant
// ticker flags are merged properly.
func TestMergeSettingsJSON_ProfilesAndTickerFields(t *testing.T) {
	base := config.DefaultConfig()

	payload := `{
		"displayFields": {
			"gridMode": "today",
			"showSunrise": 1,
			"showSunset": 1,
			"showClouds": 0,
			"showVisibility": 1,
			"showPrecipRate": 1
		},
		"pollutionFields": {
			"showTicker": 1,
			"profileEveryday": 1,
			"profileActive": 1,
			"profileFamily": 0,
			"profileSensitive": 1,
			"profileVentilation": 0,
			"tickerProfile": "custom",
			"tickerAlert": 1,
			"tickerNewsToday": 1,
			"tickerNewsPrecip": 0,
			"tickerNewsTomorrow": 1,
			"tickerWeatherTips": 1,
			"tickerAQI": 1,
			"tickerPM25": 1,
			"tickerPM10": 0,
			"tickerO3": 1,
			"tickerNO2": 1,
			"tickerSO2": 0,
			"tickerCO": 0,
			"tickerNO": 1,
			"tickerNH3": 1
		}
	}`

	out, err := mergeSettingsJSON(base, []byte(payload))
	if err != nil {
		t.Fatalf("mergeSettingsJSON returned error: %v", err)
	}

	df := out.GetDisplayFields()
	if df.GridMode != "today" {
		t.Errorf("gridMode = %q, want today", df.GridMode)
	}
	if !df.ShowSunrise || !df.ShowSunset || df.ShowClouds || !df.ShowVisibility || !df.ShowPrecipRate {
		t.Errorf("displayFields extended flags mismatch: sunrise=%v sunset=%v clouds=%v visibility=%v precipRate=%v",
			df.ShowSunrise, df.ShowSunset, df.ShowClouds, df.ShowVisibility, df.ShowPrecipRate)
	}

	pf := out.GetPollutionFields()
	if !pf.ShowTicker {
		t.Errorf("showTicker = false, want true")
	}
	if !pf.ProfileEveryday || !pf.ProfileActive || pf.ProfileFamily || !pf.ProfileSensitive || pf.ProfileVentilation {
		t.Errorf("health profiles mismatch: everyday=%v active=%v family=%v sensitive=%v ventilation=%v",
			pf.ProfileEveryday, pf.ProfileActive, pf.ProfileFamily, pf.ProfileSensitive, pf.ProfileVentilation)
	}
	if pf.TickerProfile != "custom" {
		t.Errorf("tickerProfile = %q, want custom", pf.TickerProfile)
	}
	profs := pf.SelectedProfiles()
	if len(profs) != 3 {
		t.Fatalf("SelectedProfiles length = %d, want 3", len(profs))
	}
	if !pf.TickerAlert || !pf.TickerNewsToday || pf.TickerNewsPrecip || !pf.TickerNewsTomorrow || !pf.TickerWeatherTips {
		t.Errorf("feed flags mismatch: alert=%v today=%v precip=%v tomorrow=%v tips=%v",
			pf.TickerAlert, pf.TickerNewsToday, pf.TickerNewsPrecip, pf.TickerNewsTomorrow, pf.TickerWeatherTips)
	}
	if !pf.TickerAQI || !pf.TickerPM25 || pf.TickerPM10 || !pf.TickerO3 || !pf.TickerNO2 || pf.TickerSO2 || pf.TickerCO || !pf.TickerNO || !pf.TickerNH3 {
		t.Errorf("ticker pollutants mismatch: aqi=%v pm25=%v pm10=%v o3=%v no2=%v so2=%v co=%v no=%v nh3=%v",
			pf.TickerAQI, pf.TickerPM25, pf.TickerPM10, pf.TickerO3, pf.TickerNO2, pf.TickerSO2, pf.TickerCO, pf.TickerNO, pf.TickerNH3)
	}
}

// TestMergeSettingsJSON_NoBackgroundAndCornerPosition verifies noBackground
// toggle, cornerPosition selection, and custom coordinates clearing.
func TestMergeSettingsJSON_NoBackgroundAndCornerPosition(t *testing.T) {
	base := config.DefaultConfig()
	x, y := 120, 340
	base.CustomX = &x
	base.CustomY = &y
	base.NoBackground = false
	base.CornerPosition = "top-right"

	// 1. Setting cornerPosition and clearing custom coordinates with nulls
	payload1 := `{
		"noBackground": 1,
		"cornerPosition": "bottom-left",
		"customX": null,
		"customY": null
	}`
	out1, err := mergeSettingsJSON(base, []byte(payload1))
	if err != nil {
		t.Fatalf("mergeSettingsJSON returned error: %v", err)
	}
	if !out1.NoBackground {
		t.Errorf("noBackground = false, want true")
	}
	if out1.CornerPosition != "bottom-left" {
		t.Errorf("cornerPosition = %q, want bottom-left", out1.CornerPosition)
	}
	if out1.CustomX != nil || out1.CustomY != nil {
		t.Errorf("customX/Y = (%v, %v), want nil after corner selection", out1.CustomX, out1.CustomY)
	}

	// 2. Setting custom coordinates overrides corner placement
	payload2 := `{
		"noBackground": 0,
		"cornerPosition": "top-left",
		"customX": 450,
		"customY": 250
	}`
	out2, err := mergeSettingsJSON(base, []byte(payload2))
	if err != nil {
		t.Fatalf("mergeSettingsJSON returned error: %v", err)
	}
	if out2.NoBackground {
		t.Errorf("noBackground = true, want false")
	}
	if out2.CornerPosition != "top-left" {
		t.Errorf("cornerPosition = %q, want top-left", out2.CornerPosition)
	}
	if out2.CustomX == nil || *out2.CustomX != 450 || out2.CustomY == nil || *out2.CustomY != 250 {
		t.Errorf("custom coordinates mismatch: got (%v, %v), want (450, 250)", out2.CustomX, out2.CustomY)
	}
}
