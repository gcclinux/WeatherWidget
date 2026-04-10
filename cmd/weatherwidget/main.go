// Package main is the entry point for the WeatherWidget application.
package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2/app"

	appmanager "weatherwidget/internal/app"
)

func main() {
	fyneApp := app.NewWithID("com.weatherwidget")

	appDataDir := appDataDirectory()

	manager := appmanager.NewAppManager(fyneApp, appDataDir)
	if err := manager.Run(); err != nil {
		log.Fatalf("failed to start WeatherWidget: %v", err)
	}

	fyneApp.Run()
}

// appDataDirectory returns the application data directory path.
// On Windows it uses %APPDATA%\WeatherWidget; on other platforms
// it falls back to ~/.config/WeatherWidget.
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
