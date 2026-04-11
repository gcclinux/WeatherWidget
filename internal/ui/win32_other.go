//go:build !windows && !linux

package ui

import (
	"fyne.io/fyne/v2"
)

// applyToolWindowStyle is a no-op on non-Windows platforms.
func applyToolWindowStyle(_ string) {}

// getScreenSize returns a reasonable default on non-Windows platforms.
func getScreenSize() (int, int) {
	return 1920, 1080
}

// moveWindow is a no-op on non-Windows platforms.
// Fyne does not expose a cross-platform window move API.
func moveWindow(_ fyne.Window, _, _ int) {}

// getWindowPosition returns (0, 0) on unsupported platforms.
func getWindowPosition() (int, int) {
	return 0, 0
}

// setWindowOpacity is a no-op on non-Windows platforms.
func setWindowOpacity(_ int) {}
