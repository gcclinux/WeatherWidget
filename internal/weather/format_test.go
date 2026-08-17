package weather

import (
	"testing"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
)

func TestFormatTemperature(t *testing.T) {
	tests := []struct {
		input    int
		unit     config.TemperatureUnit
		expected string
	}{
		{24, config.TemperatureUnitCelsius, "24°C"},
		{0, config.TemperatureUnitCelsius, "0°C"},
		{-5, config.TemperatureUnitCelsius, "-5°C"},
		{100, config.TemperatureUnitCelsius, "100°C"},
		{-40, config.TemperatureUnitCelsius, "-40°C"},
		// Fahrenheit spot-checks
		{0, config.TemperatureUnitFahrenheit, "32°F"},
		{100, config.TemperatureUnitFahrenheit, "212°F"},
		{-40, config.TemperatureUnitFahrenheit, "-40°F"},
		// Invalid unit defaults to Celsius
		{20, config.TemperatureUnit("invalid"), "20°C"},
	}
	for _, tc := range tests {
		got := FormatTemperature(tc.input, tc.unit)
		if got != tc.expected {
			t.Errorf("FormatTemperature(%d, %q) = %q, want %q", tc.input, tc.unit, got, tc.expected)
		}
	}
}

func TestConvertToFahrenheit(t *testing.T) {
	tests := []struct {
		celsius  int
		expected int
	}{
		{0, 32},
		{100, 212},
		{-40, -40},
		{37, 99},
	}
	for _, tc := range tests {
		got := ConvertToFahrenheit(tc.celsius)
		if got != tc.expected {
			t.Errorf("ConvertToFahrenheit(%d) = %d, want %d", tc.celsius, got, tc.expected)
		}
	}
}

func TestFormatCityRegion(t *testing.T) {
	tests := []struct {
		name, region, expected string
	}{
		{"Holambra", "SP", "Holambra, SP"},
		{"São Paulo", "SP", "São Paulo, SP"},
		{"New York", "NY", "New York, NY"},
	}
	for _, tc := range tests {
		got := FormatCityRegion(tc.name, tc.region)
		if got != tc.expected {
			t.Errorf("FormatCityRegion(%q, %q) = %q, want %q", tc.name, tc.region, got, tc.expected)
		}
	}
}

func TestFormatDateTime(t *testing.T) {
	// 2024-01-05 09:03:07 UTC
	ts := time.Date(2024, 1, 5, 9, 3, 7, 0, time.UTC)

	got := FormatDateTime(ts, "UTC", nil)
	expected := "05/01/2024 - 09:03:07"
	if got != expected {
		t.Errorf("FormatDateTime(UTC) = %q, want %q", got, expected)
	}

	// America/Sao_Paulo is UTC-3
	got = FormatDateTime(ts, "America/Sao_Paulo", nil)
	expected = "05/01/2024 - 06:03:07"
	if got != expected {
		t.Errorf("FormatDateTime(America/Sao_Paulo) = %q, want %q", got, expected)
	}
}

func TestFormatDateTimeZeroPadding(t *testing.T) {
	// Ensure zero-padding for single-digit day, month, hours, minutes, seconds
	ts := time.Date(2024, 3, 2, 1, 5, 8, 0, time.UTC)
	got := FormatDateTime(ts, "UTC", nil)
	expected := "02/03/2024 - 01:05:08"
	if got != expected {
		t.Errorf("FormatDateTime zero-padding = %q, want %q", got, expected)
	}
}

func TestFormatDateTimeInvalidTimezone(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 30, 45, 0, time.UTC)
	got := FormatDateTime(ts, "Invalid/Timezone", nil)
	expected := "15/06/2024 - 12:30:45" // Falls back to UTC
	if got != expected {
		t.Errorf("FormatDateTime(invalid tz) = %q, want %q", got, expected)
	}
}

func TestMapConditionToIcon(t *testing.T) {
	// Use a daytime timestamp so clear maps to itself.
	daytime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// All known codes should map to themselves during the day.
	for _, code := range AllIconCodes {
		got := MapConditionToIcon(code, daytime)
		// "moon" is only produced by the nighttime swap; during the day
		// it still maps to itself because it's a valid code.
		if got != code {
			t.Errorf("MapConditionToIcon(%q, daytime) = %q, want %q", code, got, code)
		}
	}

	// Unknown codes should default to "cloudy"
	unknowns := []string{"unknown", "tornado", "", "hail", "windy"}
	for _, code := range unknowns {
		got := MapConditionToIcon(code, daytime)
		if got != IconCloudy {
			t.Errorf("MapConditionToIcon(%q, daytime) = %q, want %q", code, got, IconCloudy)
		}
	}
}

func TestMapConditionToIcon_NighttimeSwaps(t *testing.T) {
	night := time.Date(2024, 6, 15, 21, 0, 0, 0, time.UTC)
	earlyMorning := time.Date(2024, 6, 15, 3, 0, 0, 0, time.UTC)
	dawn := time.Date(2024, 6, 15, 6, 0, 0, 0, time.UTC)
	dusk := time.Date(2024, 6, 15, 18, 0, 0, 0, time.UTC)

	// ── Clear → Moon at night ────────────────────────────────────────────
	got := MapConditionToIcon(IconClear, night)
	if got != IconMoon {
		t.Errorf("MapConditionToIcon(%q, 21:00) = %q, want %q", IconClear, got, IconMoon)
	}
	got = MapConditionToIcon(IconClear, earlyMorning)
	if got != IconMoon {
		t.Errorf("MapConditionToIcon(%q, 03:00) = %q, want %q", IconClear, got, IconMoon)
	}
	got = MapConditionToIcon(IconClear, dawn)
	if got != IconClear {
		t.Errorf("MapConditionToIcon(%q, 06:00) = %q, want %q", IconClear, got, IconClear)
	}
	got = MapConditionToIcon(IconClear, dusk)
	if got != IconMoon {
		t.Errorf("MapConditionToIcon(%q, 18:00) = %q, want %q", IconClear, got, IconMoon)
	}

	// ── Cloudy → CloudyMoon at night ─────────────────────────────────────
	got = MapConditionToIcon(IconCloudy, night)
	if got != IconCloudyMoon {
		t.Errorf("MapConditionToIcon(%q, 21:00) = %q, want %q", IconCloudy, got, IconCloudyMoon)
	}
	got = MapConditionToIcon(IconCloudy, earlyMorning)
	if got != IconCloudyMoon {
		t.Errorf("MapConditionToIcon(%q, 03:00) = %q, want %q", IconCloudy, got, IconCloudyMoon)
	}
	got = MapConditionToIcon(IconCloudy, dawn)
	if got != IconCloudy {
		t.Errorf("MapConditionToIcon(%q, 06:00) = %q, want %q", IconCloudy, got, IconCloudy)
	}
	got = MapConditionToIcon(IconCloudy, dusk)
	if got != IconCloudyMoon {
		t.Errorf("MapConditionToIcon(%q, 18:00) = %q, want %q", IconCloudy, got, IconCloudyMoon)
	}

	// ── Other codes unaffected by nighttime ──────────────────────────────
	for _, code := range []string{IconRain, IconSnow, IconStorm, IconFog, IconPartlyCloudy} {
		got = MapConditionToIcon(code, night)
		if got != code {
			t.Errorf("MapConditionToIcon(%q, 21:00) = %q, want %q", code, got, code)
		}
	}
}

func TestFormatDescription(t *testing.T) {
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		t.Fatalf("NewLocaleManager error = %v", err)
	}

	tests := []struct {
		desc     string
		locale   string
		expected string
	}{
		{"clear sky", "pt-BR", "Céu limpo"},
		{"overcast clouds", "de-DE", "Bedeckt"},
		{"broken clouds", "es-ES", "Nubes dispersas"},
		{"few clouds", "fr-FR", "Peu de nuages"},
		{"scattered clouds", "it-IT", "Nuvole sparse"},
		{"light rain", "nl-NL", "Lichte regen"},
		{"moderate rain", "pl-PL", "Umiarkowany deszcz"},
		{"heavy intensity rain", "tr-TR", "Kuvvetli yağmurlu"},
		{"very heavy rain", "en-GB", "Very Heavy Rain"},
		// Mixed case input
		{"CLEAR SKY", "pt-BR", "Céu limpo"},
		{"  light rain  ", "en-GB", "Light Rain"},
		// Fallback for unknown condition
		{"unusual hurricane", "pt-BR", "Unusual Hurricane"},
		{"", "pt-BR", ""},
	}

	for _, tc := range tests {
		if tc.locale != "" {
			_ = lm.SetLocale(tc.locale)
		}
		got := FormatDescription(tc.desc, lm)
		if got != tc.expected {
			t.Errorf("FormatDescription(%q, %s) = %q, want %q", tc.desc, tc.locale, got, tc.expected)
		}
	}

	// Test with nil LocaleManager
	if got := FormatDescription("clear sky", nil); got != "Clear Sky" {
		t.Errorf("FormatDescription(clear sky, nil) = %q, want %q", got, "Clear Sky")
	}
}

func TestFormatWindDir(t *testing.T) {
	tests := []struct {
		deg      int
		expected string
	}{
		{0, ""},
		{360, "↑ N"},
		{1, "↑ N"},
		{45, "⬈ NE"},
		{90, "→ E"},
		{135, "⬊ SE"},
		{180, "↓ S"},
		{202, "⬋ SSW"},
		{225, "⬋ SW"},
		{270, "← W"},
		{315, "⬉ NW"},
	}

	for _, tc := range tests {
		got := FormatWindDir(tc.deg)
		if got != tc.expected {
			t.Errorf("FormatWindDir(%d) = %q, want %q", tc.deg, got, tc.expected)
		}
	}
}

func TestFormatWindGustAndDewPoint_Localization(t *testing.T) {
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		t.Fatalf("NewLocaleManager error = %v", err)
	}

	// Nil locale manager fallback
	if got := FormatWindGust(10.5, config.WindSpeedUnitKmh, nil); got != "💨 Gust 10.5 km/h" {
		t.Errorf("FormatWindGust(10.5, config.WindSpeedUnitKmh, nil) = %q, want %q", got, "💨 Gust 10.5 km/h")
	}
	if got := FormatWindGust(10.0, config.WindSpeedUnitMph, nil); got != "💨 Gust 6.2 mph" {
		t.Errorf("FormatWindGust(10.0, config.WindSpeedUnitMph, nil) = %q, want %q", got, "💨 Gust 6.2 mph")
	}
	if got := FormatWindGust(10.0, config.WindSpeedUnitKnots, nil); got != "💨 Gust 5.4 knots" {
		t.Errorf("FormatWindGust(10.0, config.WindSpeedUnitKnots, nil) = %q, want %q", got, "💨 Gust 5.4 knots")
	}
	if got := FormatDewPoint(15.9, nil); got != "💧 Dew 15.9°C" {
		t.Errorf("FormatDewPoint(15.9, nil) = %q, want %q", got, "💧 Dew 15.9°C")
	}

	// Zero value fallback
	if got := FormatWindGust(0, config.WindSpeedUnitKmh, lm); got != "💨 Gust 0.0 km/h" {
		t.Errorf("FormatWindGust(0, config.WindSpeedUnitKmh, lm) = %q, want %q", got, "💨 Gust 0.0 km/h")
	}
	if got := FormatWindGust(0, config.WindSpeedUnitMph, lm); got != "💨 Gust 0.0 mph" {
		t.Errorf("FormatWindGust(0, config.WindSpeedUnitMph, lm) = %q, want %q", got, "💨 Gust 0.0 mph")
	}
	if got := FormatDewPoint(0, lm); got != "" {
		t.Errorf("FormatDewPoint(0, lm) = %q, want %q", got, "")
	}

	// Portuguese (pt-BR)
	_ = lm.SetLocale("pt-BR")
	if got := FormatWindGust(8.9, config.WindSpeedUnitKmh, lm); got != "💨 Rajada 8.9 km/h" {
		t.Errorf("FormatWindGust(pt-BR) = %q, want %q", got, "💨 Rajada 8.9 km/h")
	}
	if got := FormatDewPoint(15.9, lm); got != "💧 Orvalho 15.9°C" {
		t.Errorf("FormatDewPoint(pt-BR) = %q, want %q", got, "💧 Orvalho 15.9°C")
	}

	// Polish (pl-PL)
	_ = lm.SetLocale("pl-PL")
	if got := FormatWindGust(7.4, config.WindSpeedUnitKmh, lm); got != "💨 Poryw 7.4 km/h" {
		t.Errorf("FormatWindGust(pl-PL) = %q, want %q", got, "💨 Poryw 7.4 km/h")
	}
	if got := FormatDewPoint(11.0, lm); got != "💧 Rosa 11.0°C" {
		t.Errorf("FormatDewPoint(pl-PL) = %q, want %q", got, "💧 Rosa 11.0°C")
	}
}

func TestFormatHumidityWind(t *testing.T) {
	tests := []struct {
		humidity  int
		windSpeed float64
		unit      config.WindSpeedUnit
		expected  string
	}{
		{57, 2.6, config.WindSpeedUnitKmh, "💧 57%   💨 2.6 km/h"},
		{57, 10.0, config.WindSpeedUnitMph, "💧 57%   💨 6.2 mph"},
		{57, 10.0, config.WindSpeedUnitKnots, "💧 57%   💨 5.4 knots"},
	}

	for _, tc := range tests {
		got := FormatHumidityWind(tc.humidity, tc.windSpeed, tc.unit)
		if got != tc.expected {
			t.Errorf("FormatHumidityWind(%d, %f, %q) = %q, want %q", tc.humidity, tc.windSpeed, tc.unit, got, tc.expected)
		}
	}
}
