package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/ui/panel"
	"weatherwidget/internal/weather"
)

const widgetTitle = "WeatherWidget"

// UIManager manages the Fyne application windows and city panels.
type UIManager struct {
	app      fyne.App
	lm       *i18n.LocaleManager
	widget   fyne.Window
	settings fyne.Window
	panels       []*panel.CityPanel        // Enhanced view panels
	simplePanels []*panel.SimpleCityPanel  // Simple view panels
	viewMode     config.ViewMode           // Current view mode

	// curDisplayFields and curPollutionFields track the most recently applied
	// visibility config so the window can be resized to fit the actual content
	// whenever either changes.
	curDisplayFields   *config.DisplayFields
	curPollutionFields *config.PollutionFields

	// Cached weather data and units for instant restoration when toggling view modes
	lastData      []weather.WeatherData
	lastTempUnit  config.TemperatureUnit
	lastWindUnit  config.WindSpeedUnit
	lastIconTheme []config.IconTheme
}

// NewUIManager creates a new UIManager and its main widget window.
// On Linux the window is created without decorations (borderless) via
// CreateSplashWindow. On Windows decorations are removed post-creation
// by applyToolWindowStyle using Win32 API calls.
func NewUIManager(app fyne.App, lm *i18n.LocaleManager) *UIManager {
	w := createWidgetWindow(app, widgetTitle)
	initPlatformWindow(w)

	return &UIManager{
		app:    app,
		lm:     lm,
		widget: w,
	}
}

// Window returns the underlying Fyne window for external use.
func (u *UIManager) Window() fyne.Window {
	return u.widget
}

// RefreshFontCache invalidates Fyne's cached font faces so that a locale font
// change (via SetLocaleFont) takes effect. Without this, previously-cached
// faces keep being used and complex scripts such as Tamil render as tofu
// boxes, most visibly on Windows. Safe to call on the Fyne main goroutine.
func (u *UIManager) RefreshFontCache() {
	RefreshFontCache(u.app)
}

// ApplyWin32Styles applies platform-specific window styles.
// On Windows this sets WS_EX_TOOLWINDOW and HWND_TOPMOST.
// Must be called after the window is shown so the HWND exists.
func (u *UIManager) ApplyWin32Styles() {
	applyToolWindowStyle(widgetTitle)
}

// ShowWidget creates CityPanel instances for each city, arranges them
// horizontally, resizes the window to fit, and displays it.
func (u *UIManager) ShowWidget(cities []config.CityConfig) {
	u.ShowWidgetWithMode(cities, config.ViewModeEnhanced)
}

// ShowWidgetWithMode creates city panels for each city using the specified view mode.
// Enhanced mode: cities stacked vertically with horizontal data grid.
// Simple mode: cities displayed side-by-side with vertical data list.
func (u *UIManager) ShowWidgetWithMode(cities []config.CityConfig, viewMode config.ViewMode) {
	// Stop clocks on previous panels before replacing
	for _, p := range u.simplePanels {
		p.StopClock()
	}
	for _, p := range u.panels {
		p.StopClock()
	}

	count := len(cities)
	if count == 0 {
		count = 1
	}
	if count > 5 {
		count = 5
	}

	u.viewMode = config.NormalizeViewMode(viewMode)

	if u.viewMode == config.ViewModeSimple {
		// Simple view: cities side-by-side with vertical data list
		u.simplePanels = make([]*panel.SimpleCityPanel, count)
		u.panels = nil // Clear enhanced panels
		objects := make([]fyne.CanvasObject, count)
		for i := 0; i < count; i++ {
			p := panel.NewSimpleCityPanel(u.lm)
			u.simplePanels[i] = p
			objects[i] = p.Container()
			if i < len(cities) && cities[i].Timezone != "" {
				p.StartClock(cities[i].Timezone)
			} else {
				p.StartClock("UTC")
			}
		}

		// Arrange cities horizontally (side-by-side)
		grid := container.NewGridWithColumns(count, objects...)
		u.widget.SetContent(grid)

		if len(u.lastData) > 0 {
			for i, p := range u.simplePanels {
				if i < len(u.lastData) {
					d := u.lastData[i]
					p.Update(&d, u.lastTempUnit, u.lastWindUnit, u.lastIconTheme...)
				}
			}
		}
		if u.curDisplayFields != nil {
			for _, p := range u.simplePanels {
				p.ApplyDisplayFields(u.curDisplayFields)
			}
		}
		if u.curPollutionFields != nil {
			for _, p := range u.simplePanels {
				p.ApplyPollutionFields(u.curPollutionFields)
			}
		}
	} else {
		// Enhanced view: cities stacked vertically with horizontal data grid
		u.panels = make([]*panel.CityPanel, count)
		u.simplePanels = nil // Clear simple panels
		objects := make([]fyne.CanvasObject, count)
		for i := 0; i < count; i++ {
			p := panel.NewCityPanel(u.lm)
			u.panels[i] = p
			objects[i] = p.Container()
			if i < len(cities) && cities[i].Timezone != "" {
				p.StartClock(cities[i].Timezone)
			} else {
				p.StartClock("UTC")
			}
		}

		// Stack the city cards vertically — one card under another.
		stack := container.NewVBox(objects...)
		u.widget.SetContent(stack)

		if len(u.lastData) > 0 {
			for i, p := range u.panels {
				if i < len(u.lastData) {
					d := u.lastData[i]
					p.Update(&d, u.lastTempUnit, u.lastWindUnit, u.lastIconTheme...)
				}
			}
		}
		if u.curDisplayFields != nil {
			for _, p := range u.panels {
				p.ApplyDisplayFields(u.curDisplayFields)
			}
		}
		if u.curPollutionFields != nil {
			for _, p := range u.panels {
				p.ApplyPollutionFields(u.curPollutionFields)
			}
		}
	}

	u.resizeToContent(count)
	u.widget.Show()
}

// maxPollutionRows returns the maximum number of planned pollution rows across all cached city data.
func (u *UIManager) maxPollutionRows() int {
	if u.curPollutionFields == nil {
		return 0
	}
	if len(u.lastData) == 0 {
		return 0
	}
	maxR := 0
	for _, d := range u.lastData {
		r := len(weather.PlanPollutionRows(u.curPollutionFields, weather.PollutionOf(&d)))
		if r > maxR {
			maxR = r
		}
	}
	return maxR
}

func (u *UIManager) calcSimpleLayout(count int) (int, int) {
	if count < 1 {
		count = len(u.simplePanels)
	}
	if count < 1 {
		count = 1
	}
	rows := u.maxPollutionRows()
	w, h, _ := CalculateSimpleLayoutWithPollutionRows(count, u.curDisplayFields, rows)
	return w, h
}

// resizeToContent resizes the widget window to fit the given number of city
// cards using the currently applied display and pollution field visibility.
func (u *UIManager) resizeToContent(count int) {
	if u.viewMode == config.ViewModeSimple {
		w, h := u.calcSimpleLayout(count)
		u.widget.Resize(fyne.NewSize(float32(w), float32(h)))
	} else {
		if u.widget.Content() != nil {
			u.widget.Resize(u.widget.Content().MinSize())
		}
	}
}

// GetViewMode returns the current view mode.
func (u *UIManager) GetViewMode() config.ViewMode {
	return u.viewMode
}

// UpdatePanels updates each CityPanel with the corresponding weather data, units, and icon theme.
// Panels and data are matched by index; extra data entries are ignored.
func (u *UIManager) UpdatePanels(data []weather.WeatherData, tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme) {
	u.lastData = data
	u.lastTempUnit = tempUnit
	u.lastWindUnit = windUnit
	u.lastIconTheme = iconTheme

	if u.viewMode == config.ViewModeSimple {
		log.Printf("UIManager: updating %d simple panels with %d data entries", len(u.simplePanels), len(data))
		for i, p := range u.simplePanels {
			if i >= len(data) {
				break
			}
			d := data[i]
			p.Update(&d, tempUnit, windUnit, iconTheme...)
		}
		u.resizeToContent(len(u.simplePanels))
	} else {
		log.Printf("UIManager: updating %d panels with %d data entries", len(u.panels), len(data))
		for i, p := range u.panels {
			if i >= len(data) {
				break
			}
			d := data[i]
			p.Update(&d, tempUnit, windUnit, iconTheme...)
		}
	}
}

// ApplyDisplayFields applies the given display field configuration to all panels
// and resizes the widget window to fit the visible content.
func (u *UIManager) ApplyDisplayFields(df *config.DisplayFields) {
	if u.viewMode == config.ViewModeSimple {
		for _, p := range u.simplePanels {
			p.ApplyDisplayFields(df)
		}
	} else {
		for _, p := range u.panels {
			p.ApplyDisplayFields(df)
		}
	}
	u.curDisplayFields = df
	// Resize widget to match the dynamic height.
	panelCount := len(u.panels)
	if u.viewMode == config.ViewModeSimple {
		panelCount = len(u.simplePanels)
	}
	u.resizeToContent(panelCount)
}

// ApplyPollutionFields applies the given air-quality metric selection to all panels.
func (u *UIManager) ApplyPollutionFields(pf *config.PollutionFields) {
	if u.viewMode == config.ViewModeSimple {
		for _, p := range u.simplePanels {
			p.ApplyPollutionFields(pf)
		}
	} else {
		for _, p := range u.panels {
			p.ApplyPollutionFields(pf)
		}
	}
	u.curPollutionFields = pf
	// The air-quality row's presence affects the card height, so resize too.
	panelCount := len(u.panels)
	if u.viewMode == config.ViewModeSimple {
		panelCount = len(u.simplePanels)
	}
	u.resizeToContent(panelCount)
}

// RerenderPanels re-renders all panels using their cached data with new units or icon theme.
// Used when only the temperature, wind speed unit, or icon theme changes, avoiding a new weather fetch.
func (u *UIManager) RerenderPanels(tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme) {
	u.lastTempUnit = tempUnit
	u.lastWindUnit = windUnit
	u.lastIconTheme = iconTheme

	if u.viewMode == config.ViewModeSimple {
		for _, p := range u.simplePanels {
			p.Rerender(tempUnit, windUnit, iconTheme...)
		}
	} else {
		for _, p := range u.panels {
			p.Rerender(tempUnit, windUnit, iconTheme...)
		}
	}
}

// Panels returns the current list of city panels (Enhanced view).
func (u *UIManager) Panels() []*panel.CityPanel {
	return u.panels
}

// SimplePanels returns the current list of simple city panels (Simple view).
func (u *UIManager) SimplePanels() []*panel.SimpleCityPanel {
	return u.simplePanels
}

// SetCorner repositions the widget window to the specified screen corner
// on the given monitor. Valid positions: "top-left", "top-right",
// "bottom-left", "bottom-right". Unrecognised values default to "bottom-right".
func (u *UIManager) SetCorner(position string, monitorIndex int) {
	monX, monY, monW, monH := getMonitorBounds(monitorIndex)
	winSize := u.widget.Canvas().Size()
	ww := int(winSize.Width)
	wh := int(winSize.Height)

	count := len(u.panels)
	if u.viewMode == config.ViewModeSimple {
		count = len(u.simplePanels)
	}
	if count == 0 {
		count = 1
	}

	var calcW, calcH int
	if u.viewMode == config.ViewModeSimple {
		calcW, calcH = u.calcSimpleLayout(count)
	} else {
		calcW, calcH, _ = CalculateLayoutWithPollution(count, u.curDisplayFields, u.curPollutionFields)
	}

	// If canvas hasn't reported a size yet, or is stale across mode switch, use calculated size.
	if ww == 0 || wh == 0 || (u.viewMode == config.ViewModeSimple && ww != calcW) || (u.viewMode == config.ViewModeEnhanced && ww != PanelWidth) {
		ww = calcW
		wh = calcH
	}

	var x, y int
	switch position {
	case "top-left":
		x, y = monX, monY
	case "top-right":
		x = monX + monW - ww
		y = monY
	case "bottom-left":
		x = monX
		y = monY + monH - wh
	default: // "bottom-right" and any unrecognised value
		x = monX + monW - ww
		y = monY + monH - wh
	}

	// Fyne doesn't expose a direct MoveWindow API, so we use the
	// platform-specific helper on Windows and a no-op elsewhere.
	moveWindow(u.widget, x, y)
}

// GetMonitorCount returns the number of display monitors attached to the system.
func (u *UIManager) GetMonitorCount() int {
	return getMonitorCount()
}

// EnableDrag enables left-click drag-to-reposition on the widget window.
// onDragEnd is called after the user finishes dragging so the caller can
// persist the new position. Must be called after the window is shown.
func (u *UIManager) EnableDrag(onDragEnd func()) {
	enableWindowDrag(onDragEnd)
}

// SetPosition moves the widget window to exact pixel coordinates.
func (u *UIManager) SetPosition(x, y int) {
	moveWindow(u.widget, x, y)
}

// GetPosition returns the current top-left screen coordinates of the widget.
func (u *UIManager) GetPosition() (int, int) {
	return getWindowPosition()
}

// SetOpacity applies background-only transparency to the widget window.
// opacityPercent should be 25, 50, 75, or 100.
// On Windows the background color becomes transparent via Win32 color-key;
// on Linux the whole window opacity is adjusted via _NET_WM_WINDOW_OPACITY
// with mapped values to keep content readable.
func (u *UIManager) SetOpacity(opacityPercent int) {
	setWindowOpacity(opacityPercent)
	u.widget.Canvas().Refresh(u.widget.Content())
}
