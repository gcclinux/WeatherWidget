//go:build linux

package uitk

import (
	"math"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"
)

// maxMetricTiles is the number of tiles in the metrics grid (3 columns x 2 rows).
const maxMetricTiles = 6

// visibleMetrics returns the metric tiles to show in the card's grid, in order,
// capped at maxMetricTiles. It mirrors the Windows UI: the Grid Content mode
// ("live", "today", "tomorrow") selects between real-time conditions (filtered
// by the Show* toggles) and a fixed set of six forecast tiles. When d is nil
// (no data yet) tiles are returned with "--" placeholders.
func visibleMetrics(df *config.DisplayFields, d *weather.WeatherData, tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, lm *i18n.LocaleManager) []weather.MetricDisplay {
	if df == nil {
		df = config.DefaultDisplayFields()
	}
	switch config.NormalizeGridMode(df.GridMode) {
	case "today":
		return forecastMetrics(false, d, tempUnit, windUnit, lm)
	case "tomorrow":
		return forecastMetrics(true, d, tempUnit, windUnit, lm)
	}
	return liveMetrics(df, d, windUnit, lm)
}

type metricCandidate struct {
	show bool
	item weather.MetricDisplay
}

func liveMetrics(df *config.DisplayFields, d *weather.WeatherData, windUnit config.WindSpeedUnit, lm *i18n.LocaleManager) []weather.MetricDisplay {
	var (
		humidity, clouds, direction int
		wind, gust, dew, pressure   float64
		uv, vis, precipRate         float64
		sunrise, sunset             int64
		timezone                    string
	)
	if d != nil {
		humidity, wind, gust, dew, pressure, uv = d.Humidity, d.WindSpeed, d.WindGust, d.DewPoint, d.Pressure, d.UVIndex
		direction, clouds, vis = d.WindDirection, d.Clouds, d.Visibility
		sunrise, sunset = d.Sunrise, d.Sunset
		if d.PrecipRate != nil {
			precipRate = *d.PrecipRate
		}
		if !d.LocalTime.IsZero() {
			timezone = d.LocalTime.Location().String()
		}
	}
	candidates := []metricCandidate{
		{df.ShowHumidity, weather.HumidityDisplay(humidity, lm)},
		{df.ShowWind, weather.WindDisplay(wind, direction, windUnit, lm)},
		{df.ShowWindGust, weather.WindGustDisplay(gust, windUnit, lm)},
		{df.ShowDewPoint, weather.DewPointDisplay(dew, lm)},
		{df.ShowPressure, weather.PressureDisplay(pressure, lm)},
		{df.ShowUVIndex, weather.UVIndexDisplay(uv, lm)},
		{df.ShowSunrise, weather.SunriseDisplay(sunrise, timezone, lm)},
		{df.ShowSunset, weather.SunsetDisplay(sunset, timezone, lm)},
		{df.ShowClouds, weather.CloudsDisplay(clouds, lm)},
		{df.ShowVisibility, weather.VisibilityDisplay(vis, lm)},
		{df.ShowPrecipRate, weather.PrecipRateDisplay(precipRate, lm)},
	}
	var out []weather.MetricDisplay
	for _, c := range candidates {
		if !c.show {
			continue
		}
		if d == nil {
			c.item.Value = "--"
		}
		out = append(out, c.item)
		if len(out) == maxMetricTiles {
			break
		}
	}
	return out
}

func forecastMetrics(tomorrow bool, d *weather.WeatherData, tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, lm *i18n.LocaleManager) []weather.MetricDisplay {
	var out []weather.MetricDisplay
	if d != nil && d.Forecast != nil {
		fc := d.Forecast
		if tomorrow {
			out = []weather.MetricDisplay{
				weather.ForecastTempMaxDisplay(fc.TomorrowTempMax, tempUnit, lm),
				weather.ForecastTempMinDisplay(fc.TomorrowTempMin, tempUnit, lm),
				weather.ForecastPrecipProbDisplay(fc.TomorrowPrecipProbMax, lm),
				weather.ForecastPrecipAmountDisplay(fc.TomorrowPrecipSumMM, lm),
				weather.ForecastUVMaxDisplay(fc.TomorrowUVMax, lm),
				weather.ForecastGustMaxDisplay(fc.TomorrowGustMax, windUnit, lm),
			}
		} else {
			out = []weather.MetricDisplay{
				weather.ForecastTempMaxDisplay(fc.TodayTempMax, tempUnit, lm),
				weather.ForecastTempMinDisplay(fc.TodayTempMin, tempUnit, lm),
				weather.ForecastPrecipProbDisplay(fc.TodayPrecipProbMax, lm),
				weather.ForecastPrecipAmountDisplay(fc.TodayPrecipSumMM, lm),
				weather.ForecastUVMaxDisplay(fc.TodayUVMax, lm),
				weather.ForecastGustMaxDisplay(fc.TodayGustMax, windUnit, lm),
			}
		}
		return out
	}
	// No forecast available yet: show the six tiles with placeholders.
	out = []weather.MetricDisplay{
		weather.ForecastTempMaxDisplay(nil, tempUnit, lm),
		weather.ForecastTempMinDisplay(nil, tempUnit, lm),
		weather.ForecastPrecipProbDisplay(0, lm),
		weather.ForecastPrecipAmountDisplay(nil, lm),
		weather.ForecastUVMaxDisplay(nil, lm),
		weather.ForecastGustMaxDisplay(nil, windUnit, lm),
	}
	out[2].Value = "--%"
	return out
}

// headlineValues returns the icon code, temperature, "feels like" and
// description strings for a card header. For the live grid mode they are the
// real-time values. For a forecast mode (and when forecast data is present) they
// are overridden with the day's forecast: the high as temperature, the low in
// place of "feels like", the forecast summary and the WMO-derived icon. This
// mirrors the Windows simple view.
func headlineValues(df *config.DisplayFields, d *weather.WeatherData, tempUnit config.TemperatureUnit, lm *i18n.LocaleManager) (iconCode, temp, feels, desc string) {
	iconCode = d.IconCode
	temp = weather.FormatTemperature(d.Temperature, tempUnit)
	feels = weather.FormatFeelsLike(d.FeelsLike, tempUnit)
	desc = weather.FormatDescription(d.Description, lm)
	if df == nil || d.Forecast == nil {
		return
	}
	fc := d.Forecast
	var (
		code     int
		max, min *float64
		summary  string
	)
	switch config.NormalizeGridMode(df.GridMode) {
	case "today":
		code, max, min, summary = fc.TodayWeatherCode, fc.TodayTempMax, fc.TodayTempMin, fc.TodaySummary
	case "tomorrow":
		code, max, min, summary = fc.TomorrowWeatherCode, fc.TomorrowTempMax, fc.TomorrowTempMin, fc.TomorrowSummary
	default:
		return
	}
	if code != 0 {
		iconCode, _ = weather.MapWMOCodeToWeather(code)
	}
	if max != nil {
		temp = weather.FormatTemperature(int(math.Round(*max)), tempUnit)
	}
	if min != nil {
		feels = weather.FormatTemperature(int(math.Round(*min)), tempUnit)
	}
	if summary != "" {
		desc = weather.FormatDescription(summary, lm)
	}
	return
}
