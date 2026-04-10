package weather

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// **Feature: windows-weather-widget, Property 9: Display string formatting**
// **Validates: Requirements 2.2, 2.4**

func TestProperty9_FormatTemperature(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		temp := rapid.Int().Draw(t, "temperature")

		result := FormatTemperature(temp)

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

		result := FormatDateTime(ts, tz)

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
