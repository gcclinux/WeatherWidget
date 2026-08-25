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

// FormatDescription returns the translated weather description for the given freetext.
// If lm is nil or no translation key matches, it falls back to title-casing the string.
// e.g. "clear sky" → "Clear Sky" (en) / "Céu limpo" (pt-BR)
func FormatDescription(desc string, lm *i18n.LocaleManager) string {
	if desc == "" {
		return ""
	}
	normalized := strings.ToLower(strings.TrimSpace(desc))
	key := "weather.condition." + strings.ReplaceAll(normalized, " ", "_")
	if lm != nil {
		translated := lm.T(key)
		if translated != key {
			return translated
		}
	}
	return strings.Title(desc)
}

// NormalizeCondition maps an icon code or condition name into one of the base icon codes.
func NormalizeCondition(code string) string {
	code = strings.TrimSpace(strings.ToLower(code))
	// Strip directory prefixes if present
	if idx := strings.LastIndex(code, "/"); idx != -1 {
		code = code[idx+1:]
	}
	// Strip file extension if present
	if idx := strings.LastIndex(code, "."); idx != -1 {
		code = code[:idx]
	}
	// Strip _day or _night suffix if present
	code = strings.TrimSuffix(code, "_day")
	code = strings.TrimSuffix(code, "_night")

	switch code {
	case IconClear, "sunny", "clear_sky", "fair", IconMoon:
		return IconClear
	case IconPartlyCloudy, "partly-cloudy", "partlycloudy", "few_clouds", "scattered_clouds":
		return IconPartlyCloudy
	case IconCloudy, "overcast", "broken_clouds", "cloud", IconCloudyMoon:
		return IconCloudy
	case IconRain, "light_rain", "moderate_rain", "drizzle", "shower":
		return IconRain
	case IconHeavyRain, "heavy-rain", "heavyrain", "extreme_rain", "very_heavy_rain", "shower_rain":
		return IconHeavyRain
	case IconSnow, "light_snow", "heavy_snow", "sleet", "flurries":
		return IconSnow
	case IconStorm, "thunderstorm", "lightning", "thunder":
		return IconStorm
	case IconFog, "mist", "haze", "smoke", "dust", "sand", "ash", "squall", "tornado":
		return IconFog
	case IconWind, "windy", "breeze", "gale":
		return IconWind
	default:
		return IconCloudy
	}
}

// MapConditionToIcon returns the asset icon path (e.g. "day/clear_day" or "night/clear_night")
// for the given weather condition code based on the city's local time using the default new icon theme.
// Daytime is defined as 6:00 AM to 5:59 PM (hours 6 to 17); nighttime is 6:00 PM to 5:59 AM.
func MapConditionToIcon(code string, localTime time.Time) string {
	return MapConditionToIconWithTheme(code, localTime, config.IconThemeNew)
}

// MapConditionToIconWithTheme returns the asset icon path for the given weather condition code,
// local time, and icon theme (e.g. "day/clear_day", "night/clear_night", or "original/clear").
func MapConditionToIconWithTheme(code string, localTime time.Time, theme config.IconTheme) string {
	theme = config.NormalizeIconTheme(theme)
	base := NormalizeCondition(code)
	night := IsNight(localTime)

	if theme == config.IconThemeOriginal {
		if night {
			switch base {
			case IconClear:
				return "original/moon"
			case IconPartlyCloudy, IconCloudy:
				return "original/cloudy_moon"
			default:
				return "original/" + base
			}
		}
		return "original/" + base
	}

	if night {
		return "night/" + base + "_night"
	}
	return "day/" + base + "_day"
}

// IsNight returns true when the hour component of t is outside the
// 6 AM – 6 PM daytime window (i.e. before 6 or at/after 18).
func IsNight(t time.Time) bool {
	h := t.Hour()
	return h < 6 || h >= 18
}

// isNight is an alias kept for backward compatibility.
func isNight(t time.Time) bool {
	return IsNight(t)
}

// compassDirections maps 16 cardinal/intercardinal compass points.
var compassDirections = []string{
	"N", "NNE", "NE", "ENE",
	"E", "ESE", "SE", "SSE",
	"S", "SSW", "SW", "WSW",
	"W", "WNW", "NW", "NNW",
}

// DegreesToCompass converts a wind direction in degrees (0–360) to a
// compass label (e.g. 0 → "N", 90 → "E", 225 → "SW").
func DegreesToCompass(degrees int) string {
	// Normalize to 0–359 range.
	deg := ((degrees % 360) + 360) % 360
	index := int(math.Round(float64(deg)/22.5)) % 16
	return compassDirections[index]
}

// ConvertWindSpeed converts windSpeed in km/h to the target unit and returns the converted value and its display label.
func ConvertWindSpeed(windSpeed float64, unit config.WindSpeedUnit) (float64, string) {
	switch config.NormalizeWindSpeedUnit(unit) {
	case config.WindSpeedUnitMph:
		return windSpeed * 0.621371, "mph"
	case config.WindSpeedUnitKnots:
		return windSpeed * 0.539957, "knots"
	default:
		return windSpeed, "km/h"
	}
}

// FormatHumidity returns a compact string like "💧 Hum 45%".
// If lm is non-nil, the label is localized according to the active locale.
func FormatHumidity(humidity int, lm *i18n.LocaleManager) string {
	label := "Hum"
	if lm != nil {
		translated := lm.T("weather.hum")
		if translated != "weather.hum" {
			label = translated
		}
	}
	return fmt.Sprintf("💧 %s %d%%", label, humidity)
}

// FormatWind returns a compact string like "💨 4.5 km/h"
// suitable for display on its own panel row.
func FormatWind(windSpeed float64, unit config.WindSpeedUnit) string {
	convertedSpeed, unitLabel := ConvertWindSpeed(windSpeed, unit)
	return fmt.Sprintf("💨 %.1f %s", convertedSpeed, unitLabel)
}

// FormatHumidityWind returns a compact string like "💧 Hum 45%   💨 4.5 km/h"
// kept for backward compatibility; prefer FormatHumidity + FormatWind separately.
func FormatHumidityWind(humidity int, windSpeed float64, unit config.WindSpeedUnit) string {
	convertedSpeed, unitLabel := ConvertWindSpeed(windSpeed, unit)
	return fmt.Sprintf("💧 %d%%   💨 %.1f %s", humidity, convertedSpeed, unitLabel)
}

// compassArrows maps 16 cardinal/intercardinal compass points to directional arrows.
// Uses U+2190..U+2199 arrows which have broad font coverage across all platforms.
var compassArrows = []string{
	"↑", "↗", "↗", "↗",
	"→", "↘", "↘", "↘",
	"↓", "↙", "↙", "↙",
	"←", "↖", "↖", "↖",
}

// DegreesToArrow converts a wind direction in degrees (0–360) to a
// directional arrow (e.g. 0 → "↑", 90 → "→", 225 → "↙").
func DegreesToArrow(degrees int) string {
	deg := ((degrees % 360) + 360) % 360
	index := int(math.Round(float64(deg)/22.5)) % 16
	return compassArrows[index]
}

// FormatWindGust returns a compact string like "💨 Gust 12.3 km/h".
// If lm is non-nil, the label is localized according to the active locale.
func FormatWindGust(gustSpeed float64, unit config.WindSpeedUnit, lm *i18n.LocaleManager) string {
	label := "Gust"
	if lm != nil {
		translated := lm.T("weather.gust")
		if translated != "weather.gust" {
			label = translated
		}
	}
	convertedSpeed, unitLabel := ConvertWindSpeed(gustSpeed, unit)
	return fmt.Sprintf("💨 %s %.1f %s", label, convertedSpeed, unitLabel)
}

// FormatDewPoint returns a compact string like "💧 Dew 8.5°C".
// Returns an empty string if dewPoint is zero (data not available).
// If lm is non-nil, the label is localized according to the active locale.
func FormatDewPoint(dewPoint float64, lm *i18n.LocaleManager) string {
	if dewPoint == 0 {
		return ""
	}
	label := "Dew"
	if lm != nil {
		translated := lm.T("weather.dew")
		if translated != "weather.dew" {
			label = translated
		}
	}
	return fmt.Sprintf("💧 %s %.1f°C", label, dewPoint)
}

// FormatPressure returns a compact string like "🌡 1019 hPa".
// Returns an empty string if pressure is zero (data not available).
func FormatPressure(pressure float64) string {
	if pressure == 0 {
		return ""
	}
	return fmt.Sprintf("🌡 %.0f hPa", pressure)
}

// FormatUVIndex returns a compact string like "☀ UV 3.2".
// Returns an empty string if uvIndex is negative (data not available).
func FormatUVIndex(uvIndex float64) string {
	if uvIndex < 0 {
		return ""
	}
	return fmt.Sprintf("☀ UV %.1f", uvIndex)
}

// FormatWindDir returns the wind direction as a compass label like "↖ NW".
// Returns an empty string if windDirection is zero (data not available).
func FormatWindDir(windDirection int) string {
	if windDirection == 0 {
		return ""
	}
	return fmt.Sprintf("%s %s", DegreesToArrow(windDirection), DegreesToCompass(windDirection))
}
