//go:build linux

// Package main is the entry point for the native GTK3 WeatherWidget for Linux.
// This binary uses no Fyne dependencies — it renders entirely with GTK3 via
// github.com/gotk3/gotk3, giving true per-widget transparency, native look,
// and first-class Wayland/X11 support.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	uitk "weatherwidget/internal/ui-gtk"
)

// version is set at build time via -ldflags "-X main.version=1.0.0"
var version = "dev"

func main() {
	// Force X11 backend via XWayland so that window positioning (Move/GetPosition)
	// works reliably. On native Wayland these calls are no-ops — the compositor
	// controls placement. XWayland is present on all modern Wayland desktops and
	// gives us full X11 window management while still running under Wayland.
	if os.Getenv("GDK_BACKEND") == "" {
		os.Setenv("GDK_BACKEND", "x11")
	}

	debugFlag := flag.Bool("debug", false, "Enable debug logging to a file")
	settingsFlag := flag.Bool("settings", false, "Open the settings window on launch")
	flag.Parse()

	appDataDir := appDataDirectory()
	if err := os.MkdirAll(appDataDir, 0755); err != nil {
		log.Printf("failed to create app data directory: %v", err)
	}

	if *debugFlag {
		logPath := filepath.Join(appDataDir, "debug-gtk.log")
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(logFile)
			log.Printf("--- GTK debug logging enabled (version: %s) ---", version)
		}
	}

	uitk.Run(appDataDir, *settingsFlag)
}

// appDataDirectory returns ~/.config/WeatherWidget.
func appDataDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "WeatherWidget")
}
