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

func TestSettingsSwitchDatabaseProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	service := config.NewConfigService(t.TempDir())
	m := &Manager{cfg: cfg, cfgSvc: service, weatherSvc: weather.NewWeatherService(nil)}
	next := *cfg
	next.DataSource = config.DataSourceLocalDatabase
	next.RefreshInterval = 30
	next.DatabaseConfig = &config.DatabaseConfig{
		Host: "127.0.0.1", Port: 5432, DBName: "weather", Username: "user", Password: "p@ss:/word",
		Query: "SELECT temperature FROM weather WHERE city = $1",
	}
	invalid := next
	invalidDB := *next.DatabaseConfig
	invalidDB.Query = " "
	invalid.DatabaseConfig = &invalidDB
	if err := m.saveSettings(&invalid); err == nil {
		t.Fatal("expected an empty weather query to be rejected")
	}
	if m.cfg != cfg || m.dbAdapter != nil {
		t.Fatal("invalid database settings changed the active configuration")
	}
	if err := m.saveSettings(&next); err != nil {
		t.Fatalf("switch to database: %v", err)
	}
	if m.dbAdapter == nil {
		t.Fatal("database provider was not installed")
	}
	stored, err := service.Load()
	if err != nil || !reflect.DeepEqual(stored.DatabaseConfig, next.DatabaseConfig) {
		t.Fatalf("database configuration was not persisted: %v", err)
	}
	back := next
	back.DataSource = config.DataSourceRemoteAPI
	back.RefreshInterval = 120
	if err := m.saveSettings(&back); err != nil {
		m.dbAdapter.Close()
		t.Fatalf("switch to remote provider: %v", err)
	}
	if m.dbAdapter != nil {
		t.Fatal("database provider was not released")
	}
}
