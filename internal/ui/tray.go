package ui

import (
	"log"
	"weatherwidget/assets"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// SetupSystemTray configures the system tray icon and menu.
// It checks whether the app supports desktop features (desk.App) and,
// if so, creates a tray menu with Show/Hide/Settings/Exit items.
// If the app does not support system tray (e.g. non-desktop driver)
// or setup fails, a warning is logged and the app continues without tray.
//
// onSettings is called when the user selects "Settings" from the tray menu.
// onExit is called when the user selects "Quit" from the tray menu.
func (u *UIManager) SetupSystemTray(appDataDir string, onSettings func(), onExit func()) {
	desk, ok := u.app.(desktop.App)
	if !ok {
		log.Println("warning: system tray not supported on this platform, continuing without tray")
		return
	}

	// Load the tray icon from embedded assets.
	var iconData []byte
	var err error
	for _, p := range []string{"icons/original/clear_tray.png", "icons/clear_tray.png", "icons/day/clear_day.png"} {
		iconData, err = assets.Icons.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Printf("warning: failed to load tray icon: %v, continuing without custom icon", err)
		return
	}
	res := fyne.NewStaticResource("clear_tray.png", iconData)
	u.app.SetIcon(res)
	desk.SetSystemTrayIcon(res)

	// Define menu items using translated labels via u.t().
	// Because labels are resolved at build time (not cached), the tray menu
	// will display the current locale's translations whenever SetupSystemTray
	// is called again after a locale change — no extra refresh logic needed.
	settingsItem := fyne.NewMenuItem(u.t("tray.settings"), onSettings)
	quitItem := fyne.NewMenuItem(u.t("tray.quit"), onExit)
	quitItem.IsQuit = true

	m := fyne.NewMenu("WeatherWidget",
		fyne.NewMenuItem(u.t("tray.showWidget"), func() {
			u.widget.Show()
		}),
		fyne.NewMenuItem(u.t("tray.hideWidget"), func() {
			u.widget.Hide()
		}),
		fyne.NewMenuItemSeparator(),
		settingsItem,
		fyne.NewMenuItemSeparator(),
		quitItem,
	)

	desk.SetSystemTrayMenu(m)
}
