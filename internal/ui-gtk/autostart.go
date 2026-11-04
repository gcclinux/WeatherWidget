//go:build linux

// Package uitk — autostart.go manages the ~/.config/autostart desktop entry
// for the GTK WeatherWidget binary.
//
// Snap behaviour: under strict confinement $HOME is the snap's private data
// dir (~/snap/<snap>/<revision>), so the entry is written to
// ~/snap/<snap>/<revision>/.config/autostart/. At login snapd's helper
// (`snap userd --autostart`, run from /etc/xdg/autostart) scans
// ~/snap/<snap>/current/.config/autostart/ and launches any entry whose file
// name matches an app's `autostart:` value in snapcraft.yaml — which is
// "weatherwidget.desktop". The Exec line uses the stable "current" symlink
// rather than the revision (e.g. x1) so it survives snap refreshes.
package uitk

import (
	"log"
	"os"
	"path/filepath"
)

const (
	// gtkDesktopEntryName is the autostart file name for non-snap installs.
	gtkDesktopEntryName = "weatherwidget-gtk.desktop"
	// snapDesktopEntryName must match the `autostart:` value in
	// snap/snapcraft.yaml, otherwise snapd's autostart helper ignores it.
	snapDesktopEntryName = "weatherwidget.desktop"
)

// isSnap reports whether the binary is running inside a snap.
func isSnap() bool {
	return os.Getenv("SNAP") != ""
}

// desktopEntryName returns the autostart file name for the current packaging.
func desktopEntryName() string {
	if isSnap() {
		return snapDesktopEntryName
	}
	return gtkDesktopEntryName
}

// desktopEntryContentGTK returns the .desktop file content for autostart.
func desktopEntryContentGTK(exePath string) string {
	return "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=WeatherWidget (GTK)\n" +
		"Exec=" + exePath + "\n" +
		"X-GNOME-Autostart-enabled=true\n" +
		"Comment=Desktop weather widget (native GTK3)\n"
}

// autostartDirGTK returns ~/.config/autostart. Under snap $HOME is the snap's
// private data dir, which is where snapd's autostart helper looks.
func autostartDirGTK() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "autostart")
}

// autostartExecPath returns the path written to the Exec= line.
//
// Under snap it uses /snap/<instance>/current/bin/<binary> so the entry keeps
// working after a refresh (the revision directory, e.g. x1, changes on every
// update but "current" always points at the active revision). Otherwise it
// uses the resolved path of the running executable.
func autostartExecPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	if isSnap() {
		instance := os.Getenv("SNAP_INSTANCE_NAME")
		if instance == "" {
			instance = os.Getenv("SNAP_NAME")
		}
		if instance != "" {
			return filepath.Join("/snap", instance, "current", "bin", filepath.Base(exePath)), nil
		}
	}

	return filepath.EvalSymlinks(exePath)
}

// isAutoStartEnabled checks whether the GTK autostart .desktop file exists.
func isAutoStartEnabled() bool {
	path := filepath.Join(autostartDirGTK(), desktopEntryName())
	_, err := os.Stat(path)
	return err == nil
}

// setAutoStartEnabled creates or removes the GTK autostart .desktop file.
func setAutoStartEnabled(enabled bool) error {
	dir := autostartDirGTK()
	path := filepath.Join(dir, desktopEntryName())

	// Under snap, remove the legacy entry written by older versions. It used
	// a file name snapd ignores, so it never actually autostarted the app.
	if isSnap() {
		legacy := filepath.Join(dir, gtkDesktopEntryName)
		if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
			log.Printf("GTK auto-start: could not remove legacy entry %s: %v", legacy, err)
		}
	}

	if enabled {
		exePath, err := autostartExecPath()
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
