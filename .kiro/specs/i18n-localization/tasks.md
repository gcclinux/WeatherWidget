# Tasks

## Task 1: Create LocaleManager core and locale files

- [x] 1.1 Create `internal/i18n/locales/en-GB.json` with all message keys covering settings, tray, panel, weather formatting, validation, and error messages
- [x] 1.2 Create `internal/i18n/locales/pt-BR.json` with Brazilian Portuguese translations for all message keys
- [x] 1.3 Create `internal/i18n/embed.go` with `//go:embed locales/*.json` directive exposing `LocaleFS`
- [x] 1.4 Create `internal/i18n/locale.go` implementing `LocaleManager` with `NewLocaleManager`, `SetLocale`, `T`, `TWithArgs`, `ActiveLocale`, `AvailableLocales`, `MissingKeys`, and `ValidateLocaleFile`
- [x] 1.5 Write unit tests in `internal/i18n/locale_test.go` for locale loading, fallback, key lookup, missing key detection, corrupt file handling, and en-GB/pt-BR completeness

## Task 2: Property-based tests for LocaleManager

- [x] 2.1 Write property test: Lookup returns translated string for valid keys (Property 1) `[pbt]`
- [x] 2.2 Write property test: Missing key fallback to en-GB (Property 2) `[pbt]`
- [x] 2.3 Write property test: Locale file JSON round-trip (Property 5) `[pbt]`
- [x] 2.4 Write property test: Missing keys detection returns correct set difference (Property 6) `[pbt]`

## Task 3: Add locale field to Config and validation

- [x] 3.1 Add `Locale string` field to `Config` struct in `internal/config/types.go` with JSON tag `"locale"` and update `DefaultConfig()` to set `Locale: "en-GB"`
- [x] 3.2 Update `internal/config/service.go` `Load` method to default `Locale` to `"en-GB"` when the field is empty or missing
- [x] 3.3 Add locale validation to `internal/config/validation.go` that checks the `locale` field against available locale codes and produces a translated validation error for invalid values
- [x] 3.4 Write property test: Locale validation accepts valid and rejects invalid (Property 3) `[pbt]`
- [x] 3.5 Write property test: Translated validation errors (Property 7) `[pbt]`

## Task 4: Localise weather formatting functions

- [x] 4.1 Update `internal/weather/format.go` to accept locale parameter in `FormatTemperature`, `FormatDate`, `FormatTime` functions, using locale-specific format strings from the `LocaleManager`
- [x] 4.2 Update all callers of format functions (`internal/ui/panel/panel.go`, `internal/weather/service.go`) to pass locale information
- [x] 4.3 Write property test: Locale-aware formatting preserves structure (Property 4) `[pbt]`
- [x] 4.4 Update existing format tests and property tests in `internal/weather/format_test.go` and `internal/weather/format_property_test.go` to account for locale parameter

## Task 5: Localise city management error messages

- [x] 5.1 Update `internal/config/cities.go` `AddCity`, `RemoveCity`, and `ReorderCities` to accept a translation function parameter and use it for error messages
- [x] 5.2 Update all callers of city management functions to pass the `LocaleManager.T` function
- [x] 5.3 Update existing city tests in `internal/config/cities_test.go` and `internal/config/cities_property_test.go` to account for the translation function parameter

## Task 6: Localise validation error messages

- [x] 6.1 Update `internal/config/validation.go` `Validate` and helper functions to accept a translation function and use translated message keys for all `ValidationError.Message` values
- [x] 6.2 Update all callers of `Validate` to pass the translation function
- [x] 6.3 Update existing validation tests in `internal/config/validation_test.go` and `internal/config/validation_property_test.go` to account for the translation function parameter

## Task 7: Localise Settings Window

- [x] 7.1 Update `internal/ui/manager.go` `UIManager` to store a `*LocaleManager` reference and accept it in `NewUIManager`
- [x] 7.2 Update `internal/ui/settings.go` `ShowSettings` to replace all hardcoded strings with `lm.T()` calls for labels, titles, subtitles, placeholders, button text, tab names, dialog messages, and notes
- [x] 7.3 Add a language selector widget to the Appearance tab in `ShowSettings` that lists available locales and triggers live UI refresh on selection
- [x] 7.4 Update `internal/ui/settings.go` `buildConfigFromUI` to include the selected locale in the returned `Config`
- [x] 7.5 Update existing settings tests in `internal/ui/settings_test.go` to account for locale manager dependency

## Task 8: Localise System Tray Menu

- [x] 8.1 Update `internal/ui/tray.go` `SetupSystemTray` to use `lm.T()` for all menu item labels ("Show Widget", "Hide Widget", "Settings", "Quit")
- [x] 8.2 Ensure tray menu labels update on next rebuild when locale changes

## Task 9: Localise City Panel

- [x] 9.1 Update `internal/ui/panel/panel.go` `NewCityPanel` to accept a `LocaleManager` or translation function and use it for placeholder text ("City, RG", "--°C", "--", "--:--:--", "--/--/----")
- [x] 9.2 Update `internal/ui/panel/panel.go` `ShowError` to use translated stale data warning message
- [x] 9.3 Update `internal/ui/panel/panel.go` `StartClock` to use locale-aware date/time formatting
- [x] 9.4 Update existing panel tests in `internal/ui/panel/panel_test.go` to account for locale manager dependency

## Task 10: Wire LocaleManager into AppManager

- [x] 10.1 Update `internal/app/manager.go` `AppManager` to create `LocaleManager` at startup, load the configured locale, and pass it to `UIManager`, weather formatting, and validation components
- [x] 10.2 Update `internal/app/manager.go` `onSettingsSave` to handle locale changes — call `LocaleManager.SetLocale` when the locale config changes
- [x] 10.3 Update `cmd/weatherwidget/main.go` if needed to pass locale-related dependencies

## Task 11: Final integration verification

- [x] 11.1 Run all existing tests to verify no regressions from i18n changes
- [x] 11.2 Run all new property-based tests and verify they pass
- [x] 11.3 Verify the application builds successfully with `go build ./...`
