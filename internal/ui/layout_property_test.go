package ui

import (
	"testing"

	"pgregory.net/rapid"
)

// **Feature: windows-weather-widget, Property 7: Widget layout dimensions**
// **Validates: Requirements 1.3, 1.5**

func TestProperty7_WidgetLayoutDimensions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random city count N in {1, 2, 3}
		n := rapid.IntRange(1, 3).Draw(t, "cityCount")

		width, height, slots := CalculateLayout(n)

		// Assert total width = N × 160 dip
		expectedWidth := n * PanelWidth
		if width != expectedWidth {
			t.Fatalf("CalculateLayout(%d): width = %d, want %d", n, width, expectedWidth)
		}

		// Assert height = 185 dip
		if height != PanelHeight {
			t.Fatalf("CalculateLayout(%d): height = %d, want 185", n, height)
		}

		// Assert panel slots = N
		if slots != n {
			t.Fatalf("CalculateLayout(%d): slots = %d, want %d", n, slots, n)
		}
	})
}
