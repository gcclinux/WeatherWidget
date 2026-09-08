package panel

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"
)

// simplePollutionCell holds the widgets for a single air-quality metric shown
// in the Simple view: an icon and a value label.
type simplePollutionCell struct {
	container *fyne.Container
	icon      *canvas.Image
	value     *canvas.Text
}

// SimpleCityPanel renders weather data for a single city in the Simple (Classic) view.
// It reproduces the original horizontal side-by-side classic weatherwidget design
// with transparent desktop background, supporting weather metrics and air quality.
type SimpleCityPanel struct {
	lm            *i18n.LocaleManager
	container     *fyne.Container
	iconWidget    *canvas.Image
	iconRow       *fyne.Container
	tempText      *canvas.Text
	descText      *canvas.Text
	humidityText  *canvas.Text
	windText      *canvas.Text
	cityText      *canvas.Text
	timeText      *canvas.Text
	dateText      *canvas.Text
	windGustText  *canvas.Text
	dewPointText  *canvas.Text
	pressureText  *canvas.Text
	uvIndexText   *canvas.Text
	windDirText   *canvas.Text
	separatorLine *canvas.Rectangle
	separatorRow  *fyne.Container
	errorIcon     *canvas.Image

	cardBg         *canvas.Rectangle
	cornerMask     fyne.CanvasObject
	innerContainer *fyne.Container

	// Air quality & pollution metrics
	pollutionCells  map[weather.PollutionMetric]*simplePollutionCell
	pollutionBox    *fyne.Container
	pollutionFields *config.PollutionFields

	lastData      *weather.WeatherData  // cached for re-render on unit change
	lastTempUnit  config.TemperatureUnit
	lastWindUnit  config.WindSpeedUnit
	lastIconTheme config.IconTheme
	displayFields *config.DisplayFields // current visibility config

	mu         sync.Mutex
	timeTicker *time.Ticker
	stopCh     chan struct{}
	animStopCh chan struct{}
}

var (
	simpleDarkThemeTextColor  color.Color = color.White
	simpleLightThemeTextColor color.Color = color.NRGBA{R: 24, G: 24, B: 28, A: 255}
	simpleDarkThemeSepColor   color.Color = color.NRGBA{R: 255, G: 255, B: 255, A: 60}
	simpleLightThemeSepColor  color.Color = color.NRGBA{R: 0, G: 0, B: 0, A: 50}
)

// currentSimpleTextColor returns the text color for Simple View based on the OS theme.
func currentSimpleTextColor() color.Color {
	if isWindowsLightTheme() {
		return simpleLightThemeTextColor
	}
	return simpleDarkThemeTextColor
}

// currentSimpleSeparatorColor returns the separator line color for Simple View.
func currentSimpleSeparatorColor() color.Color {
	if isWindowsLightTheme() {
		return simpleLightThemeSepColor
	}
	return simpleDarkThemeSepColor
}

// newSimpleCardBg creates the frosted light card background shown in Windows Light Theme.
func newSimpleCardBg() *canvas.Rectangle {
	rect := canvas.NewRectangle(color.NRGBA{R: 245, G: 247, B: 252, A: 220})
	rect.CornerRadius = cardCornerRadius
	rect.StrokeColor = color.NRGBA{R: 255, G: 255, B: 255, A: 180}
	rect.StrokeWidth = 1
	return rect
}

// updateTheme applies the current theme's card background, corner mask, text, and separator colors.
func (p *SimpleCityPanel) updateTheme() {
	isLight := isWindowsLightTheme()
	textColor := simpleDarkThemeTextColor
	sepColor := simpleDarkThemeSepColor
	if isLight {
		textColor = simpleLightThemeTextColor
		sepColor = simpleLightThemeSepColor
	}

	if p.cardBg != nil {
		if isLight {
			p.cardBg.Show()
		} else {
			p.cardBg.Hide()
		}
		p.cardBg.Refresh()
	}

	if p.cornerMask != nil {
		if isLight {
			p.cornerMask.Show()
		} else {
			p.cornerMask.Hide()
		}
		p.cornerMask.Refresh()
	}

	if p.cityText != nil {
		p.cityText.Color = textColor
		p.cityText.Refresh()
	}
	if p.tempText != nil {
		p.tempText.Color = textColor
		p.tempText.Refresh()
	}
	if p.descText != nil {
		p.descText.Color = textColor
		p.descText.Refresh()
	}
	if p.humidityText != nil {
		p.humidityText.Color = textColor
		p.humidityText.Refresh()
	}
	if p.windText != nil {
		p.windText.Color = textColor
		p.windText.Refresh()
	}
	if p.timeText != nil {
		p.timeText.Color = textColor
		p.timeText.Refresh()
	}
	if p.dateText != nil {
		p.dateText.Color = textColor
		p.dateText.Refresh()
	}
	if p.windGustText != nil {
		p.windGustText.Color = textColor
		p.windGustText.Refresh()
	}
	if p.dewPointText != nil {
		p.dewPointText.Color = textColor
		p.dewPointText.Refresh()
	}
	if p.pressureText != nil {
		p.pressureText.Color = textColor
		p.pressureText.Refresh()
	}
	if p.uvIndexText != nil {
		p.uvIndexText.Color = textColor
		p.uvIndexText.Refresh()
	}
	if p.windDirText != nil {
		p.windDirText.Color = textColor
		p.windDirText.Refresh()
	}

	for _, cell := range p.pollutionCells {
		if cell != nil && cell.value != nil {
			cell.value.Color = textColor
			cell.value.Refresh()
		}
	}

	if p.separatorLine != nil {
		p.separatorLine.FillColor = sepColor
		p.separatorLine.Refresh()
	}
}

// updateTextColors is a backward-compatible wrapper around updateTheme.
func (p *SimpleCityPanel) updateTextColors() {
	p.updateTheme()
}

// loadSimpleAirIconResource reads an air-quality icon from the embedded AirIcons FS
// and returns it as a Fyne resource, or nil on error.
func loadSimpleAirIconResource(file string) fyne.Resource {
	data, err := assets.AirIcons.ReadFile("air/" + file)
	if err != nil {
		return nil
	}
	return fyne.NewStaticResource(file, data)
}

// translate returns the translated string for the given key using the panel's
// LocaleManager. If the LocaleManager is nil, it returns the provided fallback.
func (p *SimpleCityPanel) translate(key, fallback string) string {
	if p.lm != nil {
		return p.lm.T(key)
	}
	return fallback
}

// updateIcon updates the panel's icon widget, supporting animated motion icons.
func (p *SimpleCityPanel) updateIcon(iconCode string) {
	p.StopAnimation()

	anim, staticData, staticPath, err := loadIconAsset(iconCode)
	if err != nil {
		return
	}

	if anim != nil && len(anim.frames) > 1 {
		p.iconWidget.Resource = nil
		p.iconWidget.File = ""
		p.iconWidget.Image = anim.frames[0]
		p.iconWidget.Refresh()

		p.mu.Lock()
		stopCh := make(chan struct{})
		p.animStopCh = stopCh
		p.mu.Unlock()

		go func(frames []image.Image, delays []time.Duration, stopCh chan struct{}) {
			idx := 0
			for {
				delay := delays[idx]
				select {
				case <-stopCh:
					return
				case <-time.After(delay):
					idx = (idx + 1) % len(frames)
					frame := frames[idx]
					fyne.Do(func() {
						p.iconWidget.Image = frame
						p.iconWidget.Refresh()
					})
				}
			}
		}(anim.frames, anim.delays, stopCh)
		return
	}

	if staticData != nil {
		p.iconWidget.Image = nil
		p.iconWidget.File = ""
		p.iconWidget.Resource = fyne.NewStaticResource(staticPath, staticData)
		p.iconWidget.Refresh()
	}
}

// StopAnimation stops any running icon animation loop.
func (p *SimpleCityPanel) StopAnimation() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.animStopCh != nil {
		close(p.animStopCh)
		p.animStopCh = nil
	}
}

// simpleColumnLayout ensures each city panel maintains a minimum width
// so columns are never squished together horizontally.
type simpleColumnLayout struct {
	minWidth float32
}

func (l *simpleColumnLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		if o.Visible() {
			s := o.MinSize()
			if s.Width > w {
				w = s.Width
			}
			if s.Height > h {
				h = s.Height
			}
		}
	}
	if w < l.minWidth {
		w = l.minWidth
	}
	return fyne.NewSize(w, h)
}

func (l *simpleColumnLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		if o.Visible() {
			o.Resize(size)
			o.Move(fyne.NewPos(0, 0))
		}
	}
}

// tightVBoxLayout arranges objects vertically with zero spacing between them.
type tightVBoxLayout struct{}

func (t *tightVBoxLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		s := o.MinSize()
		if s.Width > w {
			w = s.Width
		}
		h += s.Height
	}
	return fyne.NewSize(w, h)
}

func (t *tightVBoxLayout) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	var y float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		s := o.MinSize()
		o.Resize(fyne.NewSize(containerSize.Width, s.Height))
		o.Move(fyne.NewPos(0, y))
		y += s.Height
	}
}

// NewSimpleCityPanel creates a new SimpleCityPanel with placeholder content.
func NewSimpleCityPanel(lm *i18n.LocaleManager) *SimpleCityPanel {
	p := &SimpleCityPanel{
		lm:              lm,
		displayFields:   config.DefaultDisplayFields(),
		pollutionFields: config.DefaultPollutionFields(),
	}

	// Weather icon — start with default cloudy icon.
	p.iconWidget = canvas.NewImageFromResource(loadIconFromAssets(weather.IconCloudy))
	p.iconWidget.FillMode = canvas.ImageFillContain
	p.iconWidget.SetMinSize(fyne.NewSize(64, 64))
	p.updateIcon(weather.IconCloudy)

	// Error indicator icon — hidden by default.
	p.errorIcon = canvas.NewImageFromResource(nil)
	p.errorIcon.FillMode = canvas.ImageFillContain
	p.errorIcon.SetMinSize(fyne.NewSize(16, 16))
	p.errorIcon.Hide()

	// Labels with appealing typography (matching original commit f94faa19).
	textColor := currentSimpleTextColor()
	p.cityText = canvas.NewText(p.translate("panel.placeholder.city", "City, RG"), textColor)
	p.cityText.TextSize = 18
	p.cityText.TextStyle = fyne.TextStyle{Bold: true}
	p.cityText.Alignment = fyne.TextAlignCenter

	p.tempText = canvas.NewText(p.translate("panel.placeholder.temp", "--°C"), textColor)
	p.tempText.TextSize = 42
	p.tempText.TextStyle = fyne.TextStyle{Bold: true}
	p.tempText.Alignment = fyne.TextAlignCenter

	p.descText = canvas.NewText(p.translate("panel.placeholder.desc", "--"), textColor)
	p.descText.TextSize = 12
	p.descText.TextStyle = fyne.TextStyle{Italic: true}
	p.descText.Alignment = fyne.TextAlignCenter

	p.humidityText = canvas.NewText(p.translate("panel.placeholder.humidity", "💧 Hum --%"), textColor)
	p.humidityText.TextSize = 12
	p.humidityText.TextStyle = fyne.TextStyle{Italic: true}
	p.humidityText.Alignment = fyne.TextAlignCenter

	p.windText = canvas.NewText(p.translate("panel.placeholder.wind", "💨 -- km/h"), textColor)
	p.windText.TextSize = 12
	p.windText.TextStyle = fyne.TextStyle{Italic: true}
	p.windText.Alignment = fyne.TextAlignCenter

	p.timeText = canvas.NewText(p.translate("panel.placeholder.time", "00:00:00"), textColor)
	p.timeText.TextSize = 22
	p.timeText.TextStyle = fyne.TextStyle{Bold: true}
	p.timeText.Alignment = fyne.TextAlignCenter

	p.dateText = canvas.NewText(p.translate("panel.placeholder.date", "Monday, Jan 02"), textColor)
	p.dateText.TextSize = 11
	p.dateText.Alignment = fyne.TextAlignCenter

	p.windGustText = canvas.NewText("", textColor)
	p.windGustText.TextSize = 12
	p.windGustText.TextStyle = fyne.TextStyle{Italic: true}
	p.windGustText.Alignment = fyne.TextAlignCenter

	p.dewPointText = canvas.NewText("", textColor)
	p.dewPointText.TextSize = 12
	p.dewPointText.TextStyle = fyne.TextStyle{Italic: true}
	p.dewPointText.Alignment = fyne.TextAlignCenter

	p.pressureText = canvas.NewText("", textColor)
	p.pressureText.TextSize = 12
	p.pressureText.TextStyle = fyne.TextStyle{Italic: true}
	p.pressureText.Alignment = fyne.TextAlignCenter

	p.uvIndexText = canvas.NewText("", textColor)
	p.uvIndexText.TextSize = 12
	p.uvIndexText.TextStyle = fyne.TextStyle{Italic: true}
	p.uvIndexText.Alignment = fyne.TextAlignCenter

	p.windDirText = canvas.NewText("", textColor)
	p.windDirText.TextSize = 12
	p.windDirText.TextStyle = fyne.TextStyle{Italic: true}
	p.windDirText.Alignment = fyne.TextAlignCenter

	// Pollution cells (air quality & pollution)
	p.pollutionCells = make(map[weather.PollutionMetric]*simplePollutionCell, len(weather.PollutionMetricOrder))
	for _, m := range weather.PollutionMetricOrder {
		icon := canvas.NewImageFromResource(loadSimpleAirIconResource(weather.AirIconFile(m)))
		icon.FillMode = canvas.ImageFillContain
		icon.SetMinSize(fyne.NewSize(18, 18))

		value := canvas.NewText("", textColor)
		value.TextSize = 12
		value.TextStyle = fyne.TextStyle{Italic: true}
		value.Alignment = fyne.TextAlignCenter

		cell := &simplePollutionCell{
			icon:  icon,
			value: value,
		}
		cell.container = container.NewCenter(container.NewHBox(icon, value))
		cell.container.Hide()
		p.pollutionCells[m] = cell
	}

	p.separatorLine = canvas.NewRectangle(currentSimpleSeparatorColor())
	p.separatorLine.SetMinSize(fyne.NewSize(120, 1))
	p.separatorRow = container.NewCenter(p.separatorLine)

	p.cardBg = newSimpleCardBg()
	p.cornerMask = newCornerMask()
	p.innerContainer = container.New(&simpleColumnLayout{minWidth: 160}, p.buildLayout())

	if p.cornerMask != nil {
		p.container = container.NewStack(p.cardBg, p.innerContainer, p.cornerMask)
	} else {
		p.container = container.NewStack(p.cardBg, p.innerContainer)
	}

	p.updateTheme()
	return p
}

// buildLayout constructs the vertical layout hierarchy of labels.
func (p *SimpleCityPanel) buildLayout() fyne.CanvasObject {
	var objects []fyne.CanvasObject

	if p.displayFields.ShowCity {
		objects = append(objects, p.cityText)
	}

	p.iconRow = container.NewBorder(nil, nil, nil, nil, container.NewCenter(p.iconWidget), p.errorIcon)

	if p.displayFields.ShowIcon {
		objects = append(objects, p.iconRow)
	}

	if p.displayFields.ShowTemp {
		objects = append(objects, p.tempText)
	}

	if p.displayFields.ShowDesc {
		objects = append(objects, p.descText)
	}

	if p.displayFields.ShowHumidity {
		objects = append(objects, container.NewCenter(p.humidityText))
	}

	if p.displayFields.ShowWind {
		objects = append(objects, container.NewCenter(container.NewHBox(p.windText, p.windDirText)))
	}

	// Dynamic sections: only show if fields are visible.
	hasDynamicField := p.displayFields.ShowWindGust || p.displayFields.ShowDewPoint || p.displayFields.ShowPressure || p.displayFields.ShowUVIndex
	if hasDynamicField {
		var dynamicObjects []fyne.CanvasObject
		if p.displayFields.ShowWindGust {
			dynamicObjects = append(dynamicObjects, p.windGustText)
		}
		if p.displayFields.ShowDewPoint {
			dynamicObjects = append(dynamicObjects, p.dewPointText)
		}
		if p.displayFields.ShowPressure {
			dynamicObjects = append(dynamicObjects, p.pressureText)
		}
		if p.displayFields.ShowUVIndex {
			dynamicObjects = append(dynamicObjects, p.uvIndexText)
		}

		if len(dynamicObjects) > 0 {
			objects = append(objects, container.New(&tightVBoxLayout{}, dynamicObjects...))
		}
	}

	// Pollution metrics (air quality)
	p.pollutionBox = container.New(&tightVBoxLayout{})
	for _, m := range weather.PollutionMetricOrder {
		if cell := p.pollutionCells[m]; cell != nil {
			p.pollutionBox.Add(cell.container)
		}
	}
	p.pollutionBox.Hide()
	objects = append(objects, p.pollutionBox)

	if p.displayFields.ShowTime || p.displayFields.ShowDate {
		objects = append(objects, p.separatorRow)
		var timeObjects []fyne.CanvasObject
		if p.displayFields.ShowTime {
			timeObjects = append(timeObjects, p.timeText)
		}
		if p.displayFields.ShowDate {
			timeObjects = append(timeObjects, container.NewCenter(p.dateText))
		}
		objects = append(objects, container.New(&tightVBoxLayout{}, timeObjects...))
	}

	return container.NewVBox(objects...)
}

// pollutionMetricShortLabel returns a concise label for a pollution metric
// suitable for compact display in the Simple (Classic) view column.
func pollutionMetricShortLabel(m weather.PollutionMetric) string {
	switch m {
	case weather.MetricAQI:
		return "AQI"
	case weather.MetricCO:
		return "CO"
	case weather.MetricNO:
		return "NO"
	case weather.MetricNO2:
		return "NO₂"
	case weather.MetricO3:
		return "O₃"
	case weather.MetricSO2:
		return "SO₂"
	case weather.MetricNH3:
		return "NH₃"
	case weather.MetricPM25:
		return "PM2.5"
	case weather.MetricPM10:
		return "PM10"
	default:
		return ""
	}
}

// ApplyDisplayFields updates the panel's visibility configuration and rebuilds the layout.
func (p *SimpleCityPanel) ApplyDisplayFields(df *config.DisplayFields) {
	if df == nil {
		df = config.DefaultDisplayFields()
	}
	p.displayFields = df
	if p.innerContainer != nil {
		p.innerContainer.RemoveAll()
		p.innerContainer.Add(p.buildLayout())
	}
	p.applyPollutionCells()
	p.updateTheme()
	p.container.Refresh()
}

// ApplyPollutionFields updates the air-quality metric selection and refreshes
// the pollution list using the panel's most recent data.
func (p *SimpleCityPanel) ApplyPollutionFields(pf *config.PollutionFields) {
	if pf == nil {
		pf = config.DefaultPollutionFields()
	}
	p.pollutionFields = pf
	p.applyPollutionCells()
}

// applyPollutionCells populates and shows the pollution cells selected by the
// current pollution fields and present in the latest data; others are hidden.
func (p *SimpleCityPanel) applyPollutionCells() {
	rows := weather.PlanPollutionRows(p.pollutionFields, weather.PollutionOf(p.lastData))
	planned := make(map[weather.PollutionMetric]weather.PollutionRow, len(rows))
	for _, r := range rows {
		planned[r.Metric] = r
	}

	hasAny := false
	for _, m := range weather.PollutionMetricOrder {
		cell := p.pollutionCells[m]
		if cell == nil {
			continue
		}
		if row, ok := planned[m]; ok {
			label := pollutionMetricShortLabel(m)
			if label != "" {
				cell.value.Text = fmt.Sprintf("%s: %s", label, row.ValueText)
			} else {
				cell.value.Text = row.ValueText
			}
			cell.value.Color = currentSimpleTextColor()
			cell.value.Refresh()
			cell.container.Show()
			hasAny = true
		} else {
			cell.container.Hide()
		}
	}
	if p.pollutionBox != nil {
		if hasAny {
			p.pollutionBox.Show()
		} else {
			p.pollutionBox.Hide()
		}
		p.pollutionBox.Refresh()
	}
}

// Container returns the Fyne container for embedding in a parent layout.
func (p *SimpleCityPanel) Container() *fyne.Container {
	return p.container
}

// Update sets the panel content from the given weather data using the specified units and icon theme.
func (p *SimpleCityPanel) Update(data *weather.WeatherData, tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme) {
	if data == nil {
		return
	}
	p.lastData = data // cache for re-render
	p.lastTempUnit = tempUnit
	p.lastWindUnit = windUnit
	if len(iconTheme) > 0 && iconTheme[0] != "" {
		p.lastIconTheme = iconTheme[0]
	} else if p.lastIconTheme == "" {
		p.lastIconTheme = config.IconThemeNew
	}

	// Update icon from embedded assets (with animated motion support).
	iconCode := weather.MapConditionToIconWithTheme(data.IconCode, data.LocalTime, p.lastIconTheme)
	p.updateIcon(iconCode)

	// Update labels.
	p.tempText.Text = weather.FormatTemperature(data.Temperature, tempUnit)
	p.tempText.Refresh()

	p.descText.Text = weather.FormatDescription(data.Description, p.lm)
	p.descText.Refresh()

	p.humidityText.Text = weather.FormatHumidity(data.Humidity, p.lm)
	p.humidityText.Refresh()

	p.windText.Text = weather.FormatWind(data.WindSpeed, windUnit)
	p.windText.Refresh()

	p.cityText.Text = weather.FormatCityRegion(data.CityName, data.Region)
	p.cityText.Refresh()

	p.windGustText.Text = weather.FormatWindGust(data.WindGust, windUnit, p.lm)
	p.windGustText.Refresh()

	p.dewPointText.Text = weather.FormatDewPoint(data.DewPoint, p.lm)
	p.dewPointText.Refresh()

	p.pressureText.Text = weather.FormatPressure(data.Pressure)
	p.pressureText.Refresh()

	p.uvIndexText.Text = weather.FormatUVIndex(data.UVIndex)
	p.uvIndexText.Refresh()

	p.windDirText.Text = weather.FormatWindDir(data.WindDirection)
	p.windDirText.Refresh()

	// Update air quality and pollution metrics.
	p.applyPollutionCells()
	p.updateTextColors()

	// Hide error indicator on successful update.
	p.errorIcon.Hide()
}

// Rerender re-applies the last cached WeatherData with new units or icon theme.
// If no data has been cached yet, this is a no-op.
func (p *SimpleCityPanel) Rerender(tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme) {
	if p.lastData == nil {
		return
	}
	p.Update(p.lastData, tempUnit, windUnit, iconTheme...)
}

// ShowError displays an error indicator on the panel.
// If stale is true, a persistent stale-data warning is shown.
// If stale is false, a small error indicator icon is shown.
func (p *SimpleCityPanel) ShowError(stale bool) {
	if stale {
		res := loadIconFromAssets("storm")
		if res != nil {
			p.errorIcon.Resource = res
		}
		p.errorIcon.SetMinSize(fyne.NewSize(20, 20))
		p.errorIcon.Show()
		p.errorIcon.Refresh()
		p.descText.Text = p.translate("panel.staleWarning", "Data may be stale")
		p.descText.Refresh()
	} else {
		res := loadIconFromAssets("fog")
		if res != nil {
			p.errorIcon.Resource = res
		}
		p.errorIcon.SetMinSize(fyne.NewSize(16, 16))
		p.errorIcon.Show()
		p.errorIcon.Refresh()
	}
}

// StartClock starts a 1-second ticker that updates the time label
// with the current time in the given IANA timezone.
func (p *SimpleCityPanel) StartClock(timezone string) {
	log.Printf("SimpleCityPanel: starting clock for timezone %s", timezone)
	p.StopClock() // stop any existing clock first

	p.mu.Lock()
	defer p.mu.Unlock()

	p.timeTicker = time.NewTicker(1 * time.Second)
	p.stopCh = make(chan struct{})

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}

	// Set the time immediately before the first tick.
	now := time.Now()
	p.timeText.Text = weather.FormatTime(now, timezone, p.lm)
	p.timeText.Refresh()
	p.dateText.Text = weather.FormatDate(now, timezone, p.lm)
	p.dateText.Refresh()

	lastNight := weather.IsNight(now.In(loc))
	lastLightTheme := isWindowsLightTheme()

	go func(loc *time.Location, initialNight bool, initialLightTheme bool) {
		currentNight := initialNight
		currentLightTheme := initialLightTheme
		for {
			select {
			case <-p.stopCh:
				return
			case t := <-p.timeTicker.C:
				localNow := t.In(loc)
				timeStr := weather.FormatTime(t, timezone, p.lm)
				dateStr := weather.FormatDate(t, timezone, p.lm)
				isNightNow := weather.IsNight(localNow)
				isLightNow := isWindowsLightTheme()

				fyne.Do(func() {
					p.timeText.Text = timeStr
					p.timeText.Refresh()
					p.dateText.Text = dateStr
					p.dateText.Refresh()

					if isLightNow != currentLightTheme {
						currentLightTheme = isLightNow
						p.updateTextColors()
					}

					if isNightNow != currentNight {
						currentNight = isNightNow
						if p.lastData != nil {
							p.lastData.LocalTime = localNow
							iconCode := weather.MapConditionToIconWithTheme(p.lastData.IconCode, localNow, p.lastIconTheme)
							p.updateIcon(iconCode)
						}
					}
				})
			}
		}
	}(loc, lastNight, lastLightTheme)
}

// StopClock stops the time ticker goroutine and any icon animation.
func (p *SimpleCityPanel) StopClock() {
	p.StopAnimation()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.timeTicker != nil {
		p.timeTicker.Stop()
		close(p.stopCh)
		p.timeTicker = nil
		p.stopCh = nil
	}
}
