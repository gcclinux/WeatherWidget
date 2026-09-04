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

// pollutionRowWidgets holds the reusable widgets for one pollution metric row:
// a horizontal box containing the metric icon and its value label.
type pollutionRowWidgets struct {
	row   *gtk.Box
	icon  *gtk.Image
	value *gtk.Label
}

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

	pollutionBox  *gtk.Box // vertical container for pollution rows
	pollutionRows map[weather.PollutionMetric]*pollutionRowWidgets

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
	iconWidget.SetSizeRequest(96, 96)
	iconWidget.SetHAlign(gtk.ALIGN_CENTER)
	iconWidget.SetVAlign(gtk.ALIGN_CENTER)
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

	// ── Pollution rows ───────────────────────────────────────────────────────
	// A vertical container for the per-metric icon+value rows, packed after the
	// UV-index row and before the error label. The nine reusable rows are
	// created once here (one per weather.PollutionMetric) and reused across
	// updates; their visibility is controlled later by applyPollutionRows.
	pollutionBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
	if err != nil {
		return nil, err
	}
	p.pollutionBox = pollutionBox
	root.PackStart(pollutionBox, false, false, 0)

	p.pollutionRows = make(map[weather.PollutionMetric]*pollutionRowWidgets, len(weather.PollutionMetricOrder))
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
		sc.AddClass("info-label")
		rowBox.PackStart(rowValue, false, false, 0)

		pollutionBox.PackStart(rowBox, false, false, 0)
		p.pollutionRows[metric] = &pollutionRowWidgets{
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

	// Refresh pollution rows using the panel's current metric selection and the
	// freshly stored data. PlanPollutionRows falls back to defaults when nil.
	p.applyPollutionRows(p.pollutionFields)
}

// applyPollutionRows updates the pollution metric rows to reflect the given
// field selection and the panel's most recent weather data. It stores pf as the
// current selection, plans the visible rows via the pure planner, then for each
// metric in canonical order either populates + shows the row (reordering it to
// its planned position so visible rows follow plan order) or hides it.
func (p *cityPanel) applyPollutionRows(pf *config.PollutionFields) {
	p.pollutionFields = pf

	rows := weather.PlanPollutionRows(pf, weather.PollutionOf(p.lastData))

	// Map metric -> planned row (and its position within the plan).
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
			p.loadAirIcon(w.icon, row.IconFile, pollutionIconSize)
			w.value.SetText(row.ValueText)
			p.pollutionBox.ReorderChild(w.row, planIndex[metric])
			w.row.ShowAll()
		} else {
			w.row.Hide()
		}
	}
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
		// GetStaticImage returns the static frame; if the animation is truly
		// animated, SetFromAnimation will play it. For static images we fall
		// through to the pixbuf path for proper scaling.
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
		// Scale so the largest dimension fits within `size`, preserving ratio.
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
		p.icon.SetFromPixbuf(pb)
	} else {
		p.icon.SetFromPixbuf(scaled)
	}
	// Always reserve a fixed square area so all panels align vertically.
	p.icon.SetSizeRequest(size, size)
	p.icon.SetVAlign(gtk.ALIGN_CENTER)
}

// pollutionIconSize is the pixel size for pollution metric icons rendered next to text.
const pollutionIconSize = 24

// loadAirIcon loads an air-quality icon file (e.g. "co.png") from the embedded
// AirIcons filesystem, scales it to `size` px preserving aspect ratio, and sets
// it on img. On any read/decode error it clears the image.
func (p *cityPanel) loadAirIcon(img *gtk.Image, file string, size int) {
	if size <= 0 {
		size = pollutionIconSize
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

	// Scale preserving aspect ratio to `size`, mirroring loadIcon's math.
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
		img.SetFromPixbuf(pb)
	} else {
		img.SetFromPixbuf(scaled)
	}
	img.SetSizeRequest(size, size)
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
