//go:build darwin

package uidarwin

// settings.go — Settings window for the native Darwin UI.
//
// Opens a native NSPanel settings window implemented in settings.m.
// All UI runs on the Cocoa main thread via dispatch_async — no Fyne dependency.
//
// The settings window is a tabbed NSPanel (Provider / Locations / Widget /
// Language / Appearance / About) built entirely in Objective-C.  Go provides
// config read/write and the save callback.

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#import <stdlib.h>

// Forward-declared from settings.m
extern void openSettingsNative(const char *cfgJSON, void *unusedOnSave);
extern void bringSettingsToFront(void);
*/
import "C"

import (
	"encoding/json"
	"log"
	"sync/atomic"
	"unsafe"

	"weatherwidget/internal/config"
)

// settingsOpen is 1 while the settings window is open, 0 otherwise.
// Prevents opening multiple settings windows simultaneously.
var settingsOpen atomic.Int32

// openSettingsWindow opens the native settings panel.
// Safe to call from any goroutine — dispatches to the main thread internally.
func openSettingsWindow(m *manager) {
	if !settingsOpen.CompareAndSwap(0, 1) {
		// Already open — bring it to the front via mainQueue.
		mainQueue <- func() { C.bringSettingsToFront() }
		return
	}

	// Serialize current config to JSON for the ObjC layer.
	cfgBytes, err := json.Marshal(m.cfg)
	if err != nil {
		log.Printf("uidarwin: settings: failed to marshal config: %v", err)
		settingsOpen.Store(0)
		return
	}

	// Register the save callback so settings.m can call back into Go.
	registerSettingsSaveCallback(func(newCfgJSON string) {
		var newCfg config.Config
		if err := json.Unmarshal([]byte(newCfgJSON), &newCfg); err != nil {
			log.Printf("uidarwin: settings save: bad JSON: %v", err)
			return
		}
		if err := m.onSettingsSave(&newCfg); err != nil {
			log.Printf("uidarwin: settings save failed: %v", err)
		}
	})

	cJSON := C.CString(string(cfgBytes))
	defer C.free(unsafe.Pointer(cJSON))

	// openSettingsNative dispatches to the main thread internally.
	// Pass NULL for onSave — settings.m calls settingsSaveCB (Go export) directly.
	C.openSettingsNative(cJSON, nil)
}
