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

// pollutionRowWidgets holds the reusable widgets for one pollution metric
// tile: a vertical box containing the metric icon, its full name, and its
// value label.
type pollutionRowWidgets struct {
	row   *gtk.Box
	icon  *gtk.Image
	name  *gtk.Label
	value *gtk.Label
}

// metricTile is one weather-metric tile in the right-hand grid: a bordered,
// transparent cell showing an emoji, the metric name, and its value. The
// value label is updated with fresh data; the emoji and name are set once.
type metricTile struct {
	box   *gtk.Box   // the tile container (bordered cell)
	name  *gtk.Label // metric name, e.g. "Humidity"
	value *gtk.Label // formatted value, e.g. "84%"
}

// cityPanel holds all GTK widgets for a single city's weather display.
//
// Layout is a horizontal "card": a left info block (location, weather icon
// beside time/date/temperature/condition) sits above a right metrics grid
// (humidity, wind, wind gust, dew point, pressure, UV index). A row of
// air-quality tiles runs along the bottom of the card.
type cityPanel struct {
	root *gtk.Box // top-level card box (vertical)

	topBox      *gtk.Box  // horizontal: weather icon + time/date/temp/desc
	nameBox     *gtk.Box  // left info block (vertical): location + topBox
	metricsGrid *gtk.Grid // right metrics grid
	separator   *gtk.Box  // thin divider between top region and air row

	icon    *gtk.Image
	iconBg  *gtk.EventBox // tinted background wrapper behind the weather icon
	cityLbl *gtk.Label
	timeLbl *gtk.Label
	dateLbl *gtk.Label
	tempLbl *gtk.Label
	descLbl *gtk.Label

	// Weather metric tiles (right-hand grid). Each tile shows an emoji, the
	// metric name, and its value. The value labels are updated in update().
	humidTile, windTile, windGustTile   *metricTile
	dewPointTile, pressureTile, uvTile  *metricTile

	errorLbl *gtk.Label

	pollutionBox      *gtk.Box // outer horizontal container for pollution layout
	pollutionBoxRight *gtk.Box // right-aligned box for non-AQI pollution tiles
	pollutionRows     map[weather.PollutionMetric]*pollutionRowWidgets

	// pollutionFields is the current metric selection driving row visibility.
	// nil falls back to config.DefaultPollutionFields() in PlanPollutionRows.
	pollutionFields *config.PollutionFields

	// displayFields is the current display-field selection driving which
	// metrics appear in the right-hand grid and which info elements are shown.
	displayFields *config.DisplayFields

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

// newCityPanel creates a new panel for the given city.
func newCityPanel(city, region, timezone string, lm *i18n.LocaleManager) (*cityPanel, error) {
	p := &cityPanel{city: city, region: region, timezone: timezone, lm: lm}

	// Root box — the card. Vertical: top info region, separator, air row.
	root, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 8)
	if err != nil {
		return nil, err
	}
	root.SetName("panel-" + city)
	sc, _ := root.GetStyleContext()
	sc.AddClass("city-panel")
	root.SetSizeRequest(cardWidth, -1)
	p.root = root

	// ── Location (top of the card) ───────────────────────────────────────────
	cityLbl, err := gtk.LabelNew("📍 " + city + ", " + region)
	if err != nil {
		return nil, err
	}
	cityLbl.SetHAlign(gtk.ALIGN_START)
	cityLbl.SetEllipsize(3) // PANGO_ELLIPSIZE_END
	sc, _ = cityLbl.GetStyleContext()
	sc.AddClass("city-label")
	p.cityLbl = cityLbl
	root.PackStart(cityLbl, false, false, 0)

	// ── Top region: [ left info block ] [ metrics grid ] ─────────────────────
	topRegion, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 12)
	if err != nil {
		return nil, err
	}
	root.PackStart(topRegion, false, false, 0)

	// Left info block: weather icon on the left, with time/date/temp/desc
	// stacked in a column to its right.
	nameBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 12)
	if err != nil {
		return nil, err
	}
	p.nameBox = nameBox
	topRegion.PackStart(nameBox, false, false, 0)

	iconWidget, _ := gtk.ImageNew()
	iconWidget.SetSizeRequest(96, 96)
	iconWidget.SetHAlign(gtk.ALIGN_CENTER)
	iconWidget.SetVAlign(gtk.ALIGN_CENTER)
	p.icon = iconWidget
	// The weather icon PNGs have transparent pixels. On an app-paintable,
	// fully-transparent window those pixels would reveal the desktop instead
	// of the card tint. Wrapping the image in an EventBox that carries the
	// same tinted background as the card makes the icon composite over the
	// panel colour, matching the rest of the card.
	iconBg, err := gtk.EventBoxNew()
	if err != nil {
		return nil, err
	}
	iconBg.SetHAlign(gtk.ALIGN_CENTER)
	iconBg.SetVAlign(gtk.ALIGN_CENTER)
	// Windowless so the card background composites through the icon's
	// transparent PNG pixels, matching the rest of the card.
	iconBg.SetVisibleWindow(false)
	sc, _ = iconBg.GetStyleContext()
	sc.AddClass("icon-bg")
	iconBg.Add(iconWidget)
	p.iconBg = iconBg
	nameBox.PackStart(iconBg, false, false, 0)

	// Column to the right of the icon: time, date, temperature, condition.
	infoCol, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
	if err != nil {
		return nil, err
	}
	infoCol.SetVAlign(gtk.ALIGN_CENTER)
	nameBox.PackStart(infoCol, false, false, 0)

	timeLbl, err := gtk.LabelNew("--:--:--")
	if err != nil {
		return nil, err
	}
	timeLbl.SetHAlign(gtk.ALIGN_START)
	sc, _ = timeLbl.GetStyleContext()
	sc.AddClass("time-label")
	p.timeLbl = timeLbl
	infoCol.PackStart(timeLbl, false, false, 0)

	dateLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	dateLbl.SetHAlign(gtk.ALIGN_START)
	sc, _ = dateLbl.GetStyleContext()
	sc.AddClass("date-label")
	p.dateLbl = dateLbl
	infoCol.PackStart(dateLbl, false, false, 0)

	tempLbl, err := gtk.LabelNew("--°C")
	if err != nil {
		return nil, err
	}
	tempLbl.SetHAlign(gtk.ALIGN_START)
	sc, _ = tempLbl.GetStyleContext()
	sc.AddClass("temp-label")
	p.tempLbl = tempLbl
	infoCol.PackStart(tempLbl, false, false, 0)

	descLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	descLbl.SetHAlign(gtk.ALIGN_START)
	descLbl.SetLineWrap(true)
	sc, _ = descLbl.GetStyleContext()
	sc.AddClass("desc-label")
	p.descLbl = descLbl
	infoCol.PackStart(descLbl, false, false, 0)

	// ── Right metrics grid (3 columns × 2 rows of bordered tiles) ────────────
	metricsGrid, err := gtk.GridNew()
	if err != nil {
		return nil, err
	}
	metricsGrid.SetColumnSpacing(0)
	metricsGrid.SetRowSpacing(0)
	metricsGrid.SetVAlign(gtk.ALIGN_CENTER)
	metricsGrid.SetHAlign(gtk.ALIGN_END)
	metricsGrid.SetRowHomogeneous(true)
	metricsGrid.SetColumnHomogeneous(true)
	sc, _ = metricsGrid.GetStyleContext()
	sc.AddClass("metrics-grid")
	p.metricsGrid = metricsGrid
	topRegion.PackEnd(metricsGrid, false, false, 0)

	// Build the six metric tiles (icon/emoji + name + value).
	p.humidTile, err = p.newMetricTile("💧", weather.HumidityDisplay(0, p.lm).Name)
	if err != nil {
		return nil, err
	}
	p.windTile, err = p.newMetricTile("💨", weather.WindDisplay(0, 0, config.WindSpeedUnitKmh, p.lm).Name)
	if err != nil {
		return nil, err
	}
	p.windGustTile, err = p.newMetricTile("🌬", weather.WindGustDisplay(0, config.WindSpeedUnitKmh, p.lm).Name)
	if err != nil {
		return nil, err
	}
	p.dewPointTile, err = p.newMetricTile("💧", weather.DewPointDisplay(1, p.lm).Name)
	if err != nil {
		return nil, err
	}
	p.pressureTile, err = p.newMetricTile("🌡", weather.PressureDisplay(0, p.lm).Name)
	if err != nil {
		return nil, err
	}
	p.uvTile, err = p.newMetricTile("☀", weather.UVIndexDisplay(0, p.lm).Name)
	if err != nil {
		return nil, err
	}

	// ── Separator between the top region and the air-quality row ─────────────
	separator, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		return nil, err
	}
	separator.SetSizeRequest(-1, 1)
	sc, _ = separator.GetStyleContext()
	sc.AddClass("card-separator")
	p.separator = separator
	root.PackStart(separator, false, false, 0)

	// ── Air-quality row ──────────────────────────────────────────────────────
	// A horizontal container with AQI on the left and all other pollution
	// metrics grouped on the right. Uses a spacer to push non-AQI tiles right.
	pollutionBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		return nil, err
	}
	pollutionBox.SetHAlign(gtk.ALIGN_FILL)
	pollutionBox.SetHExpand(true)
	p.pollutionBox = pollutionBox
	root.PackStart(pollutionBox, false, false, 0)

	// Right-side box for non-AQI pollution tiles (right-aligned)
	pollutionBoxRight, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 10)
	if err != nil {
		return nil, err
	}
	pollutionBoxRight.SetHAlign(gtk.ALIGN_END)
	p.pollutionBoxRight = pollutionBoxRight

	p.pollutionRows = make(map[weather.PollutionMetric]*pollutionRowWidgets, len(weather.PollutionMetricOrder))
	for _, metric := range weather.PollutionMetricOrder {
		tile, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 1)
		if err != nil {
			return nil, err
		}
		tile.SetHAlign(gtk.ALIGN_CENTER)
		sc, _ = tile.GetStyleContext()
		sc.AddClass("air-tile")

		tileIcon, err := gtk.ImageNew()
		if err != nil {
			return nil, err
		}
		tileIcon.SetHAlign(gtk.ALIGN_CENTER)
		tile.PackStart(tileIcon, false, false, 0)

		tileName, err := gtk.LabelNew(weather.PollutionMetricName(metric, p.lm))
		if err != nil {
			return nil, err
		}
		tileName.SetHAlign(gtk.ALIGN_CENTER)
		tileName.SetJustify(gtk.JUSTIFY_CENTER)
		tileName.SetLineWrap(true)
		tileName.SetLineWrapMode(2) // PANGO_WRAP_WORD_CHAR
		sc, _ = tileName.GetStyleContext()
		sc.AddClass("air-name")
		tile.PackStart(tileName, false, false, 0)

		tileValue, err := gtk.LabelNew("")
		if err != nil {
			return nil, err
		}
		tileValue.SetHAlign(gtk.ALIGN_CENTER)
		tileValue.SetJustify(gtk.JUSTIFY_CENTER)
		sc, _ = tileValue.GetStyleContext()
		sc.AddClass("air-label")
		tile.PackStart(tileValue, false, false, 0)

		// AQI goes directly in the main pollution box (left-aligned)
		// All other metrics go in the right-aligned box
		if metric == weather.MetricAQI {
			pollutionBox.PackStart(tile, false, false, 0)
		} else {
			pollutionBoxRight.PackStart(tile, false, false, 0)
		}
		p.pollutionRows[metric] = &pollutionRowWidgets{
			row:   tile,
			icon:  tileIcon,
			name:  tileName,
			value: tileValue,
		}
	}

	// Add the right-aligned box for non-AQI metrics with expand to push it right
	pollutionBox.PackStart(pollutionBoxRight, true, true, 0)

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

	// Populate the metrics grid with the default field selection.
	p.layoutMetrics()

	// Start clock ticker.
	p.startClock()

	return p, nil
}

// cardWidth is the fixed width of a city card in pixels. The card is laid out
// horizontally (info block beside a metrics grid), so it is wider than the old
// vertical column, but kept as compact as the content allows.
const cardWidth = 380

// newMetricTile builds one bordered, transparent metric cell containing the
// given emoji glyph, the metric name, and an (initially empty) value label.
// The emoji and name sit on the top row; the value is shown in bold below.
func (p *cityPanel) newMetricTile(emoji, name string) (*metricTile, error) {
	box, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
	if err != nil {
		return nil, err
	}
	box.SetHExpand(true)
	sc, _ := box.GetStyleContext()
	sc.AddClass("metric-tile")

	// Header row: emoji + name.
	header, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	if err != nil {
		return nil, err
	}
	header.SetHAlign(gtk.ALIGN_START)
	box.PackStart(header, false, false, 0)

	emojiLbl, err := gtk.LabelNew(emoji)
	if err != nil {
		return nil, err
	}
	sc, _ = emojiLbl.GetStyleContext()
	sc.AddClass("metric-emoji")
	header.PackStart(emojiLbl, false, false, 0)

	nameLbl, err := gtk.LabelNew(name)
	if err != nil {
		return nil, err
	}
	nameLbl.SetHAlign(gtk.ALIGN_START)
	sc, _ = nameLbl.GetStyleContext()
	sc.AddClass("metric-name")
	header.PackStart(nameLbl, false, false, 0)

	valueLbl, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}
	valueLbl.SetHAlign(gtk.ALIGN_START)
	sc, _ = valueLbl.GetStyleContext()
	sc.AddClass("metric-value")
	box.PackStart(valueLbl, false, false, 0)

	return &metricTile{box: box, name: nameLbl, value: valueLbl}, nil
}

// metricGridEntry pairs a metric tile with the display-field flag controlling
// its visibility, for laying out the 3×2 metrics grid.
type metricGridEntry struct {
	tile *metricTile
	show bool
}

// layoutMetrics rebuilds the right-hand metrics grid from the current
// display-field selection. Visible tiles are placed left-to-right, top-to-
// bottom in a three-column grid (so up to six tiles form 3×2). Tiles are
// reused (never destroyed) so their values persist across re-layouts.
func (p *cityPanel) layoutMetrics() {
	df := p.displayFields
	if df == nil {
		df = config.DefaultDisplayFields()
	}

	entries := []metricGridEntry{
		{p.humidTile, df.ShowHumidity},
		{p.windTile, df.ShowWind},
		{p.windGustTile, df.ShowWindGust},
		{p.dewPointTile, df.ShowDewPoint},
		{p.pressureTile, df.ShowPressure},
		{p.uvTile, df.ShowUVIndex},
	}

	// Detach any currently-attached tiles before re-adding the visible ones.
	for _, e := range entries {
		if parent, _ := e.tile.box.GetParent(); parent != nil {
			p.metricsGrid.Remove(e.tile.box)
		}
	}

	const cols = 3
	idx := 0
	for _, e := range entries {
		if !e.show {
			continue
		}
		col := idx % cols
		row := idx / cols
		p.metricsGrid.Attach(e.tile.box, col, row, 1, 1)
		e.tile.box.Show()
		idx++
	}
	p.metricsGrid.ShowAll()
}

// update refreshes all displayed fields with new weather data.
func (p *cityPanel) update(d *weather.WeatherData, tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme) {
	if d == nil {
		return
	}
	p.lastData = d
	p.lastUnit = tempUnit

	p.errorLbl.Hide()

	p.cityLbl.SetText("📍 " + d.CityName + ", " + d.Region)
	p.tempLbl.SetText(weather.FormatTemperature(d.Temperature, tempUnit))
	p.descLbl.SetText(weather.FormatDescription(d.Description, p.lm))

	// Weather metric tiles — set the value labels (names are fixed).
	p.humidTile.value.SetText(weather.HumidityDisplay(d.Humidity, p.lm).Value)
	p.windTile.value.SetText(weather.WindDisplay(d.WindSpeed, d.WindDirection, windUnit, p.lm).Value)
	p.windGustTile.value.SetText(weather.WindGustDisplay(d.WindGust, windUnit, p.lm).Value)
	p.dewPointTile.value.SetText(weather.DewPointDisplay(d.DewPoint, p.lm).Value)
	p.pressureTile.value.SetText(weather.PressureDisplay(d.Pressure, p.lm).Value)
	p.uvTile.value.SetText(weather.UVIndexDisplay(d.UVIndex, p.lm).Value)

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
			// AQI is in pollutionBox, others are in pollutionBoxRight
			if metric == weather.MetricAQI {
				p.pollutionBox.ReorderChild(w.row, 0)
			} else {
				// Calculate position within right box (excluding AQI)
				rightIndex := planIndex[metric]
				if _, hasAQI := planned[weather.MetricAQI]; hasAQI && rightIndex > 0 {
					rightIndex-- // Adjust for AQI being in separate container
				}
				p.pollutionBoxRight.ReorderChild(w.row, rightIndex)
			}
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

// setTintAlpha updates the panel's tint alpha value used for compositing icons.
// When the app opacity changes, the tint behind icons must change to match.
func (p *cityPanel) setTintAlpha(alpha float64) {
	p.tintAlpha = alpha
	// Reload icons so they composite with the new tint.
	if p.lastIconCode != "" {
		p.loadIcon(p.lastIconCode, p.iconSize)
	}
	// Reload pollution icons for all visible rows.
	if p.lastData != nil {
		p.applyPollutionRows(p.pollutionFields)
	}
}

// applyDisplayFields shows or hides individual elements based on the config.
// Left-block elements (location, icon, time, date, temperature, condition) are
// toggled directly. The right-hand metrics grid is rebuilt via layoutMetrics,
// which attaches only the visible metric tiles.
func (p *cityPanel) applyDisplayFields(df *config.DisplayFields) {
	if df == nil {
		df = config.DefaultDisplayFields()
	}
	p.displayFields = df

	setVisible := func(w gtk.IWidget, show bool) {
		if show {
			w.ToWidget().Show()
		} else {
			w.ToWidget().Hide()
		}
	}
	// Location.
	setVisible(p.cityLbl, df.ShowCity)
	// Left info block (icon + time/date/temp/desc).
	setVisible(p.nameBox, df.ShowIcon || df.ShowTemp || df.ShowDesc || df.ShowTime || df.ShowDate)
	setVisible(p.iconBg, df.ShowIcon)
	setVisible(p.timeLbl, df.ShowTime)
	setVisible(p.dateLbl, df.ShowDate)
	setVisible(p.tempLbl, df.ShowTemp)
	setVisible(p.descLbl, df.ShowDesc)

	// Right metrics grid: attach only the visible tiles.
	p.layoutMetrics()
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
		scaled = pb
		targetW = origW
		targetH = origH
	}

	// Composite over the card tint so transparent pixels show the card
	// background instead of punching through to the desktop.
	if p.tintAlpha > 0 {
		scaled = compositeOverTint(scaled, p.tintAlpha)
		// Account for the padding added by compositeOverTint
		targetW += tintPadding * 2
		targetH += tintPadding * 2
	}

	p.icon.SetFromPixbuf(scaled)
	// Use actual scaled dimensions to avoid transparent stripes around non-square icons.
	p.icon.SetSizeRequest(targetW, targetH)
	p.icon.SetVAlign(gtk.ALIGN_CENTER)
}

// pollutionIconSize is the pixel size for pollution metric icons rendered next to text.
const pollutionIconSize = 48

// tintPadding is the extra pixels added around the icon when compositing over
// the card tint. This eliminates thin gaps between the icon background and the
// card background that can appear due to sub-pixel rendering or alignment.
const tintPadding = 2

// compositeOverTint creates a new pixbuf that composites src over a solid
// fill of the card's background tint (rgba 20,20,20, alpha). This flattens
// the transparent PNG pixels so they show the card tint instead of punching
// through to the desktop. alpha is in the 0.0–1.0 range and controls how
// transparent the background fill is, matching the card's opacity.
//
// The background is extended by tintPadding pixels on all sides to ensure
// complete coverage and eliminate any visible gaps at the icon edges.
func compositeOverTint(src *gdk.Pixbuf, alpha float64) *gdk.Pixbuf {
	if src == nil {
		return nil
	}
	w := src.GetWidth()
	h := src.GetHeight()
	if w <= 0 || h <= 0 {
		return src
	}

	// Create a new RGBA pixbuf filled with the card tint, extended by padding
	// on all sides to eliminate edge gaps.
	dstW := w + tintPadding*2
	dstH := h + tintPadding*2
	dst, err := gdk.PixbufNew(gdk.COLORSPACE_RGB, true, 8, dstW, dstH)
	if err != nil || dst == nil {
		return src
	}

	// Fill with the card tint color rgba(20,20,20, alpha). The alpha matches
	// the card's transparency so the icon background blends with the card.
	r := uint32(20)
	g := uint32(20)
	b := uint32(20)
	a := uint32(alpha * 255)
	if a > 255 {
		a = 255
	}
	fillColor := (r << 24) | (g << 16) | (b << 8) | a
	dst.Fill(fillColor)

	// Composite the source icon centered over the filled background. The gotk3
	// Composite signature:
	//   Composite(dest, destX, destY, destW, destH, offsetX, offsetY, scaleX, scaleY, interp, alpha)
	// We composite at 1:1 scale, full opacity, offset by tintPadding.
	src.Composite(dst, tintPadding, tintPadding, w, h, float64(tintPadding), float64(tintPadding), 1.0, 1.0, gdk.INTERP_BILINEAR, 255)

	return dst
}

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
		scaled = pb
		targetW = origW
		targetH = origH
	}

	// Composite over the card tint so transparent pixels show the card
	// background instead of punching through to the desktop.
	if p.tintAlpha > 0 {
		scaled = compositeOverTint(scaled, p.tintAlpha)
		// Account for the padding added by compositeOverTint
		targetW += tintPadding * 2
		targetH += tintPadding * 2
	}

	img.SetFromPixbuf(scaled)
	img.SetSizeRequest(targetW, targetH)
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
