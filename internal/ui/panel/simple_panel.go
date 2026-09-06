package panel

import (
	"image"
	"image/color"
	"log"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

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
//
// The Simple view displays a vertical column of weather information:
// - City name at the top
// - Weather icon
// - Time and date
// - Temperature (large)
// - Condition description
// - Weather details (humidity, wind, etc.) in a vertical list
// - Pollution metrics in a vertical list
//
// Multiple SimpleCityPanels are arranged side-by-side (horizontally) to show
// multiple cities.
type SimpleCityPanel struct {
	lm        *i18n.LocaleManager
	container *fyne.Container

	// Main display elements
	iconWidget *canvas.Image
	iconRow    *fyne.Container
	cityText   *canvas.Text
	timeText   *canvas.Text
	dateText   *canvas.Text
	tempText   *canvas.Text
	descText   *canvas.Text

	// Weather detail texts (vertical list format)
	humidityText *canvas.Text
	windText     *canvas.Text
	windGustText *canvas.Text
	dewPointText *canvas.Text
	uvIndexText  *canvas.Text

	// Pollution cells (vertical list)
	pollutionCells map[weather.PollutionMetric]*simplePollutionCell
	pollutionBox   *fyne.Container

	errorIcon *canvas.Image

	lastData        *weather.WeatherData
	lastTempUnit    config.TemperatureUnit
	lastWindUnit    config.WindSpeedUnit
	lastIconTheme   config.IconTheme
	displayFields   *config.DisplayFields
	pollutionFields *config.PollutionFields

	mu         sync.Mutex
	timeTicker *time.Ticker
	stopCh     chan struct{}
	animStopCh chan struct{}
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

// NewSimpleCityPanel creates a new SimpleCityPanel with placeholder content.
// If lm is nil, hardcoded English defaults are used for placeholder text.
func NewSimpleCityPanel(lm *i18n.LocaleManager) *SimpleCityPanel {
	p := &SimpleCityPanel{
		lm:              lm,
		displayFields:   config.DefaultDisplayFields(),
		pollutionFields: config.DefaultPollutionFields(),
	}

	// Weather icon — start with the default cloudy icon.
	p.iconWidget = canvas.NewImageFromResource(loadIconFromAssets(weather.IconCloudy))
	p.iconWidget.FillMode = canvas.ImageFillContain
	p.iconWidget.SetMinSize(fyne.NewSize(64, 64))
	p.updateIcon(weather.IconCloudy)

	// Error indicator icon — hidden by default.
	p.errorIcon = canvas.NewImageFromResource(nil)
	p.errorIcon.FillMode = canvas.ImageFillContain
	p.errorIcon.SetMinSize(fyne.NewSize(16, 16))
	p.errorIcon.Hide()

	// Labels with appealing typography for the Simple vertical layout.
	p.cityText = canvas.NewText(p.translate("panel.placeholder.city", "City, RG"), theme.ForegroundColor())
	p.cityText.TextSize = 18
	p.cityText.TextStyle = fyne.TextStyle{Bold: true}
	p.cityText.Alignment = fyne.TextAlignCenter

	p.timeText = canvas.NewText(p.translate("panel.placeholder.time", "00:00:00"), theme.ForegroundColor())
	p.timeText.TextSize = 22
	p.timeText.TextStyle = fyne.TextStyle{Bold: true}
	p.timeText.Alignment = fyne.TextAlignCenter

	p.dateText = canvas.NewText(p.translate("panel.placeholder.date", "01/01/2026"), theme.ForegroundColor())
	p.dateText.TextSize = 11
	p.dateText.Alignment = fyne.TextAlignCenter

	p.tempText = canvas.NewText(p.translate("panel.placeholder.temp", "--°C"), theme.ForegroundColor())
	p.tempText.TextSize = 42
	p.tempText.TextStyle = fyne.TextStyle{Bold: true}
	p.tempText.Alignment = fyne.TextAlignCenter

	p.descText = canvas.NewText(p.translate("panel.placeholder.desc", "--"), theme.ForegroundColor())
	p.descText.TextSize = 12
	p.descText.TextStyle = fyne.TextStyle{Italic: true}
	p.descText.Alignment = fyne.TextAlignCenter

	// Weather detail texts with emoji icons (vertical list format)
	p.humidityText = canvas.NewText("💧 Hum --%", theme.ForegroundColor())
	p.humidityText.TextSize = 12
	p.humidityText.Alignment = fyne.TextAlignCenter

	p.windText = canvas.NewText("💨 -- km/h", theme.ForegroundColor())
	p.windText.TextSize = 12
	p.windText.Alignment = fyne.TextAlignCenter

	p.windGustText = canvas.NewText("💨 Gust -- km/h", theme.ForegroundColor())
	p.windGustText.TextSize = 12
	p.windGustText.Alignment = fyne.TextAlignCenter

	p.dewPointText = canvas.NewText("💧 Dew --°C", theme.ForegroundColor())
	p.dewPointText.TextSize = 12
	p.dewPointText.Alignment = fyne.TextAlignCenter

	p.uvIndexText = canvas.NewText("☀ UV --", theme.ForegroundColor())
	p.uvIndexText.TextSize = 12
	p.uvIndexText.Alignment = fyne.TextAlignCenter

	// Pollution cells for vertical list
	p.pollutionCells = make(map[weather.PollutionMetric]*simplePollutionCell, len(weather.PollutionMetricOrder))
	for _, m := range weather.PollutionMetricOrder {
		icon := canvas.NewImageFromResource(loadSimpleAirIconResource(weather.AirIconFile(m)))
		icon.FillMode = canvas.ImageFillContain
		icon.SetMinSize(fyne.NewSize(20, 20))

		value := canvas.NewText("", theme.ForegroundColor())
		value.TextSize = 11
		value.Alignment = fyne.TextAlignLeading

		cell := &simplePollutionCell{
			icon:  icon,
			value: value,
		}
		cell.container = container.NewHBox(
			icon,
			value,
		)
		cell.container.Hide()
		p.pollutionCells[m] = cell
	}

	p.container = container.NewMax(p.buildLayout())
	return p
}

// SimplePanelWidth is the fixed width of a single Simple view city card.
const SimplePanelWidth = 180

// buildLayout constructs the vertical layout for the Simple view.
func (p *SimpleCityPanel) buildLayout() fyne.CanvasObject {
	var objects []fyne.CanvasObject

	// City name at top
	if p.displayFields.ShowCity {
		objects = append(objects, container.NewCenter(p.cityText))
	}

	// Weather icon with error indicator
	p.iconRow = container.NewBorder(nil, nil, nil, p.errorIcon, container.NewCenter(p.iconWidget))
	if p.displayFields.ShowIcon {
		objects = append(objects, container.NewCenter(p.iconRow))
	}

	// Time and date
	if p.displayFields.ShowTime {
		objects = append(objects, container.NewCenter(p.timeText))
	}
	if p.displayFields.ShowDate {
		objects = append(objects, container.NewCenter(p.dateText))
	}

	// Temperature
	if p.displayFields.ShowTemp {
		objects = append(objects, container.NewCenter(p.tempText))
	}

	// Condition description
	if p.displayFields.ShowDesc {
		objects = append(objects, container.NewCenter(p.descText))
	}

	// Weather details in a vertical list
	if p.displayFields.ShowHumidity {
		objects = append(objects, container.NewCenter(p.humidityText))
	}
	if p.displayFields.ShowWind {
		objects = append(objects, container.NewCenter(p.windText))
	}
	if p.displayFields.ShowWindGust {
		objects = append(objects, container.NewCenter(p.windGustText))
	}
	if p.displayFields.ShowDewPoint {
		objects = append(objects, container.NewCenter(p.dewPointText))
	}
	if p.displayFields.ShowUVIndex {
		objects = append(objects, container.NewCenter(p.uvIndexText))
	}

	// Pollution cells in a vertical list
	p.pollutionBox = container.NewVBox()
	for _, m := range weather.PollutionMetricOrder {
		if cell := p.pollutionCells[m]; cell != nil {
			p.pollutionBox.Add(container.NewCenter(cell.container))
		}
	}
	objects = append(objects, p.pollutionBox)

	content := container.NewVBox(objects...)

	// Wrap content in a rounded, bordered card with separator line at top
	card := newSimpleCardBackground()
	
	// Add a vertical line separator on the left side for visual separation between cities
	separator := canvas.NewRectangle(color.NRGBA{R: 90, G: 90, B: 96, A: 180})
	separator.SetMinSize(fyne.NewSize(1, 0))
	
	cardWithContent := container.NewStack(card, container.NewPadded(content))
	return container.NewBorder(nil, nil, separator, nil, cardWithContent)
}

// newSimpleCardBackground creates the rounded rectangle for a Simple view card.
func newSimpleCardBackground() *canvas.Rectangle {
	rect := canvas.NewRectangle(color.NRGBA{R: 44, G: 44, B: 48, A: 235})
	rect.CornerRadius = 8
	rect.StrokeColor = color.NRGBA{R: 90, G: 90, B: 96, A: 128}
	rect.StrokeWidth = 0
	return rect
}

// ApplyDisplayFields updates the panel's visibility configuration and rebuilds the layout.
func (p *SimpleCityPanel) ApplyDisplayFields(df *config.DisplayFields) {
	if df == nil {
		df = config.DefaultDisplayFields()
	}
	p.displayFields = df
	p.container.RemoveAll()
	p.container.Add(p.buildLayout())
	p.applyPollutionCells()
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

	for _, m := range weather.PollutionMetricOrder {
		cell := p.pollutionCells[m]
		if cell == nil {
			continue
		}
		if row, ok := planned[m]; ok {
			cell.value.Text = row.ValueText
			cell.value.Refresh()
			cell.container.Show()
		} else {
			cell.container.Hide()
		}
	}
	if p.pollutionBox != nil {
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
	p.lastData = data
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

	// Update labels
	p.cityText.Text = weather.FormatCityRegion(data.CityName, data.Region)
	p.cityText.Refresh()

	p.tempText.Text = weather.FormatTemperature(data.Temperature, tempUnit)
	p.tempText.Refresh()

	p.descText.Text = weather.FormatDescription(data.Description, p.lm)
	p.descText.Refresh()

	// Weather details
	p.humidityText.Text = weather.FormatHumidity(data.Humidity, p.lm)
	p.humidityText.Refresh()

	p.windText.Text = weather.FormatWind(data.WindSpeed, windUnit) + " " + weather.FormatWindDir(data.WindDirection)
	p.windText.Refresh()

	p.windGustText.Text = weather.FormatWindGust(data.WindGust, windUnit, p.lm)
	p.windGustText.Refresh()

	p.dewPointText.Text = weather.FormatDewPoint(data.DewPoint, p.lm)
	p.dewPointText.Refresh()

	p.uvIndexText.Text = weather.FormatUVIndex(data.UVIndex)
	p.uvIndexText.Refresh()

	// Hide error indicator on successful update.
	p.errorIcon.Hide()

	// Rebuild the layout and refresh pollution cells
	p.container.RemoveAll()
	p.container.Add(p.buildLayout())
	p.applyPollutionCells()
	p.container.Refresh()
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
	p.StopClock()

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

	go func(loc *time.Location, initialNight bool) {
		currentNight := initialNight
		for {
			select {
			case <-p.stopCh:
				return
			case t := <-p.timeTicker.C:
				localNow := t.In(loc)
				timeStr := weather.FormatTime(t, timezone, p.lm)
				dateStr := weather.FormatDate(t, timezone, p.lm)
				isNightNow := weather.IsNight(localNow)

				fyne.Do(func() {
					p.timeText.Text = timeStr
					p.timeText.Refresh()
					p.dateText.Text = dateStr
					p.dateText.Refresh()

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
	}(loc, lastNight)
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
