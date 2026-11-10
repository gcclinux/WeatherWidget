package main

// appdata.go — shared helper for all platforms.

import (
	"os"
	"path/filepath"
	"runtime"
)

// appDataDirectory returns the application data directory path.
// On Windows: %APPDATA%\WeatherWidget
// On all other platforms: ~/.config/WeatherWidget
func appDataDirectory() string {
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("APPDATA"); dir != "" {
			return filepath.Join(dir, "WeatherWidget")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "WeatherWidget")
}
