//go:build darwin

package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// createWidgetWindow creates the main widget window on macOS.
//
// On macOS, window decorations (title bar + traffic lights) cannot be removed
// after creation via public APIs. We use Fyne's CreateSplashWindow() which
// sets the GLFW Decorated hint to false before the window is created,
// producing a truly borderless window.
func createWidgetWindow(app fyne.App, title string) fyne.Window {
	drv, ok := app.Driver().(desktop.Driver)
	if !ok {
		log.Println("macOS: desktop driver not available, falling back to standard window")
		w := app.NewWindow(title)
		w.SetFixedSize(true)
		w.SetPadded(false)
		return w
	}

	w := drv.CreateSplashWindow()
	w.SetTitle(title)
	w.SetFixedSize(true)
	w.SetPadded(false)

	log.Println("macOS: created undecorated widget window via CreateSplashWindow")
	return w
}
