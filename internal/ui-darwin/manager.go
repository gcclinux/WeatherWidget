//go:build darwin

// Package uidarwin is the native macOS UI layer for WeatherWidget.
// It uses AppKit/Objective-C via CGo for the widget window and cards,
// keeping the Fyne dependency only for the settings dialog.
// All business-logic packages (config, weather, scheduler, i18n, power)
// are shared unchanged with the Linux GTK and Windows Fyne builds.
package uidarwin

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/guard"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/power"
	"weatherwidget/internal/scheduler"
	"weatherwidget/internal/weather"
	"weatherwidget/internal/weather/remoteapi"
)

// ── manager ──────────────────────────────────────────────────────────────────

// manager wires all application components together for the native Darwin UI.
// It mirrors the structure of internal/ui-gtk/manager.go.
type manager struct {
	appDataDir string
	cfgSvc     *config.ConfigService
	cfg        *config.Config
	lm         *i18n.LocaleManager
	weatherSvc *weather.WeatherService
	sched      *scheduler.RefreshScheduler
	guard      *guard.SingleInstanceGuard

	// Native window and card container handles (opaque ObjC pointers).
	win       uintptr
	container uintptr

	// card handles indexed by city position (same order as cfg.Cities).
	cardsMu sync.Mutex
	cards   []uintptr

	// viewMode and display settings.
	viewMode   config.ViewMode
	opacity    int
	noBackground bool
	fontSizeCityTime   int
	fontSizeTempIcon   int
	fontSizeConditions int

	// System-tray NSStatusItem handle.
	trayItem uintptr

	// Drag position-save debounce.
	dragDebounceMu sync.Mutex
	dragTimer      *time.Timer

	// positioned is true once the initial position has been applied, after
	// which drag-end events are allowed to persist custom coordinates.
	positioned bool

	// stopClock is closed to stop all per-card clock goroutines.
	stopClock chan struct{}
}

// ── Run ──────────────────────────────────────────────────────────────────────

// Run is the public entry point called from cmd/weatherwidget/main.go (Darwin
// build).  It initialises all components then enters the Cocoa main run loop.
// It does not return until the application quits.
//
// openSettings is true when the --settings flag was passed; in that case the
// settings window is opened immediately after startup.
// init locks the main goroutine to the main OS thread for the life of the
// process. This is the documented way (per runtime docs) to guarantee main()
// — and therefore the [NSApp run] call — executes on the true main thread that
// AppKit requires. Doing this inside Run() is too late: the Go scheduler may
// have already migrated the main goroutine off thread 1.
func init() {
	runtime.LockOSThread()
}

func Run(appDataDir string, openSettings bool) {
	m := &manager{appDataDir: appDataDir}

	// Do the ABSOLUTE MINIMUM before RunMainLoop(). RunningBoard holds a ~6s
	// launch assertion on the process; we must reach [NSApp run] fast or the
	// assertion times out and the app is killed. Even config file I/O + i18n
	// embed parsing here was enough to blow the window when bundled.
	//
	// createTrayHook creates the status item synchronously inside initAndRun
	// (before [app run]) — this is what keeps the accessory app alive.
	createTrayHook = func() {
		// Config/locale may not be loaded yet; setupTray tolerates a nil lm
		// (labels fall back to keys, refreshed later via updateTrayMenu).
		setupTray(m)
	}

	// appReadyHook fires from applicationDidFinishLaunching. It does ALL the
	// real work (config, locale, weather, window) on a goroutine so nothing
	// blocks the Cocoa main thread or the launch handshake.
	appReadyHook = func() {
		go func() {
			m.loadConfigAndLocale()
			updateTrayMenu(m) // refresh tray labels now that locale is loaded
			if err := m.start(openSettings); err != nil {
				log.Fatalf("uidarwin: %v", err)
			}
		}()
	}

	// Enter [NSApp run]. Never returns.
	RunMainLoop()
}

// loadConfigAndLocale performs the fast, non-network startup work: config load,
// opacity/view/font settings, and locale manager. Called from Run() before
// [NSApp run] so the tray has proper labels and settings has a valid config.
func (m *manager) loadConfigAndLocale() {
	m.cfgSvc = config.NewConfigService(m.appDataDir)
	log.Printf("uidarwin: config path: %s", m.cfgSvc.ConfigPath())
	cfg, err := m.cfgSvc.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}
	m.cfg = cfg
	m.opacity = cfg.Opacity
	if m.opacity == 0 {
		m.opacity = 100
	}
	m.noBackground = cfg.NoBackground
	m.viewMode = config.NormalizeViewMode(cfg.ViewMode)
	m.fontSizeCityTime = cfg.GetFontSizeCityTime()
	m.fontSizeTempIcon = cfg.GetFontSizeTempIcon()
	m.fontSizeConditions = cfg.GetFontSizeConditions()

	if cfg.CustomX != nil && cfg.CustomY != nil {
		log.Printf("uidarwin: config loaded: customX=%d customY=%d", *cfg.CustomX, *cfg.CustomY)
	} else {
		log.Printf("uidarwin: config loaded: corner=%s", cfg.CornerPosition)
	}

	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		log.Printf("uidarwin: locale manager error: %v", err)
	} else {
		if cfg.Locale != "" {
			_ = lm.SetLocale(cfg.Locale)
		}
		m.lm = lm
	}
}

// ── start ─────────────────────────────────────────────────────────────────────

func (m *manager) start(openSettings bool) error {
	// 1. Single-instance guard (no-op on non-Windows but keeps the interface).
	g, err := guard.NewSingleInstanceGuard("WeatherWidget-Darwin")
	if err != nil {
		return fmt.Errorf("single-instance guard: %w", err)
	}
	m.guard = g

	// Config + locale already loaded synchronously in Run() → loadConfigAndLocale.
	cfg := m.cfg

	// 4. Weather provider + service.
	m.weatherSvc = weather.NewWeatherService(m.buildProvider(cfg))

	// 5. Create native window and card container.
	m.win = nativeCreateWidgetWindow()
	if m.win == 0 {
		return fmt.Errorf("failed to create native widget window")
	}
	m.container = nativeCreateCardContainer(m.win)

	// 6. Build initial city cards.
	m.buildCards(cfg.Cities)

	// 7. Apply initial position (inline — window not visible yet).
	m.applyPosition()
	m.positioned = true

	// 8. Show window. The tray was already created synchronously via
	// createTrayHook before [NSApp run]. start() runs on a goroutine after
	// applicationDidFinishLaunching, so the Cocoa loop is servicing events;
	// these native calls dispatch to the main thread safely.
	log.Printf("uidarwin: showing window")
	nativeShowWidgetWindow(m.win)
	nativeSetWidgetWindowOpacity(m.win, opacityToAlpha(m.opacity))
	if openSettings {
		openSettingsWindow(m)
	}

	// 9. Wire drag-to-reposition.
	go m.pollDragPosition()

	// 10. Scheduler.
	interval := time.Duration(cfg.RefreshInterval) * time.Minute
	m.sched = scheduler.NewRefreshScheduler(interval, m.weatherSvc)
	m.sched.SetCities(cfg.Cities)
	m.sched.SetOnUpdate(func(results []weather.WeatherResult) {
		mainQueue <- func() { m.handleWeatherUpdate(results) }
	})
	m.sched.SetOnError(func(city string, err error) {
		log.Printf("uidarwin: weather error for %s: %v", city, err)
	})
	m.sched.Start()

	// 12. Open settings immediately if requested.
	// 13. Power-resume: trigger immediate refresh on wake from sleep.
	go func() {
		for range power.ResumeNotifier() {
			log.Println("uidarwin: system resume — triggering weather refresh")
			m.sched.FetchNow()
		}
	}()

	return nil
}

// ── Card management ───────────────────────────────────────────────────────────

// buildCards creates native city-card views for each city, replaces any
// previous cards, and lays out the container.
func (m *manager) buildCards(cities []config.CityConfig) {
	m.stopClocks()

	if len(cities) == 0 {
		cities = config.DefaultCities()
	}

	// Build the card handles WITHOUT holding cardsMu. nativeAddCityCard and the
	// other native calls use dispatch_sync to the main thread; if we held
	// cardsMu across them and the main thread's timer ran a clock/weather
	// closure that also wants cardsMu, we'd deadlock (frozen app). So we do all
	// the blocking native work into a local slice first, then swap it in under
	// the lock in one quick, native-call-free critical section.
	nativeRemoveAllCards(m.container)

	newCards := make([]uintptr, 0, len(cities))
	for _, city := range cities {
		handle := nativeAddCityCard(m.container, city.Name, city.Region)
		if handle == 0 {
			log.Printf("uidarwin: failed to create card for %s", city.Name)
			continue
		}
		nativeSetCardFontSizes(handle, m.fontSizeCityTime, m.fontSizeTempIcon, m.fontSizeConditions)
		nativeSetCardFieldVisibility(handle, displayFieldMask(m.cfg.GetDisplayFields()))
		newCards = append(newCards, handle)
	}

	// Swap in the new cards under the lock — no native/dispatch calls here.
	m.cardsMu.Lock()
	m.cards = newCards
	cardCount := len(m.cards)
	m.cardsMu.Unlock()

	simple := m.viewMode == config.ViewModeSimple
	nativeSetContainerLayout(m.container, simple, cardCount)
	nativeResizeWindowToContainer(m.win, m.container, simple, cardCount)

	// Start clock goroutines.
	m.stopClock = make(chan struct{})
	for i, city := range cities {
		if i >= cardCount {
			break
		}
		go m.runClock(i, city.Timezone, m.stopClock)
	}
}

// stopClocks signals all running clock goroutines to exit.
func (m *manager) stopClocks() {
	if m.stopClock != nil {
		close(m.stopClock)
		m.stopClock = nil
	}
}

// runClock ticks once per second and updates the time/date labels for one card.
func (m *manager) runClock(cardIdx int, timezone string, stop <-chan struct{}) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case t := <-ticker.C:
			now := t.In(loc)
			timeStr := weather.FormatTime(now, timezone, m.lm)
			dateStr := weather.FormatDate(now, timezone, m.lm)
			cardIdx := cardIdx
			mainQueue <- func() {
				m.cardsMu.Lock()
				if cardIdx >= len(m.cards) {
					m.cardsMu.Unlock()
					return
				}
				card := m.cards[cardIdx]
				m.cardsMu.Unlock()
				// Update only time/date labels via a lightweight card update.
				// We pass empty strings for all other fields so the ObjC side
				// skips them (only non-empty strings are applied).
				nativeUpdateCardData(card,
					"", "", timeStr, dateStr,
					"", "", "", "", "", "", "", "",
					false, opacityToAlpha(m.opacity),
				)
			}
		}
	}
}

// ── Weather update ────────────────────────────────────────────────────────────

// handleWeatherUpdate is called on the main queue with fresh results from the
// scheduler.
func (m *manager) handleWeatherUpdate(results []weather.WeatherResult) {
	m.cardsMu.Lock()
	cards := make([]uintptr, len(m.cards))
	copy(cards, m.cards)
	m.cardsMu.Unlock()

	for i, r := range results {
		if i >= len(cards) {
			break
		}
		card := cards[i]
		if r.HasError && !r.IsStale {
			nativeShowCardError(card, true, false)
			continue
		}
		if r.HasError && r.IsStale {
			nativeShowCardError(card, true, true)
		} else {
			nativeShowCardError(card, false, false)
		}
		if r.Data == nil {
			continue
		}
		m.updateCard(card, r.Data)
	}
}

// updateCard pushes fresh WeatherData into one native card view.
func (m *manager) updateCard(card uintptr, d *weather.WeatherData) {
	if d == nil {
		return
	}

	tempUnit := m.cfg.TemperatureUnit
	windUnit := m.cfg.WindSpeedUnit
	iconTheme := m.cfg.IconTheme

	cityStr := fmt.Sprintf("%s, %s", d.CityName, d.Region)
	timeStr := weather.FormatTime(d.LocalTime, "", m.lm)
	dateStr := weather.FormatDate(d.LocalTime, "", m.lm)
	tempStr := weather.FormatTemperature(d.Temperature, tempUnit)
	descStr := weather.FormatDescription(d.Description, m.lm)

	humidDisp := weather.HumidityDisplay(d.Humidity, m.lm)
	windDisp  := weather.WindDisplay(d.WindSpeed, d.WindDirection, windUnit, m.lm)
	gustDisp  := weather.WindGustDisplay(d.WindGust, windUnit, m.lm)
	dewDisp   := weather.DewPointDisplay(d.DewPoint, m.lm)
	pressDisp := weather.PressureDisplay(d.Pressure, m.lm)
	uvDisp    := weather.UVIndexDisplay(d.UVIndex, m.lm)

	isNight := weather.IsNight(d.LocalTime)
	iconCode := weather.MapConditionToIconWithTheme(d.IconCode, d.LocalTime, iconTheme)
	iconPath := resolveIconPath(iconCode)

	nativeUpdateCardData(card,
		iconPath, cityStr, timeStr, dateStr, tempStr, descStr,
		humidDisp.Value, windDisp.Value, gustDisp.Value,
		dewDisp.Value, pressDisp.Value, uvDisp.Value,
		isNight, opacityToAlpha(m.opacity),
	)

	// Air-quality / pollution rows.
	pf := m.cfg.GetPollutionFields()
	pd := weather.PollutionOf(d)
	rows := weather.PlanPollutionRows(pf, pd)

	// AQI.
	aqiLabel := ""
	for _, row := range rows {
		if row.Metric == weather.MetricAQI {
			aqiLabel = row.ValueText
			break
		}
	}
	nativeUpdateCardAQI(card, aqiLabel)

	// Pollutant slots: CO=0 NO=1 NO2=2 O3=3 SO2=4 NH3=5 PM25=6 PM10=7
	slotMap := map[weather.PollutionMetric]int{
		weather.MetricCO: 0, weather.MetricNO: 1,
		weather.MetricNO2: 2, weather.MetricO3: 3,
		weather.MetricSO2: 4, weather.MetricNH3: 5,
		weather.MetricPM25: 6, weather.MetricPM10: 7,
	}
	// First clear all slots, then populate visible ones.
	for slot := 0; slot < 8; slot++ {
		nativeUpdateCardPollutant(card, slot, "", "")
	}
	for _, row := range rows {
		if row.Metric == weather.MetricAQI {
			continue
		}
		slot, ok := slotMap[row.Metric]
		if !ok {
			continue
		}
		iconPath := resolveAirIconPath(row.IconFile)
		nativeUpdateCardPollutant(card, slot, iconPath, row.ValueText)
	}
}

// ── Position management ───────────────────────────────────────────────────────

// applyPosition moves the window to the configured custom coordinates or
// computes it from the corner setting.
func (m *manager) applyPosition() {
	if m.cfg.CustomX != nil && m.cfg.CustomY != nil {
		x, y := *m.cfg.CustomX, *m.cfg.CustomY
		log.Printf("uidarwin: restoring position (%d, %d)", x, y)
		nativeMoveWidgetWindow(m.win, x, y)
		return
	}
	x, y := m.cornerToXY(m.cfg.CornerPosition, m.cfg.MonitorIndex)
	log.Printf("uidarwin: corner %s → (%d, %d)", m.cfg.CornerPosition, x, y)
	nativeMoveWidgetWindow(m.win, x, y)
}

// cornerToXY computes the top-left screen position for the given corner and
// monitor index.  Window dimensions are read from the current container size.
func (m *manager) cornerToXY(corner string, monitorIndex int) (int, int) {
	screenCount := nativeGetScreenCount()
	if monitorIndex < 0 || monitorIndex >= screenCount {
		monitorIndex = 0
	}
	sx, sy, sw, sh := nativeGetScreenBounds(monitorIndex)

	// Estimate window size from card count (mirrors gtk_helpers.go cornerToXY).
	count := len(m.cards)
	if count == 0 {
		count = 1
	}
	const (
		cardW = 628  // kCardWidth + 2*kCardPaddingH
		cardH = 226  // typical enhanced card height
		gap   = 6
	)
	var winW, winH int
	if m.viewMode == config.ViewModeSimple {
		const simpleW = 186 // kSimpleCardWidth + 2*kCardPaddingH
		winW = simpleW*count + gap*(count-1)
		winH = 444
	} else {
		winW = cardW
		winH = cardH*count + gap*(count-1)
	}
	const margin = 8

	switch corner {
	case "top-left":
		return sx + margin, sy + margin
	case "top-right":
		return sx + sw - winW - margin, sy + margin
	case "bottom-left":
		return sx + margin, sy + sh - winH - margin
	default: // bottom-right
		return sx + sw - winW - margin, sy + sh - winH - margin
	}
}

// pollDragPosition monitors the window position every 500 ms.
// When the position changes without a programmatic move, it treats the change
// as a user drag and persists the new coordinates (mirrors drag_darwin.go).
var (
	dragLastX, dragLastY int
	dragMovedByUs        bool
	dragMovedByUsMu      sync.Mutex
)

func notifyMovedByUs() {
	dragMovedByUsMu.Lock()
	dragMovedByUs = true
	dragMovedByUsMu.Unlock()
}

func (m *manager) pollDragPosition() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	dragLastX, dragLastY = nativeGetWidgetWindowPos(m.win)

	for range ticker.C {
		x, y := nativeGetWidgetWindowPos(m.win)

		dragMovedByUsMu.Lock()
		movedByUs := dragMovedByUs
		dragMovedByUs = false
		dragMovedByUsMu.Unlock()

		if !movedByUs && m.positioned && (x != dragLastX || y != dragLastY) {
			// User dragged the window.
			cx, cy := x, y
			mainQueue <- func() {
				m.cfg.CustomX = &cx
				m.cfg.CustomY = &cy
				go func() {
					if err := m.cfgSvc.Save(m.cfg); err != nil {
						log.Printf("uidarwin: drag position save failed: %v", err)
					} else {
						log.Printf("uidarwin: drag position saved (%d, %d)", cx, cy)
					}
				}()
			}
		}
		dragLastX, dragLastY = x, y
	}
}

// ── Settings ─────────────────────────────────────────────────────────────────

func (m *manager) openSettings() {
	openSettingsWindow(m)
}

// onSettingsSave persists a new config and rebuilds the UI as needed.
// It mirrors ui-gtk/manager.go onSettingsSave exactly.
func (m *manager) onSettingsSave(newCfg *config.Config) error {
	if !newCfg.HasLicense() {
		newCfg.Cities = config.DefaultCities()
	}

	oldCfg := m.cfg
	providerChanged := oldCfg.DataSource != newCfg.DataSource ||
		providerConfigChanged(oldCfg, newCfg)

	if providerChanged && newCfg.HasLicense() {
		p := m.buildProvider(newCfg)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := p.TestConnection(ctx); err != nil {
			return fmt.Errorf("connection test failed: %w", err)
		}
		m.weatherSvc.SwitchProvider(p)
	} else if providerChanged {
		m.weatherSvc.SwitchProvider(m.buildProvider(newCfg))
	}

	if err := m.cfgSvc.Save(newCfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	// Compute what changed BEFORE mutating m.cfg. All mutations of manager
	// fields that the Cocoa main thread also reads (m.cfg, m.viewMode,
	// m.opacity, m.fontSize*, m.noBackground) are deferred into the mainQueue
	// closure below so they happen on the main thread. onSettingsSave itself
	// runs on a worker goroutine (see settingsSaveCB → go fn(...)); writing
	// those fields here would race the clock/weather closures reading them and
	// can freeze or corrupt the widget when the view mode changes.
	localeChanged := oldCfg.Locale != newCfg.Locale
	viewModeChanged := config.NormalizeViewMode(oldCfg.ViewMode) !=
		config.NormalizeViewMode(newCfg.ViewMode)
	citiesChanged := !sameCities(oldCfg.Cities, newCfg.Cities)

	opacity := newCfg.Opacity
	if opacity == 0 {
		opacity = 100
	}

	// Apply all shared-state changes and the UI rebuild on the main thread.
	mainQueue <- func() {
		m.cfg = newCfg
		m.opacity = opacity
		m.noBackground = newCfg.NoBackground
		m.fontSizeCityTime = newCfg.GetFontSizeCityTime()
		m.fontSizeTempIcon = newCfg.GetFontSizeTempIcon()
		m.fontSizeConditions = newCfg.GetFontSizeConditions()
		m.viewMode = config.NormalizeViewMode(newCfg.ViewMode)

		if localeChanged && m.lm != nil {
			_ = m.lm.SetLocale(newCfg.Locale)
			updateTrayMenu(m)
		}

		nativeSetWidgetWindowOpacity(m.win, opacityToAlpha(m.opacity))

		if citiesChanged || viewModeChanged || localeChanged {
			// Full rebuild: new card views for the new city list / layout.
			m.buildCards(newCfg.Cities)
		} else {
			// Soft update: apply field visibility and font sizes in place.
			m.cardsMu.Lock()
			cards := make([]uintptr, len(m.cards))
			copy(cards, m.cards)
			m.cardsMu.Unlock()
			for _, card := range cards {
				nativeSetCardFieldVisibility(card, displayFieldMask(newCfg.GetDisplayFields()))
				nativeSetCardFontSizes(card, m.fontSizeCityTime, m.fontSizeTempIcon, m.fontSizeConditions)
			}
		}
		m.applyPosition()
	}

	m.sched.SetInterval(time.Duration(newCfg.RefreshInterval) * time.Minute)
	m.sched.SetCities(newCfg.Cities)
	m.sched.FetchNow()
	return nil
}

// ── Shutdown ──────────────────────────────────────────────────────────────────

func (m *manager) shutdown() {
	log.Println("uidarwin: shutting down")
	if m.sched != nil {
		m.sched.Stop()
	}
	m.stopClocks()
	if m.guard != nil {
		_ = m.guard.Release()
	}
	os.Exit(0)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// t returns the translated string for key, falling back to key itself.
func (m *manager) t(key string) string {
	if m.lm != nil {
		return m.lm.T(key)
	}
	return key
}

// buildProvider creates a WeatherProvider from config.
func (m *manager) buildProvider(cfg *config.Config) weather.WeatherProvider {
	provider := "easyweatherwidget"
	apiKey := ""
	if cfg.APIConfig != nil {
		if cfg.APIConfig.Provider != "" {
			provider = cfg.APIConfig.Provider
		}
		apiKey = cfg.APIConfig.APIKey
	}
	if apiKey == "" {
		provider = "easyweatherwidget"
	}
	return remoteapi.NewRemoteAPIAdapter(provider, apiKey)
}

// opacityToAlpha maps the user-facing opacity percent (25–100) to an NSWindow
// alphaValue (0.55–1.0) so the widget is always legible.
func opacityToAlpha(pct int) float64 {
	if pct >= 100 {
		return 1.0
	}
	if pct <= 25 {
		return 0.55
	}
	return 0.55 + float64(pct-25)*(0.45/75.0)
}

// displayFieldMask converts a DisplayFields struct into the bitmask the ObjC
// panel layer uses (must match the DFxxx constants in bridge.go / panel.m).
func displayFieldMask(df *config.DisplayFields) uint32 {
	if df == nil {
		return 0xFFFFFFFF // all visible
	}
	var mask uint32
	if df.ShowCity     { mask |= DFCity }
	if df.ShowIcon     { mask |= DFIcon }
	if df.ShowTemp     { mask |= DFTemp }
	if df.ShowDesc     { mask |= DFDesc }
	if df.ShowHumidity { mask |= DFHumidity }
	if df.ShowWind     { mask |= DFWind }
	if df.ShowTime     { mask |= DFTime }
	if df.ShowDate     { mask |= DFDate }
	if df.ShowWindGust { mask |= DFWindGust }
	if df.ShowDewPoint { mask |= DFDewPoint }
	if df.ShowPressure { mask |= DFPressure }
	if df.ShowUVIndex  { mask |= DFUVIndex }
	return mask
}

// resolveIconPath returns the absolute filesystem path for a weather-icon
// asset code (e.g. "day/clear_day").
// Icons are embedded in the binary via assets.Icons; we extract them to a
// temp dir on first use so the ObjC NSImageView can load them by path.
func resolveIconPath(code string) string {
	return resolveEmbeddedAsset("icons/"+code+".png", assets.Icons)
}

// resolveAirIconPath returns the absolute path for an air-quality icon file
// (e.g. "co.png").
func resolveAirIconPath(file string) string {
	if file == "" {
		return ""
	}
	return resolveEmbeddedAsset("air/"+file, assets.AirIcons)
}




var (
	assetCacheDir  string
	assetCacheOnce sync.Once
	assetCacheMu   sync.Mutex
	assetCacheMap  = map[string]string{}
)

// resolveEmbeddedAsset extracts an embedded file to a temp directory and
// returns its absolute path.  The extraction is cached so each asset is only
// written to disk once per process lifetime.
func resolveEmbeddedAsset(assetPath string, fsys interface{ ReadFile(string) ([]byte, error) }) string {
	assetCacheOnce.Do(func() {
		dir, err := os.MkdirTemp("", "weatherwidget-assets-*")
		if err != nil {
			log.Printf("uidarwin: failed to create asset cache dir: %v", err)
			dir = os.TempDir()
		}
		assetCacheDir = dir
	})

	assetCacheMu.Lock()
	if cached, ok := assetCacheMap[assetPath]; ok {
		assetCacheMu.Unlock()
		return cached
	}
	assetCacheMu.Unlock()

	data, err := fsys.ReadFile(assetPath)
	if err != nil {
		log.Printf("uidarwin: asset not found: %s: %v", assetPath, err)
		return ""
	}

	// Create the full directory tree for the destination file, not just the subDir root.
	destPath := filepath.Join(assetCacheDir, assetPath)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return ""
	}
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		log.Printf("uidarwin: failed to write asset %s: %v", assetPath, err)
		return ""
	}

	assetCacheMu.Lock()
	assetCacheMap[assetPath] = destPath
	assetCacheMu.Unlock()
	return destPath
}

// sameCities returns true when both slices contain the same cities in order.
func sameCities(a, b []config.CityConfig) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Region != b[i].Region {
			return false
		}
	}
	return true
}

// providerConfigChanged returns true when API credentials differ between configs.
func providerConfigChanged(old, new *config.Config) bool {
	if old.APIConfig == nil || new.APIConfig == nil {
		return old.APIConfig != new.APIConfig
	}
	return old.APIConfig.Provider != new.APIConfig.Provider ||
		old.APIConfig.APIKey != new.APIConfig.APIKey
}
