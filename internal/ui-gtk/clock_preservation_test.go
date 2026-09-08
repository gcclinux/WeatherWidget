//go:build linux

package uitk

import (
	"regexp"
	"testing"
	"time"

	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"

	"pgregory.net/rapid"
)

// **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.6**
// Preservation Property Tests for GTK Clock Crash Fix
//
// These tests capture the baseline behavior of the clock formatting functions
// that must remain unchanged after the bug fix. They verify:
// - Time formatting produces correct output for various timezones (3.1, 3.3)
// - Date formatting produces correct output for various timezones (3.2, 3.3)
// - Empty timezone defaults to UTC (3.4)
// - Invalid timezone falls back to UTC (3.6)

// validTimezones is a representative set of valid IANA timezone strings used
// to verify clock formatting behavior across different zones.
var clockTimezones = []string{
	"UTC",
	"America/New_York",
	"Europe/London",
	"Asia/Tokyo",
	"America/Sao_Paulo",
	"Australia/Sydney",
	"Europe/Berlin",
	"Asia/Kolkata",
	"America/Los_Angeles",
	"Pacific/Auckland",
	"Africa/Cairo",
	"Asia/Shanghai",
	"Europe/Paris",
	"America/Chicago",
}

// invalidTimezones is a set of timezone strings that do not correspond to valid
// IANA zones and should trigger fallback behavior.
var invalidTimezones = []string{
	"Invalid/Timezone",
	"NotARealZone",
	"Fake/Place",
	"",
	"Mars/Colony",
}

// TestPreservation_TimeFormatConsistencyAcrossTimezones verifies that FormatTime
// produces consistent time strings that correctly reflect the time in each timezone.
//
// **Validates: Requirements 3.1, 3.3**
func TestPreservation_TimeFormatConsistencyAcrossTimezones(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random time
		year := rapid.IntRange(2020, 2030).Draw(rt, "year")
		month := rapid.IntRange(1, 12).Draw(rt, "month")
		day := rapid.IntRange(1, 28).Draw(rt, "day")
		hour := rapid.IntRange(0, 23).Draw(rt, "hour")
		minute := rapid.IntRange(0, 59).Draw(rt, "minute")
		second := rapid.IntRange(0, 59).Draw(rt, "second")

		ts := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)

		// Pick a valid timezone
		tzIdx := rapid.IntRange(0, len(clockTimezones)-1).Draw(rt, "tzIndex")
		tz := clockTimezones[tzIdx]

		// Call FormatTime with nil locale manager (default format)
		result := weather.FormatTime(ts, tz, nil)

		// Verify the result matches the expected format: HH:MM:SS
		timePattern := regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`)
		if !timePattern.MatchString(result) {
			rt.Fatalf("FormatTime(%v, %q) = %q does not match HH:MM:SS pattern", ts, tz, result)
		}

		// Verify the time is correctly converted to the timezone
		loc, err := time.LoadLocation(tz)
		if err != nil {
			rt.Fatalf("failed to load timezone %q: %v", tz, err)
		}
		localTime := ts.In(loc)
		expected := localTime.Format("15:04:05")
		if result != expected {
			rt.Fatalf("FormatTime(%v, %q) = %q, want %q", ts, tz, result, expected)
		}
	})
}

// TestPreservation_DateFormatConsistencyAcrossTimezones verifies that FormatDate
// produces consistent date strings that correctly reflect the date in each timezone.
//
// **Validates: Requirements 3.2, 3.3**
func TestPreservation_DateFormatConsistencyAcrossTimezones(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random time
		year := rapid.IntRange(2020, 2030).Draw(rt, "year")
		month := rapid.IntRange(1, 12).Draw(rt, "month")
		day := rapid.IntRange(1, 28).Draw(rt, "day")
		hour := rapid.IntRange(0, 23).Draw(rt, "hour")
		minute := rapid.IntRange(0, 59).Draw(rt, "minute")
		second := rapid.IntRange(0, 59).Draw(rt, "second")

		ts := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)

		// Pick a valid timezone
		tzIdx := rapid.IntRange(0, len(clockTimezones)-1).Draw(rt, "tzIndex")
		tz := clockTimezones[tzIdx]

		// Call FormatDate with nil locale manager (default format)
		result := weather.FormatDate(ts, tz, nil)

		// Verify the result matches the expected format: DD/MM/YYYY
		datePattern := regexp.MustCompile(`^\d{2}/\d{2}/\d{4}$`)
		if !datePattern.MatchString(result) {
			rt.Fatalf("FormatDate(%v, %q) = %q does not match DD/MM/YYYY pattern", ts, tz, result)
		}

		// Verify the date is correctly converted to the timezone
		loc, err := time.LoadLocation(tz)
		if err != nil {
			rt.Fatalf("failed to load timezone %q: %v", tz, err)
		}
		localTime := ts.In(loc)
		expected := localTime.Format("02/01/2006")
		if result != expected {
			rt.Fatalf("FormatDate(%v, %q) = %q, want %q", ts, tz, result, expected)
		}
	})
}

// TestPreservation_EmptyTimezoneDefaultsToUTC verifies that when an empty timezone
// string is provided, both FormatTime and FormatDate fall back to UTC.
//
// **Validates: Requirements 3.4**
func TestPreservation_EmptyTimezoneDefaultsToUTC(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random time
		year := rapid.IntRange(2020, 2030).Draw(rt, "year")
		month := rapid.IntRange(1, 12).Draw(rt, "month")
		day := rapid.IntRange(1, 28).Draw(rt, "day")
		hour := rapid.IntRange(0, 23).Draw(rt, "hour")
		minute := rapid.IntRange(0, 59).Draw(rt, "minute")
		second := rapid.IntRange(0, 59).Draw(rt, "second")

		ts := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)

		// Call with empty timezone
		timeResult := weather.FormatTime(ts, "", nil)
		dateResult := weather.FormatDate(ts, "", nil)

		// Expected: should use UTC time
		utcTime := ts.In(time.UTC)
		expectedTime := utcTime.Format("15:04:05")
		expectedDate := utcTime.Format("02/01/2006")

		if timeResult != expectedTime {
			rt.Fatalf("FormatTime(%v, \"\") = %q, want %q (UTC)", ts, timeResult, expectedTime)
		}
		if dateResult != expectedDate {
			rt.Fatalf("FormatDate(%v, \"\") = %q, want %q (UTC)", ts, dateResult, expectedDate)
		}
	})
}

// TestPreservation_InvalidTimezoneFallsBackToUTC verifies that when an invalid
// timezone string is provided, both FormatTime and FormatDate fall back to UTC.
//
// **Validates: Requirements 3.6**
func TestPreservation_InvalidTimezoneFallsBackToUTC(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random time
		year := rapid.IntRange(2020, 2030).Draw(rt, "year")
		month := rapid.IntRange(1, 12).Draw(rt, "month")
		day := rapid.IntRange(1, 28).Draw(rt, "day")
		hour := rapid.IntRange(0, 23).Draw(rt, "hour")
		minute := rapid.IntRange(0, 59).Draw(rt, "minute")
		second := rapid.IntRange(0, 59).Draw(rt, "second")

		ts := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)

		// Pick an invalid timezone
		tzIdx := rapid.IntRange(0, len(invalidTimezones)-1).Draw(rt, "tzIndex")
		invalidTz := invalidTimezones[tzIdx]

		// Call with invalid timezone
		timeResult := weather.FormatTime(ts, invalidTz, nil)
		dateResult := weather.FormatDate(ts, invalidTz, nil)

		// Expected: should fall back to UTC time
		utcTime := ts.In(time.UTC)
		expectedTime := utcTime.Format("15:04:05")
		expectedDate := utcTime.Format("02/01/2006")

		if timeResult != expectedTime {
			rt.Fatalf("FormatTime(%v, %q) = %q, want %q (UTC fallback)", ts, invalidTz, timeResult, expectedTime)
		}
		if dateResult != expectedDate {
			rt.Fatalf("FormatDate(%v, %q) = %q, want %q (UTC fallback)", ts, invalidTz, dateResult, expectedDate)
		}
	})
}

// TestPreservation_LocaleAwareTimeFormatting verifies that FormatTime respects
// locale-specific time formats when a LocaleManager is provided.
//
// **Validates: Requirements 3.1, 3.3**
func TestPreservation_LocaleAwareTimeFormatting(t *testing.T) {
	// Load the real locale manager
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		t.Fatalf("NewLocaleManager error = %v", err)
	}

	locales := lm.AvailableLocales()
	localeCodes := make([]string, len(locales))
	for i, l := range locales {
		localeCodes[i] = l.Code
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Pick a random locale
		localeIdx := rapid.IntRange(0, len(localeCodes)-1).Draw(rt, "localeIdx")
		locale := localeCodes[localeIdx]

		// Create a fresh manager and set the locale
		mgr, err := i18n.NewLocaleManager(i18n.LocaleFS)
		if err != nil {
			rt.Fatalf("NewLocaleManager error = %v", err)
		}
		if err := mgr.SetLocale(locale); err != nil {
			rt.Fatalf("SetLocale(%q) error = %v", locale, err)
		}

		// Generate a random time
		hour := rapid.IntRange(0, 23).Draw(rt, "hour")
		minute := rapid.IntRange(0, 59).Draw(rt, "minute")
		second := rapid.IntRange(0, 59).Draw(rt, "second")

		ts := time.Date(2024, 6, 15, hour, minute, second, 0, time.UTC)

		// Pick a valid timezone
		tzIdx := rapid.IntRange(0, len(clockTimezones)-1).Draw(rt, "tzIndex")
		tz := clockTimezones[tzIdx]

		// Call FormatTime with locale manager
		result := weather.FormatTime(ts, tz, mgr)

		// Get the expected format for this locale
		timeFormat := mgr.T("weather.timeFormat")

		// Convert to expected timezone and format
		loc, err := time.LoadLocation(tz)
		if err != nil {
			rt.Fatalf("failed to load timezone %q: %v", tz, err)
		}
		localTime := ts.In(loc)
		expected := localTime.Format(timeFormat)

		if result != expected {
			rt.Fatalf("FormatTime(%v, %q, %q) = %q, want %q (format=%q)",
				ts, tz, locale, result, expected, timeFormat)
		}
	})
}

// TestPreservation_LocaleAwareDateFormatting verifies that FormatDate respects
// locale-specific date formats when a LocaleManager is provided.
//
// **Validates: Requirements 3.2, 3.3**
func TestPreservation_LocaleAwareDateFormatting(t *testing.T) {
	// Load the real locale manager
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		t.Fatalf("NewLocaleManager error = %v", err)
	}

	locales := lm.AvailableLocales()
	localeCodes := make([]string, len(locales))
	for i, l := range locales {
		localeCodes[i] = l.Code
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Pick a random locale
		localeIdx := rapid.IntRange(0, len(localeCodes)-1).Draw(rt, "localeIdx")
		locale := localeCodes[localeIdx]

		// Create a fresh manager and set the locale
		mgr, err := i18n.NewLocaleManager(i18n.LocaleFS)
		if err != nil {
			rt.Fatalf("NewLocaleManager error = %v", err)
		}
		if err := mgr.SetLocale(locale); err != nil {
			rt.Fatalf("SetLocale(%q) error = %v", locale, err)
		}

		// Generate a random date
		year := rapid.IntRange(2020, 2030).Draw(rt, "year")
		month := rapid.IntRange(1, 12).Draw(rt, "month")
		day := rapid.IntRange(1, 28).Draw(rt, "day")

		ts := time.Date(year, time.Month(month), day, 12, 0, 0, 0, time.UTC)

		// Pick a valid timezone
		tzIdx := rapid.IntRange(0, len(clockTimezones)-1).Draw(rt, "tzIndex")
		tz := clockTimezones[tzIdx]

		// Call FormatDate with locale manager
		result := weather.FormatDate(ts, tz, mgr)

		// Get the expected format for this locale
		dateFormat := mgr.T("weather.dateFormat")

		// Convert to expected timezone and format
		loc, err := time.LoadLocation(tz)
		if err != nil {
			rt.Fatalf("failed to load timezone %q: %v", tz, err)
		}
		localTime := ts.In(loc)
		expected := localTime.Format(dateFormat)

		if result != expected {
			rt.Fatalf("FormatDate(%v, %q, %q) = %q, want %q (format=%q)",
				ts, tz, locale, result, expected, dateFormat)
		}
	})
}

// TestPreservation_TimezoneConversionBoundary verifies that timezone conversion
// correctly handles times near day boundaries (midnight, etc.) where the date
// might change between timezones.
//
// **Validates: Requirements 3.1, 3.2, 3.3**
func TestPreservation_TimezoneConversionBoundary(t *testing.T) {
	// Test specific boundary cases
	testCases := []struct {
		name     string
		utcTime  time.Time
		timezone string
	}{
		{
			name:     "midnight UTC to Tokyo (+9)",
			utcTime:  time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			timezone: "Asia/Tokyo",
		},
		{
			name:     "11pm UTC to New York (-4/-5)",
			utcTime:  time.Date(2024, 6, 15, 23, 0, 0, 0, time.UTC),
			timezone: "America/New_York",
		},
		{
			name:     "noon UTC to Sao Paulo (-3)",
			utcTime:  time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
			timezone: "America/Sao_Paulo",
		},
		{
			name:     "3am UTC to Los Angeles (-7/-8)",
			utcTime:  time.Date(2024, 6, 15, 3, 0, 0, 0, time.UTC),
			timezone: "America/Los_Angeles",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			timeResult := weather.FormatTime(tc.utcTime, tc.timezone, nil)
			dateResult := weather.FormatDate(tc.utcTime, tc.timezone, nil)

			// Calculate expected values
			loc, err := time.LoadLocation(tc.timezone)
			if err != nil {
				t.Fatalf("failed to load timezone %q: %v", tc.timezone, err)
			}
			localTime := tc.utcTime.In(loc)
			expectedTime := localTime.Format("15:04:05")
			expectedDate := localTime.Format("02/01/2006")

			if timeResult != expectedTime {
				t.Errorf("FormatTime(%v, %q) = %q, want %q", tc.utcTime, tc.timezone, timeResult, expectedTime)
			}
			if dateResult != expectedDate {
				t.Errorf("FormatDate(%v, %q) = %q, want %q", tc.utcTime, tc.timezone, dateResult, expectedDate)
			}
		})
	}
}
