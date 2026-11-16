package main

// appdata.go — shared helper for all platforms.

import (
	"os"
)

// appDataDirectory returns the base directory for application data.
// On Windows os.UserConfigDir resolves to %AppData%, on macOS to ~/Library/Application Support,
// and on Linux to $XDG_CONFIG_HOME or ~/.config.
func appDataDirectory() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}
