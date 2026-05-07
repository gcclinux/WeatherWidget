# Implementation Plan: Temperature Unit Toggle

## Overview

Implement a user-selectable temperature unit (Celsius / Fahrenheit) across four layers: config types, formatting logic, city panel rendering, and settings UI. The change is wired together in `AppManager` so that saving a unit-only change re-renders panels from cache without triggering a new network fetch.

## Tasks

- [x] 1. Add `TemperatureUnit` type and field to config
  - [x] 1.1 Define `TemperatureUnit` type, constants, and `NormalizeTemperatureUnit` in `internal/config/types.go`
    - Add `TemperatureUnit` string type with `TemperatureUnitCelsius` and `TemperatureUnitFahrenheit` constants
    - Implement `NormalizeTemperatureUnit` that returns `TemperatureUnitCelsius` for any unknown value
    - Add `TemperatureUnit TemperatureUnit` field with `json:"temperatureUnit,omitempty"` to the `Config` struct
    - Update `DefaultConfig()` to set `TemperatureUnit: TemperatureUnitCelsius`
    - Call `NormalizeTemperatureUnit` on the loaded value in the config loading path
    - _Requirements: 1.1, 1.2, 1.4_

  - [x] 1.2 Write property test for config round-trip (Property 1)
    - **Property 1: Config round-trip preserves TemperatureUnit**
    - **Validates: Requirements 1.3**
    - Use `rapid.SampledFrom` to draw valid units, marshal/unmarshal, assert equality

  - [x] 1.3 Write property test for invalid unit normalization (Property 2)
    - **Property 2: Invalid TemperatureUnit normalizes to Celsius**
    - **Validates: Requirements 1.4, 4.5, 5.6**
    - Use `rapid.StringMatching` to generate non-valid unit strings, assert `NormalizeTemperatureUnit` returns `TemperatureUnitCelsius`

- [ ] 2. Update `FormatTemperature` and conversion logic in `internal/weather/format.go`
  - [x] 2.1 Replace `FormatTemperature` signature and implement `convertToFahrenheit`
    - Remove the `lm *i18n.LocaleManager` parameter; add `unit config.TemperatureUnit` parameter
    - Implement `convertToFahrenheit(celsius int) int` using `math.Round(float64(celsius)*1.8 + 32)`
    - Export `ConvertToFahrenheit` (or keep unexported and expose via a thin exported wrapper) for property test access
    - Update all call sites that previously passed `lm` to pass the unit from the active config
    - _Requirements: 5.1, 5.2, 5.3, 5.6, 3.1, 3.2_

  - [x] 2.2 Write property test for Celsius identity (Property 3)
    - **Property 3: Celsius conversion is identity**
    - **Validates: Requirements 3.2, 4.1, 5.2**
    - Use `rapid.Int()` to draw any integer, assert `FormatTemperature(c, celsius)` equals `fmt.Sprintf("%d°C", c)`

  - [x] 2.3 Write property test for Fahrenheit formula correctness (Property 4)
    - **Property 4: Fahrenheit conversion formula correctness**
    - **Validates: Requirements 3.1, 3.3, 4.2, 5.3**
    - Use `rapid.IntRange(-273, 60)`, compute expected with `math.Round`, assert output string matches

  - [x] 2.4 Write property test for Fahrenheit near-inverse (Property 5)
    - **Property 5: Fahrenheit conversion is approximately invertible**
    - **Validates: Requirements 3.4**
    - Use `rapid.IntRange(-273, 60)`, convert to F, invert, assert diff within ±1

  - [x] 2.5 Write property test for Celsius output regex (Property 6)
    - **Property 6: FormatTemperature Celsius output matches pattern**
    - **Validates: Requirements 4.1, 5.2, 5.4**
    - Use `rapid.Int()`, assert output matches `^-?\d+°C$`

  - [x] 2.6 Write property test for Fahrenheit output regex (Property 7)
    - **Property 7: FormatTemperature Fahrenheit output matches pattern**
    - **Validates: Requirements 4.2, 5.3, 5.5**
    - Use `rapid.Int()`, assert output matches `^-?\d+°F$`

- [x] 3. Checkpoint — Ensure all config and formatting tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Update `CityPanel` to cache weather data and accept unit in `Update`
  - [x] 4.1 Add `lastData *weather.WeatherData` field and update `Update` signature in `internal/ui/panel/panel.go`
    - Add `lastData *weather.WeatherData` to `CityPanel` struct
    - Change `Update(data *weather.WeatherData, unit config.TemperatureUnit)` signature
    - Store `data` in `p.lastData` at the start of `Update`
    - Replace the `FormatTemperature` call to pass `unit` instead of `lm`
    - _Requirements: 4.3, 4.4_

  - [x] 4.2 Implement `CityPanel.Rerender` method
    - Add `Rerender(unit config.TemperatureUnit)` that calls `p.Update(p.lastData, unit)` when `lastData != nil`
    - Guard against nil `lastData` (no-op)
    - _Requirements: 4.4_

  - [x] 4.3 Write unit tests for `CityPanel.Rerender`
    - Test no-op when `lastData` is nil
    - Test that `Rerender` re-renders with cached data and new unit
    - _Requirements: 4.4_

- [x] 5. Update `UIManager` with `UpdatePanels` unit parameter and `RerenderPanels`
  - [x] 5.1 Update `UIManager.UpdatePanels` to accept and forward `unit` in `internal/ui/manager.go`
    - Change signature to `UpdatePanels(data []weather.WeatherData, unit config.TemperatureUnit)`
    - Pass `unit` to each `p.Update` call
    - _Requirements: 4.3_

  - [x] 5.2 Implement `UIManager.RerenderPanels`
    - Add `RerenderPanels(unit config.TemperatureUnit)` that calls `p.Rerender(unit)` for each panel
    - _Requirements: 4.4, 2.4_

- [x] 6. Wire unit into `AppManager` — `handleWeatherUpdate` and `onSettingsSave`
  - [x] 6.1 Pass `TemperatureUnit` from config in `handleWeatherUpdate` in `internal/app/manager.go`
    - Update the `fyne.Do` call to pass `a.cfg.TemperatureUnit` to `a.ui.UpdatePanels`
    - _Requirements: 4.3_

  - [x] 6.2 Implement unit-change fast path in `onSettingsSave`
    - Detect `unitChanged`, `citiesChanged`, and `providerChanged` flags
    - If only unit changed: call `a.ui.RerenderPanels(newCfg.TemperatureUnit)` inside `fyne.Do`
    - Otherwise: call `a.scheduler.FetchNow()` as before
    - _Requirements: 2.4, 4.4_

  - [x] 6.3 Write integration tests for `onSettingsSave` routing
    - Test that saving with only a unit change calls `RerenderPanels` and not `FetchNow`
    - Test that saving with a city list change still calls `FetchNow`
    - _Requirements: 2.4, 4.4_

- [x] 7. Add temperature unit radio group to Settings UI
  - [x] 7.1 Extend `settingsState` and build radio group in `internal/ui/settings.go`
    - Add `selectedUnit config.TemperatureUnit` to `settingsState`
    - Create `unitRadio` with options `"°C (Celsius)"` and `"°F (Fahrenheit)"`, horizontal layout
    - Pre-select based on `NormalizeTemperatureUnit(cfg.TemperatureUnit)`
    - Insert `sectionBlock` for the unit selector into the Appearance tab content
    - _Requirements: 2.1, 2.2_

  - [x] 7.2 Update `buildConfigFromUI` to write `selectedUnit` to the returned `Config`
    - Set `cfg.TemperatureUnit = config.NormalizeTemperatureUnit(state.selectedUnit)`
    - _Requirements: 2.3_

  - [x] 7.3 Write unit tests for `buildConfigFromUI` unit field
    - Verify `TemperatureUnit` is written correctly for each radio selection
    - _Requirements: 2.3_

- [x] 8. Final checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for a faster MVP
- Each task references specific requirements for traceability
- Property tests use `github.com/flyingmutant/rapid` as specified in the design
- `ConvertToFahrenheit` should be exported (or wrapped) to allow direct testing in property tests
- The `lm *i18n.LocaleManager` parameter removal from `FormatTemperature` affects all call sites — update them in task 2.1

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1"] },
    { "id": 1, "tasks": ["1.2", "1.3", "2.1"] },
    { "id": 2, "tasks": ["2.2", "2.3", "2.4", "2.5", "2.6", "4.1"] },
    { "id": 3, "tasks": ["4.2", "5.1"] },
    { "id": 4, "tasks": ["4.3", "5.2", "7.1"] },
    { "id": 5, "tasks": ["6.1", "6.2", "7.2"] },
    { "id": 6, "tasks": ["6.3", "7.3"] }
  ]
}
```
