# Requirements Document

## Introduction

This feature adds a new weather service provider called "EasyWetherWidget" to the WeatherWidget desktop application. Currently the application supports OpenWeatherMap (and Weather Underground) as remote API providers. The EasyWetherWidget provider uses a different API endpoint, response format, and refresh cadence. The settings UI must allow the user to select between providers, and the refresh interval minimum must adapt based on the selected provider's rate limits.

## Glossary

- **Widget**: The WeatherWidget desktop application built with Fyne for Windows.
- **Provider_Selector**: The dropdown widget in the settings UI that allows the user to choose a remote API provider.
- **Remote_API_Adapter**: The `RemoteAPIAdapter` struct in `internal/weather/remoteapi/adapter.go` that implements the `WeatherProvider` interface for remote weather APIs.
- **EWW_API**: The EasyWetherWidget remote weather API hosted at `wagemaker.uk:8043`.
- **OWM_API**: The OpenWeatherMap remote weather API.
- **Config_Service**: The configuration persistence layer in `internal/config/` that loads, validates, and saves application settings.
- **Settings_UI**: The settings dialog window implemented in `internal/ui/settings.go`.
- **Refresh_Scheduler**: The `RefreshScheduler` in `internal/scheduler/` that periodically fetches weather data.
- **Interval_Slider**: The slider widget in the Settings_UI that controls the refresh interval in minutes.
- **Validation_Module**: The `Validate` function and associated helpers in `internal/config/validation.go`.

## Requirements

### Requirement 1: Register EasyWetherWidget as a Valid Provider

**User Story:** As a user, I want EasyWetherWidget to be a recognised provider option, so that I can use it to fetch weather data.

#### Acceptance Criteria

1. THE Validation_Module SHALL accept `"EasyWetherWidget"` as a valid value for the `apiConfig.provider` field.
2. THE Config_Service SHALL persist and load the `"EasyWetherWidget"` provider value without data loss.
3. WHEN the `apiConfig.provider` field is set to `"EasyWetherWidget"`, THE Validation_Module SHALL apply the same API key presence check as for `"openweathermap"`.

### Requirement 2: Fetch Weather Data from the EasyWetherWidget API

**User Story:** As a user, I want the application to retrieve current weather data from the EasyWetherWidget API, so that I can see weather information from this provider.

#### Acceptance Criteria

1. WHEN the provider is `"EasyWetherWidget"`, THE Remote_API_Adapter SHALL construct the request URL in the format `https://wagemaker.uk:8043/api/v1/weather/key=<api_key>/<city>,<country_code>`.
2. WHEN the EWW_API returns a successful JSON response, THE Remote_API_Adapter SHALL parse the `Temp` field as the temperature in Celsius (rounded to the nearest integer).
3. WHEN the EWW_API returns a successful JSON response, THE Remote_API_Adapter SHALL parse the `Neighborhood` field as the city name.
4. WHEN the EWW_API returns a successful JSON response, THE Remote_API_Adapter SHALL parse the `Country` field as the region.
5. WHEN the EWW_API returns a successful JSON response, THE Remote_API_Adapter SHALL parse the `FreeText` field as the weather description.
6. WHEN the EWW_API returns a successful JSON response, THE Remote_API_Adapter SHALL map the `FreeText` value to a valid internal icon code.
7. WHEN the EWW_API returns a successful JSON response, THE Remote_API_Adapter SHALL parse the `ObsTimeLocal` field as the local observation time.
8. IF the EWW_API returns a non-200 HTTP status code, THEN THE Remote_API_Adapter SHALL return a descriptive error containing the status code and response body.
9. IF the EWW_API returns malformed JSON, THEN THE Remote_API_Adapter SHALL return a descriptive parse error.

### Requirement 3: Map EasyWetherWidget FreeText to Icon Codes

**User Story:** As a user, I want weather conditions from EasyWetherWidget to display the correct icon, so that I can visually identify the weather at a glance.

#### Acceptance Criteria

1. WHEN the `FreeText` value contains the word "clear", THE Remote_API_Adapter SHALL map the condition to the `clear` icon code.
2. WHEN the `FreeText` value contains the word "cloud", THE Remote_API_Adapter SHALL map the condition to the `cloudy` icon code.
3. WHEN the `FreeText` value contains the word "rain" or "drizzle", THE Remote_API_Adapter SHALL map the condition to the `rain` icon code.
4. WHEN the `FreeText` value contains the word "snow", THE Remote_API_Adapter SHALL map the condition to the `snow` icon code.
5. WHEN the `FreeText` value contains the word "storm" or "thunder", THE Remote_API_Adapter SHALL map the condition to the `storm` icon code.
6. WHEN the `FreeText` value contains the word "fog" or "mist" or "haze", THE Remote_API_Adapter SHALL map the condition to the `fog` icon code.
7. WHEN the `FreeText` value does not match any known keyword, THE Remote_API_Adapter SHALL default to the `partly_cloudy` icon code.
8. THE Remote_API_Adapter SHALL perform case-insensitive matching on the `FreeText` value.

### Requirement 4: Test Connection for EasyWetherWidget

**User Story:** As a user, I want the application to verify my EasyWetherWidget API key before saving settings, so that I receive immediate feedback on invalid credentials.

#### Acceptance Criteria

1. WHEN the provider is `"EasyWetherWidget"`, THE Remote_API_Adapter SHALL perform a lightweight test request to the EWW_API to verify the API key.
2. IF the EWW_API test request returns an HTTP 401 status, THEN THE Remote_API_Adapter SHALL return an "invalid API key" error.
3. IF the EWW_API test request fails due to a network error, THEN THE Remote_API_Adapter SHALL return a descriptive connection error.

### Requirement 5: Provider Selection in Settings UI

**User Story:** As a user, I want to select between OpenWeatherMap and EasyWetherWidget in the settings, so that I can choose my preferred weather data source.

#### Acceptance Criteria

1. WHEN the data source is "Remote API", THE Settings_UI SHALL display a Provider_Selector dropdown with the options "OpenWeatherMap (Free)" and "EasyWetherWidget (Pro)".
2. WHEN the user selects "OpenWeatherMap (Free)" from the Provider_Selector, THE Settings_UI SHALL store `"openweathermap"` as the provider value in the configuration.
3. WHEN the user selects "EasyWetherWidget (Pro)" from the Provider_Selector, THE Settings_UI SHALL store `"EasyWetherWidget"` as the provider value in the configuration.
4. WHEN the settings dialog opens with an existing configuration, THE Provider_Selector SHALL display the currently configured provider.

### Requirement 6: Provider-Dependent Refresh Interval Minimum

**User Story:** As a user, I want the refresh interval to respect each provider's rate limits, so that I do not exceed API quotas.

#### Acceptance Criteria

1. WHEN the user selects "OpenWeatherMap (Free)" from the Provider_Selector, THE Interval_Slider SHALL set its minimum value to 120 minutes.
2. WHEN the user selects "EasyWetherWidget (Pro)" from the Provider_Selector, THE Interval_Slider SHALL set its minimum value to 30 minutes.
3. WHILE the Interval_Slider minimum is 120 minutes and the current slider value is below 120, THE Settings_UI SHALL adjust the slider value to 120 minutes.
4. WHILE the Interval_Slider minimum is 30 minutes and the current slider value is below 30, THE Settings_UI SHALL adjust the slider value to 30 minutes.
5. THE Interval_Slider SHALL retain a maximum value of 120 minutes regardless of the selected provider.
6. THE Validation_Module SHALL reject a refresh interval below 120 minutes when the provider is `"openweathermap"`.
7. THE Validation_Module SHALL reject a refresh interval below 30 minutes when the provider is `"EasyWetherWidget"`.

### Requirement 7: EasyWetherWidget Response Parsing Round-Trip

**User Story:** As a developer, I want to verify that parsing an EasyWetherWidget API response and formatting it back produces consistent data, so that no information is lost during transformation.

#### Acceptance Criteria

1. FOR ALL valid EWW_API JSON responses, parsing the response into a `WeatherData` struct and then reading back the city name, temperature, description, and icon code SHALL produce values consistent with the original JSON fields.
2. FOR ALL valid `FreeText` strings, mapping to an icon code SHALL always produce one of the seven defined icon code constants.
3. FOR ALL valid EWW_API JSON responses containing a `Temp` field, THE Remote_API_Adapter SHALL produce a temperature value equal to `math.Round(Temp)` converted to an integer.
