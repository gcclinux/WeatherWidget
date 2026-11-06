//go:build windows

package uiwindows

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jchv/go-webview2"
	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"
	"weatherwidget/internal/weather/database"
)

// webFS embeds the HTML, CSS, and JS files for the Settings window.
//
//go:embed web/*
var webFS embed.FS

var (
	settingsMu sync.Mutex
	activeWV   webview2.WebView
	openingWV  bool
)

type settingsLocale struct {
	Code        string
	DisplayName string
	Flag        string
}

// OpenSettings opens the Edge WebView2 settings dialog.
func (m *Manager) OpenSettings() {
	settingsMu.Lock()
	if activeWV != nil || openingWV {
		settingsMu.Unlock()
		return
	}
	openingWV = true
	settingsMu.Unlock()

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		wv := webview2.NewWithOptions(webview2.WebViewOptions{
			Debug:     false,
			DataPath:  m.appDataDir,
			AutoFocus: true,
			WindowOptions: webview2.WindowOptions{
				Title:  "WeatherWidget Settings",
				Width:  860,
				Height: 640,
			},
		})
		if wv == nil {
			settingsMu.Lock()
			openingWV = false
			settingsMu.Unlock()
			log.Println("uiwindows: failed to initialize WebView2 settings window")
			return
		}

		settingsMu.Lock()
		activeWV = wv
		openingWV = false
		settingsMu.Unlock()

		defer func() {
			m.clearSettingsPreview()
			settingsMu.Lock()
			activeWV = nil
			settingsMu.Unlock()
			wv.Destroy()
		}()

		var bindErr error
		bind := func(name string, callback interface{}) {
			if err := wv.Bind(name, callback); err != nil && bindErr == nil {
				bindErr = fmt.Errorf("bind %s: %w", name, err)
			}
		}
		bind("goGetConfig", func() (string, error) {
			m.mu.Lock()
			data, err := json.Marshal(m.cfg)
			m.mu.Unlock()
			if err != nil {
				return "", err
			}
			return string(data), nil
		})

		bind("goSaveConfig", func(jsonStr string) error {
			var newCfg config.Config
			if err := json.Unmarshal([]byte(jsonStr), &newCfg); err != nil {
				return fmt.Errorf("invalid config: %w", err)
			}
			return m.saveSettings(&newCfg)
		})

		bind("goGetLocales", func() ([]settingsLocale, error) {
			m.mu.Lock()
			defer m.mu.Unlock()
			if m.lm == nil {
				return nil, fmt.Errorf("locale manager unavailable")
			}
			locales := m.lm.AvailableLocales()
			result := make([]settingsLocale, 0, len(locales))
			for _, locale := range locales {
				flag, err := assets.Icons.ReadFile("icons/flags/" + locale.Code + ".png")
				if err != nil {
					return nil, fmt.Errorf("load flag for %s: %w", locale.Code, err)
				}
				result = append(result, settingsLocale{locale.Code, locale.DisplayName, "data:image/png;base64," + base64.StdEncoding.EncodeToString(flag)})
			}
			return result, nil
		})
		bind("goGetTranslations", func(locale string, keys []string) (map[string]string, error) {
			lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
			if err != nil {
				return nil, err
			}
			if err := lm.SetLocale(locale); err != nil {
				return nil, err
			}
			wv.Dispatch(func() { wv.SetTitle(lm.T("settings.title")) })
			values := make(map[string]string, len(keys))
			for _, key := range keys {
				values[key] = lm.T(key)
			}
			return values, nil
		})
		bind("goGetMonitors", func() int { return monitorCount() })
		bind("goSearchCity", func(name, region, provider, key, locale string) (config.CityConfig, error) {
			lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
			if err != nil {
				return config.CityConfig{}, err
			}
			if err := lm.SetLocale(locale); err != nil {
				return config.CityConfig{}, err
			}
			return searchSettingsCity(name, region, provider, key, lm)
		})
		bind("goPreviewConfig", func(jsonStr string) error {
			var preview config.Config
			if err := json.Unmarshal([]byte(jsonStr), &preview); err != nil {
				return fmt.Errorf("invalid preview: %w", err)
			}
			m.mu.Lock()
			m.preview = &preview
			m.mu.Unlock()
			m.redraw()
			return nil
		})

		bind("goCloseSettings", func() {
			wv.Dispatch(wv.Terminate)
		})

		bind("goTestConnection", func(jsonStr string) error {
			var cfg config.Config
			if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
				return fmt.Errorf("invalid configuration: %w", err)
			}
			adapter, db, err := m.buildProvider(&cfg)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			return adapter.TestConnection(ctx)
		})

		bind("goGetAutoStart", func() bool {
			return isAutoStartEnabled()
		})

		bind("goSetAutoStart", func(enable bool) error {
			return setAutoStartEnabled(enable)
		})
		if bindErr != nil {
			log.Printf("uiwindows: settings bridge initialization failed: %v", bindErr)
			return
		}

		// Load bundled web content
		htmlContent := buildBundledHTML()
		wv.SetHtml(htmlContent)

		wv.Run()
	}()
}

func (m *Manager) saveSettings(cfg *config.Config) error {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		return err
	}
	if err := lm.SetLocale(cfg.Locale); err != nil {
		return err
	}
	invalid := func(key string) error {
		return fmt.Errorf("%s: %s", lm.T(key), lm.T("settings.windows.invalid"))
	}
	if cfg.RefreshInterval <= 0 {
		return invalid("settings.interval.title")
	}
	if cfg.DataSource != config.DataSourceRemoteAPI && cfg.DataSource != config.DataSourceLocalDatabase {
		return invalid("settings.windows.dataSource")
	}
	if cfg.Opacity < 25 || cfg.Opacity > 100 {
		return invalid("settings.transparency.title")
	}
	if cfg.FontSizeCityTime < 8 || cfg.FontSizeCityTime > 48 || cfg.FontSizeTempIcon < 10 || cfg.FontSizeTempIcon > 80 || cfg.FontSizeConditions < 6 || cfg.FontSizeConditions > 40 {
		return invalid("settings.fontSize.title")
	}
	if errs := config.Validate(cfg, lm.T); len(errs) != 0 {
		return errs[0]
	}
	if cfg.DataSource == config.DataSourceLocalDatabase && strings.TrimSpace(cfg.DatabaseConfig.Query) == "" {
		return fmt.Errorf("%s: %s", lm.T("settings.windows.query"), lm.T("settings.windows.required"))
	}
	if !cfg.HasLicense() && !reflect.DeepEqual(cfg.Cities, config.DefaultCities()) {
		return fmt.Errorf("%s", lm.T("error.settings.licenseRequired"))
	}
	if cfg.MonitorIndex < 0 || cfg.MonitorIndex >= monitorCount() {
		return invalid("settings.monitor.label")
	}
	for _, city := range cfg.Cities {
		if _, err := time.LoadLocation(city.Timezone); err != nil {
			return fmt.Errorf("%s (%s): %s", lm.T("settings.locations.tzLabel"), city.Name, lm.T("settings.windows.invalid"))
		}
	}
	m.mu.Lock()
	old := m.cfg
	m.mu.Unlock()
	providerChanged := old.DataSource != cfg.DataSource || !reflect.DeepEqual(old.APIConfig, cfg.APIConfig) || !reflect.DeepEqual(old.DatabaseConfig, cfg.DatabaseConfig)
	var newProvider weather.WeatherProvider
	var newDB *database.DatabaseAdapter
	if providerChanged {
		newProvider, newDB, err = m.buildProvider(cfg)
		if err != nil {
			return fmt.Errorf("%s: %w", lm.T("settings.provider.title"), err)
		}
	}
	if err := m.cfgSvc.Save(cfg); err != nil {
		if newDB != nil {
			newDB.Close()
		}
		return fmt.Errorf(lm.T("error.settings.saveFailed"), err)
	}
	if providerChanged && m.weatherSvc != nil {
		m.weatherSvc.SwitchProvider(newProvider)
		if m.dbAdapter != nil {
			m.dbAdapter.Close()
		}
		m.dbAdapter = newDB
	}
	m.redrawMu.Lock()
	m.mu.Lock()
	m.cfg = cfg
	m.preview = nil
	m.lm = lm
	m.mu.Unlock()
	m.redrawMu.Unlock()
	if m.sched != nil {
		m.sched.SetInterval(time.Duration(cfg.RefreshInterval) * time.Minute)
		m.sched.SetCities(cfg.Cities)
	}
	if !reflect.DeepEqual(old.Cities, cfg.Cities) || providerChanged {
		m.resultsMu.Lock()
		m.results = nil
		m.resultsMu.Unlock()
		if m.sched != nil {
			m.sched.FetchNow()
		}
	}
	m.redraw()
	m.redrawMu.Lock()
	if m.widget != nil {
		m.widget.ApplyPlacement(cfg.CustomX, cfg.CustomY, cfg.CornerPosition, cfg.MonitorIndex)
	}
	m.redrawMu.Unlock()
	return nil
}

func (m *Manager) clearSettingsPreview() {
	m.mu.Lock()
	hadPreview := m.preview != nil
	m.preview = nil
	m.mu.Unlock()
	if hadPreview {
		m.redraw()
	}
}

// buildBundledHTML inlines CSS and JS into index.html so it loads self-contained in WebView2.
func buildBundledHTML() string {
	indexHTML, err := webFS.ReadFile("web/index.html")
	if err != nil {
		return "<html><body>Failed to load settings HTML</body></html>"
	}
	styleCSS, _ := webFS.ReadFile("web/style.css")
	appJS, _ := webFS.ReadFile("web/app.js")

	html := string(indexHTML)
	// Replace external stylesheet link with inline <style>
	html = strings.Replace(
		html,
		`<link rel="stylesheet" href="style.css">`,
		fmt.Sprintf("<style>%s</style>", string(styleCSS)),
		1,
	)
	// Replace external script tag with inline <script>
	html = strings.Replace(
		html,
		`<script src="app.js"></script>`,
		fmt.Sprintf("<script>%s</script>", string(appJS)),
		1,
	)

	return html
}
