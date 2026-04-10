package panel

import (
	"fmt"
	"log"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"weatherwidget/assets"
	"weatherwidget/internal/weather"
)

// CityPanel renders weather data for a single city within the widget.
type CityPanel struct {
	container  *fyne.Container
	iconWidget *canvas.Image
	tempLabel  *widget.Label
	descLabel  *widget.Label
	cityLabel  *widget.Label
	timeLabel  *widget.Label
	dateLabel  *widget.Label
	errorIcon  *canvas.Image

	mu         sync.Mutex
	timeTicker *time.Ticker
	stopCh     chan struct{}
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
func NewCityPanel() *CityPanel {
	p := &CityPanel{}

	// Weather icon — start with the default cloudy icon.
	res := loadIconFromAssets(weather.IconCloudy)
	p.iconWidget = canvas.NewImageFromResource(res)
	p.iconWidget.FillMode = canvas.ImageFillContain
	p.iconWidget.SetMinSize(fyne.NewSize(40, 40))

	// Error indicator icon — hidden by default.
	p.errorIcon = canvas.NewImageFromResource(nil)
	p.errorIcon.FillMode = canvas.ImageFillContain
	p.errorIcon.SetMinSize(fyne.NewSize(16, 16))
	p.errorIcon.Hide()

	// Labels with placeholder text.
	p.tempLabel = widget.NewLabel("--°C")
	p.tempLabel.Alignment = fyne.TextAlignCenter

	p.descLabel = widget.NewLabel("--")
	p.descLabel.Alignment = fyne.TextAlignCenter

	p.cityLabel = widget.NewLabel("City, RG")
	p.cityLabel.Alignment = fyne.TextAlignCenter

	p.timeLabel = widget.NewLabel("--:--:--")
	p.timeLabel.Alignment = fyne.TextAlignCenter

	p.dateLabel = widget.NewLabel("--/--/----")
	p.dateLabel.Alignment = fyne.TextAlignCenter

	// Icon row: weather icon + error icon overlay.
	iconRow := container.NewHBox(layout.NewSpacer(), p.iconWidget, p.errorIcon, layout.NewSpacer())

	// Vertical stack: city, icon, temp, description, time, date.
	p.container = container.NewVBox(
		container.NewCenter(p.cityLabel),
		iconRow,
		container.NewCenter(p.tempLabel),
		container.NewCenter(p.descLabel),
		container.NewCenter(p.timeLabel),
		container.NewCenter(p.dateLabel),
	)

	return p
}

// Container returns the Fyne container for embedding in a parent layout.
func (p *CityPanel) Container() *fyne.Container {
	return p.container
}

// Update sets the panel content from the given weather data.
func (p *CityPanel) Update(data *weather.WeatherData) {
	if data == nil {
		return
	}

	// Update icon from embedded assets.
	iconCode := weather.MapConditionToIcon(data.IconCode)
	res := loadIconFromAssets(iconCode)
	if res != nil {
		p.iconWidget.Resource = res
		p.iconWidget.Refresh()
	}

	// Update labels.
	p.tempLabel.SetText(weather.FormatTemperature(data.Temperature))
	p.descLabel.SetText(data.Description)
	p.cityLabel.SetText(weather.FormatCityRegion(data.CityName, data.Region))

	// Hide error indicator on successful update.
	p.errorIcon.Hide()
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
		p.descLabel.SetText("Data may be stale")
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
	p.timeLabel.SetText(weather.FormatTime(now, timezone))
	p.dateLabel.SetText(weather.FormatDate(now, timezone))

	go func() {
		for {
			select {
			case <-p.stopCh:
				return
			case t := <-p.timeTicker.C:
				timeStr := weather.FormatTime(t, timezone)
				dateStr := weather.FormatDate(t, timezone)
				fyne.Do(func() {
					p.timeLabel.SetText(timeStr)
					p.dateLabel.SetText(dateStr)
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
