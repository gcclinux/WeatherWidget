//go:build linux

// Package uitk implements a native GTK3 UI for WeatherWidget on Linux.
// It reuses all business-logic packages (config, weather, scheduler, i18n, etc.)
// and provides true per-widget background transparency via GTK's RGBA visual.
package uitk

import (
	"context"
	"fmt"
	"log"
	"os"
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

// isBugCondition returns true when the runtime environment matches the
// confirmed XWayland window-positioning bug trigger:
//   - GDK is forced to the X11 backend (GDK_BACKEND=x11 or x11 is active), AND
//   - the desktop session is actually Wayland (WAYLAND_DISPLAY is set or
//     XDG_SESSION_TYPE == "wayland"), AND
//   - a custom window position is configured (customX / customY are non-nil).
//
// When this condition holds, all three X11 positioning mechanisms used by the
// app (WM_NORMAL_HINTS USPosition, _NET_MOVERESIZE_WINDOW, gtk.Window.Move via
// XWayland) are silently discarded by GNOME Mutter, and the window lands at
// the compositor-chosen position (0, 0) instead of the configured coordinates.
//
// This function is package-level so it can be called from tests without
// constructing a full manager.
func isBugCondition(gdkBackend, waylandDisplay, xdgSessionType string, customX, customY *int) bool {
	backendIsX11 := gdkBackend == "x11"
	sessionIsWayland := waylandDisplay != "" || xdgSessionType == "wayland"
	hasCustomPosition := customX != nil && customY != nil
	return backendIsX11 && sessionIsWayland && hasCustomPosition
}

// isWayland returns true when the current desktop session is a Wayland session.
// It checks the two canonical environment variables:
//   - WAYLAND_DISPLAY: set by the Wayland compositor (e.g. wayland-0)
//   - XDG_SESSION_TYPE: set by the login manager to "wayland" on Wayland sessions
//
// This is the runtime guard that separates the Wayland and X11 positioning
// paths in buildWindow(): when true, X11-only hints are suppressed and GTK's
// native Wayland backend handles positioning via win.Move().
func isWayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland"
}

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
	// Set GDK_BACKEND=x11 via C's setenv before GTK reads the environment.
	// This ensures reliable window positioning via XWayland on Wayland desktops.
	ensureGDKBackendX11()

	log.Printf("GDK_BACKEND=%s", os.Getenv("GDK_BACKEND"))

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

	noBackground bool // whether panels show without background
	noBorder     bool // whether window decorations are hidden
	opacity      int  // 25 / 50 / 75 / 100
	css          string // current CSS applied to the window

	// positioned is set to true once the initial position has been applied.
	// The drag auto-save is suppressed until this flag is set, preventing
	// GNOME Mutter's WM-initiated configure-event (reporting 0,0) from
	// overwriting the saved coordinates during startup.
	positioned bool
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
	log.Printf("config path: %s", m.cfgSvc.ConfigPath())
	cfg, err := m.cfgSvc.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}
	m.cfg = cfg
	if cfg.CustomX != nil && cfg.CustomY != nil {
		log.Printf("config loaded: customX=%d customY=%d", *cfg.CustomX, *cfg.CustomY)
	} else {
		log.Printf("config loaded: no customX/customY (will use corner: %s)", cfg.CornerPosition)
	}
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
	win.SetKeepBelow(true) // stay behind all normal windows
	win.SetResizable(false)
	win.SetAppPaintable(true)

	// Use NORMAL type — UTILITY and DIALOG types cause Mutter to own placement.
	// Positioning is achieved through USPosition hints + repeated XMoveWindow calls.
	win.SetTypeHint(gdk.WINDOW_TYPE_HINT_NORMAL)

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

	// --- Window positioning ---
	//
	// GDK_BACKEND=x11 is always set (via main.go or snap environment).
	// XMoveWindow works for native builds (deb/rpm/appimage) where the app
	// connects directly to XWayland. For snaps, the X11 portal blocks
	// XMoveWindow, so positioning is done via Settings → Display controls.
	//
	// The Settings position controls work for ALL environments.

	// Compute target position once.
	var posX, posY int
	if m.cfg.CustomX != nil && m.cfg.CustomY != nil {
		posX, posY = *m.cfg.CustomX, *m.cfg.CustomY
	} else {
		pw, ph := m.panelSize()
		posX, posY = cornerToXY(m.cfg.CornerPosition, m.cfg.MonitorIndex, pw, ph)
	}

	// Set event mask before realize.
	win.SetEvents(int(gdk.BUTTON_PRESS_MASK | gdk.BUTTON_RELEASE_MASK | gdk.POINTER_MOTION_MASK))

	// Phase 1: realize → X11 hints → move → show.
	win.Realize()
	x11SetPositionHint(win, posX, posY)
	x11MoveWindow(win, posX, posY)
	m.applyPosition()

	// Phase 2: after map, force position with repeated XMoveWindow.
	win.Connect("map-event", func(_ *gtk.Window, _ *gdk.Event) bool {
		x11MoveWindow(m.win, posX, posY)
		glib.TimeoutAdd(100, func() bool {
			x11MoveWindow(m.win, posX, posY)
			return false
		})
		glib.TimeoutAdd(400, func() bool {
			x11MoveWindow(m.win, posX, posY)
			x11NetMoveWindow(m.win, posX, posY)
			return false
		})
		glib.TimeoutAdd(1000, func() bool {
			x11MoveWindow(m.win, posX, posY)
			m.positioned = true
			return false
		})
		return false
	})

	// Drag-to-reposition: left-click drag moves the window and saves position.
	moveFunc := func(x, y int) {
		win.Move(x, y)
		x11MoveWindow(win, x, y)
	}

	var saveTimer *time.Timer
	enableDrag(win, moveFunc, func(x, y int) {
		if !m.positioned {
			return
		}
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
	// Reset positioned so the new window's map-event handler re-applies the
	// delayed Move() correctly before re-enabling drag auto-save.
	m.positioned = false
	if err := m.buildWindow(); err != nil {
		log.Printf("failed to rebuild window: %v", err)
	}
	// Position is applied via the map-event handler in buildWindow().
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
