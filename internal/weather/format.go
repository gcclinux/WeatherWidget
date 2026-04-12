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
func MapConditionToIcon(code string) string {
	for _, valid := range AllIconCodes {
		if code == valid {
			return code
		}
	}
	return IconCloudy
}
