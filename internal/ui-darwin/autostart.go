//go:build darwin

package uidarwin

// autostart.go — macOS login-item management via a LaunchAgent plist.
//
// The plist is written to ~/Library/LaunchAgents/com.weatherwidget.app.plist
// and tells launchd to start the WeatherWidget executable at login.
// This matches internal/ui/autostart_darwin.go from the Fyne UI layer.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const launchAgentLabel = "com.weatherwidget.app"

// launchAgentPath returns the full path to the LaunchAgent plist file:
// ~/Library/LaunchAgents/com.weatherwidget.app.plist
func launchAgentPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

// launchAgentPlist returns the plist XML that registers exePath as a login item.
func launchAgentPlist(exePath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
</dict>
</plist>
`, launchAgentLabel, exePath)
}

// isAutoStartEnabled reports whether the LaunchAgent plist file exists.
func isAutoStartEnabled() bool {
	_, err := os.Stat(launchAgentPath())
	return err == nil
}

// setAutoStartEnabled creates or removes the LaunchAgent plist.
// When enabled is true it writes the plist for the current executable;
// when false it removes the plist file.
func setAutoStartEnabled(enabled bool) error {
	path := launchAgentPath()

	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		log.Printf("uidarwin: auto-start disabled (%s removed)", path)
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(launchAgentPlist(exePath)), 0o644); err != nil {
		return err
	}
	log.Printf("uidarwin: auto-start enabled (%s)", path)
	return nil
}
