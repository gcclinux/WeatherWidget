//go:build linux

package uitk

import (
	"fmt"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"
)

// cityPanel holds all GTK widgets for a single city's weather display.
type cityPanel struct {
	root     *gtk.Box // top-level box, added to the main window's hbox
	nameBox  *gtk.Box // row: icon + city name
	icon     *gtk.Image
	cityLbl  *gtk.Label
	timeLbl  *gtk.Label
	dateLbl  *gtk.Label
	tempLbl  *gtk.Label
	descLbl  *gtk.Label
	infoLbl  *gtk.Label // humidity + wind
	errorLbl *gtk.Label

	city     string
	region   string
	timezone string
	lm       *i18n.LocaleManager

	lastData *weather.WeatherData
	lastUnit config.TemperatureUnit

	clockTicker *time.Ticker
	clockStop   chan struct{}
}

// newCityPanel creates a new panel for the given city.
func newCityPanel(city, region, timezone string, lm *i18n.LocaleManager) (*cityPanel, error) {
	p := &cityPanel{city: city, region: region, timezone: timezone, lm: lm}

	// Root box — vertical layout with CSS class.
	root, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 4)
	if err != nil {
		return nil, err
	}
	root.SetName("panel-" + city)
	sc, _ := root.GetStyleContext()
	sc.AddClass("city-panel")
	root.SetSizeRequest(158, -1)
	p.root = root

	// ── City row (icon + name) ──────────────────────────────────────────────
	nameBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	if err != nil {
		return nil, err
	}
	p.nameBox = nameBox

	iconWidget, _ := gtk.ImageNew()
	iconWidget.SetSizeRequest(32, 32)
	p.icon = iconWidget

	cityLbl, err := gtk.LabelNew(city + ", " + region)
	if err != nil {
		return nil, err
	}
	cityLbl.SetHAlign(gtk.ALIGN_START)
	cityLbl.SetEllipsize(3) // PANGO_ELLIPSIZE_END
	sc, _ = cityLbl.GetStyleContext()
	sc.AddClass("city-label")
	p.cityLbl = cityLbl

	nameBox.PackStart(iconWidget, false, false, 0)
	nameBox.PackStart(cityLbl, true, true, 0)
	root.PackStart(nameBox, false, false, 0)

	// ── Time ────────────────────────────────────────────────────────────────
	timeLbl, err := gtk.LabelNew("--:--:--")
	if err != nil {
		return nil, err
	}
	timeLbl.SetHAlign(gtk.ALIGN_START)
	sc, _ = timeLbl.GetStyleContext()
	sc.AddClass("time-label")
	p.timeLbl = timeLbl
	root.PackStart(timeLbl, false, false, 0)

	// ── Date ────────────────────────────────────────────────────────────────
	dateLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	dateLbl.SetHAlign(gtk.ALIGN_START)
	sc, _ = dateLbl.GetStyleContext()
	sc.AddClass("date-label")
	p.dateLbl = dateLbl
	root.PackStart(dateLbl, false, false, 0)

	// ── Temperature ─────────────────────────────────────────────────────────
	tempLbl, err := gtk.LabelNew("--°C")
	if err != nil {
		return nil, err
	}
	tempLbl.SetHAlign(gtk.ALIGN_START)
	sc, _ = tempLbl.GetStyleContext()
	sc.AddClass("temp-label")
	p.tempLbl = tempLbl
	root.PackStart(tempLbl, false, false, 0)

	// ── Description ─────────────────────────────────────────────────────────
	descLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	descLbl.SetHAlign(gtk.ALIGN_START)
	descLbl.SetLineWrap(true)
	sc, _ = descLbl.GetStyleContext()
	sc.AddClass("desc-label")
	p.descLbl = descLbl
	root.PackStart(descLbl, false, false, 0)

	// ── Humidity / wind ──────────────────────────────────────────────────────
	infoLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	infoLbl.SetHAlign(gtk.ALIGN_START)
	sc, _ = infoLbl.GetStyleContext()
	sc.AddClass("info-label")
	p.infoLbl = infoLbl
	root.PackStart(infoLbl, false, false, 0)

	// ── Error label ──────────────────────────────────────────────────────────
	errorLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	errorLbl.SetHAlign(gtk.ALIGN_START)
	errorLbl.SetNoShowAll(true)
	sc, _ = errorLbl.GetStyleContext()
	sc.AddClass("error-label")
	p.errorLbl = errorLbl
	root.PackStart(errorLbl, false, false, 0)

	// Start clock ticker.
	p.startClock()

	return p, nil
}

// update refreshes all labels with the given weather data.
func (p *cityPanel) update(d *weather.WeatherData, unit config.TemperatureUnit) {
	p.lastData = d
	p.lastUnit = unit

	p.errorLbl.Hide()

	p.cityLbl.SetText(d.CityName + ", " + d.Region)
	p.tempLbl.SetText(weather.FormatTemperature(d.Temperature, unit))
	p.descLbl.SetText(weather.FormatDescription(d.Description))
	p.infoLbl.SetText(weather.FormatHumidityWind(d.Humidity, d.WindSpeed, d.WindDirection))

	// Load weather icon.
	iconCode := weather.MapConditionToIcon(d.IconCode, d.LocalTime)
	p.loadIcon(iconCode)
}

// showError displays an error state on the panel.
func (p *cityPanel) showError(isStale bool) {
	if isStale {
		p.errorLbl.SetText("⚠ Stale data")
	} else {
		p.errorLbl.SetText("⚠ Fetch error")
	}
	p.errorLbl.Show()
}

// setNoBackground adjusts the panel background CSS class.
// (CSS is updated globally via applyCSSToScreen; this is a hook for per-panel
// adjustments if needed in the future.)
func (p *cityPanel) setNoBackground(_ bool) {}

// applyDisplayFields shows or hides individual elements based on the config.
func (p *cityPanel) applyDisplayFields(df *config.DisplayFields) {
	if df == nil {
		df = config.DefaultDisplayFields()
	}
	setVisible := func(w gtk.IWidget, show bool) {
		if show {
			w.ToWidget().Show()
		} else {
			w.ToWidget().Hide()
		}
	}
	setVisible(p.nameBox, df.ShowCity || df.ShowIcon)
	setVisible(p.cityLbl, df.ShowCity)
	setVisible(p.icon, df.ShowIcon)
	setVisible(p.tempLbl, df.ShowTemp)
	setVisible(p.descLbl, df.ShowDesc)
	setVisible(p.infoLbl, df.ShowHumidWind)
	setVisible(p.timeLbl, df.ShowTime)
	setVisible(p.dateLbl, df.ShowDate)
}

// loadIcon tries to load the weather icon from embedded assets.
func (p *cityPanel) loadIcon(iconCode string) {
	data, err := assets.Icons.ReadFile(fmt.Sprintf("icons/%s.png", iconCode))
	if err != nil {
		p.icon.Clear()
		return
	}
	loader, err := gdk.PixbufLoaderNew()
	if err != nil {
		p.icon.Clear()
		return
	}
	if _, err := loader.Write(data); err != nil {
		p.icon.Clear()
		return
	}
	if err := loader.Close(); err != nil {
		p.icon.Clear()
		return
	}
	pb, err := loader.GetPixbuf()
	if err != nil || pb == nil {
		p.icon.Clear()
		return
	}
	// Scale to 32x32 — if ScaleSimple fails, use the original but constrain the widget.
	scaled, err := pb.ScaleSimple(32, 32, gdk.INTERP_BILINEAR)
	if err != nil || scaled == nil {
		p.icon.SetFromPixbuf(pb)
	} else {
		p.icon.SetFromPixbuf(scaled)
	}
	// Always constrain the image widget size to prevent oversized rendering.
	p.icon.SetSizeRequest(32, 32)
}

// startClock starts the clock ticker that updates the time/date labels every
// second using the panel's timezone.
func (p *cityPanel) startClock() {
	if p.clockStop != nil {
		return
	}
	p.clockStop = make(chan struct{})
	p.clockTicker = time.NewTicker(time.Second)

	go func() {
		for {
			select {
			case <-p.clockStop:
				return
			case t := <-p.clockTicker.C:
				tz := p.timezone
				if tz == "" {
					tz = "UTC"
				}
				loc, err := time.LoadLocation(tz)
				if err != nil {
					loc = time.UTC
				}
				localT := t.In(loc)
				timeStr := weather.FormatTime(localT, tz, p.lm)
				dateStr := weather.FormatDate(localT, tz, p.lm)
				glib.IdleAdd(func() {
					p.timeLbl.SetText(timeStr)
					p.dateLbl.SetText(dateStr)
				})
			}
		}
	}()
}

// stopClock stops the clock goroutine.
func (p *cityPanel) stopClock() {
	if p.clockStop != nil {
		close(p.clockStop)
		p.clockStop = nil
	}
	if p.clockTicker != nil {
		p.clockTicker.Stop()
		p.clockTicker = nil
	}
}
