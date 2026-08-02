//go:build linux

// Package uitk — autostart.go manages the ~/.config/autostart desktop entry
// for the GTK WeatherWidget binary.
package uitk

import (
	"log"
	"os"
	"path/filepath"
)

const gtkDesktopEntryName = "weatherwidget-gtk.desktop"

// desktopEntryContentGTK returns the .desktop file content for autostart.
func desktopEntryContentGTK(exePath string) string {
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=WeatherWidget (GTK)\n" +
		"Exec=" + exePath + "\n" +
		"X-GNOME-Autostart-enabled=true\n" +
		"Comment=Desktop weather widget (native GTK3)\n"
}

// autostartDirGTK returns ~/.config/autostart.
func autostartDirGTK() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "autostart")
}

// isAutoStartEnabled checks whether the GTK autostart .desktop file exists.
func isAutoStartEnabled() bool {
	path := filepath.Join(autostartDirGTK(), gtkDesktopEntryName)
	_, err := os.Stat(path)
	return err == nil
}

// setAutoStartEnabled creates or removes the GTK autostart .desktop file.
func setAutoStartEnabled(enabled bool) error {
	dir := autostartDirGTK()
	path := filepath.Join(dir, gtkDesktopEntryName)

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
		content := desktopEntryContentGTK(exePath)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
		log.Printf("GTK auto-start enabled: %s", path)
		return nil
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	log.Println("GTK auto-start disabled")
	return nil
}
