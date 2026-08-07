// Package main is the entry point for the WeatherWidget application.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"

	appmanager "weatherwidget/internal/app"
	"weatherwidget/internal/ui"
)

// version is set at build time via -ldflags "-X main.version=1.0.5"
var version = "dev"

func main() {
	debugFlag := flag.Bool("debug", false, "Enable debug logging to a file")
	softwareFlag := flag.Bool("software", false, "Use software rendering (fixes OpenGL driver issues)")
	settingsFlag := flag.Bool("settings", false, "Open the settings window on launch")
	flag.Parse()

	if *softwareFlag {
		os.Setenv("FYNE_RENDER", "software")
	}

	appDataDir := appDataDirectory()

	if err := os.MkdirAll(appDataDir, 0755); err != nil {
		log.Printf("failed to create app data directory: %v", err)
	}

	if *debugFlag {
		logPath := filepath.Join(appDataDir, "debug.log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(logFile)
			log.Printf("--- Debug logging enabled (version: %s) ---", version)
		} else {
			log.Printf("failed to open debug log file: %v", err)
		}
	}

	fyneApp := app.NewWithID("com.weatherwidget")
	fyneApp.Settings().SetTheme(ui.NewWidgetTheme(theme.DefaultTheme()))

	manager := appmanager.NewAppManager(fyneApp, appDataDir)
	if err := manager.Run(); err != nil {
		log.Fatalf("failed to start WeatherWidget: %v", err)
	}

	if *settingsFlag {
		manager.OpenSettings()
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
