//go:build !darwin

package ui

import (
	"fyne.io/fyne/v2"
)

// buildSettingsNavDarwin is a no-op on non-Darwin platforms.
// Returns nil to indicate standard AppTabs should be used.
func buildSettingsNavDarwin(items []struct {
	icon    fyne.Resource
	text    string
	content fyne.CanvasObject
}) (*fyne.Container, func(int)) {
	return nil, nil
}

// useDarwinNav returns false on non-Darwin platforms.
func useDarwinNav() bool {
	return false
}
