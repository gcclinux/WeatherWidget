//go:build linux

package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// createWidgetWindow creates the main widget window on Linux.
//
// On Linux (especially Wayland/GNOME), window decorations cannot be removed
// after creation. We use Fyne's CreateSplashWindow() which sets the GLFW
// Decorated hint to false before the window is created, producing a truly
// borderless window on both Wayland and X11.
func createWidgetWindow(app fyne.App, title string) fyne.Window {
	drv, ok := app.Driver().(desktop.Driver)
	if !ok {
		log.Println("Linux: desktop driver not available, falling back to standard window")
		w := app.NewWindow(title)
		w.SetFixedSize(true)
		w.SetPadded(false)
		return w
	}

	w := drv.CreateSplashWindow()
	w.SetTitle(title)
	w.SetFixedSize(true)
	w.SetPadded(false)

	log.Println("Linux: created undecorated widget window via CreateSplashWindow")
	return w
}
