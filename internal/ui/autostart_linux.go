//go:build linux

package ui

import (
	"log"
	"os"
	"path/filepath"
)

const desktopEntryName = "weatherwidget.desktop"

// desktopEntryContent returns the .desktop file content for autostart.
func desktopEntryContent(exePath string) string {
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=WeatherWidget\n" +
		"Exec=" + exePath + "\n" +
		"X-GNOME-Autostart-enabled=true\n" +
		"Comment=Desktop weather widget\n"
}

// autostartDir returns ~/.config/autostart.
func autostartDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "autostart")
}

// isAutoStartEnabled checks whether the autostart .desktop file exists.
func isAutoStartEnabled() bool {
	path := filepath.Join(autostartDir(), desktopEntryName)
	_, err := os.Stat(path)
	return err == nil
}

// setAutoStartEnabled creates or removes the autostart .desktop file.
func setAutoStartEnabled(enabled bool) error {
	dir := autostartDir()
	path := filepath.Join(dir, desktopEntryName)

	if enabled {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		exePath, err = filepath.EvalSymlinks(exePath)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		content := desktopEntryContent(exePath)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
		log.Printf("auto-start enabled: %s", path)
		return nil
	}

	// Remove the file.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	log.Printf("auto-start disabled")
	return nil
}
