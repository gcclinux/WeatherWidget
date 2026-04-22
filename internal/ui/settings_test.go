package ui

import "testing"

func TestProviderDisplayToValue(t *testing.T) {
	tests := []struct {
		display string
		value   string
	}{
		{"OpenWeatherMap (Free)", "openweathermap"},
		{"EasyWetherWidget (Pro)", "easywetherwidget"},
	}
	for _, tt := range tests {
		t.Run(tt.display, func(t *testing.T) {
			got, ok := providerDisplayToValue[tt.display]
			if !ok {
				t.Fatalf("providerDisplayToValue missing key %q", tt.display)
			}
			if got != tt.value {
				t.Errorf("providerDisplayToValue[%q] = %q, want %q", tt.display, got, tt.value)
			}
		})
	}
}

func TestProviderValueToDisplay(t *testing.T) {
	tests := []struct {
		value   string
		display string
	}{
		{"openweathermap", "OpenWeatherMap (Free)"},
		{"easywetherwidget", "EasyWetherWidget (Pro)"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, ok := providerValueToDisplay[tt.value]
			if !ok {
				t.Fatalf("providerValueToDisplay missing key %q", tt.value)
			}
			if got != tt.display {
				t.Errorf("providerValueToDisplay[%q] = %q, want %q", tt.value, got, tt.display)
			}
		})
	}
}

func TestProviderMappingRoundTrip(t *testing.T) {
	// display → value → display
	for display, value := range providerDisplayToValue {
		t.Run(display, func(t *testing.T) {
			backToDisplay, ok := providerValueToDisplay[value]
			if !ok {
				t.Fatalf("providerValueToDisplay missing key %q (mapped from display %q)", value, display)
			}
			if backToDisplay != display {
				t.Errorf("round-trip failed: %q → %q → %q, want %q", display, value, backToDisplay, display)
			}
		})
	}
}
