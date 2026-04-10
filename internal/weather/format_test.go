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
		got := FormatTemperature(tc.input)
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

	got := FormatDateTime(ts, "UTC")
	expected := "05/01/2024 - 09:03:07"
	if got != expected {
		t.Errorf("FormatDateTime(UTC) = %q, want %q", got, expected)
	}

	// America/Sao_Paulo is UTC-3
	got = FormatDateTime(ts, "America/Sao_Paulo")
	expected = "05/01/2024 - 06:03:07"
	if got != expected {
		t.Errorf("FormatDateTime(America/Sao_Paulo) = %q, want %q", got, expected)
	}
}

func TestFormatDateTimeZeroPadding(t *testing.T) {
	// Ensure zero-padding for single-digit day, month, hours, minutes, seconds
	ts := time.Date(2024, 3, 2, 1, 5, 8, 0, time.UTC)
	got := FormatDateTime(ts, "UTC")
	expected := "02/03/2024 - 01:05:08"
	if got != expected {
		t.Errorf("FormatDateTime zero-padding = %q, want %q", got, expected)
	}
}

func TestFormatDateTimeInvalidTimezone(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 30, 45, 0, time.UTC)
	got := FormatDateTime(ts, "Invalid/Timezone")
	expected := "15/06/2024 - 12:30:45" // Falls back to UTC
	if got != expected {
		t.Errorf("FormatDateTime(invalid tz) = %q, want %q", got, expected)
	}
}

func TestMapConditionToIcon(t *testing.T) {
	// All known codes should map to themselves
	for _, code := range AllIconCodes {
		got := MapConditionToIcon(code)
		if got != code {
			t.Errorf("MapConditionToIcon(%q) = %q, want %q", code, got, code)
		}
	}

	// Unknown codes should default to "cloudy"
	unknowns := []string{"unknown", "tornado", "", "hail", "windy"}
	for _, code := range unknowns {
		got := MapConditionToIcon(code)
		if got != IconCloudy {
			t.Errorf("MapConditionToIcon(%q) = %q, want %q", code, got, IconCloudy)
		}
	}
}
