package weather

import (
	"fmt"
	"math"
	"strings"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
)

// FormatTemperature returns a temperature string for the given Celsius value
// formatted according to unit. Invalid unit values default to Celsius.
func FormatTemperature(temp int, unit config.TemperatureUnit) string {
	switch config.NormalizeTemperatureUnit(unit) {
	case config.TemperatureUnitFahrenheit:
		f := convertToFahrenheit(temp)
		return fmt.Sprintf("%d°F", f)
	default: // celsius and any invalid value
		return fmt.Sprintf("%d°C", temp)
	}
}

// convertToFahrenheit converts an integer Celsius value to Fahrenheit
// using the formula F = round(C × 1.8 + 32).
func convertToFahrenheit(celsius int) int {
	return int(math.Round(float64(celsius)*1.8 + 32))
}

// ConvertToFahrenheit is the exported wrapper for property-based testing.
func ConvertToFahrenheit(celsius int) int {
	return convertToFahrenheit(celsius)
}

// FormatCityRegion returns a location string in the pattern "{name}, {region}".
func FormatCityRegion(name, region string) string {
	return fmt.Sprintf("%s, %s", name, region)
}

// FormatDateTime returns a date/time string in the pattern "DD/MM/YYYY - HH:MM:SS"
// for the given timezone. If the timezone is invalid, it falls back to UTC.
// If lm is non-nil, locale-specific date and time formats are used.
func FormatDateTime(t time.Time, timezone string, lm *i18n.LocaleManager) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	dateFormat := "02/01/2006"
	timeFormat := "15:04:05"
	if lm != nil {
		dateFormat = lm.T("weather.dateFormat")
		timeFormat = lm.T("weather.timeFormat")
	}
	localTime := t.In(loc)
	return localTime.Format(dateFormat) + " - " + localTime.Format(timeFormat)
}

// FormatTime returns a time string using the locale's time format.
// If lm is nil, falls back to the default "15:04:05" format.
func FormatTime(t time.Time, timezone string, lm *i18n.LocaleManager) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	timeFormat := "15:04:05"
	if lm != nil {
		timeFormat = lm.T("weather.timeFormat")
	}
	return t.In(loc).Format(timeFormat)
}

// FormatDate returns a date string using the locale's date format.
// If lm is nil, falls back to the default "02/01/2006" format.
func FormatDate(t time.Time, timezone string, lm *i18n.LocaleManager) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	dateFormat := "02/01/2006"
	if lm != nil {
		dateFormat = lm.T("weather.dateFormat")
	}
	return t.In(loc).Format(dateFormat)
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
