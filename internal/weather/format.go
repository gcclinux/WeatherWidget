package weather

import (
	"fmt"
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

// MapConditionToIcon maps a weather condition code to an embedded icon asset identifier.
// Known codes are returned as-is. Unknown codes default to "cloudy".
func MapConditionToIcon(code string) string {
	for _, valid := range AllIconCodes {
		if code == valid {
			return code
		}
	}
	return IconCloudy
}
