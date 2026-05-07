package weather

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"

	"pgregory.net/rapid"
)

// **Feature: temperature-unit-toggle, Property 3: Celsius conversion is identity**
// **Validates: Requirements 3.2, 4.1, 5.2**

func TestProperty3_CelsiusConversionIsIdentity(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := rapid.Int().Draw(t, "celsius")
		out := FormatTemperature(c, config.TemperatureUnitCelsius)
		expected := fmt.Sprintf("%d°C", c)
		if out != expected {
			t.Fatalf("got %q, want %q", out, expected)
		}
	})
}

// **Feature: temperature-unit-toggle, Property 5: Fahrenheit conversion is approximately invertible**
// **Validates: Requirements 3.4**

func TestProperty5_FahrenheitNearInverse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := rapid.IntRange(-273, 60).Draw(t, "celsius")
		f := ConvertToFahrenheit(c)
		back := int(math.Round((float64(f) - 32) / 1.8))
		diff := back - c
		if diff < -1 || diff > 1 {
			t.Fatalf("inverse out of range: C=%d F=%d back=%d diff=%d", c, f, back, diff)
		}
	})
}

// **Feature: temperature-unit-toggle, Property 4: Fahrenheit conversion formula correctness**
// **Validates: Requirements 3.1, 3.3, 4.2, 5.3**

func TestProperty4_FahrenheitFormulaCorrectness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := rapid.IntRange(-273, 60).Draw(t, "celsius")
		expected := int(math.Round(float64(c)*1.8 + 32))
		out := FormatTemperature(c, config.TemperatureUnitFahrenheit)
		expectedStr := fmt.Sprintf("%d°F", expected)
		if out != expectedStr {
			t.Fatalf("got %q, want %q", out, expectedStr)
		}
	})
}

// **Feature: windows-weather-widget, Property 9: Display string formatting**
// **Validates: Requirements 2.2, 2.4**

func TestProperty9_FormatTemperature(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		temp := rapid.Int().Draw(t, "temperature")

		result := FormatTemperature(temp, config.TemperatureUnitCelsius)

		expected := fmt.Sprintf("%d°C", temp)
		if result != expected {
			t.Fatalf("FormatTemperature(%d) = %q, want %q", temp, result, expected)
		}

		// Also verify the pattern: optional minus, digits, then °C
		if !strings.HasSuffix(result, "°C") {
			t.Fatalf("FormatTemperature(%d) = %q does not end with °C", temp, result)
		}
		prefix := strings.TrimSuffix(result, "°C")
		_, err := strconv.Atoi(prefix)
		if err != nil {
			t.Fatalf("FormatTemperature(%d) prefix %q is not a valid integer: %v", temp, prefix, err)
		}
	})
}

func TestProperty9_FormatCityRegion(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[A-Za-zÀ-ÿ][A-Za-zÀ-ÿ ]{0,49}`).Draw(t, "name")
		region := rapid.StringMatching(`[A-Za-z][A-Za-z]{0,9}`).Draw(t, "region")

		result := FormatCityRegion(name, region)

		expected := fmt.Sprintf("%s, %s", name, region)
		if result != expected {
			t.Fatalf("FormatCityRegion(%q, %q) = %q, want %q", name, region, result, expected)
		}
	})
}

// **Feature: windows-weather-widget, Property 10: Date/time formatting**
// **Validates: Requirements 2.5**

// validTimezones is a set of known valid IANA timezone strings used for generation.
var validTimezones = []string{
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
}

var dateTimePattern = regexp.MustCompile(`^\d{2}/\d{2}/\d{4} - \d{2}:\d{2}:\d{2}$`)

func TestProperty10_FormatDateTime(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random time within a reasonable range
		year := rapid.IntRange(1970, 2099).Draw(t, "year")
		month := rapid.IntRange(1, 12).Draw(t, "month")
		day := rapid.IntRange(1, 28).Draw(t, "day") // Use 28 to avoid invalid dates
		hour := rapid.IntRange(0, 23).Draw(t, "hour")
		minute := rapid.IntRange(0, 59).Draw(t, "minute")
		second := rapid.IntRange(0, 59).Draw(t, "second")

		ts := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)

		tzIdx := rapid.IntRange(0, len(validTimezones)-1).Draw(t, "tzIndex")
		tz := validTimezones[tzIdx]

		result := FormatDateTime(ts, tz, nil)

		// Verify the output matches the DD/MM/YYYY - HH:MM:SS pattern
		if !dateTimePattern.MatchString(result) {
			t.Fatalf("FormatDateTime(%v, %q) = %q does not match DD/MM/YYYY - HH:MM:SS pattern", ts, tz, result)
		}

		// Verify the individual components match the time converted to that timezone
		loc, err := time.LoadLocation(tz)
		if err != nil {
			t.Fatalf("failed to load timezone %q: %v", tz, err)
		}
		localTime := ts.In(loc)

		expectedDay := fmt.Sprintf("%02d", localTime.Day())
		expectedMonth := fmt.Sprintf("%02d", int(localTime.Month()))
		expectedYear := fmt.Sprintf("%04d", localTime.Year())
		expectedHour := fmt.Sprintf("%02d", localTime.Hour())
		expectedMinute := fmt.Sprintf("%02d", localTime.Minute())
		expectedSecond := fmt.Sprintf("%02d", localTime.Second())

		parts := strings.Split(result, " - ")
		if len(parts) != 2 {
			t.Fatalf("FormatDateTime result %q does not split into date and time parts", result)
		}

		dateParts := strings.Split(parts[0], "/")
		if len(dateParts) != 3 {
			t.Fatalf("date part %q does not split into 3 components", parts[0])
		}

		timeParts := strings.Split(parts[1], ":")
		if len(timeParts) != 3 {
			t.Fatalf("time part %q does not split into 3 components", parts[1])
		}

		if dateParts[0] != expectedDay {
			t.Fatalf("day mismatch: got %q, want %q (input: %v, tz: %q)", dateParts[0], expectedDay, ts, tz)
		}
		if dateParts[1] != expectedMonth {
			t.Fatalf("month mismatch: got %q, want %q (input: %v, tz: %q)", dateParts[1], expectedMonth, ts, tz)
		}
		if dateParts[2] != expectedYear {
			t.Fatalf("year mismatch: got %q, want %q (input: %v, tz: %q)", dateParts[2], expectedYear, ts, tz)
		}
		if timeParts[0] != expectedHour {
			t.Fatalf("hour mismatch: got %q, want %q (input: %v, tz: %q)", timeParts[0], expectedHour, ts, tz)
		}
		if timeParts[1] != expectedMinute {
			t.Fatalf("minute mismatch: got %q, want %q (input: %v, tz: %q)", timeParts[1], expectedMinute, ts, tz)
		}
		if timeParts[2] != expectedSecond {
			t.Fatalf("second mismatch: got %q, want %q (input: %v, tz: %q)", timeParts[2], expectedSecond, ts, tz)
		}
	})
}

// **Feature: temperature-unit-toggle, Property 6: FormatTemperature Celsius output matches pattern**
// **Validates: Requirements 4.1, 5.2, 5.4**

func TestProperty6_CelsiusOutputMatchesPattern(t *testing.T) {
	celsiusRe := regexp.MustCompile(`^-?\d+°C$`)
	rapid.Check(t, func(t *rapid.T) {
		c := rapid.Int().Draw(t, "celsius")
		out := FormatTemperature(c, config.TemperatureUnitCelsius)
		if !celsiusRe.MatchString(out) {
			t.Fatalf("%q does not match ^-?\\d+°C$", out)
		}
	})
}

// **Feature: temperature-unit-toggle, Property 7: FormatTemperature Fahrenheit output matches pattern**
// **Validates: Requirements 4.2, 5.3, 5.5**

func TestProperty7_FahrenheitOutputMatchesPattern(t *testing.T) {
	fahrenheitRe := regexp.MustCompile(`^-?\d+°F$`)
	rapid.Check(t, func(t *rapid.T) {
		c := rapid.Int().Draw(t, "celsius")
		out := FormatTemperature(c, config.TemperatureUnitFahrenheit)
		if !fahrenheitRe.MatchString(out) {
			t.Fatalf("%q does not match ^-?\\d+°F$", out)
		}
	})
}

// **Feature: i18n-localization, Property 4: Locale-aware formatting preserves structure**
// **Validates: Requirements 6.1, 6.2, 6.3**

func TestProperty4_LocaleAwareFormattingPreservesStructure(t *testing.T) {
	// Load the real locale manager with embedded locale files.
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
		// Pick a random locale.
		localeIdx := rapid.IntRange(0, len(localeCodes)-1).Draw(rt, "localeIdx")
		locale := localeCodes[localeIdx]

		// Create a fresh manager and set the locale.
		mgr, err := i18n.NewLocaleManager(i18n.LocaleFS)
		if err != nil {
			rt.Fatalf("NewLocaleManager error = %v", err)
		}
		if err := mgr.SetLocale(locale); err != nil {
			rt.Fatalf("SetLocale(%q) error = %v", locale, err)
		}

		// Get the locale's expected formats.
		dateFormat := mgr.T("weather.dateFormat")
		timeFormat := mgr.T("weather.timeFormat")

		// --- FormatTemperature ---
		temp := rapid.IntRange(-100, 100).Draw(rt, "temperature")
		tempResult := FormatTemperature(temp, config.TemperatureUnitCelsius)

		expected := fmt.Sprintf("%d°C", temp)
		if tempResult != expected {
			rt.Fatalf("FormatTemperature(%d, celsius) = %q, want %q", temp, tempResult, expected)
		}

		// --- FormatDate ---
		year := rapid.IntRange(1970, 2099).Draw(rt, "year")
		month := rapid.IntRange(1, 12).Draw(rt, "month")
		day := rapid.IntRange(1, 28).Draw(rt, "day")
		ts := time.Date(year, time.Month(month), day, 12, 0, 0, 0, time.UTC)

		dateResult := FormatDate(ts, "UTC", mgr)

		// Verify the date matches the expected format by formatting the same time
		// with the locale's date format directly.
		expectedDate := ts.Format(dateFormat)
		if dateResult != expectedDate {
			rt.Fatalf("FormatDate(%v, UTC, %q) = %q, want %q (format=%q)",
				ts, locale, dateResult, expectedDate, dateFormat)
		}

		// --- FormatTime ---
		hour := rapid.IntRange(0, 23).Draw(rt, "hour")
		minute := rapid.IntRange(0, 59).Draw(rt, "minute")
		second := rapid.IntRange(0, 59).Draw(rt, "second")
		tsTime := time.Date(2024, 1, 1, hour, minute, second, 0, time.UTC)

		timeResult := FormatTime(tsTime, "UTC", mgr)

		// Verify the time matches the expected format.
		expectedTime := tsTime.Format(timeFormat)
		if timeResult != expectedTime {
			rt.Fatalf("FormatTime(%v, UTC, %q) = %q, want %q (format=%q)",
				tsTime, locale, timeResult, expectedTime, timeFormat)
		}
	})
}
