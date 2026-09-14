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
extern void openSettingsNative(const char *cfgJSON, const char *stringsJSON, const char *langsJSON);
extern void bringSettingsToFront(void);
*/
import "C"

import (
	"encoding/json"
	"log"
	"strconv"
	"sync/atomic"
	"unsafe"

	"weatherwidget/assets"
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
		// The ObjC settings layer serializes config via NSJSONSerialization,
		// whose numeric/boolean boxing can produce JSON types that don't line
		// up 1:1 with the Go struct (e.g. a BOOL boxed as a number, or an int
		// where a float is expected). A strict json.Unmarshal into config.Config
		// would reject the whole payload on the first mismatch and silently drop
		// every change. So we merge tolerantly: start from the current config
		// and overlay only the fields the settings window owns, coercing types
		// as needed. This makes Save robust to ObjC JSON quirks.
		newCfg, err := mergeSettingsJSON(m.cfg, []byte(newCfgJSON))
		if err != nil {
			log.Printf("uidarwin: settings save: could not parse settings JSON: %v", err)
			log.Printf("uidarwin: settings save: raw JSON = %s", newCfgJSON)
			return
		}
		if err := m.onSettingsSave(newCfg); err != nil {
			log.Printf("uidarwin: settings save failed: %v", err)
		}
	})

	cJSON := C.CString(string(cfgBytes))
	defer C.free(unsafe.Pointer(cJSON))

	// Build the localized-strings table for the ObjC UI. The native settings
	// window looks each label/placeholder up by key (see settings.m), so the
	// entire window follows the user's chosen locale — matching the Fyne/GTK UIs.
	stringsJSON, err := json.Marshal(m.settingsStrings())
	if err != nil {
		log.Printf("uidarwin: settings: failed to marshal strings: %v", err)
		stringsJSON = []byte("{}")
	}
	cStrings := C.CString(string(stringsJSON))
	defer C.free(unsafe.Pointer(cStrings))

	// Build the language list (code, native name, English name, absolute flag
	// PNG path) so the native Language tab can render GTK-style flag cards.
	langsJSON, err := json.Marshal(settingsLanguages())
	if err != nil {
		log.Printf("uidarwin: settings: failed to marshal languages: %v", err)
		langsJSON = []byte("[]")
	}
	cLangs := C.CString(string(langsJSON))
	defer C.free(unsafe.Pointer(cLangs))

	// openSettingsNative dispatches to the main thread internally.
	C.openSettingsNative(cJSON, cStrings, cLangs)
}

// settingsLanguage describes one selectable UI language for the native
// Language tab. Mirrors gtkLocaleData in ui-gtk/settings.go.
type settingsLanguage struct {
	Code    string `json:"code"`
	Native  string `json:"native"`  // name in its own language, e.g. "Français"
	English string `json:"english"` // English name, e.g. "French"
	Flag    string `json:"flag"`    // absolute path to the flag PNG (may be "")
}

// settingsLanguages returns the ordered list of supported UI languages with
// their flag image paths resolved to disk, matching the GTK language grid.
func settingsLanguages() []settingsLanguage {
	rows := []struct{ code, native, english, flagFile string }{
		{"en-GB", "English", "English", "icons/flags/en-GB.png"},
		{"es-ES", "Español", "Spanish", "icons/flags/es-ES.png"},
		{"fr-FR", "Français", "French", "icons/flags/fr-FR.png"},
		{"de-DE", "Deutsch", "German", "icons/flags/de-DE.png"},
		{"it-IT", "Italiano", "Italian", "icons/flags/it-IT.png"},
		{"pt-BR", "Português", "Português (BR)", "icons/flags/pt-BR.png"},
		{"nl-NL", "Nederlands", "Dutch", "icons/flags/nl-NL.png"},
		{"pl-PL", "Polski", "Polish", "icons/flags/pl-PL.png"},
		{"tr-TR", "Türkçe", "Turkish", "icons/flags/tr-TR.png"},
		{"ta-IN", "தமிழ்", "Tamil", "icons/flags/ta-IN.png"},
		{"ja-JP", "日本語", "Japanese", "icons/flags/ja-JP.png"},
		{"zh-CN", "中文", "Chinese", "icons/flags/zh-CN.png"},
	}
	out := make([]settingsLanguage, 0, len(rows))
	for _, r := range rows {
		out = append(out, settingsLanguage{
			Code:    r.code,
			Native:  r.native,
			English: r.english,
			// resolveEmbeddedAsset extracts the embedded PNG to a temp file and
			// returns its absolute path (cached), which the ObjC NSImage loads.
			Flag: resolveEmbeddedAsset(r.flagFile, assets.Icons),
		})
	}
	return out
}

// settingsStrings returns the localized string table passed to the native
// settings window, keyed by the same i18n keys used by the Fyne/GTK UIs so all
// three front-ends stay in sync. Each value is resolved through m.t(), which
// falls back to the key itself when a translation is missing.
func (m *manager) settingsStrings() map[string]string {
	keys := []string{
		// Window + tabs + actions
		"settings.title",
		"settings.tab.provider", "settings.tab.locations", "settings.tab.widget",
		"settings.tab.language", "settings.tab.appearance", "settings.tab.about",
		"settings.save", "settings.cancel",
		// Provider tab
		"settings.provider.label", "settings.provider.apiKeyLabel",
		"settings.provider.apiKeyPlaceholder", "settings.interval.title",
		"settings.provider.note",
		// Locations tab
		"settings.locations.savedTitle", "settings.locations.savedSubtitle",
		"settings.locations.addTitle", "settings.locations.nameLabel",
		"settings.locations.namePlaceholder", "settings.locations.regionLabel",
		"settings.locations.regionPlaceholder", "settings.locations.latLabel",
		"settings.locations.latPlaceholder", "settings.locations.lonLabel",
		"settings.locations.lonPlaceholder", "settings.locations.tzLabel",
		"settings.locations.tzPlaceholder", "settings.locations.addBtn",
		"settings.locations.removeBtn",
		// Widget tab: panel display
		"settings.display.title", "settings.display.subtitle",
		"settings.display.city", "settings.display.icon", "settings.display.temp",
		"settings.display.desc", "settings.display.humidity", "settings.display.wind",
		"settings.display.time", "settings.display.date", "settings.display.windGust",
		"settings.display.dewPoint", "settings.display.pressure", "settings.display.uvIndex",
		// Widget tab: pollution
		"settings.pollution.title", "settings.pollution.subtitle",
		"settings.pollution.aqi", "settings.pollution.co", "settings.pollution.no",
		"settings.pollution.no2", "settings.pollution.o3", "settings.pollution.so2",
		"settings.pollution.nh3", "settings.pollution.pm2_5", "settings.pollution.pm10",
		// Widget tab: units
		"settings.temperature.title", "settings.windspeed.title",
		// Language tab
		"settings.language.title", "settings.language.subtitle",
		// Appearance tab
		"settings.viewMode.title", "settings.viewMode.enhanced", "settings.viewMode.simple",
		"settings.transparency.title", "settings.startup.autostart",
		"settings.icons.title", "settings.icons.new", "settings.icons.original",
		"settings.position.title",
		"settings.fontSize.title", "settings.fontSize.cityTime",
		"settings.fontSize.tempIcon", "settings.fontSize.conditions",
		// About tab
		"settings.about.appName", "settings.about.description",
		"settings.about.websiteLabel", "settings.about.manualLabel",
		"settings.about.airIndexLabel",
		// Alerts
		"error.settings.licenseRequired", "error.settings.cityNameRequired",
	}
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		out[k] = m.t(k)
	}
	return out
}

// mergeSettingsJSON overlays the fields the settings window owns onto a deep
// copy of base, coercing JSON types tolerantly. It never fails on a single
// field's type quirk — unrecognised or wrongly-typed values are simply skipped
// so the rest of the save still applies.
//
// base is deep-copied via JSON round-trip so the caller's config is untouched.
func mergeSettingsJSON(base *config.Config, data []byte) (*config.Config, error) {
	// Deep copy base so we start from the current, valid config and only change
	// what the settings window sends.
	var out config.Config
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(baseBytes, &out); err != nil {
		return nil, err
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	// ── scalar string fields ──
	if s, ok := jsonString(m["locale"]); ok && s != "" {
		out.Locale = s
	}
	if s, ok := jsonString(m["temperatureUnit"]); ok {
		out.TemperatureUnit = config.NormalizeTemperatureUnit(config.TemperatureUnit(s))
	}
	if s, ok := jsonString(m["windSpeedUnit"]); ok {
		out.WindSpeedUnit = config.NormalizeWindSpeedUnit(config.WindSpeedUnit(s))
	}
	if s, ok := jsonString(m["iconTheme"]); ok {
		out.IconTheme = config.NormalizeIconTheme(config.IconTheme(s))
	}
	if s, ok := jsonString(m["viewMode"]); ok {
		out.ViewMode = config.NormalizeViewMode(config.ViewMode(s))
	}
	if s, ok := jsonString(m["cornerPosition"]); ok && s != "" {
		out.CornerPosition = s
	}

	// ── scalar int fields ──
	if n, ok := jsonInt(m["refreshInterval"]); ok && n > 0 {
		out.RefreshInterval = n
	}
	if n, ok := jsonInt(m["opacity"]); ok && n > 0 {
		out.Opacity = n
	}
	if n, ok := jsonInt(m["monitorIndex"]); ok {
		out.MonitorIndex = n
	}
	if n, ok := jsonInt(m["fontSizeCityTime"]); ok && n > 0 {
		out.FontSizeCityTime = n
	}
	if n, ok := jsonInt(m["fontSizeTempIcon"]); ok && n > 0 {
		out.FontSizeTempIcon = n
	}
	if n, ok := jsonInt(m["fontSizeConditions"]); ok && n > 0 {
		out.FontSizeConditions = n
	}
	if n, ok := jsonInt(m["customX"]); ok {
		out.CustomX = &n
	}
	if n, ok := jsonInt(m["customY"]); ok {
		nn := n
		out.CustomY = &nn
	}

	// ── display fields ──
	if raw, ok := m["displayFields"]; ok {
		var df map[string]json.RawMessage
		if json.Unmarshal(raw, &df) == nil {
			d := out.GetDisplayFields()
			jsonSetBool(df, "showCity", &d.ShowCity)
			jsonSetBool(df, "showIcon", &d.ShowIcon)
			jsonSetBool(df, "showTemp", &d.ShowTemp)
			jsonSetBool(df, "showDesc", &d.ShowDesc)
			jsonSetBool(df, "showHumidity", &d.ShowHumidity)
			jsonSetBool(df, "showWind", &d.ShowWind)
			jsonSetBool(df, "showTime", &d.ShowTime)
			jsonSetBool(df, "showDate", &d.ShowDate)
			jsonSetBool(df, "showWindGust", &d.ShowWindGust)
			jsonSetBool(df, "showDewPoint", &d.ShowDewPoint)
			jsonSetBool(df, "showPressure", &d.ShowPressure)
			jsonSetBool(df, "showUVIndex", &d.ShowUVIndex)
			out.DisplayFields = d
		}
	}

	// ── pollution fields ──
	if raw, ok := m["pollutionFields"]; ok {
		var pf map[string]json.RawMessage
		if json.Unmarshal(raw, &pf) == nil {
			p := out.GetPollutionFields()
			jsonSetBool(pf, "showAQI", &p.ShowAQI)
			jsonSetBool(pf, "showCO", &p.ShowCO)
			jsonSetBool(pf, "showNO", &p.ShowNO)
			jsonSetBool(pf, "showNO2", &p.ShowNO2)
			jsonSetBool(pf, "showO3", &p.ShowO3)
			jsonSetBool(pf, "showSO2", &p.ShowSO2)
			jsonSetBool(pf, "showNH3", &p.ShowNH3)
			jsonSetBool(pf, "showPM25", &p.ShowPM25)
			jsonSetBool(pf, "showPM10", &p.ShowPM10)
			out.PollutionFields = p
		}
	}

	// ── apiConfig ──
	if raw, ok := m["apiConfig"]; ok {
		var api map[string]json.RawMessage
		if json.Unmarshal(raw, &api) == nil {
			if out.APIConfig == nil {
				out.APIConfig = &config.APIConfig{}
			}
			if s, ok := jsonString(api["provider"]); ok && s != "" {
				out.APIConfig.Provider = s
			}
			if s, ok := jsonString(api["apiKey"]); ok {
				out.APIConfig.APIKey = s
			}
		}
	}

	// ── cities ──
	// Only replace the city list when the payload carries a well-formed array
	// of city objects. A malformed/absent value leaves the base cities intact
	// (they can't be edited without a license anyway).
	if raw, ok := m["cities"]; ok {
		var cities []config.CityConfig
		if err := json.Unmarshal(raw, &cities); err == nil && len(cities) > 0 {
			out.Cities = cities
		} else if err != nil {
			log.Printf("uidarwin: settings save: ignoring malformed cities payload: %v; raw cities = %s", err, string(raw))
		}
	}

	return &out, nil
}

// jsonString extracts a string from a raw JSON value. Returns ok=false when the
// value is absent or not a JSON string.
func jsonString(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// jsonInt extracts an int from a raw JSON value, accepting JSON numbers,
// numeric strings, and booleans (true=1/false=0). Returns ok=false otherwise.
func jsonInt(raw json.RawMessage) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int(f), true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if g, e := strconv.ParseFloat(s, 64); e == nil {
			return int(g), true
		}
	}
	return 0, false
}

// jsonBool extracts a bool from a raw JSON value, accepting JSON booleans and
// numbers (0=false, non-zero=true) — the latter covers NSJSONSerialization
// emitting BOOLs as 0/1. Returns ok=false when absent or unparseable.
func jsonBool(raw json.RawMessage) (bool, bool) {
	if len(raw) == 0 {
		return false, false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, true
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f != 0, true
	}
	return false, false
}

// jsonSetBool applies a tolerant bool from m[key] onto dst when present.
func jsonSetBool(m map[string]json.RawMessage, key string, dst *bool) {
	if v, ok := jsonBool(m[key]); ok {
		*dst = v
	}
}
