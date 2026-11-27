//go:build windows

package uiwindows

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"
)

func wrapDescription(text string, maxChars int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	// An explicit line break (forecast title) is kept as is.
	if strings.Contains(text, "\n") {
		parts := strings.Split(text, "\n")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	if maxChars <= 0 {
		maxChars = 18
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var current strings.Builder
	currentLen := 0
	for _, word := range words {
		wLen := len([]rune(word))
		if currentLen == 0 {
			current.WriteString(word)
			currentLen = wLen
		} else if currentLen+1+wLen <= maxChars {
			current.WriteByte(' ')
			current.WriteString(word)
			currentLen += 1 + wLen
		} else {
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(word)
			currentLen = wLen
		}
	}
	if currentLen > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

func simpleCardMetricCount(fields *config.DisplayFields) int {
	switch fields.GridMode {
	case "today", "tomorrow":
		return 6
	default:
		count := 0
		for _, show := range []bool{fields.ShowHumidity, fields.ShowWind, fields.ShowWindGust, fields.ShowDewPoint, fields.ShowPressure, fields.ShowUVIndex, fields.ShowSunrise, fields.ShowSunset, fields.ShowClouds, fields.ShowVisibility, fields.ShowPrecipRate} {
			if show {
				count++
			}
		}
		return count
	}
}

func simpleCardHeight(fields *config.DisplayFields, rows []weather.PollutionRow, cfg *config.Config) int {
	return simpleCardHeightWithLines(fields, 1, rows, cfg)
}

func simpleCardHeightWithLines(fields *config.DisplayFields, descLines int, rows []weather.PollutionRow, cfg *config.Config) int {
	citySize, tempSize, conditionsSize := cfg.GetFontSizeCityTime()+4, cfg.GetFontSizeTempIcon()+10, cfg.GetFontSizeConditions()+2
	h := 16
	if fields.ShowCity {
		h += max(0, citySize+2-20) + max(27, citySize+9)
	}
	if fields.ShowIcon || fields.ShowTemp {
		if fields.ShowIcon && !fields.ShowTemp {
			h += 60
		} else {
			h += 92
			if fields.ShowTemp {
				h += max(0, tempSize-42)
			}
		}
	}
	if fields.ShowTemp {
		h += max(28, tempSize/4+conditionsSize+4)
	}
	if fields.ShowDesc {
		h += max(21, conditionsSize+9)
		if descLines > 1 {
			h += (descLines - 1) * max(14, conditionsSize+4)
		}
	}
	h += simpleCardMetricCount(fields) * max(22, conditionsSize+10)
	h += len(rows) * max(23, conditionsSize+11)
	if fields.ShowTime || fields.ShowDate {
		h += 12
		if fields.ShowTime {
			timeSize := cfg.GetFontSizeCityTime() + 8
			h += max(0, timeSize+2-22) + max(27, timeSize/4+15)
		}
		if fields.ShowDate {
			h += 20
		}
	}
	return h + 8
}

func simpleCardWeather(data *weather.WeatherData, fields *config.DisplayFields, cfg *config.Config, lm *i18n.LocaleManager) (iconCode, tempStr, feelsStr, descStr string) {
	iconCode = weather.IconCloudy
	tempStr = "--°C"
	feelsStr = "--"
	descStr = "--"

	if data == nil {
		return iconCode, tempStr, feelsStr, descStr
	}

	iconCode = data.IconCode
	tempStr = weather.FormatTemperature(data.Temperature, cfg.TemperatureUnit)
	feelsStr = weather.FormatFeelsLike(data.FeelsLike, cfg.TemperatureUnit)
	descStr = weather.FormatDescription(data.Description, lm)

	// The icon, temperature and feels-like stay the current conditions in every
	// grid mode. Forecast modes show a short localized two-line title instead of
	// the (long) forecast summary, which would widen the card.
	if title := i18n.ForecastTitle(lm, fields.GridMode); title != "" {
		descStr = title
	}

	return iconCode, tempStr, feelsStr, descStr
}

func simpleMetrics(fields *config.DisplayFields, data *weather.WeatherData, cfg *config.Config, lm *i18n.LocaleManager) []weather.MetricDisplay {
	switch fields.GridMode {
	case "today":
		if data != nil && data.Forecast != nil {
			fc := data.Forecast
			return []weather.MetricDisplay{
				weather.ForecastTempMaxDisplay(fc.TodayTempMax, cfg.TemperatureUnit, lm),
				weather.ForecastTempMinDisplay(fc.TodayTempMin, cfg.TemperatureUnit, lm),
				weather.ForecastPrecipProbDisplay(fc.TodayPrecipProbMax, lm),
				weather.ForecastPrecipAmountDisplay(fc.TodayPrecipSumMM, lm),
				weather.ForecastUVMaxDisplay(fc.TodayUVMax, lm),
				weather.ForecastGustMaxDisplay(fc.TodayGustMax, cfg.WindSpeedUnit, lm),
			}
		}
		return []weather.MetricDisplay{
			{Emoji: "🔺", Name: translateNative(lm, "weather.forecast.tempMax", "High Temp"), Value: "--"},
			{Emoji: "🔻", Name: translateNative(lm, "weather.forecast.tempMin", "Low Temp"), Value: "--"},
			{Emoji: "☔", Name: translateNative(lm, "weather.forecast.precipProb", "Rain Chance"), Value: "--%"},
			{Emoji: "🌧", Name: translateNative(lm, "weather.forecast.precipAmount", "Rain Sum"), Value: "-- mm"},
			{Emoji: "☀️", Name: translateNative(lm, "weather.forecast.uvMax", "Peak UV"), Value: "--"},
			{Emoji: "💨", Name: translateNative(lm, "weather.forecast.gustMax", "Max Gust"), Value: "--"},
		}
	case "tomorrow":
		if data != nil && data.Forecast != nil {
			fc := data.Forecast
			return []weather.MetricDisplay{
				weather.ForecastTempMaxDisplay(fc.TomorrowTempMax, cfg.TemperatureUnit, lm),
				weather.ForecastTempMinDisplay(fc.TomorrowTempMin, cfg.TemperatureUnit, lm),
				weather.ForecastPrecipProbDisplay(fc.TomorrowPrecipProbMax, lm),
				weather.ForecastPrecipAmountDisplay(fc.TomorrowPrecipSumMM, lm),
				weather.ForecastUVMaxDisplay(fc.TomorrowUVMax, lm),
				weather.ForecastGustMaxDisplay(fc.TomorrowGustMax, cfg.WindSpeedUnit, lm),
			}
		}
		return []weather.MetricDisplay{
			{Emoji: "🔺", Name: translateNative(lm, "weather.forecast.tempMax", "High Temp"), Value: "--"},
			{Emoji: "🔻", Name: translateNative(lm, "weather.forecast.tempMin", "Low Temp"), Value: "--"},
			{Emoji: "☔", Name: translateNative(lm, "weather.forecast.precipProb", "Rain Chance"), Value: "--%"},
			{Emoji: "🌧", Name: translateNative(lm, "weather.forecast.precipAmount", "Rain Sum"), Value: "-- mm"},
			{Emoji: "☀️", Name: translateNative(lm, "weather.forecast.uvMax", "Peak UV"), Value: "--"},
			{Emoji: "💨", Name: translateNative(lm, "weather.forecast.gustMax", "Max Gust"), Value: "--"},
		}
	default:
		var list []weather.MetricDisplay
		var humidity, direction int
		var wind, gust, dew, pressure, uv float64
		if data != nil {
			humidity, direction = data.Humidity, data.WindDirection
			wind, gust, dew, pressure, uv = data.WindSpeed, data.WindGust, data.DewPoint, data.Pressure, data.UVIndex
		}
		if fields.ShowHumidity {
			item := weather.HumidityDisplay(humidity, lm)
			if data == nil {
				item.Value = "--"
			}
			list = append(list, item)
		}
		if fields.ShowWind {
			item := weather.WindDisplay(wind, direction, cfg.WindSpeedUnit, lm)
			if data == nil {
				item.Value = "--"
			}
			list = append(list, item)
		}
		if fields.ShowWindGust {
			item := weather.WindGustDisplay(gust, cfg.WindSpeedUnit, lm)
			if data == nil {
				item.Value = "--"
			}
			list = append(list, item)
		}
		if fields.ShowDewPoint {
			item := weather.DewPointDisplay(dew, lm)
			if data == nil {
				item.Value = "--"
			}
			list = append(list, item)
		}
		if fields.ShowPressure {
			item := weather.PressureDisplay(pressure, lm)
			if data == nil {
				item.Value = "--"
			}
			list = append(list, item)
		}
		if fields.ShowUVIndex {
			item := weather.UVIndexDisplay(uv, lm)
			if data == nil {
				item.Value = "--"
			}
			list = append(list, item)
		}
		var clouds int
		var vis, precip float64
		var sunrise, sunset int64
		var timezone string
		if data != nil {
			clouds, vis, sunrise, sunset = data.Clouds, data.Visibility, data.Sunrise, data.Sunset
			if data.PrecipRate != nil {
				precip = *data.PrecipRate
			}
			if !data.LocalTime.IsZero() {
				timezone = data.LocalTime.Location().String()
			}
		}
		for _, c := range []struct {
			show bool
			item weather.MetricDisplay
		}{
			{fields.ShowSunrise, weather.SunriseDisplay(sunrise, timezone, lm)},
			{fields.ShowSunset, weather.SunsetDisplay(sunset, timezone, lm)},
			{fields.ShowClouds, weather.CloudsDisplay(clouds, lm)},
			{fields.ShowVisibility, weather.VisibilityDisplay(vis, lm)},
			{fields.ShowPrecipRate, weather.PrecipRateDisplay(precip, lm)},
		} {
			if !c.show {
				continue
			}
			if data == nil {
				c.item.Value = "--"
			}
			list = append(list, c.item)
		}
		return list
	}
}

func (wd *WidgetDrawer) renderSimpleView(cities []config.CityConfig, results []weather.WeatherResult, cfg *config.Config, lm *i18n.LocaleManager, now time.Time) *image.RGBA {
	const gap = 6
	fields := cfg.GetDisplayFields()
	width := 160 + max(0, cfg.GetFontSizeConditions()+2-12)*13
	height := 0
	maxDescLines := 1
	if fields.ShowDesc {
		for i := range cities {
			var data *weather.WeatherData
			if i < len(results) {
				data = results[i].Data
			}
			_, _, _, descStr := simpleCardWeather(data, fields, cfg, lm)
			if i < len(results) && results[i].IsStale {
				descStr = translateNative(lm, "panel.staleWarning", "Data may be stale")
			}
			lines := wrapDescription(descStr, 18)
			if len(lines) > maxDescLines {
				maxDescLines = len(lines)
			}
		}
	}
	for i, city := range cities {
		var data *weather.WeatherData
		if i < len(results) {
			data = results[i].Data
		}
		metrics := simpleMetrics(fields, data, cfg, lm)
		rows := weather.PlanPollutionRows(cfg.GetPollutionFields(), weather.PollutionOf(data), lm)
		if h := simpleCardHeightWithLines(fields, maxDescLines, rows, cfg); h > height {
			height = h
		}
		if fields.ShowCity {
			name := city.Name
			if city.Region != "" {
				name += ", " + city.Region
			}
			width = max(width, measureText(name, wd.getFace(true, float64(cfg.GetFontSizeCityTime()+4)))+16)
		}
		if fields.ShowTime {
			width = max(width, measureText(weather.FormatTime(now, city.Timezone, lm), wd.getFace(true, float64(cfg.GetFontSizeCityTime()+8)))+16)
		}
		if fields.ShowTemp {
			_, tempStr, _, _ := simpleCardWeather(data, fields, cfg, lm)
			width = max(width, measureText(tempStr, wd.getFace(true, float64(cfg.GetFontSizeTempIcon()+10)))+16)
		}
		if fields.ShowDesc {
			_, _, _, descStr := simpleCardWeather(data, fields, cfg, lm)
			if i < len(results) && results[i].IsStale {
				descStr = translateNative(lm, "panel.staleWarning", "Data may be stale")
			}
			lines := wrapDescription(descStr, 18)
			for _, line := range lines {
				width = max(width, measureText(line, wd.getFace(false, float64(cfg.GetFontSizeConditions()+2)))+16)
			}
		}
		for _, m := range metrics {
			width = max(width, measureText(m.Name+": "+m.Value, wd.getFace(false, float64(cfg.GetFontSizeConditions()+2)))+16)
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, len(cities)*width+(len(cities)-1)*gap, height))
	for i, city := range cities {
		var result *weather.WeatherResult
		if i < len(results) {
			result = &results[i]
		}
		wd.drawSimpleCard(dst, image.Rect(i*(width+gap), 0, i*(width+gap)+width, height), city, result, cfg, lm, now, maxDescLines)
	}
	return dst
}

func (wd *WidgetDrawer) drawSimpleCard(dst *image.RGBA, rect image.Rectangle, city config.CityConfig, result *weather.WeatherResult, cfg *config.Config, lm *i18n.LocaleManager, now time.Time, maxDescLines int) {
	text := enhancedWhite
	if cfg.NoBackground {
		drawSmoke(dst, rect, cfg.Opacity)
	} else {
		local := now.UTC()
		if loc, err := weather.LoadLocationCached(city.Timezone); err == nil {
			local = now.In(loc)
		}
		if bg := wd.cardBackground(rect.Dx(), rect.Dy(), weather.IsNight(local), cfg.Opacity); bg != nil {
			draw.Draw(dst, rect, bg, image.Point{}, draw.Over)
		}
	}
	strokeRoundedRect(dst, rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy(), 12, color.RGBA{255, 255, 255, 100}, 1)
	fields := cfg.GetDisplayFields()
	var data *weather.WeatherData
	if result != nil {
		data = result.Data
	}
	citySize, tempSize, conditionsSize := cfg.GetFontSizeCityTime()+4, cfg.GetFontSizeTempIcon()+10, cfg.GetFontSizeConditions()+2
	center, y := (rect.Min.X+rect.Max.X)/2, rect.Min.Y+20
	if fields.ShowCity {
		y += max(0, citySize+2-20)
		name := city.Name
		if city.Region != "" {
			name += ", " + city.Region
		}
		drawCentered(dst, name, center, y, wd.getFace(true, float64(citySize)), text)
		y += max(27, citySize+9)
	}

	iconCode, tempStr, feelsStr, descStr := simpleCardWeather(data, fields, cfg, lm)

	if fields.ShowIcon || fields.ShowTemp {
		if fields.ShowIcon {
			local := now.UTC()
			if loc, err := weather.LoadLocationCached(city.Timezone); err == nil {
				local = now.In(loc)
			}
			iconX := center - 32
			if fields.ShowTemp {
				iconX = center - 56
			}
			path := "icons/" + weather.MapConditionToIconWithTheme(iconCode, local, cfg.IconTheme)
			drawImage(dst, wd.weatherIcon(path, now), iconX, y-15, 64, 64)
		}
		if fields.ShowTemp {
			drawImage(dst, wd.loadIcon("icons/thermometer.png"), center+24, y-12, 14, 35)
			drawText(dst, feelsStr, center+18, y+43, wd.getFace(true, 14), enhancedCyan)
		}
		if fields.ShowIcon && !fields.ShowTemp {
			y += 60
		} else {
			y += 92
			if fields.ShowTemp {
				y += max(0, tempSize-42)
			}
		}
	}
	if fields.ShowTemp {
		drawCentered(dst, tempStr, center, y, wd.getFace(true, float64(tempSize)), text)
		y += max(28, tempSize/4+conditionsSize+4)
	}
	if fields.ShowDesc {
		desc := descStr
		if result != nil && result.IsStale {
			desc = translateNative(lm, "panel.staleWarning", "Data may be stale")
		}
		lines := wrapDescription(desc, 18)
		if len(lines) == 0 && desc != "" {
			lines = []string{desc}
		}
		lineHeight := max(14, conditionsSize+4)
		descColor := text
		if (result == nil || !result.IsStale) && i18n.ForecastTitle(lm, fields.GridMode) != "" {
			descColor = enhancedCyan // forecast title shares the feels-like colour
		}
		for _, line := range lines {
			drawCentered(dst, line, center, y, wd.getFace(false, float64(conditionsSize)), descColor)
			y += lineHeight
		}
		if maxDescLines > len(lines) {
			y += (maxDescLines - len(lines)) * lineHeight
		}
		y += max(21, conditionsSize+9) - lineHeight
	}

	for _, metric := range simpleMetrics(fields, data, cfg, lm) {
		drawCentered(dst, metric.Name+": "+metric.Value, center, y, wd.getFace(false, float64(conditionsSize)), text)
		y += max(22, conditionsSize+10)
	}

	for _, row := range weather.PlanPollutionRows(cfg.GetPollutionFields(), weather.PollutionOf(data), lm) {
		icon := wd.loadIcon("air/" + row.IconFile)
		label := row.ValueText
		face := wd.getFace(false, float64(conditionsSize))
		start := center - (18+4+measureText(label, face))/2
		drawImageContained(dst, icon, image.Rect(start, y-14, start+18, y+4))
		drawText(dst, label, start+22, y, face, text)
		y += max(23, conditionsSize+11)
	}
	if fields.ShowTime || fields.ShowDate {
		drawHorizontalLine(dst, rect.Min.X+20, rect.Max.X-20, y, color.RGBA{180, 180, 180, 100})
		y += 22
		if fields.ShowTime {
			timeSize := cfg.GetFontSizeCityTime() + 8
			y += max(0, timeSize+2-22)
			drawCentered(dst, weather.FormatTime(now, city.Timezone, lm), center, y, wd.getFace(true, float64(timeSize)), text)
			y += max(27, timeSize/4+15)
		}
		if fields.ShowDate {
			drawCentered(dst, weather.FormatDate(now, city.Timezone, lm), center, y, wd.getFace(false, 11), text)
		}
	}
}
