//go:build !windows

package panel

// isWindowsLightTheme returns false on non-Windows platforms (macOS, Linux),
// preserving standard dark/white text behavior.
func isWindowsLightTheme() bool {
	return false
}
