//go:build darwin

package ui

import "fyne.io/fyne/v2"

// initPlatformWindow performs darwin-specific window initialisation.
// It registers the window reference so the native NSWindow handle can be
// retrieved later for positioning.
func initPlatformWindow(w fyne.Window) {
	registerDarwinWindow(w)
	applyDarwinWindowSetup()
}
