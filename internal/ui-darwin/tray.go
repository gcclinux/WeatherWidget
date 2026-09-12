//go:build darwin

package uidarwin

// tray.go — NSStatusItem system-tray integration for the native Darwin UI.
//
// The ObjC tray.m stores two global C function pointers (g_onSettings /
// g_onQuit) that are called when the user clicks a menu item.  Because CGo
// cannot expose Go closures as C function pointers, we use a global callback
// registry: two slots indexed by a small integer.  The C thunks
// (traySettingsCB / trayQuitCB) call invokeCallback(0) and invokeCallback(1),
// which look up the Go func() stored in the registry.
//
// The registry lives here; tray.m's callbacks.h declares the C-side thunks.

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <stdlib.h>

// invokeCallback is exported from this file so tray.m can call it.
// index: 0 = onSettings, 1 = onQuit.
extern void invokeCallback(int index);
*/
import "C"

import "sync"

// ── Callback registry ─────────────────────────────────────────────────────────

const (
	cbIndexSettings = 0
	cbIndexQuit     = 1
	cbCount         = 2
)

var (
	cbMu        sync.Mutex
	cbRegistry  [cbCount]func()
)

// registerCallback stores a Go func in the indexed slot.
func registerCallback(index int, fn func()) {
	cbMu.Lock()
	cbRegistry[index] = fn
	cbMu.Unlock()
}

// invokeCallback is called from C (tray.m → traySettingsCB / trayQuitCB).
//
//export invokeCallback
func invokeCallback(index C.int) {
	cbMu.Lock()
	fn := cbRegistry[int(index)]
	cbMu.Unlock()
	if fn != nil {
		mainQueue <- fn
	}
}

// goWorkCallback is called by the dispatch_source timer in runloop.m
// to drain the Go mainQueue on the Cocoa main thread.
//
//export goWorkCallback
func goWorkCallback() {
	pumpMainQueue()
}

// appReadyHook is set by manager.Run() to the function that starts the manager
// (config, weather, window). Invoked from goAppReady (applicationDidFinishLaunching).
var appReadyHook func()

// createTrayHook is set by manager.Run() to the function that creates the
// status item. Invoked synchronously from goCreateTray, before [NSApp run],
// so the tray exists during the RunningBoard launch handshake.
var createTrayHook func()

// goAppReady is called from the ObjC applicationDidFinishLaunching delegate.
//
//export goAppReady
func goAppReady() {
	if appReadyHook != nil {
		appReadyHook()
	}
}

// goCreateTray is called synchronously from initAndRun before [NSApp run].
//
//export goCreateTray
func goCreateTray() {
	if createTrayHook != nil {
		createTrayHook()
	}
}

// ── setupTray wires the system tray icon with Go callbacks ────────────────────

// setupTray creates the NSStatusItem, stores the Go callbacks, and returns the
// native handle.  Must be called after the Cocoa main run loop has started (or
// from a dispatch_sync to the main queue, which createStatusItem does).
func setupTray(m *manager) {
	// Register Go-side callbacks into the indexed slots.
	registerCallback(cbIndexSettings, func() { m.openSettings() })
	registerCallback(cbIndexQuit, func() { m.shutdown() })

	// Fallback labels in case the locale manager isn't loaded yet (tray is
	// created before loadConfigAndLocale during launch).
	settingsLabel := m.t("tray.settings")
	if settingsLabel == "" || settingsLabel == "tray.settings" {
		settingsLabel = "Settings"
	}
	quitLabel := m.t("tray.quit")
	if quitLabel == "" || quitLabel == "tray.quit" {
		quitLabel = "Quit"
	}

	handle := nativeCreateStatusItem(settingsLabel, quitLabel)
	m.trayItem = handle
}

// updateTrayMenu rebuilds the tray menu after a locale change.
func updateTrayMenu(m *manager) {
	if m.trayItem == 0 {
		return
	}
	nativeUpdateStatusItemMenu(
		m.trayItem,
		m.t("tray.settings"),
		m.t("tray.quit"),
	)
}
