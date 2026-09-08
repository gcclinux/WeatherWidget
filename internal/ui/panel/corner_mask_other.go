//go:build !windows

package panel

import "fyne.io/fyne/v2"

// newCornerMask returns nil on non-Windows platforms; rounded corners are
// handled at the OS/compositor level (e.g. NSWindow layer on macOS).
func newCornerMask() fyne.CanvasObject {
	return nil
}
