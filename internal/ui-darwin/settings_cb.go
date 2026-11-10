//go:build darwin

package uidarwin

// settings_cb.go — Go↔ObjC callback bridge for the settings window.

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <stdlib.h>
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"weatherwidget/internal/tzlookup"
	"weatherwidget/internal/weather/remoteapi"
)

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
	} else {
		log.Printf("uidarwin: settingsSaveCB: no save callback registered!")
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

// settingsTestConnectionCB verifies the remote API credentials via RemoteAPIAdapter.
// Returns NULL on success, or a newly allocated C string with the error message
// (which the caller must free).
//
//export settingsTestConnectionCB
func settingsTestConnectionCB(provider *C.char, apiKey *C.char) *C.char {
	prov := C.GoString(provider)
	key := C.GoString(apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adapter := remoteapi.NewRemoteAPIAdapter(prov, key)
	if err := adapter.TestConnection(ctx); err != nil {
		return C.CString(err.Error())
	}
	return nil
}

// settingsSearchCityCB searches for a city via RemoteAPIAdapter.SearchCity and
// returns a newly allocated JSON C string (which the caller must free).
//
//export settingsSearchCityCB
func settingsSearchCityCB(provider *C.char, apiKey *C.char, name *C.char, region *C.char) *C.char {
	type searchResult struct {
		Name      string  `json:"name,omitempty"`
		Region    string  `json:"region,omitempty"`
		Latitude  float64 `json:"latitude,omitempty"`
		Longitude float64 `json:"longitude,omitempty"`
		Timezone  string  `json:"timezone,omitempty"`
		Error     string  `json:"error,omitempty"`
		NotFound  bool    `json:"notFound,omitempty"`
	}

	prov := C.GoString(provider)
	key := C.GoString(apiKey)
	cityName := C.GoString(name)
	cityRegion := C.GoString(region)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adapter := remoteapi.NewRemoteAPIAdapter(prov, key)
	city, err := adapter.SearchCity(ctx, cityName, cityRegion)
	var out searchResult
	if err != nil {
		var se *remoteapi.CitySearchError
		if errors.As(err, &se) && (se.Kind == remoteapi.SearchErrNotFound || se.Kind == remoteapi.SearchErrAPIStatus || se.Kind == remoteapi.SearchErrDecode) {
			out.NotFound = true
		}
		out.Error = err.Error()
	} else {
		tz := city.Timezone
		if tz == "" && (city.Latitude != 0 || city.Longitude != 0) {
			tz = tzlookup.Lookup(city.Latitude, city.Longitude)
		}
		out = searchResult{
			Name:      city.Name,
			Region:    city.Region,
			Latitude:  city.Latitude,
			Longitude: city.Longitude,
			Timezone:  tz,
		}
	}
	b, _ := json.Marshal(out)
	return C.CString(string(b))
}
