# Requirements Document

## Introduction

This feature adds a temperature unit toggle to the WeatherWidget application, allowing users to switch between Celsius (°C) and Fahrenheit (°F). Currently, the widget always displays temperatures in Celsius as fetched from the weather API. With this feature, users can select their preferred unit in the Settings UI (Appearance tab), and all temperature displays in the city panels will reflect that choice immediately. The selected unit is persisted in the application configuration so it survives restarts.

## Glossary

- **Temperature_Unit**: The unit of measurement for temperature display; either `"celsius"` or `"fahrenheit"`.
- **Config**: The application configuration struct (`internal/config/types.go`) that is persisted to `%APPDATA%\WeatherWidget\config.json`.
- **Settings_UI**: The settings dialog managed by `UIManager.ShowSettings` in `internal/ui/settings.go`.
- **City_Panel**: The `CityPanel` struct in `internal/ui/panel/panel.go` that renders weather data for a single city.
- **FormatTemperature**: The function in `internal/weather/format.go` responsible for converting a Celsius integer to a display string.
- **Converter**: The temperature conversion logic that applies the formula F = (C × 1.8) + 32 to convert Celsius to Fahrenheit.

---

## Requirements

### Requirement 1: Temperature Unit Configuration Storage

**User Story:** As a user, I want my preferred temperature unit to be saved, so that I do not have to re-select it every time I restart the application.

#### Acceptance Criteria

1. THE Config SHALL include a `TemperatureUnit` field of type `Temperature_Unit` with valid values `"celsius"` and `"fahrenheit"`.
2. WHEN the Config is loaded and the `TemperatureUnit` field is absent or empty, THE Config SHALL default to `"celsius"`.
3. IF a Config value is serialized to JSON and then deserialized back, THEN the resulting `TemperatureUnit` field SHALL equal the original value (round-trip identity).
4. IF the Config contains a `TemperatureUnit` value that is neither `"celsius"` nor `"fahrenheit"`, THEN THE Config SHALL treat it as `"celsius"`.

---

### Requirement 2: Temperature Unit Toggle in Settings UI

**User Story:** As a user, I want to select my preferred temperature unit in the Settings UI, so that I can control how temperatures are displayed in the widget.

#### Acceptance Criteria

1. THE Settings_UI SHALL display a temperature unit selector in the Appearance tab with exactly two options: `"°C (Celsius)"` and `"°F (Fahrenheit)"`.
2. WHEN the Settings_UI is opened, THE Settings_UI SHALL pre-select the option that matches the current `TemperatureUnit` value in the loaded Config; if the value is absent or invalid, the selector SHALL default to `"°C (Celsius)"`.
3. WHEN the user selects a temperature unit option and confirms the save action, THE Settings_UI SHALL write the corresponding `TemperatureUnit` value (`"celsius"` or `"fahrenheit"`) to the Config before the settings dialog closes.
4. WHEN the settings are saved with a new `TemperatureUnit` value, THE Settings_UI SHALL trigger a re-render of all city panels using their cached `WeatherData` within 2 seconds, without initiating a new weather data fetch.

---

### Requirement 3: Temperature Conversion

**User Story:** As a user, I want temperatures to be accurately converted to Fahrenheit when I select that unit, so that I see correct values in the widget.

#### Acceptance Criteria

1. WHEN the `TemperatureUnit` is `"fahrenheit"` and the input is an integer Celsius value C in the range −273 to 60, THE Converter SHALL compute the Fahrenheit value using the formula `F = round(C × 1.8 + 32)` and return that integer.
2. WHEN the `TemperatureUnit` is `"celsius"`, THE Converter SHALL return the original integer Celsius value unchanged.
3. THE Converter SHALL produce a Fahrenheit value satisfying `F = round(C × 1.8 + 32)` for any integer Celsius input C in the range −273 to 60.
4. THE Converter SHALL produce a Fahrenheit value F such that `round((F − 32) / 1.8)` is within ±1 of the original integer Celsius input C, for all C in the range −273 to 60.

---

### Requirement 4: Temperature Display in City Panel

**User Story:** As a user, I want the temperature shown in each city panel to reflect my selected unit, so that I always see temperatures in my preferred format.

#### Acceptance Criteria

1. WHEN the `TemperatureUnit` is `"celsius"`, THE City_Panel SHALL display the temperature in the format `{integer}°C`.
2. WHEN the `TemperatureUnit` is `"fahrenheit"`, THE City_Panel SHALL display the temperature in the format `{integer}°F`.
3. WHEN `City_Panel.Update` is called with a `WeatherData` value and a `Temperature_Unit` parameter, THE City_Panel SHALL use the provided unit to format the temperature display string.
4. WHEN the settings are saved with a new `TemperatureUnit` value, THE City_Panel SHALL re-render using the new unit and its cached `WeatherData` without requiring a new weather data fetch.
5. IF the `TemperatureUnit` passed to `City_Panel.Update` is absent or invalid, THEN THE City_Panel SHALL display the temperature in the format `{integer}°C`.

---

### Requirement 5: Temperature Formatting Function

**User Story:** As a developer, I want a single formatting function that handles both units, so that temperature display logic is consistent and testable across the codebase.

#### Acceptance Criteria

1. THE FormatTemperature function SHALL accept an integer Celsius value and a `Temperature_Unit` parameter, and return a formatted string.
2. WHEN the `Temperature_Unit` parameter is `"celsius"`, THE FormatTemperature function SHALL return a string matching the pattern `{integer}°C`.
3. WHEN the `Temperature_Unit` parameter is `"fahrenheit"`, THE FormatTemperature function SHALL return a string matching the pattern `{integer}°F`, where the integer is computed using `round(C × 1.8 + 32)` as defined in Requirement 3.
4. THE FormatTemperature function SHALL return a string matching the regex `^-?\d+°C$` for any integer Celsius input when the unit is `"celsius"`.
5. THE FormatTemperature function SHALL return a string matching the regex `^-?\d+°F$` for any integer Celsius input when the unit is `"fahrenheit"`.
6. IF the `Temperature_Unit` parameter is neither `"celsius"` nor `"fahrenheit"`, THEN THE FormatTemperature function SHALL return a string in the format `{integer}°C` (defaulting to Celsius).
