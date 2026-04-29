# Requirements Document

## Introduction

This feature adds internationalisation (i18n) and localisation (l10n) support to the WeatherWidget desktop application. The goal is to make every piece of human-readable content — widget display, settings window, system tray menu, error messages, weather descriptions, date/time formatting, and validation messages — available in multiple languages. The initial supported locales are British English (en-GB) and Brazilian Portuguese (pt-BR). The architecture must allow additional locales to be added by simply dropping in a new locale file.

## Glossary

- **Locale_Manager**: The component responsible for loading, caching, and serving translated strings for the active locale.
- **Locale_File**: A structured data file (JSON) containing all translated strings for a single locale, keyed by a stable message identifier.
- **Active_Locale**: The locale currently selected by the user and persisted in the configuration.
- **Message_Key**: A unique, stable string identifier used to look up a translated message (e.g. `settings.title`, `tray.show_widget`).
- **Settings_Window**: The Fyne-based settings dialog containing Appearance, Data Provider, Locations, and About tabs.
- **System_Tray_Menu**: The desktop system tray context menu with Show Widget, Hide Widget, Settings, and Quit items.
- **City_Panel**: The per-city weather display panel showing city name, temperature, weather description, time, and date.
- **Weather_Formatter**: The component that formats temperature, date, time, and weather description strings for display.
- **Config_Service**: The component that loads, validates, and persists the application configuration file.
- **Validation_Engine**: The component that validates configuration fields and produces user-facing error messages.

## Requirements

### Requirement 1: Locale File Structure and Loading

**User Story:** As a developer, I want locale files stored in a well-defined JSON format, so that adding a new language requires only creating a new file without code changes.

#### Acceptance Criteria

1. THE Locale_Manager SHALL load locale data from JSON Locale_Files embedded in the application binary.
2. WHEN the application starts, THE Locale_Manager SHALL load the Locale_File matching the Active_Locale from the persisted configuration.
3. IF a Locale_File for the Active_Locale is missing or corrupt, THEN THE Locale_Manager SHALL fall back to the en-GB Locale_File.
4. THE Locale_Manager SHALL provide a lookup function that accepts a Message_Key and returns the translated string for the Active_Locale.
5. IF a Message_Key is not found in the Active_Locale Locale_File, THEN THE Locale_Manager SHALL return the en-GB translation for that Message_Key.
6. WHEN a new Locale_File is added to the embedded assets, THE Locale_Manager SHALL make the new locale available without any code changes beyond the file addition.

### Requirement 2: Locale File Content — en-GB and pt-BR

**User Story:** As a user, I want the application available in British English and Brazilian Portuguese, so that I can use the application in my preferred language.

#### Acceptance Criteria

1. THE Locale_File for en-GB SHALL contain translations for every Message_Key used in the application.
2. THE Locale_File for pt-BR SHALL contain translations for every Message_Key used in the application.
3. THE Locale_File for en-GB and pt-BR SHALL each cover the following categories of Message_Keys:
   - Settings_Window title and tab labels (Appearance, Data Provider, Locations, About)
   - Settings_Window section titles and subtitles (Widget Position, Background Transparency, Refresh Interval, Startup, Data Provider & API Key)
   - Settings_Window form labels (Provider, API Key, Name, Region, Latitude, Longitude, Timezone)
   - Settings_Window button labels (Get FREE API, Get PRO API, Add City, Search API, Searching..., Save, Remove)
   - Settings_Window placeholder text (API Key, City name, Region / Country, Latitude (optional), Longitude (optional), Timezone)
   - Settings_Window position labels (Top-Left, Top-Right, Bottom-Left, Bottom-Right)
   - Settings_Window opacity labels (25%, 50%, 75%, 100%)
   - Settings_Window note text (Free = 120 minutes refresh rate, Pro = 10 minutes refresh rate)
   - Settings_Window auto-start checkbox label (Launch WeatherWidget when Windows starts)
   - Settings_Window card titles and subtitles (Saved Cities, Add New City, Manage your tracked locations)
   - Settings_Window About tab content (version label, application description, Website label, Preview label)
   - Settings_Window dialog messages (Settings saved successfully!, Saved)
   - System_Tray_Menu item labels (Show Widget, Hide Widget, Settings, Quit)
   - City_Panel placeholder text (City, RG; --°C; --; --:--:--; --/--/----)
   - City_Panel stale data warning (Data may be stale)
   - Weather_Formatter temperature unit suffix (°C)
   - Validation_Engine error messages (must contain 1 to 5 cities, must not be empty, invalid latitude, etc.)
   - Config city management error messages (maximum of 5 cities reached, cannot remove the last city)
   - Settings_Window error messages (city name is required, city name is required to search, API Key is required, Region / Country is required to search via EasyWeatherWidget)
   - Monitor selector label (Display Monitor, Monitor N)
   - Refresh interval display format (N min)

### Requirement 3: Locale Configuration Persistence

**User Story:** As a user, I want my language preference saved, so that the application remembers my choice across restarts.

#### Acceptance Criteria

1. THE Config_Service SHALL store the Active_Locale as a string field named `locale` in the configuration file.
2. WHEN the configuration file does not contain a `locale` field, THE Config_Service SHALL default the Active_Locale to "en-GB".
3. WHEN the user selects a different locale in the Settings_Window, THE Config_Service SHALL persist the new Active_Locale value.
4. THE Validation_Engine SHALL validate that the `locale` field matches one of the available Locale_Files.
5. IF the `locale` field contains an unsupported value, THEN THE Validation_Engine SHALL produce a validation error and THE Config_Service SHALL fall back to "en-GB".

### Requirement 4: Settings Window Localisation

**User Story:** As a user, I want the settings window displayed in my chosen language, so that I can configure the application without language barriers.

#### Acceptance Criteria

1. WHEN the Settings_Window opens, THE Settings_Window SHALL render all labels, titles, subtitles, placeholders, button text, and tab names using translations from the Active_Locale.
2. THE Settings_Window SHALL include a language selector control in the Appearance tab that lists all available locales by their display name (e.g. "English (UK)", "Português (Brasil)").
3. WHEN the user selects a new locale in the language selector, THE Settings_Window SHALL update all visible text to the newly selected locale without requiring a restart.
4. THE Settings_Window SHALL display the refresh interval label in the format translated for the Active_Locale (e.g. "10 min" in en-GB, "10 min" in pt-BR).
5. THE Settings_Window SHALL display the auto-start checkbox label translated for the Active_Locale.
6. THE Settings_Window SHALL display all dialog messages (save confirmation, error dialogs) translated for the Active_Locale.

### Requirement 5: System Tray Menu Localisation

**User Story:** As a user, I want the system tray menu in my chosen language, so that I can control the widget without switching mental context.

#### Acceptance Criteria

1. WHEN the system tray menu is created, THE System_Tray_Menu SHALL render all menu item labels using translations from the Active_Locale.
2. THE System_Tray_Menu SHALL display the following items translated: "Show Widget", "Hide Widget", "Settings", "Quit".
3. WHEN the Active_Locale changes, THE System_Tray_Menu SHALL update its menu item labels to the new locale on the next menu rebuild or application restart.

### Requirement 6: Weather Display Localisation

**User Story:** As a user, I want weather information displayed in my language, so that I can read conditions and times naturally.

#### Acceptance Criteria

1. THE Weather_Formatter SHALL format temperature strings using the Active_Locale temperature unit suffix (e.g. "25°C").
2. THE Weather_Formatter SHALL format date strings according to the Active_Locale convention (en-GB: "DD/MM/YYYY", pt-BR: "DD/MM/YYYY").
3. THE Weather_Formatter SHALL format time strings according to the Active_Locale convention (en-GB: "HH:MM:SS", pt-BR: "HH:MM:SS").
4. THE City_Panel SHALL display the stale data warning message translated for the Active_Locale.
5. THE City_Panel SHALL display placeholder text translated for the Active_Locale when no weather data is available.

### Requirement 7: Validation and Error Message Localisation

**User Story:** As a user, I want error messages in my language, so that I can understand and fix configuration problems.

#### Acceptance Criteria

1. WHEN a validation error occurs, THE Validation_Engine SHALL produce error messages translated for the Active_Locale.
2. WHEN a city management operation fails, THE Config_Service SHALL return error messages translated for the Active_Locale (e.g. "maximum of 5 cities reached", "cannot remove the last city").
3. WHEN a settings form validation fails, THE Settings_Window SHALL display error dialog text translated for the Active_Locale (e.g. "city name is required", "API Key is required in the settings above to search").
4. IF a connection test fails, THEN THE Settings_Window SHALL display the failure message translated for the Active_Locale.

### Requirement 8: Locale File Round-Trip Integrity

**User Story:** As a developer, I want to verify that locale files can be serialised and deserialised without data loss, so that translations are never silently corrupted.

#### Acceptance Criteria

1. FOR ALL valid Locale_Files, serialising to JSON then deserialising SHALL produce an equivalent Locale_File (round-trip property).
2. THE Locale_Manager SHALL reject Locale_Files that contain duplicate Message_Keys.
3. THE Locale_Manager SHALL reject Locale_Files that are not valid JSON.

### Requirement 9: Locale Completeness Validation

**User Story:** As a developer, I want automated validation that every locale file covers all required keys, so that missing translations are caught before release.

#### Acceptance Criteria

1. THE Locale_Manager SHALL expose a function that compares a Locale_File against the en-GB reference and returns a list of missing Message_Keys.
2. WHEN a Locale_File is loaded, THE Locale_Manager SHALL log a warning for each Message_Key present in en-GB but missing from the loaded Locale_File.
3. FOR ALL non-default Locale_Files, the set of Message_Keys SHALL be a superset of the en-GB Message_Keys (completeness property).
