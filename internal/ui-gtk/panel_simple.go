//go:build linux

package uitk

import (
	"strings"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"
)

// simplePollutionRowWidgets holds the reusable widgets for one pollution metric
// row in the simple view: a horizontal box containing the metric icon and its
// value label.
type simplePollutionRowWidgets struct {
	row   *gtk.Box
	icon  *gtk.Image
	value *gtk.Label
}

// simpleCityPanel holds all GTK widgets for a single city's weather display in
// the "simple / classic" layout: a narrow vertical column with the city name,
// weather icon, time, date, temperature, description, and a stacked list of
// metric and pollution rows. Multiple simpleCityPanels are laid out
// side-by-side by the manager (matching the original pre-enhanced design).
//
// This is a self-contained alternative to the enhanced *cityPanel; it reuses
// none of the enhanced panel's tile/grid machinery so the two views stay
// independent.
type simpleCityPanel struct {
	root        *gtk.Box      // top-level card box (vertical), packed side-by-side
	nameBox     *gtk.Box      // vertical: city name + icon
	icon        *gtk.Image
	iconBg      *gtk.EventBox // tinted background wrapper behind the weather icon
	cityLbl     *gtk.Label
	timeLbl     *gtk.Label
	dateLbl     *gtk.Label
	tempLbl     *gtk.Label
	descLbl     *gtk.Label
	humidLbl    *gtk.Label
	windLbl     *gtk.Label
	windRowBox  *gtk.Box
	windGustLbl *gtk.Label
	dewPointLbl *gtk.Label
	pressureLbl *gtk.Label
	uvIndexLbl  *gtk.Label
	windDirLbl  *gtk.Label
	errorLbl    *gtk.Label

	pollutionBox  *gtk.Box
	pollutionRows map[weather.PollutionMetric]*simplePollutionRowWidgets

	// pollutionFields is the current metric selection driving row visibility.
	// nil falls back to config.DefaultPollutionFields() in PlanPollutionRows.
	pollutionFields *config.PollutionFields

	city     string
	region   string
	timezone string
	lm       *i18n.LocaleManager

	lastData     *weather.WeatherData
	lastUnit     config.TemperatureUnit
	lastIconCode string // most recently loaded icon code, for resize without re-fetch
	iconSize     int    // current icon pixel size; 0 means default (96px)

	// tintAlpha is the card background alpha (0.0–1.0) used for compositing
	// icons. When icons are loaded, their transparent pixels are filled with
	// this tint so they match the card background instead of revealing the
	// desktop. Updated via setTintAlpha().
	tintAlpha float64

	clockTicker *time.Ticker
	clockStop   chan struct{}
}

// simpleCardWidth is the fixed width of a simple-view city column in pixels.
const simpleCardWidth = 158

// newSimpleCityPanel creates a new simple-view panel for the given city.
func newSimpleCityPanel(city, region, timezone string, lm *i18n.LocaleManager) (*simpleCityPanel, error) {
	p := &simpleCityPanel{city: city, region: region, timezone: timezone, lm: lm}

	// Root box — vertical layout with CSS class.
	root, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 4)
	if err != nil {
		return nil, err
	}
	root.SetName("panel-" + city)
	sc, _ := root.GetStyleContext()
	sc.AddClass("simple-city-panel")
	root.SetSizeRequest(simpleCardWidth, -1)
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
	sc.AddClass("simple-city-label")
	p.cityLbl = cityLbl

	iconWidget, _ := gtk.ImageNew()
	iconWidget.SetSizeRequest(96, 96)
	iconWidget.SetHAlign(gtk.ALIGN_CENTER)
	iconWidget.SetVAlign(gtk.ALIGN_CENTER)
	p.icon = iconWidget

	// The weather icon PNGs have transparent pixels. On an app-paintable,
	// fully-transparent window those pixels would reveal the desktop instead
	// of the card tint (visible as a coloured square behind the icon).
	// Wrapping the image in a windowless EventBox that carries the same tinted
	// background as the card makes the icon composite over the panel colour,
	// matching the enhanced view.
	iconBg, err := gtk.EventBoxNew()
	if err != nil {
		return nil, err
	}
	iconBg.SetHAlign(gtk.ALIGN_CENTER)
	iconBg.SetVAlign(gtk.ALIGN_CENTER)
	iconBg.SetVisibleWindow(false)
	sc, _ = iconBg.GetStyleContext()
	sc.AddClass("icon-bg")
	iconBg.Add(iconWidget)
	p.iconBg = iconBg

	nameBox.PackStart(cityLbl, false, false, 0)
	nameBox.PackStart(iconBg, false, false, 0)
	root.PackStart(nameBox, false, false, 0)

	// ── Time ────────────────────────────────────────────────────────────────
	timeLbl, err := gtk.LabelNew("--:--:--")
	if err != nil {
		return nil, err
	}
	timeLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = timeLbl.GetStyleContext()
	sc.AddClass("simple-time-label")
	p.timeLbl = timeLbl
	root.PackStart(timeLbl, false, false, 0)

	// ── Date ────────────────────────────────────────────────────────────────
	dateLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	dateLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = dateLbl.GetStyleContext()
	sc.AddClass("simple-date-label")
	p.dateLbl = dateLbl
	root.PackStart(dateLbl, false, false, 0)

	// ── Temperature ─────────────────────────────────────────────────────────
	tempLbl, err := gtk.LabelNew("--°C")
	if err != nil {
		return nil, err
	}
	tempLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = tempLbl.GetStyleContext()
	sc.AddClass("simple-temp-label")
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
	sc.AddClass("simple-desc-label")
	p.descLbl = descLbl
	root.PackStart(descLbl, false, false, 0)

	// ── Humidity row ─────────────────────────────────────────────────────────
	humidLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	humidLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = humidLbl.GetStyleContext()
	sc.AddClass("simple-info-label")
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
	sc.AddClass("simple-info-label")
	p.windLbl = windLbl
	windRowBox.PackStart(windLbl, false, false, 0)

	windDirLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	windDirLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = windDirLbl.GetStyleContext()
	sc.AddClass("simple-info-label")
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
	sc.AddClass("simple-info-label")
	p.windGustLbl = windGustLbl
	root.PackStart(windGustLbl, false, false, 0)

	// ── Dew point ────────────────────────────────────────────────────────────
	dewPointLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	dewPointLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = dewPointLbl.GetStyleContext()
	sc.AddClass("simple-info-label")
	p.dewPointLbl = dewPointLbl
	root.PackStart(dewPointLbl, false, false, 0)

	// ── Pressure ─────────────────────────────────────────────────────────────
	pressureLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	pressureLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = pressureLbl.GetStyleContext()
	sc.AddClass("simple-info-label")
	p.pressureLbl = pressureLbl
	root.PackStart(pressureLbl, false, false, 0)

	// ── UV index ─────────────────────────────────────────────────────────────
	uvIndexLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	uvIndexLbl.SetHAlign(gtk.ALIGN_CENTER)
	sc, _ = uvIndexLbl.GetStyleContext()
	sc.AddClass("simple-info-label")
	p.uvIndexLbl = uvIndexLbl
	root.PackStart(uvIndexLbl, false, false, 0)

	// ── Pollution rows ───────────────────────────────────────────────────────
	// A vertical container for the per-metric icon+value rows. The reusable
	// rows are created once here (one per weather.PollutionMetric) and reused
	// across updates; their visibility is controlled by applyPollutionRows.
	pollutionBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
	if err != nil {
		return nil, err
	}
	pollutionBox.SetHAlign(gtk.ALIGN_CENTER)
	p.pollutionBox = pollutionBox
	root.PackStart(pollutionBox, false, false, 0)

	p.pollutionRows = make(map[weather.PollutionMetric]*simplePollutionRowWidgets, len(weather.PollutionMetricOrder))
	for _, metric := range weather.PollutionMetricOrder {
		rowBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
		if err != nil {
			return nil, err
		}
		rowBox.SetHAlign(gtk.ALIGN_CENTER)

		rowIcon, err := gtk.ImageNew()
		if err != nil {
			return nil, err
		}
		rowBox.PackStart(rowIcon, false, false, 0)

		rowValue, err := gtk.LabelNew("")
		if err != nil {
			return nil, err
		}
		rowValue.SetHAlign(gtk.ALIGN_CENTER)
		sc, _ = rowValue.GetStyleContext()
		sc.AddClass("simple-info-label")
		rowBox.PackStart(rowValue, false, false, 0)

		pollutionBox.PackStart(rowBox, false, false, 0)
		p.pollutionRows[metric] = &simplePollutionRowWidgets{
			row:   rowBox,
			icon:  rowIcon,
			value: rowValue,
		}
	}

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

// rootBox returns the simple panel's top-level column box.
func (p *simpleCityPanel) rootBox() *gtk.Box { return p.root }

// setIconSize sets the weather icon pixel size for the simple panel.
func (p *simpleCityPanel) setIconSize(size int) { p.iconSize = size }

// update refreshes all displayed fields with new weather data.
func (p *simpleCityPanel) update(d *weather.WeatherData, tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme) {
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

	// Refresh pollution rows using the panel's current metric selection and the
	// freshly stored data. PlanPollutionRows falls back to defaults when nil.
	p.applyPollutionRows(p.pollutionFields)
}

// applyPollutionRows updates the pollution metric rows to reflect the given
// field selection and the panel's most recent weather data.
func (p *simpleCityPanel) applyPollutionRows(pf *config.PollutionFields) {
	p.pollutionFields = pf

	rows := weather.PlanPollutionRows(pf, weather.PollutionOf(p.lastData))

	planned := make(map[weather.PollutionMetric]weather.PollutionRow, len(rows))
	planIndex := make(map[weather.PollutionMetric]int, len(rows))
	for i, row := range rows {
		planned[row.Metric] = row
		planIndex[row.Metric] = i
	}

	for _, metric := range weather.PollutionMetricOrder {
		w := p.pollutionRows[metric]
		if w == nil {
			continue
		}
		if row, ok := planned[metric]; ok {
			p.loadAirIcon(w.icon, row.IconFile, simplePollutionIconSize)
			w.value.SetText(row.ValueText)
			p.pollutionBox.ReorderChild(w.row, planIndex[metric])
			w.row.ShowAll()
		} else {
			w.row.Hide()
		}
	}
}

// showError displays an error state on the panel.
func (p *simpleCityPanel) showError(isStale bool) {
	if isStale {
		p.errorLbl.SetText("⚠ Stale data")
	} else {
		p.errorLbl.SetText("⚠ Fetch error")
	}
	p.errorLbl.Show()
}

// setNoBackground is a hook for per-panel background adjustments. The card fill
// is painted globally (see manager.paintCards), so nothing per-panel is needed.
func (p *simpleCityPanel) setNoBackground(_ bool) {}

// setTintAlpha updates the panel's tint alpha value used for compositing icons.
// When the app opacity changes, the tint behind icons must change to match so
// the icon's transparent pixels show the card background instead of the desktop.
func (p *simpleCityPanel) setTintAlpha(alpha float64) {
	p.tintAlpha = alpha
	// Reload the weather icon so it composites with the new tint.
	if p.lastIconCode != "" {
		p.loadIcon(p.lastIconCode, p.iconSize)
	}
	// Reload pollution icons for all visible rows so they re-composite too.
	if p.lastData != nil {
		p.applyPollutionRows(p.pollutionFields)
	}
}

// applyDisplayFields shows or hides individual elements based on the config.
func (p *simpleCityPanel) applyDisplayFields(df *config.DisplayFields) {
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
	setVisible(p.iconBg, df.ShowIcon)
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
// pixel size. Pass size=0 to use the default 96 px.
func (p *simpleCityPanel) loadIcon(iconCode string, size int) {
	if size <= 0 {
		size = 96
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

	var data []byte
	var found bool
	for _, base := range candidateBases {
		for _, ext := range []string{".gif", ".webp", ".png", ""} {
			d, readErr := assets.Icons.ReadFile("icons/" + base + ext)
			if readErr == nil {
				data = d
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
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

	// Try animation path first (for animated GIFs).
	anim, animErr := loader.GetAnimation()
	if animErr == nil && anim != nil {
		staticPb := anim.GetStaticImage()
		if staticPb == nil {
			// Truly animated — let GTK handle playback.
			p.icon.SetFromAnimation(anim)
			p.icon.SetSizeRequest(size, size)
			p.icon.SetVAlign(gtk.ALIGN_CENTER)
			return
		}
	}

	// Static image path — scale to requested size preserving aspect ratio.
	pb, err := loader.GetPixbuf()
	if err != nil || pb == nil {
		p.icon.Clear()
		return
	}
	origW := pb.GetWidth()
	origH := pb.GetHeight()
	targetW := size
	targetH := size
	if origW > 0 && origH > 0 {
		if origW >= origH {
			targetH = int(float64(origH) * float64(size) / float64(origW))
		} else {
			targetW = int(float64(origW) * float64(size) / float64(origH))
		}
		if targetW < 1 {
			targetW = 1
		}
		if targetH < 1 {
			targetH = 1
		}
	}
	scaled, err := pb.ScaleSimple(targetW, targetH, gdk.INTERP_BILINEAR)
	if err != nil || scaled == nil {
		scaled = pb
		targetW = origW
		targetH = origH
	}

	// Composite over the card tint so the icon's transparent PNG pixels show
	// the card background instead of punching through to the desktop. This is
	// the same treatment the enhanced view applies (see compositeOverTint in
	// panel.go), which is what gives the icon its shadow/tinted backing.
	if p.tintAlpha > 0 {
		scaled = compositeOverTint(scaled, p.tintAlpha)
		// Account for the padding added by compositeOverTint.
		targetW += tintPadding * 2
		targetH += tintPadding * 2
	}

	p.icon.SetFromPixbuf(scaled)
	// Use actual scaled dimensions to avoid transparent stripes around non-square icons.
	p.icon.SetSizeRequest(targetW, targetH)
	p.icon.SetVAlign(gtk.ALIGN_CENTER)
}

// simplePollutionIconSize is the pixel size for pollution metric icons rendered
// next to text in the simple view.
const simplePollutionIconSize = 24

// loadAirIcon loads an air-quality icon file (e.g. "co.png") from the embedded
// AirIcons filesystem, scales it preserving aspect ratio, and sets it on img.
func (p *simpleCityPanel) loadAirIcon(img *gtk.Image, file string, size int) {
	if size <= 0 {
		size = simplePollutionIconSize
	}
	data, err := assets.AirIcons.ReadFile("air/" + file)
	if err != nil {
		img.Clear()
		return
	}
	loader, err := gdk.PixbufLoaderNew()
	if err != nil {
		img.Clear()
		return
	}
	if _, err := loader.Write(data); err != nil {
		img.Clear()
		return
	}
	if err := loader.Close(); err != nil {
		img.Clear()
		return
	}
	pb, err := loader.GetPixbuf()
	if err != nil || pb == nil {
		img.Clear()
		return
	}
	origW := pb.GetWidth()
	origH := pb.GetHeight()
	targetW := size
	targetH := size
	if origW > 0 && origH > 0 {
		if origW >= origH {
			targetH = int(float64(origH) * float64(size) / float64(origW))
		} else {
			targetW = int(float64(origW) * float64(size) / float64(origH))
		}
		if targetW < 1 {
			targetW = 1
		}
		if targetH < 1 {
			targetH = 1
		}
	}
	scaled, err := pb.ScaleSimple(targetW, targetH, gdk.INTERP_BILINEAR)
	if err != nil || scaled == nil {
		scaled = pb
		targetW = origW
		targetH = origH
	}

	// Composite over the card tint so the pollution icon's transparent PNG
	// pixels show the card background instead of punching through to the
	// desktop — the same treatment as the enhanced view's loadAirIcon.
	if p.tintAlpha > 0 {
		scaled = compositeOverTint(scaled, p.tintAlpha)
		targetW += tintPadding * 2
		targetH += tintPadding * 2
	}

	img.SetFromPixbuf(scaled)
	img.SetSizeRequest(targetW, targetH)
}

// startClock starts the clock ticker that updates the time/date labels every
// second using the panel's timezone.
func (p *simpleCityPanel) startClock() {
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
func (p *simpleCityPanel) stopClock() {
	if p.clockStop != nil {
		close(p.clockStop)
		p.clockStop = nil
	}
	if p.clockTicker != nil {
		p.clockTicker.Stop()
		p.clockTicker = nil
	}
}
