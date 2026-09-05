package weather

import (
	"fmt"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
)

// MetricDisplay is a presentation-ready view of a single weather metric,
// split into its emoji glyph, human-readable name, and formatted value. UI
// renderers use it to lay out a tile with the icon, the metric name, and the
// value on separate lines (matching the design mockup) rather than a single
// packed string.
type MetricDisplay struct {
	Emoji string // leading glyph, e.g. "💧"
	Name  string // metric name, e.g. "Humidity"
	Value string // formatted value, e.g. "84%"
}

// tr returns the locale translation for key, or fallback when lm is nil or the
// key is missing (LocaleManager.T returns the key itself when not found).
func tr(lm *i18n.LocaleManager, key, fallback string) string {
	if lm == nil {
		return fallback
	}
	if v := lm.T(key); v != "" && v != key {
		return v
	}
	return fallback
}

// HumidityDisplay returns the humidity metric as emoji + name + value.
func HumidityDisplay(humidity int, lm *i18n.LocaleManager) MetricDisplay {
	return MetricDisplay{
		Emoji: "💧",
		Name:  tr(lm, "weather.humidity", "Humidity"),
		Value: fmt.Sprintf("%d%%", humidity),
	}
}

// WindDisplay returns the wind metric as emoji + name + value, appending the
// compass direction (e.g. "7.2 km/h ↙ WSW") when a direction is available.
func WindDisplay(windSpeed float64, windDirection int, unit config.WindSpeedUnit, lm *i18n.LocaleManager) MetricDisplay {
	speed, unitLabel := ConvertWindSpeed(windSpeed, unit)
	value := fmt.Sprintf("%.1f %s", speed, unitLabel)
	if dir := FormatWindDir(windDirection); dir != "" {
		value = fmt.Sprintf("%s %s", value, dir)
	}
	return MetricDisplay{
		Emoji: "💨",
		Name:  tr(lm, "weather.wind", "Wind"),
		Value: value,
	}
}

// WindGustDisplay returns the wind-gust metric as emoji + name + value.
func WindGustDisplay(gustSpeed float64, unit config.WindSpeedUnit, lm *i18n.LocaleManager) MetricDisplay {
	speed, unitLabel := ConvertWindSpeed(gustSpeed, unit)
	return MetricDisplay{
		Emoji: "🌬",
		Name:  tr(lm, "weather.windGust", "Wind Gust"),
		Value: fmt.Sprintf("%.1f %s", speed, unitLabel),
	}
}

// DewPointDisplay returns the dew-point metric as emoji + name + value.
func DewPointDisplay(dewPoint float64, lm *i18n.LocaleManager) MetricDisplay {
	return MetricDisplay{
		Emoji: "💧",
		Name:  tr(lm, "weather.dewPoint", "Dew Point"),
		Value: fmt.Sprintf("%.1f°C", dewPoint),
	}
}

// PressureDisplay returns the pressure metric as emoji + name + value.
func PressureDisplay(pressure float64, lm *i18n.LocaleManager) MetricDisplay {
	return MetricDisplay{
		Emoji: "🌡",
		Name:  tr(lm, "weather.pressure", "Pressure"),
		Value: fmt.Sprintf("%.0f hPa", pressure),
	}
}

// UVIndexDisplay returns the UV-index metric as emoji + name + value.
func UVIndexDisplay(uvIndex float64, lm *i18n.LocaleManager) MetricDisplay {
	return MetricDisplay{
		Emoji: "☀",
		Name:  tr(lm, "weather.uvIndex", "UV Index"),
		Value: fmt.Sprintf("%.1f", uvIndex),
	}
}

// pollutionNameKey maps each metric to its i18n key and English fallback name.
var pollutionNameKey = map[PollutionMetric]struct {
	key      string
	fallback string
}{
	MetricAQI:  {"pollution.aqi", "Air Quality Index"},
	MetricCO:   {"pollution.co", "Carbon Monoxide"},
	MetricNO:   {"pollution.no", "Nitric Oxide"},
	MetricNO2:  {"pollution.no2", "Nitrogen Dioxide"},
	MetricO3:   {"pollution.o3", "Ozone"},
	MetricSO2:  {"pollution.so2", "Sulfur Dioxide"},
	MetricNH3:  {"pollution.nh3", "Ammonia"},
	MetricPM25: {"pollution.pm25", "Particulate 2.5"},
	MetricPM10: {"pollution.pm10", "Particulate 10"},
}

// PollutionMetricName returns the full human-readable name for a pollution
// metric (e.g. "Carbon Monoxide"), localized when lm is non-nil.
func PollutionMetricName(m PollutionMetric, lm *i18n.LocaleManager) string {
	if e, ok := pollutionNameKey[m]; ok {
		return tr(lm, e.key, e.fallback)
	}
	return ""
}
