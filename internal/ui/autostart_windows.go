//go:build windows

package ui

import (
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const autoStartRegistryKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const autoStartValueName = "WeatherWidget"

// isAutoStartEnabled checks whether the WeatherWidget auto-start registry
// entry exists under HKCU\Software\Microsoft\Windows\CurrentVersion\Run.
func isAutoStartEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, autoStartRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	_, _, err = key.GetStringValue(autoStartValueName)
	return err == nil
}

// setAutoStartEnabled creates or removes the WeatherWidget auto-start
// registry entry. When enabled, it points to the current executable path.
func setAutoStartEnabled(enabled bool) error {
	if enabled {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		exePath, err = filepath.EvalSymlinks(exePath)
		if err != nil {
			return err
		}

		key, _, err := registry.CreateKey(registry.CURRENT_USER, autoStartRegistryKey, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer key.Close()

		if err := key.SetStringValue(autoStartValueName, exePath); err != nil {
			return err
		}
		log.Printf("auto-start enabled: %s", exePath)
		return nil
	}

	// Remove the entry.
	key, err := registry.OpenKey(registry.CURRENT_USER, autoStartRegistryKey, registry.SET_VALUE)
	if err != nil {
		// Key doesn't exist — nothing to remove.
		return nil
	}
	defer key.Close()

	if err := key.DeleteValue(autoStartValueName); err != nil {
		// Value doesn't exist — already disabled.
		log.Printf("auto-start entry not found, already disabled")
		return nil
	}
	log.Printf("auto-start disabled")
	return nil
}
