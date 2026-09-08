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
	"fyne.io/fyne/v2/layout"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"
)

// fixedWidthLayout is a custom Fyne layout that always reports a fixed width
// for its container, regardless of the child content width. This ensures the
// left info block takes consistent horizontal space across all city cards.
type fixedWidthLayout struct {
	width float32
}

func (f *fixedWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, o := range objects {
		if o.Visible() {
			h = fyne.Max(h, o.MinSize().Height)
		}
	}
	return fyne.NewSize(f.width, h)
}

func (f *fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		if o.Visible() {
			o.Resize(fyne.NewSize(f.width, size.Height))
			o.Move(fyne.NewPos(0, 0))
		}
	}
}

// compactPaddingLayout provides reduced vertical padding for metric tiles,
// giving a tighter cell height while maintaining horizontal padding.
type compactPaddingLayout struct {
	hPad float32 // horizontal padding
	vPad float32 // vertical padding (reduced for compact cells)
}

func (c *compactPaddingLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		if o.Visible() {
			s := o.MinSize()
			w = fyne.Max(w, s.Width)
			h = fyne.Max(h, s.Height)
		}
	}
	return fyne.NewSize(w+c.hPad*2, h+c.vPad*2)
}

func (c *compactPaddingLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	innerW := size.Width - c.hPad*2
	innerH := size.Height - c.vPad*2
	for _, o := range objects {
		if o.Visible() {
			o.Resize(fyne.NewSize(innerW, innerH))
			o.Move(fyne.NewPos(c.hPad, c.vPad))
		}
	}
}

// spacedHBoxLayout arranges objects horizontally with a fixed spacing between them.
type spacedHBoxLayout struct {
	spacing float32
}

func (s *spacedHBoxLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	visibleCount := 0
	for _, o := range objects {
		if o.Visible() {
			size := o.MinSize()
			w += size.Width
			h = fyne.Max(h, size.Height)
			visibleCount++
		}
	}
	if visibleCount > 1 {
		w += s.spacing * float32(visibleCount-1)
	}
	return fyne.NewSize(w, h)
}

func (s *spacedHBoxLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var x float32
	for _, o := range objects {
		if o.Visible() {
			oSize := o.MinSize()
			o.Resize(fyne.NewSize(oSize.Width, size.Height))
			o.Move(fyne.NewPos(x, 0))
			x += oSize.Width + s.spacing
		}
	}
}

// airMetricCell holds the widgets for a single air-quality metric shown in the
// bottom row of the card: an icon, the full metric name, and a value label.
type airMetricCell struct {
	container *fyne.Container
	icon      *canvas.Image
	name      *canvas.Text
	value     *canvas.Text
}

// metricTileWidget is one bordered, transparent cell in the right-hand metrics
// grid: an emoji and metric name on top, with the value in bold below.
type metricTileWidget struct {
	container *fyne.Container
	value     *canvas.Text
}

// CityPanel renders weather data for a single city within the widget.
//
// The card is laid out horizontally: a left "info" block (location, weather
// icon, time, date, temperature, condition) sits beside a right metrics grid
// (humidity, wind, wind gust, dew point, pressure, UV index). A row of
// air-quality metrics runs along the bottom of the card.
type CityPanel struct {
	lm        *i18n.LocaleManager
	container *fyne.Container

	// Left info block.
	iconWidget *canvas.Image
	iconRow    *fyne.Container
	tempText   *canvas.Text
	descText   *canvas.Text
	cityText   *canvas.Text
	timeText   *canvas.Text
	dateText   *canvas.Text

	// Right metrics grid — one bordered tile per metric (emoji + name + value).
	humidTile, windTile, windGustTile  *metricTileWidget
	dewPointTile, pressureTile, uvTile *metricTileWidget

	// Air-quality row (bottom of the card).
	airCells        map[weather.PollutionMetric]*airMetricCell
	airRow          *fyne.Container // full bottom row container
	aqiLeftRow      *fyne.Container // AQI cell on the left
	otherAirRightRow *fyne.Container // other pollution cells on the right

	errorIcon *canvas.Image

	// cardBgImg is the background image for the card (day.jpg or night.jpg).
	// cardBgOverlay is a semi-transparent dark rect stacked over it to keep
	// white text readable regardless of the image brightness.
	// Both are stored so they can be swapped live when day/night changes.
	cardBgImg     *canvas.Image
	cardBgOverlay *canvas.Rectangle

	lastData        *weather.WeatherData // cached for re-render on unit change
	lastTempUnit    config.TemperatureUnit
	lastWindUnit    config.WindSpeedUnit
	lastIconTheme   config.IconTheme
	displayFields   *config.DisplayFields   // current visibility config
	pollutionFields *config.PollutionFields // current air-quality selection

	mu         sync.Mutex
	timeTicker *time.Ticker
	stopCh     chan struct{}
	animStopCh chan struct{}
}

// loadAirIconResource reads an air-quality icon (e.g. "co.png") from the
// embedded AirIcons FS and returns it as a Fyne resource, or nil on error.
func loadAirIconResource(file string) fyne.Resource {
	data, err := assets.AirIcons.ReadFile("air/" + file)
	if err != nil {
		return nil
	}
	return fyne.NewStaticResource(file, data)
}

// translate returns the translated string for the given key using the panel's
// LocaleManager. If the LocaleManager is nil, it returns the provided fallback.
func (p *CityPanel) translate(key, fallback string) string {
	if p.lm != nil {
		return p.lm.T(key)
	}
	return fallback
}

// loadIconFromAssets reads an icon from the embedded assets FS and returns it as a Fyne resource.
func loadIconFromAssets(iconCode string) fyne.Resource {
	_, staticData, staticPath, err := loadIconAsset(iconCode)
	if err == nil && staticData != nil {
		return fyne.NewStaticResource(staticPath, staticData)
	}
	return nil
}

// updateIcon updates the panel's icon widget, supporting animated motion icons.
func (p *CityPanel) updateIcon(iconCode string) {
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
func (p *CityPanel) StopAnimation() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.animStopCh != nil {
		close(p.animStopCh)
		p.animStopCh = nil
	}
}

// metricBorderColor is the thin border color used for the metric grid tiles.
var metricBorderColor = color.NRGBA{R: 255, G: 255, B: 255, A: 36}

// newMetricTile builds one bordered, transparent metric cell containing the
// given emoji glyph, the metric name, and an (initially empty) value label.
// Content is centered both horizontally and vertically within the cell.
func newMetricTile(emoji, name string) *metricTileWidget {
	emojiText := canvas.NewText(emoji, color.White)
	emojiText.TextSize = 15

	nameText := canvas.NewText(name, color.NRGBA{R: 221, G: 221, B: 221, A: 255})
	nameText.TextSize = 12

	valueText := canvas.NewText("", color.White)
	valueText.TextSize = 15
	valueText.TextStyle = fyne.TextStyle{Bold: true}

	// Center the header (emoji + name) and value text within the cell
	header := container.NewHBox(emojiText, nameText)
	centeredHeader := container.NewCenter(header)
	centeredValue := container.NewCenter(valueText)
	content := container.NewVBox(centeredHeader, centeredValue)

	// A thin rectangle border behind the padded content forms the grid lines.
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = metricBorderColor
	border.StrokeWidth = 1

	// Use compact padding (reduced vertical padding for ~10% shorter cells)
	compactPad := container.New(&compactPaddingLayout{hPad: 8, vPad: 4}, container.NewCenter(content))
	tile := container.NewStack(border, compactPad)
	return &metricTileWidget{container: tile, value: valueText}
}

// setMetricValue updates a metric tile's value label and refreshes it.
func (p *CityPanel) setMetricValue(tile *metricTileWidget, value string) {
	if tile == nil {
		return
	}
	tile.value.Text = value
	tile.value.Refresh()
}

// NewCityPanel creates a new CityPanel with placeholder content.
// If lm is nil, hardcoded English defaults are used for placeholder text.
func NewCityPanel(lm *i18n.LocaleManager) *CityPanel {
	p := &CityPanel{
		lm:              lm,
		displayFields:   config.DefaultDisplayFields(),
		pollutionFields: config.DefaultPollutionFields(),
	}

	// Weather icon — start with the default cloudy icon.
	p.iconWidget = canvas.NewImageFromResource(loadIconFromAssets(weather.IconCloudy))
	p.iconWidget.FillMode = canvas.ImageFillContain
	p.iconWidget.SetMinSize(fyne.NewSize(96, 96))
	p.updateIcon(weather.IconCloudy)

	// Error indicator icon — hidden by default.
	p.errorIcon = canvas.NewImageFromResource(nil)
	p.errorIcon.FillMode = canvas.ImageFillContain
	p.errorIcon.SetMinSize(fyne.NewSize(16, 16))
	p.errorIcon.Hide()

	// ── Left info block ──────────────────────────────────────────────────────
	p.cityText = canvas.NewText(p.translate("panel.placeholder.city", "📍 City, RG"), color.White)
	p.cityText.TextSize = 18
	p.cityText.TextStyle = fyne.TextStyle{Bold: true}
	p.cityText.Alignment = fyne.TextAlignLeading

	p.tempText = canvas.NewText(p.translate("panel.placeholder.temp", "--°C"), color.White)
	p.tempText.TextSize = 44
	p.tempText.TextStyle = fyne.TextStyle{Bold: true}
	p.tempText.Alignment = fyne.TextAlignCenter

	p.descText = canvas.NewText(p.translate("panel.placeholder.desc", "--"), color.White)
	p.descText.TextSize = 13
	p.descText.TextStyle = fyne.TextStyle{Italic: true}
	p.descText.Alignment = fyne.TextAlignCenter

	p.timeText = canvas.NewText(p.translate("panel.placeholder.time", "00:00:00"), color.White)
	p.timeText.TextSize = 24
	p.timeText.TextStyle = fyne.TextStyle{Bold: true}
	p.timeText.Alignment = fyne.TextAlignCenter

	p.dateText = canvas.NewText(p.translate("panel.placeholder.date", "Monday, Jan 02"), color.White)
	p.dateText.TextSize = 12
	p.dateText.Alignment = fyne.TextAlignCenter

	// ── Right metrics grid — bordered tiles (emoji + name + value) ───────────
	p.humidTile = newMetricTile("💧", weather.HumidityDisplay(0, p.lm).Name)
	p.windTile = newMetricTile("💨", weather.WindDisplay(0, 0, config.WindSpeedUnitKmh, p.lm).Name)
	p.windGustTile = newMetricTile("🌬", weather.WindGustDisplay(0, config.WindSpeedUnitKmh, p.lm).Name)
	p.dewPointTile = newMetricTile("💧", weather.DewPointDisplay(1, p.lm).Name)
	p.pressureTile = newMetricTile("🌡", weather.PressureDisplay(0, p.lm).Name)
	p.uvTile = newMetricTile("☀", weather.UVIndexDisplay(0, p.lm).Name)

	// ── Air-quality cells (bottom row) — icon + name + value ─────────────────
	p.airCells = make(map[weather.PollutionMetric]*airMetricCell, len(weather.PollutionMetricOrder))
	for _, m := range weather.PollutionMetricOrder {
		icon := canvas.NewImageFromResource(loadAirIconResource(weather.AirIconFile(m)))
		icon.FillMode = canvas.ImageFillContain
		icon.SetMinSize(fyne.NewSize(30, 30))

		name := canvas.NewText(weather.PollutionMetricName(m, p.lm), color.NRGBA{R: 187, G: 187, B: 187, A: 255})
		name.TextSize = 10
		name.Alignment = fyne.TextAlignCenter

		value := canvas.NewText("", color.White)
		value.TextSize = 12
		value.TextStyle = fyne.TextStyle{Bold: true}
		value.Alignment = fyne.TextAlignCenter

		cell := &airMetricCell{
			icon:  icon,
			name:  name,
			value: value,
		}
		cell.container = container.NewVBox(
			container.NewCenter(icon),
			container.NewCenter(name),
			container.NewCenter(value),
		)
		cell.container.Hide()
		p.airCells[m] = cell
	}

	p.container = container.NewMax(p.buildLayout())
	return p
}

// metricsGridWidth is the fixed width for the right-hand metrics grid so that
// all city panels have consistent grid sizing regardless of content width.
const metricsGridWidth = 380

// metricGrid builds the right-hand three-column grid of metric tiles based on
// the current display-field visibility (up to six tiles form a 3×2 grid).
// The grid is wrapped in a fixed-width container to ensure consistent sizing.
func (p *CityPanel) metricGrid() fyne.CanvasObject {
	var cells []fyne.CanvasObject

	add := func(show bool, tile *metricTileWidget) {
		if show {
			cells = append(cells, tile.container)
		}
	}

	add(p.displayFields.ShowHumidity, p.humidTile)
	add(p.displayFields.ShowWind, p.windTile)
	add(p.displayFields.ShowWindGust, p.windGustTile)
	add(p.displayFields.ShowDewPoint, p.dewPointTile)
	add(p.displayFields.ShowPressure, p.pressureTile)
	add(p.displayFields.ShowUVIndex, p.uvTile)

	if len(cells) == 0 {
		return layout.NewSpacer()
	}

	grid := container.NewGridWithColumns(3, cells...)
	// Wrap the grid in a fixed-width container for consistent sizing across all city panels,
	// then add a 6px right margin spacer
	gridWithWidth := container.New(&fixedWidthLayout{width: metricsGridWidth}, grid)
	gridRightSpacer := canvas.NewRectangle(color.Transparent)
	gridRightSpacer.SetMinSize(fyne.NewSize(6, 1))
	return container.NewHBox(gridWithWidth, gridRightSpacer)
}

// buildLayout constructs the horizontal card: a left info block (weather icon
// beside a time/date/temp/condition column) with a location line on top and a
// right metrics grid, plus the air-quality row along the bottom.
func (p *CityPanel) buildLayout() fyne.CanvasObject {
	// ── Left info block: icon on the left, info column on the right ──────────
	p.iconRow = container.NewBorder(nil, nil, nil, p.errorIcon, container.NewCenter(p.iconWidget))

	var infoColObjects []fyne.CanvasObject
	if p.displayFields.ShowTime {
		infoColObjects = append(infoColObjects, p.timeText)
	}
	if p.displayFields.ShowDate {
		infoColObjects = append(infoColObjects, p.dateText)
	}
	if p.displayFields.ShowTemp {
		infoColObjects = append(infoColObjects, p.tempText)
	}
	if p.displayFields.ShowDesc {
		infoColObjects = append(infoColObjects, p.descText)
	}
	infoCol := container.NewVBox(infoColObjects...)

	var leftRowObjects []fyne.CanvasObject
	if p.displayFields.ShowIcon {
		leftRowObjects = append(leftRowObjects, container.NewCenter(p.iconRow))
	}
	leftRowObjects = append(leftRowObjects, container.NewCenter(infoCol))
	leftBlock := container.NewHBox(leftRowObjects...)

	// Give the left block a fixed width so the metrics grid always starts at
	// the same position across all city cards, with a consistent gap after the
	// icon/time/temp section regardless of content width.
	const leftBlockWidth = 210
	left := container.New(&fixedWidthLayout{width: leftBlockWidth}, leftBlock)

	// ── Right metrics grid ───────────────────────────────────────────────────
	right := p.metricGrid()

	// Middle region: left block (icon + time/date/temp/condition) beside the metrics grid.
	// The grid now starts at the same vertical level as the time/date/temp/condition,
	// below the city name line.
	middle := container.NewHBox(left, right)

	// ── Air-quality row (bottom) ─────────────────────────────────────────────
	// AQI on the left (aligned with weather icon), other pollution icons on the right
	// (right-aligned, adding inward from right edge with 2px spacing between icons)
	p.aqiLeftRow = container.NewHBox()

	// Collect other pollution cells (non-AQI) for the right side with 2px spacing
	var otherCells []fyne.CanvasObject
	for _, m := range weather.PollutionMetricOrder {
		if c := p.airCells[m]; c != nil {
			if m == weather.MetricAQI {
				p.aqiLeftRow.Add(c.container)
			} else {
				otherCells = append(otherCells, c.container)
			}
		}
	}
	// Use spaced layout with 4px between pollution icons on the right
	p.otherAirRightRow = container.New(&spacedHBoxLayout{spacing: 12}, otherCells...)

	// Add 6px buffer padding: left margin for AQI, right margin for other pollution icons
	leftSpacer := canvas.NewRectangle(color.Transparent)
	leftSpacer.SetMinSize(fyne.NewSize(6, 1))
	rightSpacer := canvas.NewRectangle(color.Transparent)
	rightSpacer.SetMinSize(fyne.NewSize(6, 1))

	aqiWithPadding := container.NewHBox(leftSpacer, p.aqiLeftRow)
	otherWithPadding := container.NewHBox(p.otherAirRightRow, rightSpacer)

	// Use Border layout: AQI on left with padding, other pollution icons on right with padding
	p.airRow = container.NewBorder(nil, nil, aqiWithPadding, otherWithPadding, nil)
	bottom := p.airRow

	// Build the final content: city name on top, then middle row, then air row.
	var contentObjects []fyne.CanvasObject
	if p.displayFields.ShowCity {
		contentObjects = append(contentObjects, p.cityText)
	}
	contentObjects = append(contentObjects, middle, bottom)
	content := container.NewVBox(contentObjects...)

	// Wrap content in a rounded, bordered card backed by a day/night image.
	// Reuse existing image widget if already created (avoids re-decoding the
	// resource on every layout rebuild triggered by displayFields changes).
	if p.cardBgImg == nil {
		isNight := false
		if p.lastData != nil {
			isNight = weather.IsNight(p.lastData.LocalTime)
		}
		p.cardBgImg = newCardBgImage(isNight)
		p.cardBgOverlay = newCardBgOverlay()
	}

	// On Windows, stack a per-pixel corner mask on top of the card so that the
	// four corner areas (outside the rounded rect) are painted with the Win32
	// LWA_COLORKEY color and therefore become transparent — matching macOS
	// where the NSWindow content-view layer clips to a corner radius.
	// newCornerMask() returns nil on all non-Windows platforms and is a no-op.
	if mask := newCornerMask(); mask != nil {
		return container.NewStack(p.cardBgImg, p.cardBgOverlay, container.NewPadded(content), mask)
	}
	return container.NewStack(p.cardBgImg, p.cardBgOverlay, container.NewPadded(content))
}

// cardCornerRadius and cardStrokeWidth define the rounded, bordered card geometry.
const (
	cardCornerRadius = 12
	cardStrokeWidth  = 1
)

// Lazily-loaded background image resources so we only decode each file once.
var (
	bgDayOnce   sync.Once
	bgDayRes    fyne.Resource
	bgNightOnce sync.Once
	bgNightRes  fyne.Resource
)

// loadBgResource reads a background image from the embedded Backgrounds FS,
// caching the result so subsequent calls are instant.
func loadBgResource(name string, once *sync.Once, res *fyne.Resource) fyne.Resource {
	once.Do(func() {
		data, err := assets.Backgrounds.ReadFile("backgrounds/" + name)
		if err != nil {
			log.Printf("panel: failed to load background %s: %v", name, err)
			return
		}
		*res = fyne.NewStaticResource(name, data)
	})
	return *res
}

// dayBgResource returns the embedded daytime background image resource.
func dayBgResource() fyne.Resource { return loadBgResource("day.jpg", &bgDayOnce, &bgDayRes) }

// nightBgResource returns the embedded nighttime background image resource.
func nightBgResource() fyne.Resource { return loadBgResource("night.jpg", &bgNightOnce, &bgNightRes) }

// newCardBgImage creates a canvas.Image pre-loaded with the day or night
// background, sized to fill its parent via ImageFillStretch.
func newCardBgImage(isNight bool) *canvas.Image {
	res := dayBgResource()
	if isNight {
		res = nightBgResource()
	}
	img := canvas.NewImageFromResource(res)
	img.FillMode = canvas.ImageFillStretch
	img.ScaleMode = canvas.ImageScaleFastest
	return img
}

// newCardBgOverlay returns a semi-transparent dark rectangle to overlay on top
// of the background image so that white text remains readable.
func newCardBgOverlay() *canvas.Rectangle {
	rect := canvas.NewRectangle(color.NRGBA{R: 0, G: 0, B: 0, A: 120})
	rect.CornerRadius = cardCornerRadius
	return rect
}

// applyDayNightBg swaps the card background image to the day or night variant.
// Must be called on the Fyne main goroutine.
func (p *CityPanel) applyDayNightBg(isNight bool) {
	if p.cardBgImg == nil {
		return
	}
	if isNight {
		p.cardBgImg.Resource = nightBgResource()
	} else {
		p.cardBgImg.Resource = dayBgResource()
	}
	p.cardBgImg.Refresh()
}

// ApplyDisplayFields updates the panel's visibility configuration and rebuilds the layout.
func (p *CityPanel) ApplyDisplayFields(df *config.DisplayFields) {
	if df == nil {
		df = config.DefaultDisplayFields()
	}
	p.displayFields = df
	p.container.RemoveAll()
	p.container.Add(p.buildLayout())
	p.applyAirCells()
	p.container.Refresh()
}

// ApplyPollutionFields updates the air-quality metric selection and refreshes
// the bottom row using the panel's most recent data.
func (p *CityPanel) ApplyPollutionFields(pf *config.PollutionFields) {
	if pf == nil {
		pf = config.DefaultPollutionFields()
	}
	p.pollutionFields = pf
	p.applyAirCells()
}

// applyAirCells populates and shows the air-quality cells selected by the
// current pollution fields and present in the latest data; others are hidden.
func (p *CityPanel) applyAirCells() {
	rows := weather.PlanPollutionRows(p.pollutionFields, weather.PollutionOf(p.lastData))
	planned := make(map[weather.PollutionMetric]weather.PollutionRow, len(rows))
	for _, r := range rows {
		planned[r.Metric] = r
	}

	for _, m := range weather.PollutionMetricOrder {
		cell := p.airCells[m]
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
	if p.aqiLeftRow != nil {
		p.aqiLeftRow.Refresh()
	}
	if p.otherAirRightRow != nil {
		p.otherAirRightRow.Refresh()
	}
	if p.airRow != nil {
		p.airRow.Refresh()
	}
}

// Container returns the Fyne container for embedding in a parent layout.
func (p *CityPanel) Container() *fyne.Container {
	return p.container
}

// Update sets the panel content from the given weather data using the specified units and icon theme.
func (p *CityPanel) Update(data *weather.WeatherData, tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme) {
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

	p.cityText.Text = "📍 " + weather.FormatCityRegion(data.CityName, data.Region)
	p.cityText.Refresh()

	// Metric tiles — set the value labels (names are fixed at construction).
	p.setMetricValue(p.humidTile, weather.HumidityDisplay(data.Humidity, p.lm).Value)
	p.setMetricValue(p.windTile, weather.WindDisplay(data.WindSpeed, data.WindDirection, windUnit, p.lm).Value)
	p.setMetricValue(p.windGustTile, weather.WindGustDisplay(data.WindGust, windUnit, p.lm).Value)
	p.setMetricValue(p.dewPointTile, weather.DewPointDisplay(data.DewPoint, p.lm).Value)
	p.setMetricValue(p.pressureTile, weather.PressureDisplay(data.Pressure, p.lm).Value)
	p.setMetricValue(p.uvTile, weather.UVIndexDisplay(data.UVIndex, p.lm).Value)

	// Hide error indicator on successful update.
	p.errorIcon.Hide()

	// Rebuild the layout so newly-available (or newly-empty) metric cells and
	// air-quality cells appear or disappear, then repopulate the air row.
	p.container.RemoveAll()
	p.container.Add(p.buildLayout())
	p.applyAirCells()
	p.container.Refresh()
}

// Rerender re-applies the last cached WeatherData with new units or icon theme.
// If no data has been cached yet, this is a no-op.
func (p *CityPanel) Rerender(tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme) {
	if p.lastData == nil {
		return
	}
	p.Update(p.lastData, tempUnit, windUnit, iconTheme...)
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

	// Apply the initial background immediately (before the first tick).
	p.applyDayNightBg(lastNight)

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
						// Recolor the card background for the new day/night state.
						p.applyDayNightBg(isNightNow)
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
func (p *CityPanel) StopClock() {
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
