package weather

import (
	"fmt"
	"math"
	"time"

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

// SunriseDisplay returns the sunrise metric as emoji + name + formatted time.
func SunriseDisplay(sunrise int64, timezone string, lm *i18n.LocaleManager) MetricDisplay {
	val := "--:--"
	if sunrise > 0 {
		t := time.Unix(sunrise, 0)
		val = FormatTime(t, timezone, lm)
		if len(val) >= 5 {
			val = val[:5]
		}
	}
	return MetricDisplay{
		Emoji: "🌅",
		Name:  tr(lm, "weather.sunrise", "Sunrise"),
		Value: val,
	}
}

// SunsetDisplay returns the sunset metric as emoji + name + formatted time.
func SunsetDisplay(sunset int64, timezone string, lm *i18n.LocaleManager) MetricDisplay {
	val := "--:--"
	if sunset > 0 {
		t := time.Unix(sunset, 0)
		val = FormatTime(t, timezone, lm)
		if len(val) >= 5 {
			val = val[:5]
		}
	}
	return MetricDisplay{
		Emoji: "🌇",
		Name:  tr(lm, "weather.sunset", "Sunset"),
		Value: val,
	}
}

// CloudsDisplay returns cloud cover as emoji + name + percentage.
func CloudsDisplay(clouds int, lm *i18n.LocaleManager) MetricDisplay {
	return MetricDisplay{
		Emoji: "☁",
		Name:  tr(lm, "weather.clouds", "Clouds"),
		Value: fmt.Sprintf("%d%%", clouds),
	}
}

// VisibilityDisplay returns visibility as emoji + name + formatted km.
func VisibilityDisplay(vis float64, lm *i18n.LocaleManager) MetricDisplay {
	val := "--"
	if vis > 0 {
		if vis >= 1000 {
			val = fmt.Sprintf("%.1f km", vis/1000.0)
		} else {
			val = fmt.Sprintf("%.0f m", vis)
		}
	}
	return MetricDisplay{
		Emoji: "👁",
		Name:  tr(lm, "weather.visibility", "Visibility"),
		Value: val,
	}
}

// PrecipRateDisplay returns rainfall rate as emoji + name + mm/h.
func PrecipRateDisplay(rate float64, lm *i18n.LocaleManager) MetricDisplay {
	return MetricDisplay{
		Emoji: "🌧",
		Name:  tr(lm, "weather.precipRate", "Rain Rate"),
		Value: fmt.Sprintf("%.1f mm/h", rate),
	}
}

// ForecastTempMaxDisplay returns daily maximum temperature.
func ForecastTempMaxDisplay(max *float64, unit config.TemperatureUnit, lm *i18n.LocaleManager) MetricDisplay {
	val := "--"
	if max != nil {
		val = FormatTemperature(int(math.Round(*max)), unit)
	}
	return MetricDisplay{
		Emoji: "🔺",
		Name:  tr(lm, "weather.forecast.tempMax", "High Temp"),
		Value: val,
	}
}

// ForecastTempMinDisplay returns daily minimum temperature.
func ForecastTempMinDisplay(min *float64, unit config.TemperatureUnit, lm *i18n.LocaleManager) MetricDisplay {
	val := "--"
	if min != nil {
		val = FormatTemperature(int(math.Round(*min)), unit)
	}
	return MetricDisplay{
		Emoji: "🔻",
		Name:  tr(lm, "weather.forecast.tempMin", "Low Temp"),
		Value: val,
	}
}

// ForecastTempRangeDisplay returns high/low temperature span.
func ForecastTempRangeDisplay(min, max *float64, unit config.TemperatureUnit, lm *i18n.LocaleManager) MetricDisplay {
	val := "-- / --"
	if min != nil && max != nil {
		maxStr := FormatTemperature(int(math.Round(*max)), unit)
		minStr := FormatTemperature(int(math.Round(*min)), unit)
		val = fmt.Sprintf("%s / %s", maxStr, minStr)
	}
	return MetricDisplay{
		Emoji: "🌡",
		Name:  tr(lm, "weather.forecast.tempRange", "High / Low"),
		Value: val,
	}
}

// ForecastPrecipProbDisplay returns precipitation probability.
func ForecastPrecipProbDisplay(prob int, lm *i18n.LocaleManager) MetricDisplay {
	return MetricDisplay{
		Emoji: "☔",
		Name:  tr(lm, "weather.forecast.precipProb", "Rain Chance"),
		Value: fmt.Sprintf("%d%%", prob),
	}
}

// ForecastPrecipAmountDisplay returns expected rainfall in mm.
func ForecastPrecipAmountDisplay(amount *float64, lm *i18n.LocaleManager) MetricDisplay {
	val := "-- mm"
	if amount != nil {
		val = fmt.Sprintf("%.1f mm", *amount)
	}
	return MetricDisplay{
		Emoji: "🌧",
		Name:  tr(lm, "weather.forecast.precipAmount", "Rain Sum"),
		Value: val,
	}
}

// ForecastUVMaxDisplay returns peak UV index.
func ForecastUVMaxDisplay(uv *float64, lm *i18n.LocaleManager) MetricDisplay {
	val := "--"
	if uv != nil {
		val = fmt.Sprintf("%.1f", *uv)
	}
	return MetricDisplay{
		Emoji: "☀️",
		Name:  tr(lm, "weather.forecast.uvMax", "Peak UV"),
		Value: val,
	}
}

// ForecastGustMaxDisplay returns max wind gust.
func ForecastGustMaxDisplay(gust *float64, unit config.WindSpeedUnit, lm *i18n.LocaleManager) MetricDisplay {
	val := "--"
	if gust != nil {
		speed, unitLabel := ConvertWindSpeed(*gust, unit)
		val = fmt.Sprintf("%.1f %s", speed, unitLabel)
	}
	return MetricDisplay{
		Emoji: "💨",
		Name:  tr(lm, "weather.forecast.gustMax", "Max Gust"),
		Value: val,
	}
}

// ForecastSummaryDisplay returns brief condition summary.
func ForecastSummaryDisplay(summary string, lm *i18n.LocaleManager) MetricDisplay {
	val := "--"
	if summary != "" {
		val = summary
	}
	return MetricDisplay{
		Emoji: "📋",
		Name:  tr(lm, "weather.forecast.summary", "Forecast"),
		Value: val,
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
