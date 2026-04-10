package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"weatherwidget/assets"
)

// SetupSystemTray configures the system tray icon and menu.
// It checks whether the app supports desktop features (desk.App) and,
// if so, creates a tray menu with Show/Hide/Settings/Exit items.
// If the app does not support system tray (e.g. non-desktop driver)
// or setup fails, a warning is logged and the app continues without tray.
//
// onSettings is called when the user selects "Settings" from the tray menu.
// onExit is called when the user selects "Exit" from the tray menu.
func (u *UIManager) SetupSystemTray(onSettings func(), onExit func()) {
	desk, ok := u.app.(desktop.App)
	if !ok {
		log.Println("warning: system tray not supported on this platform, continuing without tray")
		return
	}

	m := fyne.NewMenu("WeatherWidget",
		fyne.NewMenuItem("Show Widget", func() {
			u.widget.Show()
		}),
		fyne.NewMenuItem("Hide Widget", func() {
			u.widget.Hide()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Settings", onSettings),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Exit", onExit),
	)

	desk.SetSystemTrayMenu(m)

	// Use the embedded clear weather icon as the tray icon.
	iconData, err := assets.Icons.ReadFile("icons/clear.png")
	if err != nil {
		log.Println("warning: failed to load tray icon asset:", err)
		return
	}
	desk.SetSystemTrayIcon(fyne.NewStaticResource("tray-icon.png", iconData))
}
