package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"weatherwidget/internal/config"
	"weatherwidget/internal/ui/panel"
	"weatherwidget/internal/weather"
)

// cornerPositions lists the valid screen corner positions for the widget.
var cornerPositions = []struct {
	label string
	value string
}{
	{"Top-Left", "top-left"},
	{"Top-Right", "top-right"},
	{"Bottom-Left", "bottom-left"},
	{"Bottom-Right", "bottom-right"},
}

const widgetTitle = "WeatherWidget"

// UIManager manages the Fyne application windows and city panels.
type UIManager struct {
	app      fyne.App
	widget   fyne.Window
	settings fyne.Window
	panels   []*panel.CityPanel
	menu     *fyne.Menu // context menu shown on right-click
}

// NewUIManager creates a new UIManager and its main widget window.
// After the window is shown, call applyToolWindowStyle to set Win32
// WS_EX_TOOLWINDOW and HWND_TOPMOST styles (no-op on non-Windows).
func NewUIManager(app fyne.App) *UIManager {
	w := app.NewWindow(widgetTitle)
	w.SetFixedSize(true)
	w.SetPadded(false)

	return &UIManager{
		app:    app,
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
	if count > 3 {
		count = 3
	}

	u.panels = make([]*panel.CityPanel, count)
	objects := make([]fyne.CanvasObject, count)
	for i := 0; i < count; i++ {
		p := panel.NewCityPanel()
		u.panels[i] = p
		objects[i] = p.Container()
	}

	grid := container.NewGridWithColumns(count, objects...)
	var content fyne.CanvasObject = grid
	if u.menu != nil {
		content = newRightClickOverlay(grid, u.menu)
	}
	u.widget.SetContent(content)

	w, h, _ := CalculateLayout(count)
	u.widget.Resize(fyne.NewSize(float32(w), float32(h)))
	u.widget.Show()
}

// UpdatePanels updates each CityPanel with the corresponding weather data.
// Panels and data are matched by index; extra data entries are ignored.
func (u *UIManager) UpdatePanels(data []weather.WeatherData) {
	log.Printf("UIManager: updating %d panels with %d data entries", len(u.panels), len(data))
	for i, p := range u.panels {
		if i >= len(data) {
			break
		}
		d := data[i]
		p.Update(&d)
	}
}

// Panels returns the current list of city panels.
func (u *UIManager) Panels() []*panel.CityPanel {
	return u.panels
}

// SetCorner repositions the widget window to the specified screen corner.
// Valid positions: "top-left", "top-right", "bottom-left", "bottom-right".
// Unrecognised values default to "bottom-right".
func (u *UIManager) SetCorner(position string) {
	screenW, screenH := getScreenSize()
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
		x, y = 0, 0
	case "top-right":
		x = screenW - ww
		y = 0
	case "bottom-left":
		x = 0
		y = screenH - wh
	default: // "bottom-right" and any unrecognised value
		x = screenW - ww
		y = screenH - wh
	}

	// Fyne doesn't expose a direct MoveWindow API, so we use the
	// platform-specific helper on Windows and a no-op elsewhere.
	moveWindow(u.widget, x, y)
}

// SetupContextMenu builds the right-click context menu for the widget window.
// onSettings is called when the user selects "Settings".
// onExit is called when the user selects "Exit".
func (u *UIManager) SetupContextMenu(onSettings func(), onExit func()) {
	// Build Position submenu items.
	posItems := make([]*fyne.MenuItem, len(cornerPositions))
	for i, pos := range cornerPositions {
		p := pos.value // capture for closure
		posItems[i] = fyne.NewMenuItem(pos.label, func() {
			u.SetCorner(p)
		})
	}

	positionItem := fyne.NewMenuItem("Position", nil)
	positionItem.ChildMenu = fyne.NewMenu("", posItems...)

	u.menu = fyne.NewMenu("",
		fyne.NewMenuItem("Settings", onSettings),
		positionItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Exit", onExit),
	)
}
