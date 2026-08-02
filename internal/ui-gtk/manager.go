//go:build linux

// Package uitk implements a native GTK3 UI for WeatherWidget on Linux.
// It reuses all business-logic packages (config, weather, scheduler, i18n, etc.)
// and provides true per-widget background transparency via GTK's RGBA visual.
package uitk

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"weatherwidget/internal/config"
	"weatherwidget/internal/guard"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/power"
	"weatherwidget/internal/scheduler"
	"weatherwidget/internal/weather"
	"weatherwidget/internal/weather/remoteapi"
)

// t returns the translated string for the given i18n key.
// Falls back to the key itself if no locale manager is set.
func (m *manager) t(key string) string {
	if m.lm != nil {
		return m.lm.T(key)
	}
	return key
}

// tFmt returns a translated string formatted with fmt.Sprintf-style args.
func (m *manager) tFmt(key string, args ...interface{}) string {
	tmpl := m.t(key)
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}

// Run initialises GTK, sets up all application components, and starts the
// GTK main loop. It does not return until the application exits.
func Run(appDataDir string, openSettings bool) {
	gtk.Init(nil)

	m := newManager(appDataDir)
	if err := m.start(openSettings); err != nil {
		log.Fatalf("weatherwidget-gtk: %v", err)
	}

	gtk.Main()
}

// manager wires together all application components: config, weather, i18n,
// scheduler, UI windows, and the system tray.
type manager struct {
	appDataDir string
	cfgSvc     *config.ConfigService
	cfg        *config.Config
	lm         *i18n.LocaleManager
	weather    *weather.WeatherService
	sched      *scheduler.RefreshScheduler
	guard      *guard.SingleInstanceGuard

	win    *gtk.Window // main transparent widget window
	panels []*cityPanel

	noBackground bool   // whether panels show without background
	noBorder     bool   // whether window decorations are hidden
	opacity      int    // 25 / 50 / 75 / 100
	css          string // current CSS applied to the window
}

func newManager(appDataDir string) *manager {
	return &manager{appDataDir: appDataDir}
}

func (m *manager) start(openSettings bool) error {
	// Single-instance guard.
	g, err := guard.NewSingleInstanceGuard("WeatherWidget-GTK")
	if err != nil {
		return err
	}
	m.guard = g

	// Load config.
	m.cfgSvc = config.NewConfigService(m.appDataDir)
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
	m.noBorder = cfg.NoBorder

	// Locale manager.
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		log.Printf("locale manager error: %v", err)
	} else {
		if cfg.Locale != "" {
			_ = lm.SetLocale(cfg.Locale)
		}
		m.lm = lm
	}

	// Weather provider + service.
	provider := m.buildProvider(cfg)
	m.weather = weather.NewWeatherService(provider)

	// Build main widget window.
	if err := m.buildWindow(); err != nil {
		return err
	}
	// Position is applied via "map-event" after the window is shown by the WM.

	// Setup system tray (best-effort).
	setupTray(m)

	if openSettings {
		m.openSettings()
	}

	// Scheduler.
	interval := time.Duration(cfg.RefreshInterval) * time.Minute
	m.sched = scheduler.NewRefreshScheduler(interval, m.weather)
	m.sched.SetCities(cfg.Cities)
	m.sched.SetOnUpdate(func(results []weather.WeatherResult) {
		glib.IdleAdd(func() { m.handleWeatherUpdate(results) })
	})
	m.sched.SetOnError(func(city string, err error) {
		log.Printf("weather error for %s: %v", city, err)
	})
	m.sched.Start()

	// Listen for power-resume events.
	go func() {
		for range power.ResumeNotifier() {
			log.Println("system resume — triggering weather refresh")
			m.sched.FetchNow()
		}
	}()

	return nil
}

// buildWindow creates the transparent borderless main window and populates it
// with one cityPanel per configured city.
func (m *manager) buildWindow() error {
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return err
	}
	m.win = win

	win.SetTitle("WeatherWidget")
	win.SetDecorated(false)
	win.SetSkipTaskbarHint(true)
	win.SetSkipPagerHint(true)
	win.SetKeepBelow(true)   // stay behind all normal windows
	win.SetResizable(false)
	win.SetAppPaintable(true)

	// UTILITY type: no taskbar entry, no decoration request, but unlike DOCK
	// it respects SetKeepBelow so the widget stays behind normal windows.
	// DOCK forces always-on-top which is the opposite of what we want.
	win.SetTypeHint(gdk.WINDOW_TYPE_HINT_UTILITY)

	// Remove title bar decorations if the user enabled no-border mode.
	if m.noBorder {
		removeDecorations(win)
	}

	// CSS provider — sets transparent window background and panel styles.
	m.applyCSS()

	// City panels in a horizontal box.
	hbox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	if err != nil {
		return err
	}
	hbox.SetName("panelbox")

	cities := m.cfg.Cities
	if len(cities) == 0 {
		cities = config.DefaultCities()
	}
	m.panels = make([]*cityPanel, 0, len(cities))
	for _, city := range cities {
		p, err := newCityPanel(city.Name, city.Region, city.Timezone, m.lm)
		if err != nil {
			log.Printf("failed to create panel for %s: %v", city.Name, err)
			continue
		}
		p.setNoBackground(m.noBackground)
		p.applyDisplayFields(m.cfg.GetDisplayFields())
		hbox.PackStart(p.root, false, false, 0)
		m.panels = append(m.panels, p)
	}

	win.Add(hbox)

	// Connect draw signal to paint a transparent background.
	// gotk3 resolves the instance to its most specific Go type (*gtk.Window),
	// so the callback's first arg must be *gtk.Window, not *gtk.Widget.
	win.Connect("draw", func(w *gtk.Window, cr *cairoContext) {
		paintTransparent(cr)
	})

	// Apply saved position and reinforce keep-below after the WM maps the window.
	// win.Move() before ShowAll() is overridden by WM initial placement.
	win.Connect("map-event", func(_ *gtk.Window, _ *gdk.Event) bool {
		m.applyPosition()
		m.win.SetKeepBelow(true) // reinforce — some WMs reset this on map
		return false
	})

	// Drag-to-reposition via left-click drag anywhere on the window.
	// Auto-saves position 300ms after the last move — no Save button needed.
	win.SetEvents(int(gdk.BUTTON_PRESS_MASK | gdk.BUTTON_RELEASE_MASK | gdk.POINTER_MOTION_MASK))
	var saveTimer *time.Timer
	enableDrag(win, func(x, y int) {
		cx, cy := x, y
		m.cfg.CustomX = &cx
		m.cfg.CustomY = &cy
		if saveTimer != nil {
			saveTimer.Stop()
		}
		saveTimer = time.AfterFunc(300*time.Millisecond, func() {
			if err := m.cfgSvc.Save(m.cfg); err != nil {
				log.Printf("failed to save position (%d,%d): %v", cx, cy, err)
			} else {
				log.Printf("position auto-saved: (%d,%d)", cx, cy)
			}
		})
	})

	win.ShowAll()
	return nil
}

// applyCSS loads and applies the GTK CSS that controls panel appearance.
func (m *manager) applyCSS() {
	css := buildCSS(m.opacity, m.noBackground)
	applyCSSToScreen(css)
}

// applyPosition moves the widget window to the configured position.
// On X11/XWayland, win.Move() works correctly after the window is mapped.
func (m *manager) applyPosition() {
	var x, y int
	if m.cfg.CustomX != nil && m.cfg.CustomY != nil {
		x, y = *m.cfg.CustomX, *m.cfg.CustomY
		log.Printf("restoring position to saved coordinates (%d,%d)", x, y)
	} else {
		pw, ph := m.panelSize()
		x, y = cornerToXY(m.cfg.CornerPosition, m.cfg.MonitorIndex, pw, ph)
		log.Printf("positioning to corner %s: (%d,%d)", m.cfg.CornerPosition, x, y)
	}
	m.win.Move(x, y)
}

// panelSize returns the total (width, height) of the current panels.
func (m *manager) panelSize() (int, int) {
	count := len(m.panels)
	if count == 0 {
		count = 1
	}
	// Each panel is approximately 160px wide, 220px tall.
	return count*160 + (count-1)*4, 220
}

// handleWeatherUpdate updates each panel with fresh weather data.
func (m *manager) handleWeatherUpdate(results []weather.WeatherResult) {
	for i, p := range m.panels {
		if i >= len(results) {
			break
		}
		r := results[i]
		if r.Data != nil {
			p.update(r.Data, m.cfg.TemperatureUnit)
		} else if r.HasError {
			p.showError(r.IsStale)
		}
	}
}

// openSettings opens the GTK settings dialog.
func (m *manager) openSettings() {
	showSettingsDialog(m)
}

// SetOpacity updates the opacity level and refreshes CSS.
func (m *manager) SetOpacity(pct int) {
	m.opacity = pct
	m.applyCSS()
}

// SetNoBackground toggles background-removal mode and refreshes CSS.
func (m *manager) SetNoBackground(enable bool) {
	m.noBackground = enable
	for _, p := range m.panels {
		p.setNoBackground(enable)
	}
	m.applyCSS()
}

// SetNoBorder toggles window decorations. Because the GTK titlebar override
// must be applied before the window is shown, changing this setting requires
// a window rebuild. The window is destroyed and recreated with the new setting.
func (m *manager) SetNoBorder(enable bool) {
	if m.noBorder == enable {
		return
	}
	m.noBorder = enable
	// Rebuild the window to apply the titlebar change.
	m.rebuildPanels(m.cfg.Cities)
}

// rebuildPanels destroys existing panels and creates new ones from config.
func (m *manager) rebuildPanels(cities []config.CityConfig) {
	// Destroy old window and rebuild.
	if m.win != nil {
		m.win.Destroy()
		m.win = nil
	}
	m.panels = nil
	if err := m.buildWindow(); err != nil {
		log.Printf("failed to rebuild window: %v", err)
	}
	m.applyPosition()
}

// onSettingsSave persists the new config and updates the UI.
func (m *manager) onSettingsSave(newCfg *config.Config) error {
	if !newCfg.HasLicense() {
		newCfg.Cities = config.DefaultCities()
	}

	oldCfg := m.cfg
	providerChanged := oldCfg.DataSource != newCfg.DataSource || providerConfigChanged(oldCfg, newCfg)

	if providerChanged && newCfg.HasLicense() {
		p := m.buildProvider(newCfg)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := p.TestConnection(ctx); err != nil {
			return err
		}
		m.weather.SwitchProvider(p)
	} else if providerChanged {
		m.weather.SwitchProvider(m.buildProvider(newCfg))
	}

	if err := m.cfgSvc.Save(newCfg); err != nil {
		return err
	}
	m.cfg = newCfg

	if m.lm != nil && oldCfg.Locale != newCfg.Locale {
		_ = m.lm.SetLocale(newCfg.Locale)
	}

	newOpacity := newCfg.Opacity
	if newOpacity == 0 {
		newOpacity = 100
	}
	m.opacity = newOpacity

	citiesChanged := len(oldCfg.Cities) != len(newCfg.Cities) || !sameCities(oldCfg.Cities, newCfg.Cities)
	if citiesChanged {
		m.rebuildPanels(newCfg.Cities)
	} else {
		m.applyCSS()
		m.applyPosition()
		// Apply display field changes to existing panels.
		for _, p := range m.panels {
			p.applyDisplayFields(newCfg.GetDisplayFields())
		}
	}

	m.sched.SetInterval(time.Duration(newCfg.RefreshInterval) * time.Minute)
	m.sched.SetCities(newCfg.Cities)
	m.sched.FetchNow()
	return nil
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

// sameCities checks whether two city slices are identical.
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

// providerConfigChanged checks whether provider credentials changed.
func providerConfigChanged(old, new *config.Config) bool {
	if old.APIConfig == nil || new.APIConfig == nil {
		return old.APIConfig != new.APIConfig
	}
	return old.APIConfig.Provider != new.APIConfig.Provider ||
		old.APIConfig.APIKey != new.APIConfig.APIKey
}
