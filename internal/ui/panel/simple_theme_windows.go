//go:build windows

package panel

import "golang.org/x/sys/windows/registry"

// isWindowsLightTheme reports whether the Windows OS is configured in Light Theme.
// It checks the Personalize registry key for SystemUsesLightTheme or AppsUseLightTheme.
func isWindowsLightTheme() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	if val, _, err := k.GetIntegerValue("SystemUsesLightTheme"); err == nil && val != 0 {
		return true
	}
	if val, _, err := k.GetIntegerValue("AppsUseLightTheme"); err == nil && val != 0 {
		return true
	}
	return false
}
