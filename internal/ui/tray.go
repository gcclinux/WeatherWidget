package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
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

	// Using a Fyne default vector icon to test if the tray is capable of rendering at all.
	res := theme.SettingsIcon()
	u.app.SetIcon(res)
	desk.SetSystemTrayIcon(res)

	// Define menu items with icons for a more premium look and feel.
	settingsItem := fyne.NewMenuItem("Settings", onSettings)
	quitItem := fyne.NewMenuItem("Quit", onExit)
	quitItem.IsQuit = true

	m := fyne.NewMenu("WeatherWidget",
		fyne.NewMenuItem("Show Widget", func() {
			u.widget.Show()
		}),
		fyne.NewMenuItem("Hide Widget", func() {
			u.widget.Hide()
		}),
		fyne.NewMenuItemSeparator(),
		settingsItem,
		fyne.NewMenuItemSeparator(),
		quitItem,
	)

	desk.SetSystemTrayMenu(m)
}
