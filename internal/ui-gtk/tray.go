//go:build linux

package uitk

/*
#cgo pkg-config: ayatana-appindicator3-0.1
#cgo CFLAGS: -Wno-deprecated-declarations

#include <libayatana-appindicator/app-indicator.h>
#include <gtk/gtk.h>
#include <stdlib.h>

// silentLogHandler swallows log messages — used to suppress the
// "libayatana-appindicator is deprecated" warning the library emits about itself.
static void silentLogHandler(const gchar *domain, GLogLevelFlags level,
                              const gchar *message, gpointer user_data) {
	(void)domain; (void)level; (void)message; (void)user_data;
}

// suppressAppIndicatorWarnings registers the silent handler for
// libayatana-appindicator's warning log domain.
static void suppressAppIndicatorWarnings(void) {
	g_log_set_handler(
		"libayatana-appindicator",
		G_LOG_LEVEL_WARNING,
		silentLogHandler,
		NULL
	);
	// Suppress GLib CRITICAL messages that occur when icon theme paths
	// contain NULL entries (common in snap confinement).
	g_log_set_handler(
		"GLib",
		G_LOG_LEVEL_CRITICAL,
		silentLogHandler,
		NULL
	);
}

// createIndicator creates an AppIndicator with the given ID and icon name.
static AppIndicator* createIndicator(const char* id, const char* icon) {
	return app_indicator_new(id, icon, APP_INDICATOR_CATEGORY_APPLICATION_STATUS);
}

// createIndicatorWithPath creates an AppIndicator with a custom icon theme path.
static AppIndicator* createIndicatorWithPath(const char* id, const char* icon, const char* icon_theme_path) {
	return app_indicator_new_with_path(id, icon, APP_INDICATOR_CATEGORY_APPLICATION_STATUS, icon_theme_path);
}

// setIndicatorMenu attaches a GtkMenu to the indicator.
static void setIndicatorMenu(AppIndicator* ind, GtkWidget* menu) {
	app_indicator_set_menu(ind, GTK_MENU(menu));
}

// setIndicatorStatus shows or hides the indicator.
static void setIndicatorActive(AppIndicator* ind, int active) {
	if (active) {
		app_indicator_set_status(ind, APP_INDICATOR_STATUS_ACTIVE);
	} else {
		app_indicator_set_status(ind, APP_INDICATOR_STATUS_PASSIVE);
	}
}

*/
import "C"

import (
	"log"
	"os"
	"path/filepath"
	"unsafe"

	"weatherwidget/assets"

	dbus "github.com/godbus/dbus/v5"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// hasSNIHost checks whether a StatusNotifierWatcher is registered on the
// session D-Bus. This is the host service that consumes AppIndicator/SNI
// registrations and renders the tray icon. On GNOME, this requires the
// "AppIndicator and KStatusNotifierItem Support" extension; KDE Plasma and
// other DEs ship one by default.
func hasSNIHost() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	reply, err := conn.RequestName("org.freedesktop.StatusNotifierWatcher",
		dbus.NameFlagDoNotQueue)
	if err != nil {
		return false
	}
	// If we acquired the name, no host is registered — release it immediately.
	if reply == dbus.RequestNameReplyPrimaryOwner {
		_, _ = conn.ReleaseName("org.freedesktop.StatusNotifierWatcher")
		return false
	}
	// Name is already taken — a host is running.
	return true
}

// prepareTrayIcon extracts the embedded tray icon into a proper freedesktop
// icon theme directory structure so that the SNI host (gnome-shell-extension-
// appindicator) can resolve it via Gtk.IconTheme. The extension receives the
// IconThemePath over D-Bus and uses it as a search path for icon theme lookup,
// which requires a valid index.theme file.
//
// IMPORTANT: The path must be the same as seen from the host filesystem because
// the SNI host (GNOME Shell) runs outside the snap. Snap confinement remaps
// $SNAP_USER_COMMON to a path under /snap/... that doesn't exist on the host.
// We use $HOME/.local/share/weatherwidget/tray-icons which is accessible from
// both inside and outside the snap (via the home plug).
//
// Returns the theme base directory and the icon name (without extension).
func prepareTrayIcon() (themeDir string, iconName string) {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/tmp"
	}
	baseDir := filepath.Join(home, ".local", "share", "weatherwidget", "tray-icons")

	// Structure: <baseDir>/hicolor/48x48/apps/weather-clear.png
	//            <baseDir>/hicolor/index.theme
	// The IconThemePath sent over D-Bus will be <baseDir>.
	hicolorDir := filepath.Join(baseDir, "hicolor")
	iconDir := filepath.Join(hicolorDir, "48x48", "apps")
	if err := os.MkdirAll(iconDir, 0755); err != nil {
		log.Printf("GTK tray: failed to create icon theme dir: %v", err)
		return "", "weather-clear"
	}

	// Write the index.theme — required by Gtk.IconTheme to recognize the directory.
	indexTheme := "[Icon Theme]\nName=hicolor\nComment=Hicolor Icon Theme\nDirectories=48x48/apps\n\n[48x48/apps]\nSize=48\nContext=Applications\nType=Fixed\n"
	indexPath := filepath.Join(hicolorDir, "index.theme")
	if err := os.WriteFile(indexPath, []byte(indexTheme), 0644); err != nil {
		log.Printf("GTK tray: failed to write index.theme: %v", err)
		return "", "weather-clear"
	}

	// Read the embedded tray icon.
	iconData, err := assets.Icons.ReadFile("icons/clear_tray.png")
	if err != nil {
		log.Printf("GTK tray: failed to read embedded tray icon: %v", err)
		return "", "weather-clear"
	}

	// Write the icon as weather-clear.png (matching the icon name used by the indicator).
	iconPath := filepath.Join(iconDir, "weather-clear.png")
	if err := os.WriteFile(iconPath, iconData, 0644); err != nil {
		log.Printf("GTK tray: failed to write tray icon: %v", err)
		return "", "weather-clear"
	}

	log.Printf("GTK tray: icon theme prepared at %s", baseDir)
	return baseDir, "weather-clear"
}

// setupTray installs an AppIndicator system tray icon with a context menu
// providing Show, Hide, Settings, and Quit actions.
func setupTray(m *manager) {
	// Check if an SNI host is available on D-Bus. Without one the icon will
	// be registered but never rendered (common on vanilla GNOME).
	// Skip this check inside snap confinement — AppArmor blocks the D-Bus
	// probe, giving a false negative and potentially interfering with the
	// session bus connection that AppIndicator uses to register.
	if os.Getenv("SNAP") == "" && !hasSNIHost() {
		log.Println("GTK tray: no SNI host found — tray icon may not be visible")
		log.Println("Alternatively, launch with --settings to open settings directly.")
	}

	id := C.CString("com.weatherwidget.gtk")
	defer C.free(unsafe.Pointer(id))

	// Suppress the "libayatana-appindicator is deprecated" warning and GLib
	// CRITICAL messages before creating the indicator.
	C.suppressAppIndicatorWarnings()

	var ind *C.AppIndicator

	// Extract the embedded tray icon and use app_indicator_new_with_path so
	// the SNI host uses our custom colored icon instead of the system theme's
	// monochrome/symbolic icon.
	iconDir, trayIconName := prepareTrayIcon()
	iconName := C.CString(trayIconName)
	defer C.free(unsafe.Pointer(iconName))
	cIconDir := C.CString(iconDir)
	defer C.free(unsafe.Pointer(cIconDir))
	ind = C.createIndicatorWithPath(id, iconName, cIconDir)

	if ind == nil {
		log.Println("GTK tray: failed to create AppIndicator")
		return
	}

	// Build the context menu.
	menu, err := gtk.MenuNew()
	if err != nil {
		log.Printf("GTK tray: failed to create menu: %v", err)
		return
	}

	showItem, _ := gtk.MenuItemNewWithLabel(m.t("tray.showWidget"))
	showItem.Connect("activate", func() {
		glib.IdleAdd(func() {
			m.win.ShowAll()
			m.win.SetKeepBelow(true)
			m.applyPosition()
		})
	})
	menu.Append(showItem)

	hideItem, _ := gtk.MenuItemNewWithLabel(m.t("tray.hideWidget"))
	hideItem.Connect("activate", func() {
		glib.IdleAdd(func() { m.win.Hide() })
	})
	menu.Append(hideItem)

	sep, _ := gtk.SeparatorMenuItemNew()
	menu.Append(sep)

	settingsItem, _ := gtk.MenuItemNewWithLabel(m.t("tray.settings"))
	settingsItem.Connect("activate", func() {
		glib.IdleAdd(func() { m.openSettings() })
	})
	menu.Append(settingsItem)

	sep2, _ := gtk.SeparatorMenuItemNew()
	menu.Append(sep2)

	quitItem, _ := gtk.MenuItemNewWithLabel(m.t("tray.quit"))
	quitItem.Connect("activate", func() {
		glib.IdleAdd(func() {
			if m.sched != nil {
				m.sched.Stop()
			}
			if m.guard != nil {
				_ = m.guard.Release()
			}
			gtk.MainQuit()
		})
	})
	menu.Append(quitItem)

	menu.ShowAll()

	// Attach menu to the indicator.
	C.setIndicatorMenu(ind, (*C.GtkWidget)(unsafe.Pointer(menu.Native())))
	C.setIndicatorActive(ind, 1)

	log.Println("GTK tray: AppIndicator tray installed")
}
