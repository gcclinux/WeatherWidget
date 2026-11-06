//go:build windows

package uiwindows

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
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
)

// ewwAPIKeyLen is the expected length of an EasyWeatherWidget API key (UUID v4).
const ewwAPIKeyLen = 36

const stripeProPaymentURL = "https://buy.stripe.com/bJe3cvaJOa650fQ8cPdZ603?client_reference_id="

// generateClientReferenceID creates a random UUID v4 string (e.g. "f169fecc-de0c-4de1-b8c8-4ec3701b671c")
// to be used as a unique client_reference_id for Stripe payment links.
func generateClientReferenceID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

func openBrowserURL(urlStr string) error {
	if !strings.HasPrefix(urlStr, "https://") && !strings.HasPrefix(urlStr, "http://") {
		return fmt.Errorf("unsupported URL scheme")
	}
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr).Start()
}

// activateProKey ensures a 36-character UUID v4 Pro API key is set on the
// configuration for EasyWeatherWidget, immediately persists it to config.json,
// and switches the active weather provider.
func (m *Manager) activateProKey(currentKey string) (string, error) {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()

	clientRef := strings.TrimSpace(currentKey)
	if len(clientRef) != ewwAPIKeyLen {
		clientRef = generateClientReferenceID()
	}

	m.mu.Lock()
	if m.cfg == nil {
		m.cfg = config.DefaultConfig()
	}
	cfgCopy := *m.cfg
	cfgCopy.DataSource = config.DataSourceRemoteAPI
	cfgCopy.DatabaseConfig = nil
	cfgCopy.APIConfig = &config.APIConfig{
		Provider: "easyweatherwidget",
		APIKey:   clientRef,
	}
	m.cfg = &cfgCopy
	m.mu.Unlock()

	if m.cfgSvc != nil {
		if err := m.cfgSvc.Save(&cfgCopy); err != nil {
			return "", err
		}
	}
	if m.weatherSvc != nil {
		if provider, err := m.buildProvider(&cfgCopy); err == nil {
			m.weatherSvc.SwitchProvider(provider)
		}
	}
	return clientRef, nil
}

// webFS embeds the HTML, CSS, and JS files for the Settings window.
//
//go:embed web/*
var webFS embed.FS

var (
	settingsMu sync.Mutex
	activeWV   webview2.WebView
	openingWV  bool
)

// nativeLocaleNames supplies native-script names for locales whose embedded
// display name is not in that language.
var nativeLocaleNames = map[string]string{
	"ta-IN": "தமிழ் (IN)",
	"ja-JP": "日本語 (JP)",
	"zh-CN": "中文 (CN)",
}

type settingsLocale struct {
	Code        string
	DisplayName string
	Flag        string
}

// computeSettingsWindowBounds calculates the initial Settings window size and
// placement. On standard 96-DPI displays (scale <= 1.0), it preserves the exact
// 1150x724 size and default OS placement so normal 100%-scale physical systems
// are completely unaffected. On high-DPI displays (scale > 1.0, such as Retina
// VMs at 192 DPI / 200%), it scales the window proportionally to the monitor
// DPI and centers it within the monitor's work area (above the taskbar).
func computeSettingsWindowBounds(dpi, monLeft, monTop, monWidth, monHeight int) (width, height uint, center bool, posX, posY int) {
	const (
		defaultWidth  = 1150
		defaultHeight = 724
	)

	if dpi <= 96 {
		return defaultWidth, defaultHeight, false, 0, 0
	}

	scale := float64(dpi) / 96.0
	w := int(float64(1080)*scale + 0.5)
	h := int(float64(790)*scale + 0.5)

	if monWidth > 160 && w > monWidth-120 {
		w = monWidth - 120
	}
	if monHeight > 120 && h > monHeight-80 {
		h = monHeight - 80
	}
	if w < defaultWidth {
		w = defaultWidth
	}
	if h < defaultHeight {
		h = defaultHeight
	}

	x := monLeft + (monWidth-w)/2
	y := monTop + (monHeight-h)/2
	if x < monLeft {
		x = monLeft
	}
	if y < monTop {
		y = monTop
	}

	return uint(w), uint(h), true, x, y
}

func (m *Manager) settingsWindowBounds() (width, height uint, center bool, posX, posY int) {
	var hwnd uintptr
	monIdx := 0
	m.mu.Lock()
	if m.widget != nil {
		hwnd = m.widget.hwnd
	}
	if m.cfg != nil {
		monIdx = m.cfg.MonitorIndex
	}
	m.mu.Unlock()

	dpi := GetWindowDPI(hwnd)
	monLeft, monTop, monWidth, monHeight := getMonitorBounds(monIdx)
	return computeSettingsWindowBounds(dpi, monLeft, monTop, monWidth, monHeight)
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

		winW, winH, centerWin, winX, winY := m.settingsWindowBounds()

		wv := webview2.NewWithOptions(webview2.WebViewOptions{
			Debug:     false,
			DataPath:  m.appDataDir,
			AutoFocus: true,
			WindowOptions: webview2.WindowOptions{
				Title:  "WeatherWidget Settings",
				Width:  winW,
				Height: winH,
				Center: centerWin,
			},
		})
		if wv == nil {
			settingsMu.Lock()
			openingWV = false
			settingsMu.Unlock()
			log.Println("uiwindows: failed to initialize WebView2 settings window")
			return
		}

		if centerWin {
			if wvHwnd := uintptr(wv.Window()); wvHwnd != 0 {
				procSetWindowPos.Call(
					wvHwnd,
					0,
					uintptr(winX),
					uintptr(winY),
					0,
					0,
					swpNoSize|swpNoZOrder|swpNoActivate,
				)
			}
		}

		settingsMu.Lock()
		activeWV = wv
		openingWV = false
		settingsMu.Unlock()

		origX, origY := 0, 0
		if m.widget != nil {
			origX, origY = m.widget.GetPosition()
		}
		saved := false
		defer func() {
			if !saved && m.widget != nil {
				m.widget.SetPosition(origX, origY)
			}
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
			if err := m.saveSettings(&newCfg); err != nil {
				return err
			}
			saved = true
			return nil
		})

		bind("goGetPosition", func() []int {
			if m.widget == nil {
				return []int{0, 0}
			}
			x, y := m.widget.GetPosition()
			return []int{x, y}
		})
		bind("goMoveWidget", func(x, y int) {
			if m.widget != nil {
				m.widget.SetPosition(x, y)
			}
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
				name := locale.DisplayName
				if native, ok := nativeLocaleNames[locale.Code]; ok {
					name = native
				}
				result = append(result, settingsLocale{locale.Code, name, "data:image/png;base64," + base64.StdEncoding.EncodeToString(flag)})
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
			adapter, err := m.buildProvider(&cfg)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			return adapter.TestConnection(ctx)
		})

		bind("goGetProAPI", func(currentKey string) (string, error) {
			clientRef, err := m.activateProKey(currentKey)
			if err != nil {
				return "", err
			}
			saved = true
			_ = openBrowserURL(stripeProPaymentURL + clientRef)
			return clientRef, nil
		})

		bind("goOpenURL", func(urlStr string) error {
			return openBrowserURL(urlStr)
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
	if cfg.DataSource == "" {
		cfg.DataSource = config.DataSourceRemoteAPI
	}
	if cfg.DataSource != config.DataSourceRemoteAPI {
		return invalid("settings.windows.dataSource")
	}
	cfg.DatabaseConfig = nil
	if cfg.Opacity < 25 || cfg.Opacity > 100 {
		return invalid("settings.transparency.title")
	}
	if cfg.FontSizeCityTime < 8 || cfg.FontSizeCityTime > 48 || cfg.FontSizeTempIcon < 10 || cfg.FontSizeTempIcon > 80 || cfg.FontSizeConditions < 6 || cfg.FontSizeConditions > 40 {
		return invalid("settings.fontSize.title")
	}
	if errs := config.Validate(cfg, lm.T); len(errs) != 0 {
		return errs[0]
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
	providerChanged := old.DataSource != cfg.DataSource || !reflect.DeepEqual(old.APIConfig, cfg.APIConfig)
	var newProvider weather.WeatherProvider
	if providerChanged {
		newProvider, err = m.buildProvider(cfg)
		if err != nil {
			return fmt.Errorf("%s: %w", lm.T("settings.provider.title"), err)
		}
	}
	if err := m.cfgSvc.Save(cfg); err != nil {
		return fmt.Errorf(lm.T("error.settings.saveFailed"), err)
	}
	if providerChanged && m.weatherSvc != nil {
		m.weatherSvc.SwitchProvider(newProvider)
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
