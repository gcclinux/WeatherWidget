// Package main is the entry point for WeatherWidget on macOS (Darwin).
// It delegates entirely to the native AppKit UI layer in internal/ui-darwin,
// which replaces Fyne for the widget window while sharing all business logic.

//go:build darwin

package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	uidarwin "weatherwidget/internal/ui-darwin"
)

// version is set at build time via -ldflags "-X main.version=1.0.x"
var version = "dev"

func main() {
	debugFlag    := flag.Bool("debug",    false, "Enable debug logging to a file")
	settingsFlag := flag.Bool("settings", false, "Open the settings window on launch")
	flag.Parse()

	appDataDir := appDataDirectory()
	if err := os.MkdirAll(appDataDir, 0o755); err != nil {
		log.Printf("failed to create app data directory: %v", err)
	}

	if *debugFlag {
		logPath := filepath.Join(appDataDir, "debug-darwin.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err == nil {
			log.SetOutput(f)
			log.Printf("--- Darwin debug logging enabled (version: %s) ---", version)
		} else {
			log.Printf("failed to open debug log: %v", err)
		}
	}

	// Run the native AppKit UI.  This function does not return until the
	// application quits (it enters the Cocoa event loop internally).
	uidarwin.Run(appDataDir, *settingsFlag)
}
