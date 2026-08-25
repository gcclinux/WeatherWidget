package weather

import (
	"fmt"
	"testing"
	"time"

	"weatherwidget/assets"
	"weatherwidget/internal/config"

	"pgregory.net/rapid"
)

// **Feature: windows-weather-widget, Property 8: Weather condition to icon mapping is total**
// **Validates: Requirements 2.1**

func verifyAssetExists(t *rapid.T, iconPath string) {
	for _, ext := range []string{".png", ".gif"} {
		f, err := assets.Icons.Open(fmt.Sprintf("icons/%s%s", iconPath, ext))
		if err == nil {
			f.Close()
			return
		}
	}
	t.Fatalf("Embedded asset not found for icon path %q", iconPath)
}

func TestProperty8_MapConditionToIcon_ValidCodes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random valid icon code from AllIconCodes
		idx := rapid.IntRange(0, len(AllIconCodes)-1).Draw(t, "iconIndex")
		code := AllIconCodes[idx]

		// Generate a random hour to test both day and night paths.
		hour := rapid.IntRange(0, 23).Draw(t, "hour")
		localTime := time.Date(2024, 6, 15, hour, 0, 0, 0, time.UTC)

		// Test default MapConditionToIcon
		result := MapConditionToIcon(code, localTime)
		if result == "" {
			t.Fatalf("MapConditionToIcon(%q) returned empty string", code)
		}
		verifyAssetExists(t, result)

		// Test MapConditionToIconWithTheme for both New and Original themes
		themeIdx := rapid.IntRange(0, 1).Draw(t, "themeIndex")
		theme := config.IconThemeNew
		if themeIdx == 1 {
			theme = config.IconThemeOriginal
		}

		themedResult := MapConditionToIconWithTheme(code, localTime, theme)
		if themedResult == "" {
			t.Fatalf("MapConditionToIconWithTheme(%q, %v) returned empty string", code, theme)
		}
		verifyAssetExists(t, themedResult)
	})
}

func TestProperty8_MapConditionToIcon_ArbitraryCodes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random arbitrary string (not necessarily a valid code)
		code := rapid.String().Draw(t, "arbitraryCode")

		// Generate a random hour to test both day and night paths.
		hour := rapid.IntRange(0, 23).Draw(t, "hour")
		localTime := time.Date(2024, 6, 15, hour, 0, 0, 0, time.UTC)

		themeIdx := rapid.IntRange(0, 1).Draw(t, "themeIndex")
		theme := config.IconThemeNew
		if themeIdx == 1 {
			theme = config.IconThemeOriginal
		}

		result := MapConditionToIconWithTheme(code, localTime, theme)
		if result == "" {
			t.Fatalf("MapConditionToIconWithTheme(%q, %v) returned empty string", code, theme)
		}
		verifyAssetExists(t, result)
	})
}
