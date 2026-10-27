//go:build windows

package uiwindows

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"sync"
	"time"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/fixed"
)

// WidgetDrawer handles all 32-bit ARGB offscreen drawing for desktop cards.
type WidgetDrawer struct {
	mu sync.Mutex

	// Cached font families
	regularFont *opentype.Font
	boldFont    *opentype.Font

	// Cached font faces key: "bold/regular:size"
	faceCache map[string]font.Face

	// Cached decoded icons key: path
	iconCache map[string]image.Image
}

// NewWidgetDrawer creates and initializes the card drawer.
func NewWidgetDrawer() *WidgetDrawer {
	wd := &WidgetDrawer{
		faceCache: make(map[string]font.Face),
		iconCache: make(map[string]image.Image),
	}

	// Load Segoe UI regular
	if data, err := assets.Fonts.ReadFile("fonts/segoeui.ttf"); err == nil {
		if f, err := opentype.Parse(data); err == nil {
			wd.regularFont = f
		}
	}

	// Load Segoe UI bold
	if data, err := assets.Fonts.ReadFile("fonts/segoeuib.ttf"); err == nil {
		if f, err := opentype.Parse(data); err == nil {
			wd.boldFont = f
		}
	}

	return wd
}

// panelAlpha maps the opacity percent (and no-background flag) to card alpha.
func panelAlpha(opacity int, noBackground bool) float64 {
	if noBackground {
		return 0.0
	}
	if opacity < 25 {
		opacity = 25
	}
	if opacity > 100 {
		opacity = 100
	}
	const (
		minAlpha = 0.40
		maxAlpha = 0.85
	)
	frac := float64(opacity-25) / float64(100-25)
	return minAlpha + frac*(maxAlpha-minAlpha)
}

// getFace returns a cached font face for the requested weight and point size.
func (wd *WidgetDrawer) getFace(bold bool, size float64) font.Face {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	key := fmt.Sprintf("%v:%.1f", bold, size)
	if f, ok := wd.faceCache[key]; ok {
		return f
	}

	f := wd.regularFont
	if bold && wd.boldFont != nil {
		f = wd.boldFont
	}
	if f == nil {
		f = wd.regularFont
	}
	if f == nil {
		return nil
	}

	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Printf("uiwindows: failed to create font face size %.1f: %v", size, err)
		return nil
	}

	wd.faceCache[key] = face
	return face
}

// loadIcon loads and caches an image from an embed.FS.
func (wd *WidgetDrawer) loadIcon(path string) image.Image {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	if img, ok := wd.iconCache[path]; ok {
		return img
	}

	data, err := assets.Icons.ReadFile(path)
	if err != nil {
		// Try air icons if not in main icons
		data, err = assets.AirIcons.ReadFile(path)
		if err != nil {
			return nil
		}
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}

	wd.iconCache[path] = img
	return img
}

// Render draws all city cards and returns a 32-bit ARGB image ready for UpdateLayeredWindow.
func (wd *WidgetDrawer) Render(
	cities []config.CityConfig,
	results []weather.WeatherResult,
	cfg *config.Config,
	lm *i18n.LocaleManager,
	now time.Time,
) *image.RGBA {
	if len(cities) == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}

	viewMode := config.NormalizeViewMode(cfg.ViewMode)
	if viewMode == config.ViewModeSimple {
		return wd.renderSimpleView(cities, results, cfg, lm, now)
	}
	return wd.renderEnhancedView(cities, results, cfg, lm, now)
}

// renderEnhancedView renders cards stacked vertically (width: 380, height: 250 per card).
func (wd *WidgetDrawer) renderEnhancedView(
	cities []config.CityConfig,
	results []weather.WeatherResult,
	cfg *config.Config,
	lm *i18n.LocaleManager,
	now time.Time,
) *image.RGBA {
	const (
		cardW   = 380
		cardH   = 250
		spacing = 8
	)

	n := len(cities)
	totalH := n*cardH + (n-1)*spacing
	dst := image.NewRGBA(image.Rect(0, 0, cardW, totalH))

	for i, city := range cities {
		var res *weather.WeatherResult
		if i < len(results) {
			res = &results[i]
		}
		cardY := i * (cardH + spacing)
		wd.renderSingleEnhancedCard(dst, 0, cardY, cardW, cardH, city, res, cfg, lm, now)
	}

	return dst
}

// renderSingleEnhancedCard draws one enhanced weather card at (x, y, w, h).
func (wd *WidgetDrawer) renderSingleEnhancedCard(
	dst *image.RGBA,
	x, y, w, h int,
	city config.CityConfig,
	res *weather.WeatherResult,
	cfg *config.Config,
	lm *i18n.LocaleManager,
	now time.Time,
) {
	// 1. Draw Card Background
	cardAlpha := panelAlpha(cfg.Opacity, cfg.NoBackground)
	if cfg.NoBackground {
		// Stroke subtle outline
		strokeRoundedRect(dst, x+1, y+1, w-2, h-2, 16, color.RGBA{255, 255, 255, 60}, 1)
	} else {
		// Tinted translucent card with soft border
		bgAlpha := uint8(cardAlpha * 255)
		fillRoundedRect(dst, x, y, w, h, 16, color.RGBA{18, 22, 28, bgAlpha})
		strokeRoundedRect(dst, x, y, w, h, 16, color.RGBA{255, 255, 255, 30}, 1)
	}

	// Font sizes from config
	fsCity := float64(cfg.GetFontSizeCityTime())
	fsTime := math.Round(fsCity * (16.0 / 14.0))
	fsTemp := float64(cfg.GetFontSizeTempIcon())
	fsCond := float64(cfg.GetFontSizeConditions())

	// Colors
	colWhite := color.RGBA{255, 255, 255, 255}
	colMuted := color.RGBA{255, 255, 255, 180}
	colFaint := color.RGBA{255, 255, 255, 120}

	// Fonts
	faceCity := wd.getFace(true, fsCity)
	faceTime := wd.getFace(true, fsTime)
	faceDate := wd.getFace(false, 11)
	faceTemp := wd.getFace(true, fsTemp)
	faceDesc := wd.getFace(true, fsCond)
	faceMetric := wd.getFace(false, 11)

	// Local Time calculation for this city
	cityTime := now
	if city.Timezone != "" {
		if loc, err := time.LoadLocation(city.Timezone); err == nil {
			cityTime = now.In(loc)
		}
	}

	// 2. City Name & Location
	cityName := city.Name
	if city.Region != "" {
		cityName = fmt.Sprintf("%s, %s", city.Name, city.Region)
	}
	drawText(dst, cityName, x+18, y+28, faceCity, colWhite)

	// 3. Clock & Date
	timeStr := cityTime.Format("15:04:05")
	dateStr := cityTime.Format("Mon, 02 Jan")
	drawText(dst, timeStr, x+18, y+50, faceTime, colWhite)
	drawText(dst, dateStr, x+18+measureText(timeStr, faceTime)+10, y+50, faceDate, colMuted)

	// Weather data values
	var (
		tempStr   = "--°"
		feelsStr  = ""
		descStr   = "Loading..."
		iconPath  = "icons/original/clear.png"
		humidity  = "--"
		windSpeed = "--"
		windGust  = "--"
		dewPoint  = "--"
		pressure  = "--"
		uvIndex   = "--"
	)

	var d *weather.WeatherData
	if res != nil && res.Data != nil {
		d = res.Data
		tempStr = weather.FormatTemperature(d.Temperature, cfg.TemperatureUnit)
		if d.FeelsLike != nil {
			feelsStr = fmt.Sprintf("Feels %s", weather.FormatFeelsLike(d.FeelsLike, cfg.TemperatureUnit))
		}
		if d.Description != "" {
			descStr = d.Description
		}
		if d.IconCode != "" {
			iconPath = fmt.Sprintf("icons/original/%s.png", d.IconCode)
		}
		humidity = fmt.Sprintf("%d%%", d.Humidity)
		windSpeed = formatWind(d.WindSpeed, cfg.WindSpeedUnit)
		if d.WindGust > 0 {
			windGust = formatWind(d.WindGust, cfg.WindSpeedUnit)
		}
		if d.DewPoint != 0 {
			dewPoint = weather.FormatTemperature(int(math.Round(d.DewPoint)), cfg.TemperatureUnit)
		}
		if d.Pressure > 0 {
			pressure = fmt.Sprintf("%.0f hPa", d.Pressure)
		}
		if d.UVIndex > 0 {
			uvIndex = fmt.Sprintf("%.1f", d.UVIndex)
		}
	} else if res != nil && res.HasError {
		descStr = "Update failed"
		if res.IsStale {
			descStr = "Data stale"
		}
	}

	// 4. Weather Icon (Left block)
	weatherImg := wd.loadIcon(iconPath)
	if weatherImg != nil {
		drawImage(dst, weatherImg, x+16, y+68, 64, 64)
	}

	// 5. Temperature & Feels Like
	drawText(dst, tempStr, x+90, y+102, faceTemp, colWhite)
	if feelsStr != "" {
		drawText(dst, feelsStr, x+92, y+120, faceDate, colMuted)
	}
	drawText(dst, descStr, x+18, y+152, faceDesc, colWhite)

	// 6. Metrics Grid (Right Column: x=190..365)
	const (
		col1X = 195
		col2X = 285
		row1Y = 32
		row2Y = 72
		row3Y = 112
	)

	// Grid background / divider
	fillRoundedRect(dst, x+185, y+18, 180, 138, 10, color.RGBA{255, 255, 255, 8})

	drawMetricTile(dst, col1X, y+row1Y, "Humidity", humidity, faceDate, faceMetric, colFaint, colWhite)
	drawMetricTile(dst, col2X, y+row1Y, "Wind", windSpeed, faceDate, faceMetric, colFaint, colWhite)
	drawMetricTile(dst, col1X, y+row2Y, "Gust", windGust, faceDate, faceMetric, colFaint, colWhite)
	drawMetricTile(dst, col2X, y+row2Y, "Dew Point", dewPoint, faceDate, faceMetric, colFaint, colWhite)
	drawMetricTile(dst, col1X, y+row3Y, "Pressure", pressure, faceDate, faceMetric, colFaint, colWhite)
	drawMetricTile(dst, col2X, y+row3Y, "UV Index", uvIndex, faceDate, faceMetric, colFaint, colWhite)

	// 7. Air Quality Strip (Bottom: y=168..234)
	strokeRoundedRect(dst, x+14, y+168, w-28, 66, 10, color.RGBA{255, 255, 255, 15}, 1)
	wd.renderPollutionStrip(dst, x+22, y+176, w-44, d, lm)
}

// renderPollutionStrip renders AQI pill and pollutant values along the card bottom.
func (wd *WidgetDrawer) renderPollutionStrip(
	dst *image.RGBA,
	x, y, w int,
	d *weather.WeatherData,
	lm *i18n.LocaleManager,
) {
	faceLabel := wd.getFace(false, 9)
	faceValue := wd.getFace(true, 11)
	colFaint := color.RGBA{255, 255, 255, 140}
	colWhite := color.RGBA{255, 255, 255, 255}

	// 1. AQI Pill
	aqiVal := "--"
	aqiColor := color.RGBA{100, 100, 100, 200}
	if d != nil && d.AQI != nil {
		aqiVal = fmt.Sprintf("AQI %d", *d.AQI)
		switch *d.AQI {
		case 1:
			aqiColor = color.RGBA{46, 204, 113, 220} // Green
		case 2:
			aqiColor = color.RGBA{241, 196, 15, 220} // Yellow
		case 3:
			aqiColor = color.RGBA{230, 126, 34, 220} // Orange
		case 4:
			aqiColor = color.RGBA{231, 76, 60, 220}  // Red
		case 5:
			aqiColor = color.RGBA{142, 68, 173, 220} // Purple
		}
	} else {
		aqiVal = "AQI --"
	}

	fillRoundedRect(dst, x, y+4, 62, 38, 8, aqiColor)
	drawText(dst, aqiVal, x+10, y+27, faceValue, colWhite)

	// 2. Pollutants (PM2.5, PM10, O3, NO2)
	metrics := []struct {
		name  string
		value string
		icon  string
	}{
		{"PM2.5", formatPollutant(d, func(w *weather.WeatherData) *float64 { return w.PM25 }), "air/pm25.png"},
		{"PM10", formatPollutant(d, func(w *weather.WeatherData) *float64 { return w.PM10 }), "air/pm10.png"},
		{"O3", formatPollutant(d, func(w *weather.WeatherData) *float64 { return w.O3 }), "air/o3.png"},
		{"NO2", formatPollutant(d, func(w *weather.WeatherData) *float64 { return w.NO2 }), "air/no2.png"},
	}

	tileX := x + 72
	const tileW = 62
	for _, m := range metrics {
		iconImg := wd.loadIcon(m.icon)
		if iconImg != nil {
			drawImage(dst, iconImg, tileX, y+2, 16, 16)
		}
		drawText(dst, m.name, tileX+20, y+14, faceLabel, colFaint)
		drawText(dst, m.value, tileX+2, y+34, faceValue, colWhite)
		tileX += tileW
	}
}

// formatPollutant formats an optional float pollutant value.
func formatPollutant(d *weather.WeatherData, getter func(*weather.WeatherData) *float64) string {
	if d == nil {
		return "--"
	}
	val := getter(d)
	if val == nil {
		return "--"
	}
	return fmt.Sprintf("%.1f", *val)
}

// drawMetricTile draws a name + value label pair.
func drawMetricTile(dst *image.RGBA, x, y int, name, val string, faceName, faceVal font.Face, colName, colVal color.RGBA) {
	drawText(dst, name, x, y+10, faceName, colName)
	drawText(dst, val, x, y+26, faceVal, colVal)
}

// renderSimpleView renders narrow vertical cards arranged horizontally (width: 140, height: 560 per card).
func (wd *WidgetDrawer) renderSimpleView(
	cities []config.CityConfig,
	results []weather.WeatherResult,
	cfg *config.Config,
	lm *i18n.LocaleManager,
	now time.Time,
) *image.RGBA {
	const (
		cardW   = 140
		cardH   = 560
		spacing = 8
	)

	n := len(cities)
	totalW := n*cardW + (n-1)*spacing
	dst := image.NewRGBA(image.Rect(0, 0, totalW, cardH))

	for i, city := range cities {
		var res *weather.WeatherResult
		if i < len(results) {
			res = &results[i]
		}
		cardX := i * (cardW + spacing)
		wd.renderSingleSimpleCard(dst, cardX, 0, cardW, cardH, city, res, cfg, lm, now)
	}

	return dst
}

// renderSingleSimpleCard draws one vertical column card in simple view mode.
func (wd *WidgetDrawer) renderSingleSimpleCard(
	dst *image.RGBA,
	x, y, w, h int,
	city config.CityConfig,
	res *weather.WeatherResult,
	cfg *config.Config,
	lm *i18n.LocaleManager,
	now time.Time,
) {
	cardAlpha := panelAlpha(cfg.Opacity, cfg.NoBackground)
	if cfg.NoBackground {
		strokeRoundedRect(dst, x+1, y+1, w-2, h-2, 14, color.RGBA{255, 255, 255, 60}, 1)
	} else {
		bgAlpha := uint8(cardAlpha * 255)
		fillRoundedRect(dst, x, y, w, h, 14, color.RGBA{18, 22, 28, bgAlpha})
		strokeRoundedRect(dst, x, y, w, h, 14, color.RGBA{255, 255, 255, 30}, 1)
	}

	colWhite := color.RGBA{255, 255, 255, 255}
	colMuted := color.RGBA{255, 255, 255, 180}
	colFaint := color.RGBA{255, 255, 255, 120}

	faceCity := wd.getFace(true, 14)
	faceTime := wd.getFace(true, 15)
	faceDate := wd.getFace(false, 11)
	faceTemp := wd.getFace(true, 30)
	faceDesc := wd.getFace(false, 11)
	faceMetric := wd.getFace(false, 11)

	cityTime := now
	if city.Timezone != "" {
		if loc, err := time.LoadLocation(city.Timezone); err == nil {
			cityTime = now.In(loc)
		}
	}

	// 1. City & Clock
	drawText(dst, city.Name, x+14, y+26, faceCity, colWhite)
	drawText(dst, cityTime.Format("15:04:05"), x+14, y+48, faceTime, colWhite)
	drawText(dst, cityTime.Format("Mon, 02 Jan"), x+14, y+66, faceDate, colMuted)

	tempStr := "--°"
	descStr := "Loading..."
	iconPath := "icons/original/clear.png"
	humidity := "--"
	windSpeed := "--"
	uvIndex := "--"

	var d *weather.WeatherData
	if res != nil && res.Data != nil {
		d = res.Data
		tempStr = weather.FormatTemperature(d.Temperature, cfg.TemperatureUnit)
		if d.Description != "" {
			descStr = d.Description
		}
		if d.IconCode != "" {
			iconPath = fmt.Sprintf("icons/original/%s.png", d.IconCode)
		}
		humidity = fmt.Sprintf("%d%%", d.Humidity)
		windSpeed = formatWind(d.WindSpeed, cfg.WindSpeedUnit)
		if d.UVIndex > 0 {
			uvIndex = fmt.Sprintf("%.1f", d.UVIndex)
		}
	}

	// 2. Weather Icon & Temp
	weatherImg := wd.loadIcon(iconPath)
	if weatherImg != nil {
		drawImage(dst, weatherImg, x+36, y+84, 68, 68)
	}

	drawText(dst, tempStr, x+32, y+184, faceTemp, colWhite)
	drawText(dst, descStr, x+14, y+208, faceDesc, colMuted)

	// 3. Divider
	drawHorizontalLine(dst, x+12, x+w-12, y+224, color.RGBA{255, 255, 255, 25})

	// 4. Vertical metrics
	currY := y + 246
	drawMetricRow(dst, x+14, currY, "Humidity", humidity, faceMetric, colFaint, colWhite)
	currY += 36
	drawMetricRow(dst, x+14, currY, "Wind", windSpeed, faceMetric, colFaint, colWhite)
	currY += 36
	drawMetricRow(dst, x+14, currY, "UV Index", uvIndex, faceMetric, colFaint, colWhite)

	// 5. AQI Box
	currY += 46
	drawHorizontalLine(dst, x+12, x+w-12, currY-12, color.RGBA{255, 255, 255, 25})
	aqiStr := "AQI --"
	if d != nil && d.AQI != nil {
		aqiStr = fmt.Sprintf("AQI %d", *d.AQI)
	}
	fillRoundedRect(dst, x+14, currY, w-28, 30, 6, color.RGBA{46, 204, 113, 200})
	drawText(dst, aqiStr, x+38, currY+20, faceMetric, colWhite)
}

func formatWind(speed float64, unit config.WindSpeedUnit) string {
	val, label := weather.ConvertWindSpeed(speed, unit)
	return fmt.Sprintf("%.0f %s", val, label)
}

func drawMetricRow(dst *image.RGBA, x, y int, name, val string, face font.Face, colName, colVal color.RGBA) {
	drawText(dst, name, x, y, face, colName)
	drawText(dst, val, x, y+16, face, colVal)
}

// ── Low-Level Pure-Go Anti-Aliased Graphics Utilities ─────────────────────────

// drawText draws a UTF-8 string on dst at the given baseline position (x, y).
func drawText(dst *image.RGBA, text string, x, y int, face font.Face, c color.RGBA) {
	if face == nil || text == "" {
		return
	}
	d := font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

// measureText measures the pixel width of a string with the given font face.
func measureText(text string, face font.Face) int {
	if face == nil || text == "" {
		return 0
	}
	d := font.Drawer{Face: face}
	return d.MeasureString(text).Round()
}

// drawImage draws and scales an image onto dst with bilinear interpolation.
func drawImage(dst *image.RGBA, src image.Image, x, y, w, h int) {
	if src == nil {
		return
	}
	rect := image.Rect(x, y, x+w, y+h)
	xdraw.BiLinear.Scale(dst, rect, src, src.Bounds(), draw.Over, nil)
}

// drawHorizontalLine draws a 1-pixel horizontal line.
func drawHorizontalLine(dst *image.RGBA, x1, x2, y int, c color.RGBA) {
	for x := x1; x <= x2; x++ {
		blendPixel(dst, x, y, c)
	}
}

// fillRoundedRect fills a rounded rectangle with smooth anti-aliased corners.
func fillRoundedRect(dst *image.RGBA, x, y, w, h, radius int, c color.RGBA) {
	if radius <= 0 {
		draw.Draw(dst, image.Rect(x, y, x+w, y+h), image.NewUniform(c), image.Point{}, draw.Over)
		return
	}

	rF := float64(radius)
	minX, maxX := x, x+w-1
	minY, maxY := y, y+h-1

	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			// Check if pixel falls inside one of the four corner quadrants
			var dist float64
			inCorner := false

			if px < minX+radius && py < minY+radius { // Top-left
				dx := float64(minX+radius) - float64(px) - 0.5
				dy := float64(minY+radius) - float64(py) - 0.5
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px > maxX-radius && py < minY+radius { // Top-right
				dx := float64(px) + 0.5 - float64(maxX-radius)
				dy := float64(minY+radius) - float64(py) - 0.5
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px < minX+radius && py > maxY-radius { // Bottom-left
				dx := float64(minX+radius) - float64(px) - 0.5
				dy := float64(py) + 0.5 - float64(maxY-radius)
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px > maxX-radius && py > maxY-radius { // Bottom-right
				dx := float64(px) + 0.5 - float64(maxX-radius)
				dy := float64(py) + 0.5 - float64(maxY-radius)
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			}

			if !inCorner {
				blendPixel(dst, px, py, c)
			} else if dist <= rF-0.5 {
				blendPixel(dst, px, py, c)
			} else if dist < rF+0.5 {
				// Anti-aliased outer edge
				alphaFrac := (rF + 0.5 - dist)
				subAlpha := uint8(float64(c.A) * alphaFrac)
				blendPixel(dst, px, py, color.RGBA{c.R, c.G, c.B, subAlpha})
			}
		}
	}
}

// strokeRoundedRect draws a 1px rounded rectangle outline.
func strokeRoundedRect(dst *image.RGBA, x, y, w, h, radius int, c color.RGBA, lineWidth int) {
	rF := float64(radius)
	minX, maxX := x, x+w-1
	minY, maxY := y, y+h-1

	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			var dist float64
			inCorner := false

			if px < minX+radius && py < minY+radius {
				dx := float64(minX+radius) - float64(px) - 0.5
				dy := float64(minY+radius) - float64(py) - 0.5
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px > maxX-radius && py < minY+radius {
				dx := float64(px) + 0.5 - float64(maxX-radius)
				dy := float64(minY+radius) - float64(py) - 0.5
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px < minX+radius && py > maxY-radius {
				dx := float64(minX+radius) - float64(px) - 0.5
				dy := float64(py) + 0.5 - float64(maxY-radius)
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px > maxX-radius && py > maxY-radius {
				dx := float64(px) + 0.5 - float64(maxX-radius)
				dy := float64(py) + 0.5 - float64(maxY-radius)
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			}

			if inCorner {
				if math.Abs(dist-rF) <= 0.8 {
					blendPixel(dst, px, py, c)
				}
			} else if px == minX || px == maxX || py == minY || py == maxY {
				blendPixel(dst, px, py, c)
			}
		}
	}
}

// blendPixel blends a color onto dst at (x, y) with alpha compositing.
func blendPixel(dst *image.RGBA, x, y int, src color.RGBA) {
	if x < dst.Rect.Min.X || x >= dst.Rect.Max.X || y < dst.Rect.Min.Y || y >= dst.Rect.Max.Y {
		return
	}
	if src.A == 0 {
		return
	}

	offset := dst.PixOffset(x, y)
	if src.A == 255 {
		dst.Pix[offset] = src.R
		dst.Pix[offset+1] = src.G
		dst.Pix[offset+2] = src.B
		dst.Pix[offset+3] = 255
		return
	}

	dR := uint32(dst.Pix[offset])
	dG := uint32(dst.Pix[offset+1])
	dB := uint32(dst.Pix[offset+2])
	dA := uint32(dst.Pix[offset+3])

	sR := uint32(src.R)
	sG := uint32(src.G)
	sB := uint32(src.B)
	sA := uint32(src.A)

	outA := sA + dA*(255-sA)/255
	if outA == 0 {
		return
	}

	outR := (sR*sA + dR*dA*(255-sA)/255) / outA
	outG := (sG*sA + dG*dA*(255-sA)/255) / outA
	outB := (sB*sA + dB*dA*(255-sA)/255) / outA

	dst.Pix[offset] = uint8(outR)
	dst.Pix[offset+1] = uint8(outG)
	dst.Pix[offset+2] = uint8(outB)
	dst.Pix[offset+3] = uint8(outA)
}
