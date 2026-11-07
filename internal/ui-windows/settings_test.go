//go:build windows

package uiwindows

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/scheduler"
	"weatherwidget/internal/weather"
)

func TestBuildBundledHTML(t *testing.T) {
	html := buildBundledHTML()
	if !strings.Contains(html, "<style>") {
		t.Error("expected inlined <style> in bundled HTML")
	}
	if !strings.Contains(html, "<script>") {
		t.Error("expected inlined <script> in bundled HTML")
	}
	if !strings.Contains(html, "WeatherWidget Settings") {
		t.Error("expected title in bundled HTML")
	}
}

func TestSettingsTranslationsAndFlagsExistForEveryLocale(t *testing.T) {
	script, err := webFS.ReadFile("web/app.js")
	if err != nil {
		t.Fatal(err)
	}
	keys := regexp.MustCompile(`'(settings\.[\w.]+|error\.settings\.[\w.]+)'`).FindAllStringSubmatch(string(script), -1)
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		t.Fatal(err)
	}
	for _, locale := range lm.AvailableLocales() {
		data, err := i18n.LocaleFS.ReadFile("locales/" + locale.Code + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatal(err)
		}
		for _, match := range keys {
			if values[match[1]] == "" {
				t.Errorf("%s missing %s", locale.Code, match[1])
			}
		}
		if _, err := assets.Icons.ReadFile("icons/flags/" + locale.Code + ".png"); err != nil {
			t.Errorf("%s flag: %v", locale.Code, err)
		}
	}
}

func TestSaveSettingsPersistsFieldsAndRejectsZeroInterval(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultConfig()
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		t.Fatal(err)
	}
	service := config.NewConfigService(dir)
	if err := service.Save(cfg); err != nil {
		t.Fatal(err)
	}
	ws := weather.NewWeatherService(nil)
	sched := scheduler.NewRefreshScheduler(120*time.Minute, ws)
	sched.Start()
	defer sched.Stop()
	m := &Manager{cfg: cfg, cfgSvc: service, lm: lm, sched: sched, weatherSvc: ws}

	copyJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var next config.Config
	if err := json.Unmarshal(copyJSON, &next); err != nil {
		t.Fatal(err)
	}
	next.DisplayFields = config.DefaultDisplayFields()
	next.DisplayFields.ShowWind = false
	next.PollutionFields = &config.PollutionFields{ShowNO: true, ShowNH3: true, ShowTicker: true, TickerProfile: config.TickerProfileSensitive}
	next.NoBackground = true
	next.Locale = "de-DE"
	m.preview = &config.Config{NoBackground: true}
	if err := m.saveSettings(&next); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if m.preview != nil {
		t.Fatal("saving did not clear the preview")
	}
	stored, err := service.Load()
	if err != nil {
		t.Fatal(err)
	}
	if stored.RefreshInterval != 120 || stored.GetDisplayFields().ShowWind || !reflect.DeepEqual(stored.PollutionFields, next.PollutionFields) || !stored.NoBackground || m.lm.ActiveLocale() != "de-DE" {
		t.Fatalf("settings were not applied and persisted: %+v", stored)
	}
	bad := next
	bad.RefreshInterval = 0
	if err := m.saveSettings(&bad); err == nil {
		t.Fatal("expected invalid interval to be rejected")
	} else if !strings.Contains(err.Error(), "Ungültiger Wert") {
		t.Fatalf("expected a localized validation error, got %v", err)
	}
	stored, err = service.Load()
	if err != nil || stored.RefreshInterval != 120 {
		t.Fatalf("invalid save changed persisted interval: %+v, %v", stored, err)
	}
}

func TestSettingsPreviewRollback(t *testing.T) {
	cfg := config.DefaultConfig()
	preview := *cfg
	preview.NoBackground = !cfg.NoBackground
	m := &Manager{cfg: cfg, preview: &preview}
	m.clearSettingsPreview()
	if m.preview != nil || m.cfg.NoBackground != cfg.NoBackground {
		t.Fatal("closing the preview changed the saved configuration")
	}
}

func TestSettingsSwitchRemoteProviderAndRejectDatabase(t *testing.T) {
	cfg := config.DefaultConfig()
	service := config.NewConfigService(t.TempDir())
	m := &Manager{cfg: cfg, cfgSvc: service, weatherSvc: weather.NewWeatherService(nil)}

	// Reject local_database data source since only Remote API providers are supported.
	invalid := *cfg
	invalid.DataSource = config.DataSourceLocalDatabase
	if err := m.saveSettings(&invalid); err == nil {
		t.Fatal("expected local_database data source to be rejected")
	}

	// Switch between EasyWeatherGateway (Pro) and OpenWeatherMap (Free).
	next := *cfg
	next.DataSource = config.DataSourceRemoteAPI
	next.RefreshInterval = 120
	next.APIConfig = &config.APIConfig{
		Provider: "openweathermap",
		APIKey:   "0123456789abcdef0123456789abcdef",
	}
	if err := m.saveSettings(&next); err != nil {
		t.Fatalf("switch to openweathermap provider: %v", err)
	}
	stored, err := service.Load()
	if err != nil || !reflect.DeepEqual(stored.APIConfig, next.APIConfig) {
		t.Fatalf("remote provider configuration was not persisted: %v", err)
	}
}

func TestComputeSettingsWindowBounds(t *testing.T) {
	// Standard DPI keeps the existing width and default OS placement.
	w, h, center, _, _ := computeSettingsWindowBounds(96, 0, 0, 1920, 1040)
	if w != 1150 || h != 880 || center {
		t.Fatalf("expected (1150, 880, false) at 96 DPI, got (%d, %d, %v)", w, h, center)
	}

	// The larger initial height still fits a short standard-DPI work area.
	w, h, center, _, _ = computeSettingsWindowBounds(96, 0, 0, 1366, 728)
	if w != 1150 || h != 648 || center {
		t.Fatalf("expected (1150, 648, false) on a short work area, got (%d, %d, %v)", w, h, center)
	}

	// High DPI scales the height when the work area is large enough.
	w, h, center, x, y := computeSettingsWindowBounds(192, 0, 0, 2880, 2000)
	if w != 2160 || h != 1760 || !center || x != 360 || y != 120 {
		t.Fatalf("expected (2160, 1760, true, 360, 120), got (%d, %d, %v, %d, %d)", w, h, center, x, y)
	}

	// High-DPI windows are clamped and centered when the work area is shorter.
	w, h, center, x, y = computeSettingsWindowBounds(192, 0, 0, 2880, 1704)
	if w != 2160 || h != 1624 || !center || x != 360 || y != 40 {
		t.Fatalf("expected (2160, 1624, true, 360, 40) at 192 DPI, got (%d, %d, %v, %d, %d)", w, h, center, x, y)
	}

	// High DPI on a smaller screen remains inside the work area.
	w, h, center, x, y = computeSettingsWindowBounds(144, 0, 0, 1920, 1032)
	if !center || int(w) > 1920-120 || int(h) > 1032-80 || x < 0 || y < 0 || y+int(h) > 1032 {
		t.Fatalf("expected bounds clamped within 1920x1032 work area, got (%d, %d, %v, %d, %d)", w, h, center, x, y)
	}

	// A work area smaller than the normal window dimensions must still win.
	w, h, center, x, y = computeSettingsWindowBounds(120, -1000, 100, 1000, 600)
	if w != 880 || h != 520 || !center || x != -940 || y != 140 {
		t.Fatalf("expected bounds clamped and centered on a small monitor, got (%d, %d, %v, %d, %d)", w, h, center, x, y)
	}
}

func TestActivateProKeyGeneratesAndPersistsUUID(t *testing.T) {
	cfg := config.DefaultConfig()
	service := config.NewConfigService(t.TempDir())
	m := &Manager{cfg: cfg, cfgSvc: service, weatherSvc: weather.NewWeatherService(nil)}

	// 1. Empty key generates a 36-char UUID v4 and saves it under easyweatherwidget.
	key1, err := m.activateProKey("")
	if err != nil {
		t.Fatalf("activateProKey failed: %v", err)
	}
	if len(key1) != ewwAPIKeyLen {
		t.Fatalf("expected %d-char UUID v4 key, got %q (len=%d)", ewwAPIKeyLen, key1, len(key1))
	}
	stored, err := service.Load()
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if stored.APIConfig == nil || stored.APIConfig.Provider != "easyweatherwidget" || stored.APIConfig.APIKey != key1 {
		t.Fatalf("expected persisted easyweatherwidget key %q, got %+v", key1, stored.APIConfig)
	}

	// 2. Existing 36-char key is preserved instead of overwritten.
	key2, err := m.activateProKey(key1)
	if err != nil {
		t.Fatalf("activateProKey with existing key failed: %v", err)
	}
	if key2 != key1 {
		t.Fatalf("expected existing 36-char key %q to be preserved, got %q", key1, key2)
	}
}
