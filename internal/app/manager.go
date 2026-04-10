// Package app provides the application orchestrator that wires all components together.
package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"fyne.io/fyne/v2"

	"weatherwidget/internal/config"
	"weatherwidget/internal/guard"
	"weatherwidget/internal/scheduler"
	"weatherwidget/internal/ui"
	"weatherwidget/internal/weather"
	"weatherwidget/internal/weather/database"
	"weatherwidget/internal/weather/remoteapi"
)

// AppManager orchestrates all application components: configuration, weather
// data fetching, scheduling, UI, and single-instance guard.
type AppManager struct {
	app        fyne.App
	appDataDir string
	config     *config.ConfigService
	weather    *weather.WeatherService
	scheduler  *scheduler.RefreshScheduler
	ui         *ui.UIManager
	guard      *guard.SingleInstanceGuard
	cfg        *config.Config // current loaded config
	dbAdapter  *database.DatabaseAdapter
}

// NewAppManager creates an AppManager with the given Fyne app and data directory.
func NewAppManager(app fyne.App, appDataDir string) *AppManager {
	return &AppManager{
		app:        app,
		appDataDir: appDataDir,
	}
}

// Run initialises all components and starts the application.
// It acquires the single-instance guard, loads config, creates the UI,
// weather service, and scheduler, then shows the widget.
// The caller is responsible for running the Fyne event loop (app.Run()).
func (a *AppManager) Run() error {
	// 1. Acquire single-instance guard.
	g, err := guard.NewSingleInstanceGuard("WeatherWidget")
	if err != nil {
		return fmt.Errorf("single instance check: %w", err)
	}
	a.guard = g

	// 2. Load configuration.
	a.config = config.NewConfigService(a.appDataDir)
	cfg, err := a.config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}
	a.cfg = cfg

	// 3. Create UI manager.
	a.ui = ui.NewUIManager(a.app)

	// 4. Setup context menu and system tray.
	a.ui.SetupContextMenu(
		func() { a.openSettings() },
		func(pos string) { a.saveCornerPosition(pos) },
		func() { a.Shutdown() },
	)
	a.ui.SetupSystemTray(
		func() { a.openSettings() },
		func() { a.Shutdown() },
	)

	// 5. If config is default (no API key / no DB config), open settings.
	if a.isDefaultConfig(cfg) {
		a.openSettings()
	}

	// 6. Create weather provider and service.
	provider := a.createProvider(cfg)
	a.weather = weather.NewWeatherService(provider)

	// 7. Show widget and apply Win32 styles.
	a.ui.ShowWidget(cfg.Cities)
	a.ui.ApplyWin32Styles()
	a.ui.SetCorner(cfg.CornerPosition)
	log.Printf("WeatherWidget window shown at %s", cfg.CornerPosition)

	// 8. Start clocks for each panel.
	a.startPanelClocks(cfg.Cities)

	// 9. Create and start refresh scheduler.
	interval := time.Duration(cfg.RefreshInterval) * time.Minute
	a.scheduler = scheduler.NewRefreshScheduler(interval, a.weather)
	a.scheduler.SetCities(cfg.Cities)
	a.scheduler.SetOnUpdate(func(results []weather.WeatherResult) {
		a.handleWeatherUpdate(results)
	})
	a.scheduler.SetOnError(func(city string, err error) {
		log.Printf("weather fetch error for %s: %v", city, err)
	})
	a.scheduler.Start()

	return nil
}

// Shutdown performs a graceful shutdown: stops the scheduler, stops panel
// clocks, closes the database adapter if active, and releases the instance
// guard. A 2-second safety net forces os.Exit(1) if cleanup stalls.
func (a *AppManager) Shutdown() {
	// Safety net: force exit after 2 seconds.
	time.AfterFunc(2*time.Second, func() {
		os.Exit(1)
	})

	if a.scheduler != nil {
		a.scheduler.Stop()
	}

	a.stopPanelClocks()

	if a.dbAdapter != nil {
		a.dbAdapter.Close()
		a.dbAdapter = nil
	}

	if a.guard != nil {
		_ = a.guard.Release()
	}

	a.app.Quit()
}

// openSettings opens the settings page with the current config.
func (a *AppManager) openSettings() {
	a.ui.ShowSettings(a.cfg, a.onSettingsSave)
}

// saveCornerPosition updates the in-memory config with the new corner position
// and persists it to disk. Called when the user picks a position from the
// right-click context menu.
func (a *AppManager) saveCornerPosition(pos string) {
	a.cfg.CornerPosition = pos
	if err := a.config.Save(a.cfg); err != nil {
		log.Printf("failed to save corner position %q: %v", pos, err)
	} else {
		log.Printf("corner position saved: %s", pos)
	}
}

// onSettingsSave is the callback invoked when the user saves settings.
// It persists the config, switches the weather provider only when credentials
// changed (running a connection test first), resets the scheduler interval,
// and rebuilds city panels if the city list changed.
func (a *AppManager) onSettingsSave(newCfg *config.Config) error {
	oldCfg := a.cfg
	providerChanged := oldCfg.DataSource != newCfg.DataSource || a.providerConfigChanged(oldCfg, newCfg)

	// Only hit the network when the data source or credentials actually changed.
	// Skipping this for position / interval / city-order changes avoids blocking
	// the save on a network round-trip that cannot affect those fields.
	var provider weather.WeatherProvider
	if providerChanged {
		provider = a.createProvider(newCfg)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := provider.TestConnection(ctx); err != nil {
			return fmt.Errorf("connection test failed: %w", err)
		}
	}

	// Persist config — always, regardless of what changed.
	if err := a.config.Save(newCfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	a.cfg = newCfg

	// Switch weather provider if data source / credentials changed.
	if providerChanged {
		// Close old database adapter if switching away from database.
		if a.dbAdapter != nil {
			a.dbAdapter.Close()
			a.dbAdapter = nil
		}
		a.weather.SwitchProvider(provider)
	}

	// Reset scheduler interval.
	a.scheduler.SetInterval(time.Duration(newCfg.RefreshInterval) * time.Minute)
	a.scheduler.SetCities(newCfg.Cities)

	// Rebuild city panels if city list changed.
	if len(oldCfg.Cities) != len(newCfg.Cities) || !sameCities(oldCfg.Cities, newCfg.Cities) {
		a.stopPanelClocks()
		a.ui.ShowWidget(newCfg.Cities)
		a.ui.ApplyWin32Styles()
		a.startPanelClocks(newCfg.Cities)
	}

	// Always reposition the widget window (position may have changed independently).
	a.ui.SetCorner(newCfg.CornerPosition)

	return nil
}

// createProvider builds a WeatherProvider based on the given config.
func (a *AppManager) createProvider(cfg *config.Config) weather.WeatherProvider {
	switch cfg.DataSource {
	case config.DataSourceLocalDatabase:
		if cfg.DatabaseConfig == nil {
			// Fallback: return a remote adapter with empty key (will fail on fetch).
			return remoteapi.NewRemoteAPIAdapter("openweathermap", "")
		}
		connStr := fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s",
			cfg.DatabaseConfig.Username,
			cfg.DatabaseConfig.Password,
			cfg.DatabaseConfig.Host,
			cfg.DatabaseConfig.Port,
			cfg.DatabaseConfig.DBName,
		)
		adapter, err := database.NewDatabaseAdapter(connStr, cfg.DatabaseConfig.Query)
		if err != nil {
			log.Printf("failed to create database adapter: %v, falling back to remote API", err)
			return remoteapi.NewRemoteAPIAdapter("openweathermap", "")
		}
		// Close any previous database adapter.
		if a.dbAdapter != nil {
			a.dbAdapter.Close()
		}
		a.dbAdapter = adapter
		return adapter

	default: // DataSourceRemoteAPI
		provider := "openweathermap"
		apiKey := ""
		if cfg.APIConfig != nil {
			provider = cfg.APIConfig.Provider
			apiKey = cfg.APIConfig.APIKey
		}
		return remoteapi.NewRemoteAPIAdapter(provider, apiKey)
	}
}

// handleWeatherUpdate processes weather results from the scheduler and
// updates the UI panels accordingly.
func (a *AppManager) handleWeatherUpdate(results []weather.WeatherResult) {
	panels := a.ui.Panels()
	log.Printf("handling weather update: %d results, %d UI panels available", len(results), len(panels))

	data := make([]weather.WeatherData, 0, len(results))
	for i, r := range results {
		if r.Data != nil {
			data = append(data, *r.Data)
		} else {
			// Provide an empty Data object but preserve Name if possible
			emptyData := weather.WeatherData{}
			if i < len(a.cfg.Cities) {
				emptyData.CityName = a.cfg.Cities[i].Name
				emptyData.Region = a.cfg.Cities[i].Region
			}
			data = append(data, emptyData)
		}
		if i < len(panels) {
			if r.HasError {
				idx := i
				isStale := r.IsStale
				log.Printf("notifying UI of error for panel %d (city: %s, stale: %v)", idx, a.cfg.Cities[idx].Name, isStale)
				fyne.Do(func() {
					panels[idx].ShowError(isStale)
				})
			}
		}
	}
	log.Printf("dispatching %d data updates to UI panels", len(data))
	fyne.Do(func() {
		a.ui.UpdatePanels(data)
	})
}

// isDefaultConfig checks whether the config is effectively the default
// (no API key configured and no database config).
func (a *AppManager) isDefaultConfig(cfg *config.Config) bool {
	hasAPIKey := cfg.APIConfig != nil && cfg.APIConfig.APIKey != ""
	hasDBConfig := cfg.DatabaseConfig != nil && cfg.DatabaseConfig.Host != ""
	return !hasAPIKey && !hasDBConfig
}

// providerConfigChanged checks whether the provider-specific configuration
// has changed between old and new configs.
func (a *AppManager) providerConfigChanged(old, new *config.Config) bool {
	if old.DataSource != new.DataSource {
		return true
	}
	switch new.DataSource {
	case config.DataSourceRemoteAPI:
		if old.APIConfig == nil || new.APIConfig == nil {
			return old.APIConfig != new.APIConfig
		}
		return old.APIConfig.Provider != new.APIConfig.Provider ||
			old.APIConfig.APIKey != new.APIConfig.APIKey
	case config.DataSourceLocalDatabase:
		if old.DatabaseConfig == nil || new.DatabaseConfig == nil {
			return old.DatabaseConfig != new.DatabaseConfig
		}
		return old.DatabaseConfig.Host != new.DatabaseConfig.Host ||
			old.DatabaseConfig.Port != new.DatabaseConfig.Port ||
			old.DatabaseConfig.DBName != new.DatabaseConfig.DBName ||
			old.DatabaseConfig.Username != new.DatabaseConfig.Username ||
			old.DatabaseConfig.Password != new.DatabaseConfig.Password ||
			old.DatabaseConfig.Query != new.DatabaseConfig.Query
	}
	return false
}

// startPanelClocks starts the clock ticker on each city panel.
func (a *AppManager) startPanelClocks(cities []config.CityConfig) {
	panels := a.ui.Panels()
	for i, p := range panels {
		if i < len(cities) {
			tz := cities[i].Timezone
			if tz == "" {
				tz = "UTC"
			}
			p.StartClock(tz)
		}
	}
}

// stopPanelClocks stops the clock ticker on all city panels.
func (a *AppManager) stopPanelClocks() {
	panels := a.ui.Panels()
	for _, p := range panels {
		p.StopClock()
	}
}

// sameCities checks whether two city slices have the same cities in the same order.
func sameCities(a, b []config.CityConfig) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Region != b[i].Region || a[i].Timezone != b[i].Timezone {
			return false
		}
	}
	return true
}
