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
	panels   []*panel.CityPanel
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

// ApplyWin32Styles applies platform-specific window styles.
// On Windows this sets WS_EX_TOOLWINDOW and HWND_TOPMOST.
// Must be called after the window is shown so the HWND exists.
func (u *UIManager) ApplyWin32Styles() {
	applyToolWindowStyle(widgetTitle)
}

// ShowWidget creates CityPanel instances for each city, arranges them
// horizontally, resizes the window to fit, and displays it.
func (u *UIManager) ShowWidget(cities []config.CityConfig) {
	count := len(cities)
	if count == 0 {
		count = 1
	}
	if count > 5 {
		count = 5
	}

	u.panels = make([]*panel.CityPanel, count)
	objects := make([]fyne.CanvasObject, count)
	for i := 0; i < count; i++ {
		p := panel.NewCityPanel(u.lm)
		u.panels[i] = p
		objects[i] = p.Container()
	}

	grid := container.NewGridWithColumns(count, objects...)
	u.widget.SetContent(grid)

	w, h, _ := CalculateLayout(count)
	u.widget.Resize(fyne.NewSize(float32(w), float32(h)))
	u.widget.Show()
}

// UpdatePanels updates each CityPanel with the corresponding weather data, units, and icon theme.
// Panels and data are matched by index; extra data entries are ignored.
func (u *UIManager) UpdatePanels(data []weather.WeatherData, tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme) {
	log.Printf("UIManager: updating %d panels with %d data entries", len(u.panels), len(data))
	for i, p := range u.panels {
		if i >= len(data) {
			break
		}
		d := data[i]
		p.Update(&d, tempUnit, windUnit, iconTheme...)
	}
}

// ApplyDisplayFields applies the given display field configuration to all panels
// and resizes the widget window to fit the visible content.
func (u *UIManager) ApplyDisplayFields(df *config.DisplayFields) {
	for _, p := range u.panels {
		p.ApplyDisplayFields(df)
	}
	// Resize widget to match the dynamic height.
	count := len(u.panels)
	if count == 0 {
		count = 1
	}
	w, h, _ := CalculateLayoutWithFields(count, df)
	u.widget.Resize(fyne.NewSize(float32(w), float32(h)))
}

// RerenderPanels re-renders all panels using their cached data with new units or icon theme.
// Used when only the temperature, wind speed unit, or icon theme changes, avoiding a new weather fetch.
func (u *UIManager) RerenderPanels(tempUnit config.TemperatureUnit, windUnit config.WindSpeedUnit, iconTheme ...config.IconTheme) {
	for _, p := range u.panels {
		p.Rerender(tempUnit, windUnit, iconTheme...)
	}
}

// Panels returns the current list of city panels.
func (u *UIManager) Panels() []*panel.CityPanel {
	return u.panels
}

// SetCorner repositions the widget window to the specified screen corner
// on the given monitor. Valid positions: "top-left", "top-right",
// "bottom-left", "bottom-right". Unrecognised values default to "bottom-right".
func (u *UIManager) SetCorner(position string, monitorIndex int) {
	monX, monY, monW, monH := getMonitorBounds(monitorIndex)
	winSize := u.widget.Canvas().Size()
	ww := int(winSize.Width)
	wh := int(winSize.Height)

	// Fallback if canvas hasn't reported a size yet.
	if ww == 0 || wh == 0 {
		count := len(u.panels)
		if count == 0 {
			count = 1
		}
		ww, wh, _ = CalculateLayout(count)
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
