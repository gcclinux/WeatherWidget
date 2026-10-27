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

	tray   *Tray
	widget *Widget
	drawer *WidgetDrawer

	isWidgetVisible bool
	isSettingsOpen  bool

	results   []weather.WeatherResult
	resultsMu sync.Mutex

	dragTimer *time.Timer
	stopClock chan struct{}

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
		stopClock:       make(chan struct{}),
		drawer:          NewWidgetDrawer(),
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

	// 4. Initialize Desktop Layered Window.
	widget, err := NewWidget(m.onWidgetDragEnd, m.onWidgetContextMenu)
	if err != nil {
		log.Printf("uiwindows: failed to create desktop widget window: %v", err)
	} else {
		m.widget = widget
		defer m.widget.Destroy()

		// Initial render to establish dimensions and surface
		m.redraw()

		// Apply placement (custom X/Y or corner pinning)
		m.widget.ApplyPlacement(cfg.CustomX, cfg.CustomY, cfg.CornerPosition, cfg.MonitorIndex)

		if m.isWidgetVisible {
			m.widget.Show()
		}
	}

	// 5. Initialize 1-second clock ticker for live local times.
	go m.startClockLoop()

	// 6. Initialize weather service and scheduler.
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

	// 7. Initialize power resume listener to immediately refresh when waking from sleep.
	resumeCh := power.ResumeNotifier()
	go func() {
		for range resumeCh {
			log.Println("uiwindows: system woke from sleep/hibernation, triggering refresh")
			m.manualRefresh()
		}
	}()

	// 8. Setup system tray icon and menu.
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

	// 9. Win32 Message pump.
	var winMsg msg
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&winMsg)),
			0, 0, 0,
		)
		if ret == 0 || ret == ^uintptr(0) {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&winMsg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&winMsg)))
	}

	log.Println("uiwindows: main message loop terminated cleanly")
	return nil
}

// redraw renders all cards and updates the layered window surface.
func (m *Manager) redraw() {
	if m.widget == nil || m.drawer == nil || m.cfg == nil {
		return
	}

	m.resultsMu.Lock()
	res := m.results
	m.resultsMu.Unlock()

	img := m.drawer.Render(m.cfg.Cities, res, m.cfg, m.lm, time.Now())
	m.widget.UpdateSurface(img)
}

// startClockLoop runs a 1-second ticker that updates clocks on the widget.
func (m *Manager) startClockLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopClock:
			return
		case <-ticker.C:
			if m.isWidgetVisible {
				m.redraw()
			}
		}
	}
}

// onWeatherUpdate is called by the scheduler whenever new weather data is received.
func (m *Manager) onWeatherUpdate(results []weather.WeatherResult) {
	log.Printf("uiwindows: received weather update for %d cities", len(results))
	m.resultsMu.Lock()
	m.results = results
	m.resultsMu.Unlock()
	m.redraw()
}

// onWidgetDragEnd debounces and persists custom coordinates to config.
func (m *Manager) onWidgetDragEnd(x, y int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.dragTimer != nil {
		m.dragTimer.Stop()
	}

	m.dragTimer = time.AfterFunc(400*time.Millisecond, func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		m.cfg.CustomX = &x
		m.cfg.CustomY = &y
		if err := m.cfgSvc.Save(m.cfg); err != nil {
			log.Printf("uiwindows: failed to save custom position (%d, %d): %v", x, y, err)
		} else {
			log.Printf("uiwindows: saved custom position: (%d, %d)", x, y)
		}
	})
}

// onWidgetContextMenu displays the tray context menu on right click.
func (m *Manager) onWidgetContextMenu() {
	if m.tray != nil {
		m.tray.showContextMenu()
	}
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

// showWidget makes the widget window visible.
func (m *Manager) showWidget() {
	m.mu.Lock()
	m.isWidgetVisible = true
	m.mu.Unlock()

	if m.widget != nil {
		m.widget.Show()
		m.redraw()
	}
	log.Println("uiwindows: show widget requested")
}

// hideWidget hides the widget window.
func (m *Manager) hideWidget() {
	m.mu.Lock()
	m.isWidgetVisible = false
	m.mu.Unlock()

	if m.widget != nil {
		m.widget.Hide()
	}
	log.Println("uiwindows: hide widget requested")
}

// manualRefresh triggers an immediate weather data fetch and re-renders clocks.
func (m *Manager) manualRefresh() {
	log.Println("uiwindows: manual weather refresh triggered")
	m.redraw()
	if m.sched != nil {
		m.sched.FetchNow()
	}
}

// openSettings opens the WebView2 settings window.
func (m *Manager) openSettings() {
	m.OpenSettings()
}

// shutdown tears down components and posts WM_QUIT to terminate the message loop.
func (m *Manager) shutdown() {
	log.Println("uiwindows: shutting down application")

	select {
	case <-m.stopClock:
	default:
		close(m.stopClock)
	}

	if m.widget != nil {
		m.widget.Destroy()
		m.widget = nil
	}

	if m.tray != nil {
		m.tray.Destroy()
		m.tray = nil
	}

	procPostQuitMessage.Call(0)
}
