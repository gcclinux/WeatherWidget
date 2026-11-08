//go:build windows

package uiwindows

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"log"
	"math"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"
)

const (
	enhancedWidth        = 550
	enhancedInfoLeft     = 96
	enhancedInfoMinWidth = 118
	enhancedInfoGap      = 6
	enhancedMetricsWidth = 330
	enhancedPollutionTop = 168
)

var (
	enhancedWhite = color.RGBA{255, 255, 255, 255}
	enhancedMuted = color.RGBA{221, 221, 221, 255}
	enhancedCyan  = color.RGBA{0, 190, 255, 255}
)

// enhancedCardSize grows for additional selected pollutants, rather than
// squeezing nine labelled values into the width intended for six.
func enhancedCardSize(rows []weather.PollutionRow) (int, int) {
	w := enhancedWidth
	if needed := len(rows)*90 + 12; needed > w {
		w = needed
	}
	h := 172
	if len(rows) > 0 {
		h = 236
	}
	return w, h
}

func (wd *WidgetDrawer) renderEnhancedView(cities []config.CityConfig, results []weather.WeatherResult, cfg *config.Config, lm *i18n.LocaleManager, now time.Time) *image.RGBA {
	const spacing = 6
	width, height, infoWidth := enhancedWidth, 0, enhancedInfoMinWidth
	heights := make([]int, len(cities))
	maxCol1Needed := 0
	for i := range cities {
		var data *weather.WeatherData
		var result *weather.WeatherResult
		if i < len(results) {
			result = &results[i]
			data = result.Data
		}
		infoWidth = max(infoWidth, wd.enhancedInfoWidth(cities[i], result, cfg, lm, now))
		visible := enhancedVisibleMetrics(cfg.GetDisplayFields(), data, cfg, lm)
		maxCol1Needed = max(maxCol1Needed, wd.enhancedMiddleColumnNeededWidth(visible, data))
		rows := weather.PlanPollutionRows(cfg.GetPollutionFields(), weather.PollutionOf(data), lm)
		w, h := enhancedCardSize(rows)
		if w > width {
			width = w
		}
		heights[i] = h
		height += h
	}
	gridLeft := enhancedInfoLeft + infoWidth + enhancedInfoGap
	width = max(width, gridLeft+enhancedMetricsWidth)
	height += (len(cities) - 1) * spacing
	metricsDx := (width - 5) - gridLeft
	edges := computeEnhancedMetricEdges(maxCol1Needed, metricsDx)
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	y := 0
	for i, city := range cities {
		var result *weather.WeatherResult
		if i < len(results) {
			result = &results[i]
		}
		wd.drawEnhancedCard(dst, image.Rect(0, y, width, y+heights[i]), city, result, cfg, lm, now, i, infoWidth, edges)
		y += heights[i] + spacing
	}
	return dst
}

func enhancedDescription(result *weather.WeatherResult, lm *i18n.LocaleManager) string {
	desc := "--"
	if result != nil {
		if result.Data != nil {
			desc = weather.FormatDescription(result.Data.Description, lm)
		}
		if result.HasError && result.IsStale {
			return translateNative(lm, "panel.staleWarning", "Data may be stale")
		}
		if result.HasError && result.Data == nil {
			return "Update failed"
		}
	}
	return desc
}

func enhancedTemperature(result *weather.WeatherResult, cfg *config.Config, lm *i18n.LocaleManager) (string, string) {
	temp, feels := "--°C", "--"
	if result != nil && result.Data != nil {
		temp = weather.FormatTemperature(result.Data.Temperature, cfg.TemperatureUnit)
		feels = weather.FormatFeelsLike(result.Data.FeelsLike, cfg.TemperatureUnit)
	}
	return temp, translateNative(lm, "weather.feelsLike", "Feels Like") + " " + feels
}

func (wd *WidgetDrawer) enhancedInfoWidth(city config.CityConfig, result *weather.WeatherResult, cfg *config.Config, lm *i18n.LocaleManager, now time.Time) int {
	fields := cfg.GetDisplayFields()
	width := enhancedInfoMinWidth
	if fields.ShowTime {
		width = max(width, measureText(weather.FormatTime(now, city.Timezone, lm), wd.getFace(true, float64(cfg.GetFontSizeCityTime())+10))+6)
	}
	if fields.ShowDate {
		width = max(width, measureText(weather.FormatDate(now, city.Timezone, lm), wd.getFace(false, 12))+6)
	}
	if fields.ShowTemp {
		temp, label := enhancedTemperature(result, cfg, lm)
		width = max(width, measureText(temp, wd.getFace(true, float64(cfg.GetFontSizeTempIcon())+4))+6)
		width = max(width, measureText(label, wd.getFace(true, 13))+24)
	}
	if fields.ShowDesc {
		width = max(width, measureText(enhancedDescription(result, lm), wd.getFace(false, float64(cfg.GetFontSizeConditions())+3))+6)
	}
	return width
}

func (wd *WidgetDrawer) background(night bool) image.Image {
	name := "day.jpg"
	if night {
		name = "night.jpg"
	}
	key := "backgrounds/" + name
	wd.mu.Lock()
	defer wd.mu.Unlock()
	if img, ok := wd.iconCache[key]; ok {
		return img
	}
	data, err := assets.Backgrounds.ReadFile(key)
	if err != nil {
		log.Printf("uiwindows: failed to load %s: %v", key, err)
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("uiwindows: failed to decode %s: %v", key, err)
		return nil
	}
	wd.iconCache[key] = img
	return img
}

func (wd *WidgetDrawer) cardBackground(w, h int, night bool, opacity int) *image.RGBA {
	if opacity < 25 {
		opacity = 25
	}
	if opacity > 100 {
		opacity = 100
	}
	key := fmt.Sprintf("%dx%d:%t:%d", w, h, night, opacity)
	wd.mu.Lock()
	cached := wd.backgroundCache[key]
	wd.mu.Unlock()
	if cached != nil {
		return cached
	}
	bg := wd.background(night)
	if bg == nil {
		return nil
	}
	rect := image.Rect(0, 0, w, h)
	scaled := image.NewRGBA(rect)
	xdraw.BiLinear.Scale(scaled, rect, bg, bg.Bounds(), draw.Src, nil)
	fillRoundedRect(scaled, 0, 0, w, h, 12, color.RGBA{0, 0, 0, 120})
	mask := image.NewRGBA(rect)
	fillRoundedRect(mask, 0, 0, w, h, 12, color.RGBA{255, 255, 255, uint8(opacity * 255 / 100)})
	layer := image.NewRGBA(rect)
	draw.DrawMask(layer, rect, scaled, image.Point{}, mask, image.Point{}, draw.Src)
	wd.mu.Lock()
	wd.backgroundCache[key] = layer
	wd.mu.Unlock()
	return layer
}

// smokeAlpha maps the transparency slider (25–100) to the dark tint drawn when
// the panel background is removed: 100 is fully transparent, 25 is the darkest.
func smokeAlpha(opacity int) uint8 {
	if opacity <= 0 || opacity > 100 {
		opacity = 100
	}
	if opacity < 25 {
		opacity = 25
	}
	const maxAlpha = 0.70
	return uint8(float64(100-opacity) / 75 * maxAlpha * 255)
}

// drawSmoke fills the card with a translucent dark shadow for transparent mode.
func drawSmoke(dst *image.RGBA, rect image.Rectangle, opacity int) {
	if a := smokeAlpha(opacity); a > 0 {
		fillRoundedRect(dst, rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy(), 12, color.RGBA{0, 0, 0, a})
	}
}

func (wd *WidgetDrawer) drawEnhancedCard(dst *image.RGBA, rect image.Rectangle, city config.CityConfig, result *weather.WeatherResult, cfg *config.Config, lm *i18n.LocaleManager, now time.Time, index, infoWidth int, edges [4]int) {
	x, y, w, h := rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy()
	local := now.UTC()
	if loc, err := time.LoadLocation(city.Timezone); err == nil {
		local = now.In(loc)
	}
	if cfg.NoBackground {
		drawSmoke(dst, rect, cfg.Opacity)
	} else {
		if bg := wd.cardBackground(w, h, weather.IsNight(local), cfg.Opacity); bg != nil {
			draw.Draw(dst, rect, bg, image.Point{}, draw.Over)
		}
	}
	strokeRoundedRect(dst, x, y, w, h, 12, color.RGBA{255, 255, 255, 100}, 1)

	fields := cfg.GetDisplayFields()
	var data *weather.WeatherData
	if result != nil {
		data = result.Data
	}
	cityName := city.Name
	if city.Region != "" {
		cityName += ", " + city.Region
	}
	cityFace := wd.getFace(true, float64(cfg.GetFontSizeCityTime())+4)
	headerEnd := x + 8
	if fields.ShowCity {
		drawLocationPin(dst, x+10, y+7)
		drawText(dst, cityName, x+27, y+21, cityFace, enhancedWhite)
		headerEnd += measureText(cityName, cityFace) + 39
	}
	wd.drawEnhancedTicker(dst, image.Rect(headerEnd, y+2, x+w-7, y+27), cfg, data, lm, now, index)

	timeFace := wd.getFace(true, float64(cfg.GetFontSizeCityTime())+10)
	dateFace := wd.getFace(false, 12)
	tempFace := wd.getFace(true, float64(cfg.GetFontSizeTempIcon())+4)
	descFace := wd.getFace(false, float64(cfg.GetFontSizeConditions())+3)
	infoCenter := x + enhancedInfoLeft + infoWidth/2
	if fields.ShowTime {
		drawCentered(dst, weather.FormatTime(now, city.Timezone, lm), infoCenter, y+56, timeFace, enhancedWhite)
	}
	if fields.ShowDate {
		drawCentered(dst, weather.FormatDate(now, city.Timezone, lm), infoCenter, y+76, dateFace, enhancedWhite)
	}
	if fields.ShowIcon {
		code := weather.IconCloudy
		if data != nil {
			code = data.IconCode
		}
		path := "icons/" + weather.MapConditionToIconWithTheme(code, local, cfg.IconTheme)
		drawImage(dst, wd.weatherIcon(path, now), x+10, y+55, 84, 84)
	}
	if fields.ShowTemp {
		temp, label := enhancedTemperature(result, cfg, lm)
		drawCentered(dst, temp, infoCenter, y+116, tempFace, enhancedWhite)
		face := wd.getFace(true, 13)
		pillW := measureText(label, face) + 24
		pillX := infoCenter - pillW/2
		strokeRoundedRect(dst, pillX, y+121, pillW, 21, 10, enhancedCyan, 1)
		drawImage(dst, wd.loadIcon("icons/thermometer.png"), pillX+5, y+122, 10, 18)
		drawText(dst, label, pillX+18, y+136, face, enhancedCyan)
	}
	if fields.ShowDesc {
		drawCentered(dst, enhancedDescription(result, lm), infoCenter, y+158, descFace, enhancedWhite)
	}
	wd.drawEnhancedMetrics(dst, image.Rect(x+enhancedInfoLeft+infoWidth+enhancedInfoGap, y+30, x+w-5, y+165), fields, data, cfg, lm, edges)
	wd.drawEnhancedPollution(dst, image.Rect(x+6, y+enhancedPollutionTop, x+w-6, y+h-4), cfg, data, lm)
}

func translateNative(lm *i18n.LocaleManager, key, fallback string) string {
	if lm != nil {
		if value := lm.T(key); value != "" && value != key {
			return value
		}
	}
	return fallback
}

func drawCentered(dst *image.RGBA, text string, center, baseline int, face font.Face, c color.RGBA) {
	drawText(dst, text, center-measureText(text, face)/2, baseline, face, c)
}

func computeEnhancedMetricEdges(neededPx int, rectDx int) [4]int {
	const (
		baseTotal  = 316
		currentMid = 140
		minMid     = 112 // 140 - 20%
	)
	if rectDx <= 0 {
		rectDx = baseTotal
	}
	neededIn316 := int(math.Ceil(float64(neededPx) * float64(baseTotal) / float64(rectDx)))
	midW := currentMid
	if neededIn316 < currentMid {
		if neededIn316 < minMid {
			midW = minMid
		} else {
			midW = neededIn316
		}
	}
	rem := baseTotal - midW
	col0W := rem / 2
	return [4]int{0, col0W, col0W + midW, baseTotal}
}

func (wd *WidgetDrawer) enhancedMiddleColumnNeededWidth(visible []weather.MetricDisplay, data *weather.WeatherData) int {
	if len(visible) < 2 {
		return 112
	}
	nameFace := wd.getFace(false, 12)
	valueFace := wd.getFace(true, 15)
	maxContentWidth := 0

	var direction int
	if data != nil {
		direction = data.WindDirection
	}

	for _, idx := range []int{1, 4} {
		if idx >= len(visible) {
			continue
		}
		m := visible[idx]
		nw := measureText(m.Name, nameFace)
		if m.Emoji == "💧" {
			nw += 18
		}
		if nw > maxContentWidth {
			maxContentWidth = nw
		}

		vw := 0
		if m.Emoji == "💨" && data != nil && direction != 0 {
			parts := strings.SplitN(m.Value, " "+weather.DegreesToArrow(direction)+" ", 2)
			if len(parts) == 2 {
				vw = measureText(parts[0], valueFace) + 17 + measureText(parts[1], valueFace)
			} else {
				vw = measureText(m.Value, valueFace)
			}
		} else {
			vw = measureText(m.Value, valueFace)
		}
		if vw > maxContentWidth {
			maxContentWidth = vw
		}
	}

	return maxContentWidth + 20
}

func enhancedVisibleMetrics(fields *config.DisplayFields, data *weather.WeatherData, cfg *config.Config, lm *i18n.LocaleManager) []weather.MetricDisplay {
	var timezone string
	var direction int
	if data != nil {
		if !data.LocalTime.IsZero() {
			timezone = data.LocalTime.Location().String()
		}
		direction = data.WindDirection
	}

	type metricCandidate struct {
		show bool
		item weather.MetricDisplay
	}
	var metrics []metricCandidate

	switch fields.GridMode {
	case "today":
		if data != nil && data.Forecast != nil {
			fc := data.Forecast
			metrics = []metricCandidate{
				{true, weather.ForecastTempMaxDisplay(fc.TodayTempMax, cfg.TemperatureUnit, lm)},
				{true, weather.ForecastTempMinDisplay(fc.TodayTempMin, cfg.TemperatureUnit, lm)},
				{true, weather.ForecastPrecipProbDisplay(fc.TodayPrecipProbMax, lm)},
				{true, weather.ForecastPrecipAmountDisplay(fc.TodayPrecipSumMM, lm)},
				{true, weather.ForecastUVMaxDisplay(fc.TodayUVMax, lm)},
				{true, weather.ForecastGustMaxDisplay(fc.TodayGustMax, cfg.WindSpeedUnit, lm)},
			}
		} else {
			metrics = []metricCandidate{
				{true, weather.MetricDisplay{Emoji: "🔺", Name: translateNative(lm, "weather.forecast.tempMax", "High Temp"), Value: "--"}},
				{true, weather.MetricDisplay{Emoji: "🔻", Name: translateNative(lm, "weather.forecast.tempMin", "Low Temp"), Value: "--"}},
				{true, weather.MetricDisplay{Emoji: "☔", Name: translateNative(lm, "weather.forecast.precipProb", "Rain Chance"), Value: "--%"}},
				{true, weather.MetricDisplay{Emoji: "🌧", Name: translateNative(lm, "weather.forecast.precipAmount", "Rain Sum"), Value: "-- mm"}},
				{true, weather.MetricDisplay{Emoji: "☀️", Name: translateNative(lm, "weather.forecast.uvMax", "Peak UV"), Value: "--"}},
				{true, weather.MetricDisplay{Emoji: "💨", Name: translateNative(lm, "weather.forecast.gustMax", "Max Gust"), Value: "--"}},
			}
		}
	case "tomorrow":
		if data != nil && data.Forecast != nil {
			fc := data.Forecast
			metrics = []metricCandidate{
				{true, weather.ForecastTempMaxDisplay(fc.TomorrowTempMax, cfg.TemperatureUnit, lm)},
				{true, weather.ForecastTempMinDisplay(fc.TomorrowTempMin, cfg.TemperatureUnit, lm)},
				{true, weather.ForecastPrecipProbDisplay(fc.TomorrowPrecipProbMax, lm)},
				{true, weather.ForecastPrecipAmountDisplay(fc.TomorrowPrecipSumMM, lm)},
				{true, weather.ForecastUVMaxDisplay(fc.TomorrowUVMax, lm)},
				{true, weather.ForecastGustMaxDisplay(fc.TomorrowGustMax, cfg.WindSpeedUnit, lm)},
			}
		} else {
			metrics = []metricCandidate{
				{true, weather.MetricDisplay{Emoji: "🔺", Name: translateNative(lm, "weather.forecast.tempMax", "High Temp"), Value: "--"}},
				{true, weather.MetricDisplay{Emoji: "🔻", Name: translateNative(lm, "weather.forecast.tempMin", "Low Temp"), Value: "--"}},
				{true, weather.MetricDisplay{Emoji: "☔", Name: translateNative(lm, "weather.forecast.precipProb", "Rain Chance"), Value: "--%"}},
				{true, weather.MetricDisplay{Emoji: "🌧", Name: translateNative(lm, "weather.forecast.precipAmount", "Rain Sum"), Value: "-- mm"}},
				{true, weather.MetricDisplay{Emoji: "☀️", Name: translateNative(lm, "weather.forecast.uvMax", "Peak UV"), Value: "--"}},
				{true, weather.MetricDisplay{Emoji: "💨", Name: translateNative(lm, "weather.forecast.gustMax", "Max Gust"), Value: "--"}},
			}
		}
	default: // "live"
		var humidity, clouds int
		var wind, gust, dew, pressure, uv, vis float64
		var precipRate *float64
		var sunrise, sunset int64
		if data != nil {
			humidity, wind, gust, dew, pressure, uv = data.Humidity, data.WindSpeed, data.WindGust, data.DewPoint, data.Pressure, data.UVIndex
			clouds = data.Clouds
			vis = data.Visibility
			sunrise, sunset = data.Sunrise, data.Sunset
			precipRate = data.PrecipRate
		}
		var pRate float64
		if precipRate != nil {
			pRate = *precipRate
		}
		metrics = []metricCandidate{
			{fields.ShowHumidity, weather.HumidityDisplay(humidity, lm)},
			{fields.ShowWind, weather.WindDisplay(wind, direction, cfg.WindSpeedUnit, lm)},
			{fields.ShowWindGust, weather.WindGustDisplay(gust, cfg.WindSpeedUnit, lm)},
			{fields.ShowDewPoint, weather.DewPointDisplay(dew, lm)},
			{fields.ShowPressure, weather.PressureDisplay(pressure, lm)},
			{fields.ShowUVIndex, weather.UVIndexDisplay(uv, lm)},
			{fields.ShowSunrise, weather.SunriseDisplay(sunrise, timezone, lm)},
			{fields.ShowSunset, weather.SunsetDisplay(sunset, timezone, lm)},
			{fields.ShowClouds, weather.CloudsDisplay(clouds, lm)},
			{fields.ShowVisibility, weather.VisibilityDisplay(vis, lm)},
			{fields.ShowPrecipRate, weather.PrecipRateDisplay(pRate, lm)},
		}
	}

	var visible []weather.MetricDisplay
	for _, metric := range metrics {
		if metric.show {
			if data == nil {
				metric.item.Value = "--"
			}
			visible = append(visible, metric.item)
			if len(visible) == 6 {
				break
			}
		}
	}
	return visible
}

func (wd *WidgetDrawer) drawEnhancedMetrics(dst *image.RGBA, rect image.Rectangle, fields *config.DisplayFields, data *weather.WeatherData, cfg *config.Config, lm *i18n.LocaleManager, edges [4]int) {
	visible := enhancedVisibleMetrics(fields, data, cfg, lm)
	if len(visible) == 0 {
		return
	}
	cols := int(math.Min(3, float64(len(visible))))
	rows := (len(visible) + cols - 1) / cols
	nameFace, valueFace := wd.getFace(false, 12), wd.getFace(true, 15)
	for i, metric := range visible {
		col, row := i%cols, i/cols
		left, right := rect.Min.X+col*rect.Dx()/cols, rect.Min.X+(col+1)*rect.Dx()/cols-2
		if cols == 3 {
			gridEdges := edges
			if gridEdges == [4]int{} {
				gridEdges = [4]int{0, 88, 228, 316}
			}
			left = rect.Min.X + gridEdges[col]*rect.Dx()/316
			right = rect.Min.X + gridEdges[col+1]*rect.Dx()/316 - 2
		}
		top := rect.Min.Y + row*rect.Dy()/rows
		bottom := rect.Min.Y + (row+1)*rect.Dy()/rows - 2
		strokeRoundedRect(dst, left, top, right-left, bottom-top, 0, color.RGBA{255, 255, 255, 36}, 1)
		clipped := dst.SubImage(image.Rect(left+2, top+1, right-2, bottom-1).Intersect(dst.Bounds())).(*image.RGBA)
		center := (left + right) / 2
		nameY := top + (bottom-top)/2 - 5
		if metric.Emoji == "💧" {
			nameWidth := measureText(metric.Name, nameFace)
			drawDroplet(clipped, center-(nameWidth+18)/2, nameY-14, 14)
			drawText(clipped, metric.Name, center-(nameWidth+18)/2+18, nameY, nameFace, enhancedMuted)
		} else {
			drawCentered(clipped, metric.Name, center, nameY, nameFace, enhancedMuted)
		}
		value := metric.Value
		var direction int
		if data != nil {
			direction = data.WindDirection
		}
		if metric.Emoji == "💨" && data != nil && direction != 0 {
			parts := strings.SplitN(value, " "+weather.DegreesToArrow(direction)+" ", 2)
			if len(parts) == 2 {
				face := valueFace
				for size := 14.0; measureText(parts[0], face)+measureText(parts[1], face)+17 > right-left-4 && size >= 10; size-- {
					face = wd.getFace(true, size)
				}
				start := center - (measureText(parts[0], face)+measureText(parts[1], face)+17)/2
				baseline := top + (bottom-top)/2 + 18
				drawText(clipped, parts[0], start, baseline, face, enhancedWhite)
				start += measureText(parts[0], face) + 3
				drawWindArrow(clipped, start+5, baseline-5, direction)
				drawText(clipped, parts[1], start+14, baseline, face, enhancedWhite)
				continue
			}
		}
		drawCentered(clipped, value, center, top+(bottom-top)/2+18, valueFace, enhancedWhite)
	}
}

func drawLocationPin(dst *image.RGBA, x, y int) {
	for py := 0; py < 12; py++ {
		for px := 0; px < 12; px++ {
			dx, dy := px-6, py-5
			if dx*dx+dy*dy <= 16 {
				blendPixel(dst, x+px, y+py, color.RGBA{231, 74, 101, 255})
			}
		}
	}
	for i := 0; i < 7; i++ {
		blendPixel(dst, x+4-i/2, y+9+i, enhancedWhite)
	}
}

func drawWindArrow(dst *image.RGBA, x, y, direction int) {
	angle := float64(direction) * math.Pi / 180
	dx, dy := math.Sin(angle), -math.Cos(angle)
	for step := -5; step <= 5; step++ {
		blendPixel(dst, x+int(math.Round(float64(step)*dx)), y+int(math.Round(float64(step)*dy)), enhancedWhite)
	}
	for step := 0; step < 4; step++ {
		for _, sign := range []float64{-1, 1} {
			px := 5*dx - float64(step)*dx + sign*float64(step)*dy*0.6
			py := 5*dy - float64(step)*dy - sign*float64(step)*dx*0.6
			blendPixel(dst, x+int(math.Round(px)), y+int(math.Round(py)), enhancedWhite)
		}
	}
}

func drawDroplet(dst *image.RGBA, x, y, size int) {
	s := float64(size)
	cx, tipY, radius := s/2, s*0.04, s*0.34
	cy := s - radius - s*0.04
	d := cy - tipY
	tanA := radius / math.Sqrt(d*d-radius*radius)
	tangentY := cy - radius*radius/d
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			fx, fy := float64(px)+0.5, float64(py)+0.5
			inside := fy >= tipY && fy <= tangentY && math.Abs(fx-cx) <= (fy-tipY)*tanA
			if fy > tangentY {
				inside = (fx-cx)*(fx-cx)+(fy-cy)*(fy-cy) <= radius*radius
			}
			if inside {
				blendPixel(dst, x+px, y+py, color.RGBA{35, 155, 255, 255})
			}
		}
	}
}

func (wd *WidgetDrawer) drawEnhancedPollution(dst *image.RGBA, rect image.Rectangle, cfg *config.Config, data *weather.WeatherData, lm *i18n.LocaleManager) {
	rows := weather.PlanPollutionRows(cfg.GetPollutionFields(), weather.PollutionOf(data), lm)
	if len(rows) == 0 {
		return
	}
	nameFace, valueFace := wd.getFace(false, 10), wd.getFace(true, 12)
	// AQI stays on the left; the other metrics pack from the right edge.
	slots := len(rows)
	if slots < 7 {
		slots = 7
	}
	cell := rect.Dx() / slots
	for i, row := range rows {
		var left, right int
		if row.Metric == weather.MetricAQI {
			aqiW := cell
			if aqiW < 110 {
				aqiW = 110
			}
			left, right = rect.Min.X, rect.Min.X+aqiW
		} else {
			right = rect.Max.X - (len(rows)-1-i)*cell
			left = right - cell
		}
		center := (left + right) / 2
		drawImageContained(dst, wd.loadIcon("air/"+row.IconFile), image.Rect(center-15, rect.Min.Y+2, center+15, rect.Min.Y+23))
		name := weather.PollutionMetricName(row.Metric, lm)
		keys := []string{"aqi", "co", "no", "no2", "o3", "so2", "nh3", "pm25", "pm10"}
		if int(row.Metric) < len(keys) {
			name = translateNative(lm, "pollution.short."+keys[row.Metric], name)
		}
		clipped := dst.SubImage(rect.Intersect(dst.Bounds())).(*image.RGBA)
		drawCentered(clipped, name, center, rect.Min.Y+39, nameFace, enhancedMuted)
		drawCentered(clipped, row.ValueText, center, rect.Min.Y+61, valueFace, enhancedWhite)
	}
}

func (wd *WidgetDrawer) drawEnhancedTicker(dst *image.RGBA, rect image.Rectangle, cfg *config.Config, data *weather.WeatherData, lm *i18n.LocaleManager, now time.Time, index int) {
	items := weather.PlanPollutionTickerWithWeather(cfg.GetPollutionFields(), data, cfg.TemperatureUnit, lm)
	rect = rect.Intersect(dst.Bounds())
	if len(items) == 0 || rect.Dx() < 20 || rect.Dy() == 0 {
		return
	}
	bold, regular := wd.getFace(true, 12), wd.getFace(false, 12)
	type segment struct {
		head, status, tail string
		width, duration    int
		level              int
	}
	segments := make([]segment, len(items))
	total := 0
	for i, item := range items {
		head := item.Symbol
		if item.FullName != "" && item.FullName != item.Symbol {
			head += " (" + item.FullName + ")"
		}
		head += ": "
		status := weather.StripEmoji(item.StatusText)
		tail := " • " + weather.StripEmoji(item.SourcesText)
		width := measureText(head, bold) + measureText(status, bold) + measureText(tail, regular)
		duration := (rect.Dx() + width + 30) * 25
		segments[i] = segment{head, status, tail, width, duration, item.StatusLevel}
		total += duration
	}
	elapsed := int((now.UnixMilli() + int64(index*4000)) % int64(total))
	current := segments[0]
	for _, s := range segments {
		current = s
		if elapsed < s.duration {
			break
		}
		elapsed -= s.duration
	}
	colors := []color.RGBA{{192, 132, 252, 255}, {52, 211, 153, 255}, {251, 191, 36, 255}, {251, 146, 60, 255}, {248, 113, 113, 255}}
	level := current.level
	if level < 1 || level > 4 {
		level = 0
	}
	clipped := dst.SubImage(rect).(*image.RGBA)
	x := rect.Max.X - elapsed/25
	drawText(clipped, current.head, x, rect.Min.Y+17, bold, enhancedWhite)
	x += measureText(current.head, bold)
	drawText(clipped, current.status, x, rect.Min.Y+17, bold, colors[level])
	x += measureText(current.status, bold)
	drawText(clipped, current.tail, x, rect.Min.Y+17, regular, enhancedCyan)
}
