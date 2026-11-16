// Package main is the entry point for the WeatherWidget application on Windows.
// It uses the native Windows UI layer (uiwindows).

//go:build windows

package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	uiwindows "weatherwidget/internal/ui-windows"
)

// version is set at build time via -ldflags "-X main.version=1.0.6"
var version = "dev"

func main() {
	debugFlag := flag.Bool("debug", false, "Enable debug logging to a file")
	settingsFlag := flag.Bool("settings", false, "Open the settings window on launch")
	flag.Parse()

	appDataDir := appDataDirectory()

	widgetDir := filepath.Join(appDataDir, "WeatherWidget")
	if err := os.MkdirAll(widgetDir, 0755); err != nil {
		log.Printf("failed to create app data directory: %v", err)
	}

	if *debugFlag {
		logPath := filepath.Join(widgetDir, "debug.log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(logFile)
			log.Printf("--- Debug logging enabled (version: %s) ---", version)
		} else {
			log.Printf("failed to open debug log file: %v", err)
		}
	}

	if err := uiwindows.Run(appDataDir, *settingsFlag); err != nil {
		log.Fatalf("failed to start WeatherWidget: %v", err)
	}
}
