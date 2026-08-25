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
	root        *gtk.Box // top-level box, added to the main window's hbox
	nameBox     *gtk.Box // row: icon + city name
	icon        *gtk.Image
	cityLbl     *gtk.Label
	timeLbl     *gtk.Label
	dateLbl     *gtk.Label
	tempLbl     *gtk.Label
	descLbl     *gtk.Label
	humidLbl    *gtk.Label // humidity row
	windLbl     *gtk.Label // wind speed row
	windRowBox  *gtk.Box
	windGustLbl *gtk.Label
	dewPointLbl *gtk.Label
	pressureLbl *gtk.Label
	uvIndexLbl  *gtk.Label
	windDirLbl  *gtk.Label
	errorLbl    *gtk.Label

	city     string
	region   string
	timezone string
	lm       *i18n.LocaleManager

	lastData     *weather.WeatherData
	lastUnit     config.TemperatureUnit
	lastIconCode string // most recently loaded icon code, for resize without re-fetch
	iconSize     int    // current icon pixel size; 0 means default (32px)

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

	// ── City row (name then icon below) ─────────────────────────────────────
	nameBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 4)
	if err != nil {
		return nil, err
	}
	p.nameBox = nameBox

	cityLbl, err := gtk.LabelNew(city + ", " + region)
	if err != nil {
		return nil, err
	}
	cityLbl.SetHAlign(gtk.ALIGN_CENTER)
	cityLbl.SetEllipsize(3) // PANGO_ELLIPSIZE_END
	sc, _ = cityLbl.GetStyleContext()
	sc.AddClass("city-label")
	p.cityLbl = cityLbl

	iconWidget, _ := gtk.ImageNew()
	iconWidget.SetSizeRequest(32, 32)
	iconWidget.SetHAlign(gtk.ALIGN_CENTER)
	p.icon = iconWidget

	nameBox.PackStart(cityLbl, false, false, 0)
	nameBox.PackStart(iconWidget, false, false, 0)
	root.PackStart(nameBox, false, false, 0)

	// ── Time ────────────────────────────────────────────────────────────────
	timeLbl, err := gtk.LabelNew("--:--:--")
	if err != nil {
		return nil, err
	}
	timeLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = timeLbl.GetStyleContext()
	sc.AddClass("time-label")
	p.timeLbl = timeLbl
	root.PackStart(timeLbl, false, false, 0)

	// ── Date ────────────────────────────────────────────────────────────────
	dateLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	dateLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = dateLbl.GetStyleContext()
	sc.AddClass("date-label")
	p.dateLbl = dateLbl
	root.PackStart(dateLbl, false, false, 0)

	// ── Temperature ─────────────────────────────────────────────────────────
	tempLbl, err := gtk.LabelNew("--°C")
	if err != nil {
		return nil, err
	}
	tempLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = tempLbl.GetStyleContext()
	sc.AddClass("temp-label")
	p.tempLbl = tempLbl
	root.PackStart(tempLbl, false, false, 0)

	// ── Description ─────────────────────────────────────────────────────────
	descLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	descLbl.SetHAlign(gtk.ALIGN_CENTER)
	descLbl.SetLineWrap(true)
	sc, _ = descLbl.GetStyleContext()
	sc.AddClass("desc-label")
	p.descLbl = descLbl
	root.PackStart(descLbl, false, false, 0)

	// ── Humidity row ─────────────────────────────────────────────────────────
	humidLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	humidLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = humidLbl.GetStyleContext()
	sc.AddClass("info-label")
	p.humidLbl = humidLbl
	root.PackStart(humidLbl, false, false, 0)

	// ── Wind / wind-direction row ─────────────────────────────────────────────
	windRowBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	if err != nil {
		return nil, err
	}
	windRowBox.SetHAlign(gtk.ALIGN_CENTER)
	p.windRowBox = windRowBox

	windLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	windLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = windLbl.GetStyleContext()
	sc.AddClass("info-label")
	p.windLbl = windLbl
	windRowBox.PackStart(windLbl, false, false, 0)

	// ── Wind direction ───────────────────────────────────────────────────────
	windDirLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	windDirLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = windDirLbl.GetStyleContext()
	sc.AddClass("info-label")
	p.windDirLbl = windDirLbl
	windRowBox.PackStart(windDirLbl, false, false, 0)

	root.PackStart(windRowBox, false, false, 0)

	// ── Wind gust ────────────────────────────────────────────────────────────
	windGustLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	windGustLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = windGustLbl.GetStyleContext()
	sc.AddClass("info-label")
	p.windGustLbl = windGustLbl
	root.PackStart(windGustLbl, false, false, 0)

	// ── Dew point ────────────────────────────────────────────────────────────
	dewPointLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	dewPointLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = dewPointLbl.GetStyleContext()
	sc.AddClass("info-label")
	p.dewPointLbl = dewPointLbl
	root.PackStart(dewPointLbl, false, false, 0)

	// ── Pressure ─────────────────────────────────────────────────────────────
	pressureLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	pressureLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = pressureLbl.GetStyleContext()
	sc.AddClass("info-label")
	p.pressureLbl = pressureLbl
	root.PackStart(pressureLbl, false, false, 0)

	// ── UV index ─────────────────────────────────────────────────────────────
	uvIndexLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	uvIndexLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = uvIndexLbl.GetStyleContext()
	sc.AddClass("info-label")
	p.uvIndexLbl = uvIndexLbl
	root.PackStart(uvIndexLbl, false, false, 0)

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

// update refreshes all displayed fields with new weather data.
func (p *cityPanel) update(d *weather.WeatherData, tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme) {
	if d == nil {
		return
	}
	p.lastData = d
	p.lastUnit = tempUnit

	p.errorLbl.Hide()

	p.cityLbl.SetText(d.CityName + ", " + d.Region)
	p.tempLbl.SetText(weather.FormatTemperature(d.Temperature, tempUnit))
	p.descLbl.SetText(weather.FormatDescription(d.Description, p.lm))
	p.humidLbl.SetText(weather.FormatHumidity(d.Humidity, p.lm))
	p.windLbl.SetText(weather.FormatWind(d.WindSpeed, windUnit))
	p.windGustLbl.SetText(weather.FormatWindGust(d.WindGust, windUnit, p.lm))
	p.dewPointLbl.SetText(weather.FormatDewPoint(d.DewPoint, p.lm))
	p.pressureLbl.SetText(weather.FormatPressure(d.Pressure))
	p.uvIndexLbl.SetText(weather.FormatUVIndex(d.UVIndex))
	p.windDirLbl.SetText(weather.FormatWindDir(d.WindDirection))

	// Load weather icon at the current icon size.
	theme := config.IconThemeNew
	if len(iconTheme) > 0 && iconTheme[0] != "" {
		theme = iconTheme[0]
	}
	iconCode := weather.MapConditionToIconWithTheme(d.IconCode, d.LocalTime, theme)
	p.loadIcon(iconCode, p.iconSize)
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
	setVisible(p.humidLbl, df.ShowHumidity)
	setVisible(p.windLbl, df.ShowWind)
	setVisible(p.windDirLbl, df.ShowWind)
	setVisible(p.windRowBox, df.ShowWind)
	setVisible(p.windGustLbl, df.ShowWindGust)
	setVisible(p.dewPointLbl, df.ShowDewPoint)
	setVisible(p.pressureLbl, df.ShowPressure)
	setVisible(p.uvIndexLbl, df.ShowUVIndex)
	setVisible(p.timeLbl, df.ShowTime)
	setVisible(p.dateLbl, df.ShowDate)
}

// loadIcon tries to load the weather icon from embedded assets at the given
// pixel size. Pass size=0 to use the default 32 px.
func (p *cityPanel) loadIcon(iconCode string, size int) {
	if size <= 0 {
		size = 32
	}
	p.lastIconCode = iconCode // remember for live resize
	clean := strings.TrimPrefix(iconCode, "icons/")
	clean = strings.TrimPrefix(clean, "/")

	var candidateBases []string
	candidateBases = append(candidateBases, clean)
	if !strings.Contains(clean, "/") {
		candidateBases = append(candidateBases,
			"day/"+clean+"_day",
			"day/"+clean,
			"night/"+clean+"_night",
			"night/"+clean,
			"original/"+clean,
		)
	}

	for _, base := range candidateBases {
		for _, ext := range []string{".gif", ".webp", ".png", ""} {
			data, err = assets.Icons.ReadFile("icons/" + base + ext)
			if err == nil {
				break
			}
		}
		if err == nil {
			break
		}
	}
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

	anim, animErr := loader.GetAnimation()
	if animErr == nil && anim != nil && !anim.IsStaticImage() {
		p.icon.SetFromAnimation(anim)
	} else {
		pb, err := loader.GetPixbuf()
		if err != nil || pb == nil {
			p.icon.Clear()
			return
		}
		// Scale to the requested size; fall back to original on failure.
		scaled, err := pb.ScaleSimple(size, size, gdk.INTERP_BILINEAR)
		if err != nil || scaled == nil {
			p.icon.SetFromPixbuf(pb)
		} else {
			p.icon.SetFromPixbuf(scaled)
		}
	}
	p.icon.SetSizeRequest(size, size)
}

// applyIconSize rescales the currently displayed icon to a new pixel size.
// It is called from manager.SetFontSizes for live preview.
func (p *cityPanel) applyIconSize(size int) {
	if p.lastIconCode == "" {
		return
	}
	p.loadIcon(p.lastIconCode, size)
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
