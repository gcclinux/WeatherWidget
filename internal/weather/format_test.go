package weather

import (
	"testing"
	"time"
)

func TestFormatTemperature(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{24, "24°C"},
		{0, "0°C"},
		{-5, "-5°C"},
		{100, "100°C"},
		{-40, "-40°C"},
	}
	for _, tc := range tests {
		got := FormatTemperature(tc.input, nil)
		if got != tc.expected {
			t.Errorf("FormatTemperature(%d) = %q, want %q", tc.input, got, tc.expected)
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
