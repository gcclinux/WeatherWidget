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
}

// createIndicator creates an AppIndicator with the given ID and icon name.
static AppIndicator* createIndicator(const char* id, const char* icon) {
	return app_indicator_new(id, icon, APP_INDICATOR_CATEGORY_APPLICATION_STATUS);
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
	"unsafe"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// setupTray installs an AppIndicator system tray icon with a context menu
// providing Show, Hide, Settings, and Quit actions.
func setupTray(m *manager) {
	id := C.CString("com.weatherwidget.gtk")
	defer C.free(unsafe.Pointer(id))

	// Use a standard icon name that every freedesktop theme provides.
	iconName := C.CString("weather-clear")
	defer C.free(unsafe.Pointer(iconName))

	// Suppress the "libayatana-appindicator is deprecated" warning — the library
	// emits this about itself on every launch. It's informational only; the API
	// still works correctly. We silence it via a no-op GLib log handler.
	C.suppressAppIndicatorWarnings()

	ind := C.createIndicator(id, iconName)
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
