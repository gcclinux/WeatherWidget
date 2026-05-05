//go:build !linux && !darwin

package ui

import (
	"fyne.io/fyne/v2"
)

// createWidgetWindow creates the main widget window on non-Linux platforms.
// On Windows, decorations are removed post-creation via Win32 API calls.
func createWidgetWindow(app fyne.App, title string) fyne.Window {
	w := app.NewWindow(title)
	w.SetFixedSize(true)
	w.SetPadded(false)
	return w
}
