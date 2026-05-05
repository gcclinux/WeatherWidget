//go:build !darwin

package ui

import "fyne.io/fyne/v2"

// initPlatformWindow is a no-op on non-darwin platforms.
func initPlatformWindow(_ fyne.Window) {}
