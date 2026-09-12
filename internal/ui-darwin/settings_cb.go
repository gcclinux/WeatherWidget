//go:build darwin

package uidarwin

// settings_cb.go — Go↔ObjC callback bridge for the settings window.

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <stdlib.h>
*/
import "C"

import "sync"

var (
	settingsSaveMu sync.Mutex
	settingsSaveFn func(string)
)

// registerSettingsSaveCallback stores the Go function to call when settings are saved.
func registerSettingsSaveCallback(fn func(string)) {
	settingsSaveMu.Lock()
	settingsSaveFn = fn
	settingsSaveMu.Unlock()
}

// settingsSaveCB is called from settings.m on the main thread when the user
// saves settings.
//
//export settingsSaveCB
func settingsSaveCB(newCfgJSON *C.char) {
	if newCfgJSON == nil {
		return
	}
	s := C.GoString(newCfgJSON)
	settingsSaveMu.Lock()
	fn := settingsSaveFn
	settingsSaveMu.Unlock()
	if fn != nil {
		go fn(s) // run save on a goroutine so the main thread is freed immediately
	}
}

// settingsClosedCB is called from settings.m when the settings window closes.
//
//export settingsClosedCB
func settingsClosedCB() {
	settingsOpen.Store(0)
}

// settingsAutostartGetCB reports whether launch-at-login is currently enabled.
// Returns 1 when enabled, 0 otherwise. Called from settings.m on load.
//
//export settingsAutostartGetCB
func settingsAutostartGetCB() C.int {
	if isAutoStartEnabled() {
		return 1
	}
	return 0
}

// settingsAutostartSetCB enables or disables launch-at-login. Called from
// settings.m when the user toggles the checkbox. Returns 1 on success, 0 on
// failure (so the ObjC side can revert the checkbox state).
//
//export settingsAutostartSetCB
func settingsAutostartSetCB(enabled C.int) C.int {
	if err := setAutoStartEnabled(enabled != 0); err != nil {
		return 0
	}
	return 1
}
