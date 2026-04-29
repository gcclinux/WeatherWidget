package weather

import (
	"fmt"
	"strings"
	"time"

	"weatherwidget/internal/i18n"
)

// FormatTemperature returns a temperature string using the locale's temperature format.
// If lm is nil, falls back to the default "%d°C" format.
func FormatTemperature(temp int, lm *i18n.LocaleManager) string {
	format := "%d°C"
	if lm != nil {
		format = lm.T("weather.tempFormat")
	}
	return fmt.Sprintf(format, temp)
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
