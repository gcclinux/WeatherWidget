// callbacks.h — shared Go-callable C callback stubs for the tray.
// These are defined in tray.m and called from the ObjC action handlers.
#pragma once

// traySettingsCB is invoked when the user clicks "Settings" in the tray menu.
// Defined in tray.m; bridges to the Go onSettings callback stored in g_onSettings.
void traySettingsCB(void);

// trayQuitCB is invoked when the user clicks "Quit" in the tray menu.
// Defined in tray.m; bridges to the Go onQuit callback stored in g_onQuit.
void trayQuitCB(void);
