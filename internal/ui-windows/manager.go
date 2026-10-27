//go:build windows

package uiwindows

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"weatherwidget/internal/config"
	"weatherwidget/internal/guard"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/power"
	"weatherwidget/internal/scheduler"
	"weatherwidget/internal/weather"
	"weatherwidget/internal/weather/remoteapi"
)

var (
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
)

// msg mirrors the Win32 MSG structure.
type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

// Manager orchestrates all application components for the native Windows UI.
type Manager struct {
	appDataDir string
	cfgSvc     *config.ConfigService
	cfg        *config.Config
	lm         *i18n.LocaleManager
	weatherSvc *weather.WeatherService
	sched      *scheduler.RefreshScheduler
	guard      *guard.SingleInstanceGuard

	tray            *Tray
	isWidgetVisible bool
	isSettingsOpen  bool

	mu sync.Mutex
}

func init() {
	// Lock the main goroutine to the main OS thread for Win32 GUI message pump.
	runtime.LockOSThread()
}

// Run is the public entry point for the native Windows UI layer.
func Run(appDataDir string, openSettings bool) error {
	m := &Manager{
		appDataDir:      appDataDir,
		isWidgetVisible: true,
	}

	// 1. Acquire single-instance guard.
	g, err := guard.NewSingleInstanceGuard("WeatherWidget")
	if err != nil {
		return fmt.Errorf("single instance check: %w", err)
	}
	m.guard = g
	defer m.guard.Release()

	// 2. Load configuration.
	m.cfgSvc = config.NewConfigService(m.appDataDir)
	log.Printf("uiwindows: config path: %s", m.cfgSvc.ConfigPath())
	cfg, err := m.cfgSvc.Load()
	if err != nil {
		log.Printf("uiwindows: failed to load config, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}
	m.cfg = cfg

	// 3. Load localization manager.
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		log.Printf("uiwindows: locale manager error: %v", err)
	} else {
		m.lm = lm
		if cfg.Locale != "" {
			_ = m.lm.SetLocale(cfg.Locale)
		}
	}

	// 4. Initialize weather service and scheduler.
	provider := m.buildProvider(cfg)
	m.weatherSvc = weather.NewWeatherService(provider)

	interval := time.Duration(cfg.RefreshInterval) * time.Minute
	m.sched = scheduler.NewRefreshScheduler(interval, m.weatherSvc)
	m.sched.SetCities(cfg.Cities)
	m.sched.SetOnUpdate(func(results []weather.WeatherResult) {
		m.onWeatherUpdate(results)
	})
	m.sched.SetOnError(func(city string, err error) {
		log.Printf("uiwindows: weather error for %s: %v", city, err)
	})
	m.sched.Start()
	defer m.sched.Stop()

	// 5. Initialize power resume listener to immediately refresh when waking from sleep.
	resumeCh := power.ResumeNotifier()
	go func() {
		for range resumeCh {
			log.Println("uiwindows: system woke from sleep/hibernation, triggering refresh")
			m.manualRefresh()
		}
	}()

	// 6. Setup system tray icon and menu.
	trayCallbacks := TrayCallbacks{
		OnShowWidget: func() { m.showWidget() },
		OnHideWidget: func() { m.hideWidget() },
		OnRefresh:    func() { m.manualRefresh() },
		OnSettings:   func() { m.openSettings() },
		OnQuit:       func() { m.shutdown() },
		GetLabel:     func(key string) string { return m.t(key) },
	}

	tray, err := newTray(trayCallbacks)
	if err != nil {
		log.Printf("uiwindows: failed to create system tray: %v", err)
	} else {
		m.tray = tray
		defer m.tray.Destroy()
	}

	if openSettings {
		m.openSettings()
	}

	log.Println("uiwindows: entering main message loop")

	// 7. Win32 Message pump.
	var winMsg msg
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&winMsg)),
			0, 0, 0,
		)
		if ret == 0 || ret == ^uintptr(0) {
			// WM_QUIT or error
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&winMsg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&winMsg)))
	}

	log.Println("uiwindows: main message loop terminated cleanly")
	return nil
}

// buildProvider creates a WeatherProvider based on the config.
func (m *Manager) buildProvider(cfg *config.Config) weather.WeatherProvider {
	provider := "easyweatherwidget"
	apiKey := ""
	if cfg.APIConfig != nil {
		if cfg.APIConfig.Provider != "" {
			provider = cfg.APIConfig.Provider
		}
		apiKey = cfg.APIConfig.APIKey
	}
	if apiKey == "" {
		provider = "openmeteo"
	}
	return remoteapi.NewRemoteAPIAdapter(provider, apiKey)
}

// t translates a localization key.
func (m *Manager) t(key string) string {
	if m.lm == nil {
		return key
	}
	return m.lm.T(key)
}

// showWidget makes the widget window visible (Phase 2 hook).
func (m *Manager) showWidget() {
	m.mu.Lock()
	m.isWidgetVisible = true
	m.mu.Unlock()
	log.Println("uiwindows: show widget requested")
}

// hideWidget hides the widget window (Phase 2 hook).
func (m *Manager) hideWidget() {
	m.mu.Lock()
	m.isWidgetVisible = false
	m.mu.Unlock()
	log.Println("uiwindows: hide widget requested")
}

// manualRefresh triggers an immediate weather data fetch.
func (m *Manager) manualRefresh() {
	log.Println("uiwindows: manual weather refresh triggered")
	if m.sched != nil {
		m.sched.FetchNow()
	}
}

// openSettings opens the WebView2 settings window (Phase 4 hook).
func (m *Manager) openSettings() {
	m.mu.Lock()
	m.isSettingsOpen = true
	m.mu.Unlock()
	log.Println("uiwindows: open settings requested")
}

// shutdown tears down components and posts WM_QUIT to terminate the message loop.
func (m *Manager) shutdown() {
	log.Println("uiwindows: shutting down application")
	if m.tray != nil {
		m.tray.Destroy()
		m.tray = nil
	}
	procPostQuitMessage.Call(0)
}

// onWeatherUpdate is called by the scheduler whenever new weather data is received.
func (m *Manager) onWeatherUpdate(results []weather.WeatherResult) {
	log.Printf("uiwindows: received weather update for %d cities", len(results))
	// In Phase 2/3 this will redraw the widget window.
}
