package panel

import (
	"fmt"
	"log"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"
)

// CityPanel renders weather data for a single city within the widget.
type CityPanel struct {
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
	separatorRow  *fyne.Container
	errorIcon     *canvas.Image
	lastData      *weather.WeatherData  // cached for re-render on unit change
	lastTempUnit  config.TemperatureUnit
	lastWindUnit  config.WindSpeedUnit
	displayFields *config.DisplayFields // current visibility config

	mu         sync.Mutex
	timeTicker *time.Ticker
	stopCh     chan struct{}
}

// translate returns the translated string for the given key using the panel's
// LocaleManager. If the LocaleManager is nil, it returns the provided fallback.
func (p *CityPanel) translate(key, fallback string) string {
	if p.lm != nil {
		return p.lm.T(key)
	}
	return fallback
}

// loadIconFromAssets reads an icon PNG from the embedded assets FS and returns it as a Fyne resource.
func loadIconFromAssets(iconCode string) fyne.Resource {
	path := fmt.Sprintf("icons/%s.png", iconCode)
	data, err := assets.Icons.ReadFile(path)
	if err != nil {
		return nil
	}
	return fyne.NewStaticResource(path, data)
}

// NewCityPanel creates a new CityPanel with placeholder content.
// If lm is nil, hardcoded English defaults are used for placeholder text.
func NewCityPanel(lm *i18n.LocaleManager) *CityPanel {
	p := &CityPanel{
		lm:            lm,
		displayFields: config.DefaultDisplayFields(),
	}

	// Weather icon — start with the default cloudy icon.
	res := loadIconFromAssets(weather.IconCloudy)
	p.iconWidget = canvas.NewImageFromResource(res)
	p.iconWidget.FillMode = canvas.ImageFillContain
	p.iconWidget.SetMinSize(fyne.NewSize(64, 64))

	// Error indicator icon — hidden by default.
	p.errorIcon = canvas.NewImageFromResource(nil)
	p.errorIcon.FillMode = canvas.ImageFillContain
	p.errorIcon.SetMinSize(fyne.NewSize(16, 16))
	p.errorIcon.Hide()

	// Labels with appealing typography.
	p.cityText = canvas.NewText(p.translate("panel.placeholder.city", "City, RG"), theme.ForegroundColor())
	p.cityText.TextSize = 18
	p.cityText.TextStyle = fyne.TextStyle{Bold: true}
	p.cityText.Alignment = fyne.TextAlignCenter

	p.tempText = canvas.NewText(p.translate("panel.placeholder.temp", "--°C"), theme.ForegroundColor())
	p.tempText.TextSize = 42
	p.tempText.TextStyle = fyne.TextStyle{Bold: true}
	p.tempText.Alignment = fyne.TextAlignCenter

	p.descText = canvas.NewText(p.translate("panel.placeholder.desc", "--"), theme.ForegroundColor())
	p.descText.TextSize = 12
	p.descText.TextStyle = fyne.TextStyle{Italic: true}
	p.descText.Alignment = fyne.TextAlignCenter

	p.humidityText = canvas.NewText(p.translate("panel.placeholder.humidity", "💧 Hum --%"), theme.ForegroundColor())
	p.humidityText.TextSize = 12
	p.humidityText.TextStyle = fyne.TextStyle{Italic: true}
	p.humidityText.Alignment = fyne.TextAlignCenter

	p.windText = canvas.NewText(p.translate("panel.placeholder.wind", "💨 -- km/h"), theme.ForegroundColor())
	p.windText.TextSize = 12
	p.windText.TextStyle = fyne.TextStyle{Italic: true}
	p.windText.Alignment = fyne.TextAlignCenter

	p.timeText = canvas.NewText(p.translate("panel.placeholder.time", "00:00:00"), theme.ForegroundColor())
	p.timeText.TextSize = 22
	p.timeText.TextStyle = fyne.TextStyle{Bold: true}
	p.timeText.Alignment = fyne.TextAlignCenter

	p.dateText = canvas.NewText(p.translate("panel.placeholder.date", "Monday, Jan 02"), theme.ForegroundColor())
	p.dateText.TextSize = 11
	p.dateText.Alignment = fyne.TextAlignCenter

	p.windGustText = canvas.NewText("", theme.ForegroundColor())
	p.windGustText.TextSize = 12
	p.windGustText.TextStyle = fyne.TextStyle{Italic: true}
	p.windGustText.Alignment = fyne.TextAlignCenter

	p.dewPointText = canvas.NewText("", theme.ForegroundColor())
	p.dewPointText.TextSize = 12
	p.dewPointText.TextStyle = fyne.TextStyle{Italic: true}
	p.dewPointText.Alignment = fyne.TextAlignCenter

	p.pressureText = canvas.NewText("", theme.ForegroundColor())
	p.pressureText.TextSize = 12
	p.pressureText.TextStyle = fyne.TextStyle{Italic: true}
	p.pressureText.Alignment = fyne.TextAlignCenter

	p.uvIndexText = canvas.NewText("", theme.ForegroundColor())
	p.uvIndexText.TextSize = 12
	p.uvIndexText.TextStyle = fyne.TextStyle{Italic: true}
	p.uvIndexText.Alignment = fyne.TextAlignCenter

	p.windDirText = canvas.NewText("", theme.ForegroundColor())
	p.windDirText.TextSize = 12
	p.windDirText.TextStyle = fyne.TextStyle{Italic: true}
	p.windDirText.Alignment = fyne.TextAlignCenter

	p.separatorRow = container.NewCenter(widget.NewSeparator())

	p.container = container.NewMax(p.buildLayout())
	return p
}

// buildLayout constructs the vertical layout hierarchy of labels.
func (p *CityPanel) buildLayout() fyne.CanvasObject {
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

// ApplyDisplayFields updates the panel's visibility configuration and rebuilds the layout.
func (p *CityPanel) ApplyDisplayFields(df *config.DisplayFields) {
	if df == nil {
		df = config.DefaultDisplayFields()
	}
	p.displayFields = df
	p.container.RemoveAll()
	p.container.Add(p.buildLayout())
	p.container.Refresh()
}

// Container returns the Fyne container for embedding in a parent layout.
func (p *CityPanel) Container() *fyne.Container {
	return p.container
}

// Update sets the panel content from the given weather data using the specified units.
func (p *CityPanel) Update(data *weather.WeatherData, tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit) {
	if data == nil {
		return
	}
	p.lastData = data // cache for re-render
	p.lastTempUnit = tempUnit
	p.lastWindUnit = windUnit

	// Update icon from embedded assets.
	iconCode := weather.MapConditionToIcon(data.IconCode, data.LocalTime)
	res := loadIconFromAssets(iconCode)
	if res != nil {
		p.iconWidget.Resource = res
		p.iconWidget.Refresh()
	}

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

	// Hide error indicator on successful update.
	p.errorIcon.Hide()
}

// Rerender re-applies the last cached WeatherData with new units.
// If no data has been cached yet, this is a no-op.
func (p *CityPanel) Rerender(tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit) {
	if p.lastData == nil {
		return
	}
	p.Update(p.lastData, tempUnit, windUnit)
}

// ShowError displays an error indicator on the panel.
// If stale is true, a persistent stale-data warning is shown.
// If stale is false, a small error indicator icon is shown.
func (p *CityPanel) ShowError(stale bool) {
	if stale {
		// Show stale warning — use storm icon as a warning indicator.
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
		// Show small error indicator — use fog icon as error indicator.
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
func (p *CityPanel) StartClock(timezone string) {
	log.Printf("CityPanel: starting clock for timezone %s", timezone)
	p.StopClock() // stop any existing clock first

	p.mu.Lock()
	defer p.mu.Unlock()

	p.timeTicker = time.NewTicker(1 * time.Second)
	p.stopCh = make(chan struct{})

	// Set the time immediately before the first tick.
	now := time.Now()
	p.timeText.Text = weather.FormatTime(now, timezone, p.lm)
	p.timeText.Refresh()
	p.dateText.Text = weather.FormatDate(now, timezone, p.lm)
	p.dateText.Refresh()

	go func() {
		for {
			select {
			case <-p.stopCh:
				return
			case t := <-p.timeTicker.C:
				timeStr := weather.FormatTime(t, timezone, p.lm)
				dateStr := weather.FormatDate(t, timezone, p.lm)
				fyne.Do(func() {
					p.timeText.Text = timeStr
					p.timeText.Refresh()
					p.dateText.Text = dateStr
					p.dateText.Refresh()
				})
			}
		}
	}()
}

// StopClock stops the time ticker goroutine.
func (p *CityPanel) StopClock() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.timeTicker != nil {
		p.timeTicker.Stop()
		close(p.stopCh)
		p.timeTicker = nil
		p.stopCh = nil
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
