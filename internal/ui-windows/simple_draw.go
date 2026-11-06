//go:build windows

package uiwindows

import (
	"image"
	"image/color"
	"image/draw"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"
)

func simpleCardHeight(fields *config.DisplayFields, rows []weather.PollutionRow, cfg *config.Config) int {
	citySize, tempSize, conditionsSize := cfg.GetFontSizeCityTime()+4, cfg.GetFontSizeTempIcon()+10, cfg.GetFontSizeConditions()+2
	h := 16
	if fields.ShowCity {
		h += max(0, citySize+2-20) + max(27, citySize+9)
	}
	if fields.ShowIcon || fields.ShowTemp {
		h += 70
		if fields.ShowTemp {
			h += max(0, tempSize-42)
		}
	}
	if fields.ShowTemp {
		h += max(49, tempSize/4+conditionsSize+8)
	}
	if fields.ShowDesc {
		h += max(21, conditionsSize+9)
	}
	for _, show := range []bool{fields.ShowHumidity, fields.ShowWind, fields.ShowWindGust, fields.ShowDewPoint, fields.ShowPressure, fields.ShowUVIndex} {
		if show {
			h += max(22, conditionsSize+10)
		}
	}
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

func (wd *WidgetDrawer) renderSimpleView(cities []config.CityConfig, results []weather.WeatherResult, cfg *config.Config, lm *i18n.LocaleManager, now time.Time) *image.RGBA {
	const gap = 6
	fields := cfg.GetDisplayFields()
	width := 160 + max(0, cfg.GetFontSizeConditions()+2-12)*13
	height := 0
	for i, city := range cities {
		var data *weather.WeatherData
		if i < len(results) {
			data = results[i].Data
		}
		rows := weather.PlanPollutionRows(cfg.GetPollutionFields(), weather.PollutionOf(data), lm)
		if h := simpleCardHeight(fields, rows, cfg); h > height {
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
			value := "--°C"
			if data != nil {
				value = weather.FormatTemperature(data.Temperature, cfg.TemperatureUnit)
			}
			width = max(width, measureText(value, wd.getFace(true, float64(cfg.GetFontSizeTempIcon()+10)))+16)
		}
		if fields.ShowDesc && data != nil {
			width = max(width, measureText(weather.FormatDescription(data.Description, lm), wd.getFace(false, float64(cfg.GetFontSizeConditions()+2)))+16)
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, len(cities)*width+(len(cities)-1)*gap, height))
	for i, city := range cities {
		var result *weather.WeatherResult
		if i < len(results) {
			result = &results[i]
		}
		wd.drawSimpleCard(dst, image.Rect(i*(width+gap), 0, i*(width+gap)+width, height), city, result, cfg, lm, now)
	}
	return dst
}

func (wd *WidgetDrawer) drawSimpleCard(dst *image.RGBA, rect image.Rectangle, city config.CityConfig, result *weather.WeatherResult, cfg *config.Config, lm *i18n.LocaleManager, now time.Time) {
	text := enhancedWhite
	if cfg.NoBackground {
		drawSmoke(dst, rect, cfg.Opacity)
	} else {
		local := now.UTC()
		if loc, err := time.LoadLocation(city.Timezone); err == nil {
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
	if fields.ShowIcon || fields.ShowTemp {
		if fields.ShowIcon {
			local := now.UTC()
			if loc, err := time.LoadLocation(city.Timezone); err == nil {
				local = now.In(loc)
			}
			code := weather.IconCloudy
			if data != nil {
				code = data.IconCode
			}
			iconX := center - 32
			if fields.ShowTemp {
				iconX = center - 56
			}
			path := "icons/" + weather.MapConditionToIconWithTheme(code, local, cfg.IconTheme)
			drawImage(dst, wd.weatherIcon(path, now), iconX, y-15, 64, 64)
		}
		if fields.ShowTemp {
			feels := "--"
			if data != nil {
				feels = weather.FormatFeelsLike(data.FeelsLike, cfg.TemperatureUnit)
			}
			drawImage(dst, wd.loadIcon("icons/thermometer.png"), center+14, y-12, 14, 35)
			drawText(dst, feels, center+8, y+43, wd.getFace(true, 14), enhancedCyan)
		}
		y += 70
		if fields.ShowTemp {
			y += max(0, tempSize-42)
		}
	}
	if fields.ShowTemp {
		value := "--°C"
		if data != nil {
			value = weather.FormatTemperature(data.Temperature, cfg.TemperatureUnit)
		}
		drawCentered(dst, value, center, y, wd.getFace(true, float64(tempSize)), text)
		y += max(49, tempSize/4+conditionsSize+8)
	}
	if fields.ShowDesc {
		desc := "--"
		if data != nil {
			desc = weather.FormatDescription(data.Description, lm)
		}
		if result != nil && result.IsStale {
			desc = translateNative(lm, "panel.staleWarning", "Data may be stale")
		}
		drawCentered(dst, desc, center, y, wd.getFace(false, float64(conditionsSize)), text)
		y += max(21, conditionsSize+9)
	}
	var humidity, direction int
	var wind, gust, dew, pressure, uv float64
	if data != nil {
		humidity, direction = data.Humidity, data.WindDirection
		wind, gust, dew, pressure, uv = data.WindSpeed, data.WindGust, data.DewPoint, data.Pressure, data.UVIndex
	}
	metrics := []struct {
		show bool
		item weather.MetricDisplay
	}{
		{fields.ShowHumidity, weather.HumidityDisplay(humidity, lm)},
		{fields.ShowWind, weather.WindDisplay(wind, direction, cfg.WindSpeedUnit, lm)},
		{fields.ShowWindGust, weather.WindGustDisplay(gust, cfg.WindSpeedUnit, lm)},
		{fields.ShowDewPoint, weather.DewPointDisplay(dew, lm)},
		{fields.ShowPressure, weather.PressureDisplay(pressure, lm)},
		{fields.ShowUVIndex, weather.UVIndexDisplay(uv, lm)},
	}
	for _, metric := range metrics {
		if metric.show {
			value := metric.item.Value
			if data == nil {
				value = "--"
			}
			drawCentered(dst, metric.item.Name+": "+value, center, y, wd.getFace(false, float64(conditionsSize)), text)
			y += max(22, conditionsSize+10)
		}
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
