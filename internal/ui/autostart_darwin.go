//go:build darwin

package ui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const launchAgentLabel = "com.weatherwidget.app"

// launchAgentPath returns ~/Library/LaunchAgents/com.weatherwidget.app.plist.
func launchAgentPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

// launchAgentPlist returns the plist XML content for a LaunchAgent that
// starts the given executable at login.
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

// isAutoStartEnabled checks whether the LaunchAgent plist exists.
func isAutoStartEnabled() bool {
	_, err := os.Stat(launchAgentPath())
	return err == nil
}

// setAutoStartEnabled creates or removes the LaunchAgent plist.
// When enabled, it writes a plist that tells launchd to run the current
// executable at login. When disabled, it removes the plist file.
func setAutoStartEnabled(enabled bool) error {
	path := launchAgentPath()

	if enabled {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		exePath, err = filepath.EvalSymlinks(exePath)
		if err != nil {
			return err
		}

		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}

		content := launchAgentPlist(exePath)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return err
		}
		log.Printf("macOS: auto-start enabled: %s", path)
		return nil
	}

	// Remove the plist.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	log.Printf("macOS: auto-start disabled")
	return nil
}
