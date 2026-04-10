# Implementation Plan: Windows Weather Widget

## Overview

Build a Windows 11 desktop weather widget as a single native Go binary using Fyne v2 for the GUI. Implementation proceeds bottom-up: data models and config first, then weather providers, scheduling, UI components, and finally the application orchestrator wiring everything together.

## Tasks

- [x] 1. Set up project structure and core data models
  - [x] 1.1 Initialize Go module and directory structure
    - Create `go.mod` with module name and Go version
    - Create directory layout: `cmd/weatherwidget/`, `internal/config/`, `internal/weather/`, `internal/weather/remoteapi/`, `internal/weather/database/`, `internal/scheduler/`, `internal/guard/`, `internal/ui/`, `internal/ui/panel/`, `assets/icons/`
    - Add placeholder icon PNG files in `assets/icons/` for each weather condition (clear, partly_cloudy, cloudy, rain, snow, storm, fog)
    - Add `go:embed` directive in an `assets/embed.go` file to embed the `icons` directory
    - _Requirements: 7.1, 7.2_

  - [x] 1.2 Define core data types and interfaces
    - Create `internal/weather/types.go` with `WeatherData`, `CityConfig`, `WeatherProvider` interface, and icon code constants
    - Create `internal/config/types.go` with `Config`, `APIConfig`, `DatabaseConfig`, `DataSourceType`, `ValidationError` types
    - Implement `DefaultConfig()` returning a config with one default city, 10-minute refresh, bottom-right corner
    - _Requirements: 1.4, 3.4, 8.3_

  - [x] 1.3 Implement display formatting functions
    - Create `internal/weather/format.go` with `FormatTemperature(temp int) string`, `FormatCityRegion(name, region string) string`, `FormatDateTime(t time.Time, timezone string) string`
    - `FormatTemperature` returns `"{int}°C"` pattern
    - `FormatCityRegion` returns `"{name}, {region}"` pattern
    - `FormatDateTime` returns `"DD/MM/YYYY - HH:MM:SS"` pattern in the given timezone
    - Implement `MapConditionToIcon(code string) string` that maps weather condition codes to embedded icon asset identifiers
    - _Requirements: 2.1, 2.2, 2.4, 2.5_

  - [x] 1.4 Write property tests for display formatting (Properties 9, 10)
    - **Property 9: Display string formatting** — Generate random integer temperatures → assert output matches `{int}°C`; generate random non-empty name/region strings → assert output matches `"{name}, {region}"`
    - **Validates: Requirements 2.2, 2.4**
    - **Property 10: Date/time formatting** — Generate random `time.Time` values and valid IANA timezone strings → assert output matches `DD/MM/YYYY - HH:MM:SS` with correct components
    - **Validates: Requirements 2.5**
    - Use `pgregory.net/rapid` as the PBT library with minimum 100 iterations per property

  - [x] 1.5 Write property test for icon mapping (Property 8)
    - **Property 8: Weather condition to icon mapping is total** — Generate random valid weather condition codes → assert `MapConditionToIcon` returns a non-empty icon identifier that exists in the embedded asset set
    - **Validates: Requirements 2.1**

  - [x] 1.6 Write property test for widget layout dimensions (Property 7)
    - **Property 7: Widget layout dimensions** — Generate random city count N in {1, 2, 3} → call layout calculation → assert total width = N × 300 dip, height = 120 dip, panel slots = N
    - **Validates: Requirements 1.3, 1.5**
    - Implement `CalculateLayout(cityCount int) (width, height, slots int)` in `internal/ui/layout.go`

- [x] 2. Checkpoint - Verify core data models
  - Ensure all tests pass, ask the user if questions arise.

- [x] 3. Implement configuration service
  - [x] 3.1 Implement ConfigService Load/Save
    - Create `internal/config/service.go` with `ConfigService` struct
    - Implement `NewConfigService(appDataDir string)` that sets config path to `{appDataDir}/WeatherWidget/config.json`
    - Implement `Load()` that reads and unmarshals JSON; returns `DefaultConfig()` on any error (missing file, corrupt JSON, invalid schema) without panicking
    - Implement `Save(cfg *Config)` that marshals to JSON and writes atomically (write to temp file, then rename)
    - Implement `ConfigPath()` returning the full path
    - _Requirements: 8.1, 8.2, 8.3_

  - [x] 3.2 Implement configuration validation
    - Implement `Validate(cfg *Config) []ValidationError` in `internal/config/validation.go`
    - Validate: cities length 1–3, refresh interval 1–60, corner position in allowed set
    - When `dataSource` is `remote_api`: validate non-empty API key, provider is `openweathermap` or `weatherunderground`
    - When `dataSource` is `local_database`: validate non-empty host/dbName/username, port 1–65535
    - Validate each city: non-empty name; if coordinates provided, lat -90..90, lon -180..180
    - _Requirements: 1.4, 4.2, 4.3, 5.2, 5.3, 6.2_

  - [x] 3.3 Implement city list operations
    - Create `internal/config/cities.go` with functions: `AddCity(cities []CityConfig, city CityConfig) ([]CityConfig, error)`, `RemoveCity(cities []CityConfig, index int) ([]CityConfig, error)`, `ReorderCities(cities []CityConfig, newOrder []int) ([]CityConfig, error)`
    - `AddCity` rejects if list already has 3 cities
    - `RemoveCity` rejects if list has only 1 city
    - `ReorderCities` validates permutation and returns reordered slice
    - _Requirements: 3a.2, 3a.3, 3a.4, 3a.5, 3a.6_

  - [x] 3.4 Write property tests for config serialization (Property 1)
    - **Property 1: Configuration serialization round-trip** — Generate random valid `Config` structs → save to temp file → load → assert deep equality with original
    - **Validates: Requirements 3.4, 8.1, 8.2**

  - [x] 3.5 Write property tests for config validation (Property 2)
    - **Property 2: Configuration validation correctness** — Generate random `Config` structs (valid and invalid) → call `Validate` → assert errors match exactly the violated constraints and no errors for valid configs
    - **Validates: Requirements 1.4, 4.2, 4.3, 5.2, 5.3, 6.2**

  - [x] 3.6 Write property test for corrupt config fallback (Property 3)
    - **Property 3: Corrupt configuration fallback** — Generate random byte sequences (partial JSON, empty, binary data) → write to config file → call `Load` → assert returns `DefaultConfig()` without panic
    - **Validates: Requirements 8.3**

  - [x] 3.7 Write property tests for city list operations (Properties 4, 5, 6)
    - **Property 4: City list add preserves existing and appends** — Generate random city lists (len 1–2) + random city → `AddCity` → assert length+1, prefix unchanged, last element is new city
    - **Validates: Requirements 3a.2**
    - **Property 5: City list remove decreases length and excludes target** — Generate random city lists (len 2–3) + random valid index → `RemoveCity` → assert length-1, removed city absent
    - **Validates: Requirements 3a.4**
    - **Property 6: City list reorder preserves set** — Generate random city lists + random permutation → `ReorderCities` → assert same multiset of cities in new order
    - **Validates: Requirements 3a.5**

- [x] 4. Checkpoint - Verify configuration service
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Implement weather data providers
  - [x] 5.1 Implement Remote API adapter
    - Create `internal/weather/remoteapi/adapter.go` with `RemoteAPIAdapter` struct
    - Implement `NewRemoteAPIAdapter(provider, apiKey string)` with configurable `http.Client` (timeout 10s)
    - Implement `FetchWeather` for OpenWeatherMap: call `/data/2.5/weather` endpoint, parse JSON response, map to `WeatherData`
    - Implement `FetchWeather` for Weather Underground: call appropriate endpoint, parse response, map to `WeatherData`
    - Implement `TestConnection` that performs a lightweight API call to verify credentials
    - Map API-specific condition codes to internal `IconCode` constants
    - _Requirements: 3.5, 4.1, 4.2, 4.4_

  - [x] 5.2 Implement Database adapter
    - Create `internal/weather/database/adapter.go` with `DatabaseAdapter` struct
    - Implement `NewDatabaseAdapter(connString, query string)` using `pgxpool.New` for connection pooling
    - Implement `FetchWeather` that executes the user-configured query with city name as parameter, scans result into `WeatherData`
    - Implement `TestConnection` that calls `pool.Ping(ctx)`
    - Implement `Close()` to release the connection pool
    - _Requirements: 3.5, 5.1, 5.2, 5.4_

  - [x] 5.3 Implement WeatherService orchestrator
    - Create `internal/weather/service.go` with `WeatherService` struct
    - Hold a reference to the active `WeatherProvider` and per-city failure counters
    - Implement `FetchAll(ctx context.Context, cities []CityConfig) []WeatherResult` that fetches weather for all cities, caches last successful result per city
    - On fetch failure: increment failure counter, return cached data with error flag
    - After 3 consecutive failures: set stale warning flag
    - On success: reset failure counter, update cache
    - Implement `SwitchProvider(provider WeatherProvider)` that resets all failure counters
    - _Requirements: 6.1, 6.3, 6.4, 3.5_

  - [x] 5.4 Write unit tests for weather providers
    - Test Remote API adapter with mock HTTP server returning realistic OpenWeatherMap/Weather Underground JSON responses
    - Test Database adapter with mock pgx pool
    - Test WeatherService failure counter logic: error indicator after 1 failure, stale warning after 3, reset on success
    - _Requirements: 6.3, 6.4_

- [x] 6. Implement refresh scheduler
  - [x] 6.1 Implement RefreshScheduler
    - Create `internal/scheduler/scheduler.go` with `RefreshScheduler` struct
    - Implement `NewRefreshScheduler(interval time.Duration, ws *WeatherService)` with `onUpdate` and `onError` callbacks
    - Implement `Start()` that launches a goroutine with `time.Ticker`, calling `WeatherService.FetchAll` on each tick
    - Implement `Stop()` that stops the ticker and signals the goroutine to exit via `stopCh`
    - Implement `SetInterval(d time.Duration)` that resets the ticker with the new interval
    - Trigger an immediate fetch on `Start()` before waiting for the first tick
    - _Requirements: 6.1, 6.2_

  - [x] 6.2 Write unit tests for scheduler
    - Test that scheduler fires at configured interval using short test intervals
    - Test that `SetInterval` changes the tick rate
    - Test that `Stop` cancels pending ticks and goroutine exits cleanly
    - _Requirements: 6.1, 6.2_

- [x] 7. Implement single instance guard
  - [x] 7.1 Implement SingleInstanceGuard
    - Create `internal/guard/guard.go` with `SingleInstanceGuard` struct
    - Implement `NewSingleInstanceGuard(name string)` that calls `CreateMutexW` via `golang.org/x/sys/windows`
    - If mutex already exists (`ERROR_ALREADY_EXISTS`), attempt `FindWindow`/`SetForegroundWindow` to bring existing instance to front, then return error
    - Implement `Release()` that closes the mutex handle
    - _Requirements: 9.2_

- [x] 8. Checkpoint - Verify backend components
  - Ensure all tests pass, ask the user if questions arise.

- [x] 9. Implement UI components
  - [x] 9.1 Implement CityPanel widget
    - Create `internal/ui/panel/panel.go` with `CityPanel` struct using Fyne widgets
    - Layout: vertical stack within 300×120 dip — weather icon (top), temperature label, description label, city/region label, date/time label (bottom)
    - Implement `NewCityPanel()` that creates all sub-widgets with placeholder content
    - Implement `Update(data *WeatherData)` that sets icon image from embedded assets, temperature via `FormatTemperature`, description, city/region via `FormatCityRegion`
    - Implement `ShowError(stale bool)` that shows error indicator icon or persistent stale warning
    - Implement `StartClock(timezone string)` that starts a 1-second ticker updating the time label via `FormatDateTime`
    - Implement `StopClock()` that stops the ticker
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 6.3, 6.4_

  - [x] 9.2 Implement widget window and layout
    - Create `internal/ui/manager.go` with `UIManager` struct
    - Implement `NewUIManager(app fyne.App)` that creates the main Fyne window
    - Apply Win32 styles post-creation: `WS_EX_TOOLWINDOW` (borderless) and `HWND_TOPMOST` (always-on-top) via `golang.org/x/sys/windows` syscalls
    - Implement `ShowWidget(cities []CityConfig)` that creates N `CityPanel` instances arranged horizontally, resizes window to N×300 × 120 dip
    - Implement `UpdatePanels(data []WeatherData)` that calls `Update` on each panel
    - Implement `SetCorner(position string)` that repositions the window to the specified screen corner
    - _Requirements: 1.1, 1.2, 1.3, 1.5_

  - [x] 9.3 Implement context menu
    - Implement right-click context menu on the widget window with options: "Settings", "Position" (submenu: Top-Left, Top-Right, Bottom-Left, Bottom-Right), "Exit"
    - Wire "Settings" to open the Settings Page
    - Wire "Position" submenu items to call `SetCorner`
    - Wire "Exit" to trigger `AppManager.Shutdown()`
    - _Requirements: 1.6_

  - [x] 9.4 Implement system tray integration
    - Implement system tray icon using Fyne's `desk.App` interface or `getlantern/systray`
    - Add tray menu items: "Show Widget", "Hide Widget", "Settings", "Exit"
    - Wire menu items to corresponding `UIManager` and `AppManager` methods
    - Handle tray creation failure gracefully (log warning, continue without tray)
    - _Requirements: 9.3_

  - [x] 9.5 Implement Settings Page
    - Create settings dialog window with tabs/sections for: Data Source selection, City List management, Refresh Interval, API Configuration, Database Configuration
    - Data Source section: radio buttons for "Remote API" / "Local Database", toggling visibility of API vs Database config forms
    - City List section: display current cities with position numbers, add/remove/reorder controls, enforce 1–3 city limit with user messages
    - API Configuration form: provider dropdown (OpenWeatherMap, Weather Underground), API key field, per-city name/coordinates fields
    - Database Configuration form: host, port, database name, username, password, query fields
    - Refresh Interval: slider or input field, 1–60 minutes
    - Save button: call `ConfigService.Validate`, highlight invalid fields with error messages, on valid config call `Save` and trigger test connection, show success/failure notification
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3a.1, 3a.2, 3a.3, 3a.4, 3a.5, 3a.6, 4.1, 4.2, 4.3, 4.4, 5.1, 5.2, 5.3, 5.4, 6.2_

- [x] 10. Checkpoint - Verify UI components
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. Implement application orchestrator and wiring
  - [x] 11.1 Implement AppManager
    - Create `internal/app/manager.go` with `AppManager` struct
    - Implement `Run()`: acquire single instance guard → load config (open Settings Page if defaults) → create UI manager → create weather service with appropriate provider → create and start refresh scheduler → show widget → run Fyne event loop
    - Implement `Shutdown()`: stop scheduler, stop all city panel clocks, close database adapter if active, release single instance guard, exit within 2 seconds (force `os.Exit(1)` if exceeded)
    - Wire settings save callback: persist config, switch weather provider if data source changed, reset scheduler interval, rebuild city panels if city list changed
    - _Requirements: 1.1, 8.2, 8.3, 9.1, 9.2_

  - [x] 11.2 Create main entry point
    - Create `cmd/weatherwidget/main.go` with `main()` function
    - Initialize Fyne app with `app.New()`
    - Set app data directory to `%APPDATA%\WeatherWidget`
    - Create `AppManager` and call `Run()`
    - Handle fatal errors with log output
    - _Requirements: 7.1, 7.3_

  - [x] 11.3 Set up build configuration
    - Create `Makefile` or build script with: `go build -ldflags="-H windowsgui -s -w" -o weatherwidget.exe ./cmd/weatherwidget/`
    - `-H windowsgui` suppresses console window on Windows
    - `-s -w` strips debug info for smaller binary
    - Target `GOOS=windows GOARCH=amd64`
    - _Requirements: 7.1, 7.2, 7.3_

- [x] 12. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests use `pgregory.net/rapid` (Go PBT library) with minimum 100 iterations per property
- Unit tests validate specific examples and edge cases
- All embedded assets use `go:embed` for single-binary deployment
