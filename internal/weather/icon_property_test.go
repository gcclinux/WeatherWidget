package weather

import (
	"fmt"
	"testing"
	"time"

	"weatherwidget/assets"

	"pgregory.net/rapid"
)

// **Feature: windows-weather-widget, Property 8: Weather condition to icon mapping is total**
// **Validates: Requirements 2.1**

func TestProperty8_MapConditionToIcon_ValidCodes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random valid icon code from AllIconCodes
		idx := rapid.IntRange(0, len(AllIconCodes)-1).Draw(t, "iconIndex")
		code := AllIconCodes[idx]

		// Generate a random hour to test both day and night paths.
		hour := rapid.IntRange(0, 23).Draw(t, "hour")
		localTime := time.Date(2024, 6, 15, hour, 0, 0, 0, time.UTC)

		result := MapConditionToIcon(code, localTime)

		// Assert the result is non-empty
		if result == "" {
			t.Fatalf("MapConditionToIcon(%q) returned empty string", code)
		}

		// Assert the result exists in AllIconCodes (valid icon identifier)
		found := false
		for _, valid := range AllIconCodes {
			if result == valid {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("MapConditionToIcon(%q) = %q, which is not in AllIconCodes", code, result)
		}

		// Verify the corresponding icon file exists in the embedded asset set
		path := fmt.Sprintf("icons/%s.png", result)
		f, err := assets.Icons.Open(path)
		if err != nil {
			t.Fatalf("MapConditionToIcon(%q) = %q, but embedded asset %q not found: %v", code, result, path, err)
		}
		f.Close()
	})
}

func TestProperty8_MapConditionToIcon_ArbitraryCodes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random arbitrary string (not necessarily a valid code)
		code := rapid.String().Draw(t, "arbitraryCode")

		// Generate a random hour to test both day and night paths.
		hour := rapid.IntRange(0, 23).Draw(t, "hour")
		localTime := time.Date(2024, 6, 15, hour, 0, 0, 0, time.UTC)

		result := MapConditionToIcon(code, localTime)

		// Assert the result is non-empty
		if result == "" {
			t.Fatalf("MapConditionToIcon(%q) returned empty string", code)
		}

		// Assert the result exists in AllIconCodes (valid icon identifier)
		found := false
		for _, valid := range AllIconCodes {
			if result == valid {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("MapConditionToIcon(%q) = %q, which is not in AllIconCodes", code, result)
		}

		// Verify the corresponding icon file exists in the embedded asset set
		path := fmt.Sprintf("icons/%s.png", result)
		f, err := assets.Icons.Open(path)
		if err != nil {
			t.Fatalf("MapConditionToIcon(%q) = %q, but embedded asset %q not found: %v", code, result, path, err)
		}
		f.Close()
	})
}
