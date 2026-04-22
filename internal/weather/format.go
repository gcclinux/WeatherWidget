package weather

import (
	"fmt"
	"strings"
	"time"
)

// FormatTemperature returns a temperature string in the pattern "{int}°C".
func FormatTemperature(temp int) string {
	return fmt.Sprintf("%d°C", temp)
}

// FormatCityRegion returns a location string in the pattern "{name}, {region}".
func FormatCityRegion(name, region string) string {
	return fmt.Sprintf("%s, %s", name, region)
}

// FormatDateTime returns a date/time string in the pattern "DD/MM/YYYY - HH:MM:SS"
// for the given timezone. If the timezone is invalid, it falls back to UTC.
func FormatDateTime(t time.Time, timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	return t.In(loc).Format("02/01/2006 - 15:04:05")
}

// FormatTime returns a time string in the pattern "HH:MM:SS"
func FormatTime(t time.Time, timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	return t.In(loc).Format("15:04:05")
}

// FormatDate returns a date string in the pattern "DD/MM/YYYY"
func FormatDate(t time.Time, timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	return t.In(loc).Format("02/01/2006")
}

// FormatDescription title-cases a weather description string.
// e.g. "clear sky" → "Clear Sky", "broken clouds" → "Broken Clouds"
func FormatDescription(desc string) string {
	return strings.Title(desc)
}

// Known codes are returned as-is. Unknown codes default to "cloudy".
// When the code is "clear" and localTime falls between 6 PM and 6 AM,
// the icon is swapped to "moon" to reflect nighttime.
func MapConditionToIcon(code string, localTime time.Time) string {
	for _, valid := range AllIconCodes {
		if code == valid {
			if isNight(localTime) {
				switch code {
				case IconClear:
					return IconMoon
				case IconCloudy:
					return IconCloudyMoon
				}
			}
			return code
		}
	}
	return IconCloudy
}

// isNight returns true when the hour component of t is outside the
// 6 AM – 6 PM daytime window (i.e. before 6 or at/after 18).
func isNight(t time.Time) bool {
	h := t.Hour()
	return h < 6 || h >= 18
}
