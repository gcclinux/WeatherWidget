# Implementation Plan: EasyWetherWidget Provider

## Overview

Add the EasyWetherWidget (EWW) provider to the WeatherWidget desktop application. Implementation proceeds bottom-up: configuration and validation first, then the remote API adapter with icon mapping, then the settings UI wiring, and finally property-based and unit tests. Each task builds on the previous one so there is no orphaned code.

## Tasks

- [x] 1. Register EasyWetherWidget in configuration and update validation
  - [x] 1.1 Add `"EasyWetherWidget"` to `allowedProviders` map and implement provider-dependent refresh interval validation
    - In `internal/config/validation.go`, add `"EasyWetherWidget": true` to the `allowedProviders` map
    - Update the `Validate` function: when `DataSource` is `remote_api` and `APIConfig.Provider` is `"openweathermap"`, reject `RefreshInterval` below 120 with message `"must be at least 120 for openweathermap"`; when provider is `"EasyWetherWidget"`, reject below 30 with message `"must be at least 30 for EasyWetherWidget"`; reject above 120 with message `"must be at most 120"` for both remote API providers
    - Keep the existing `1–60` range check for non-`remote_api` data sources
    - Update the `validateAPIConfig` error message for invalid provider to list both valid providers
    - _Requirements: 1.1, 1.3, 6.6, 6.7_

  - [x] 1.2 Add unit tests for EasyWetherWidget validation rules
    - In `internal/config/validation_test.go`, add tests: `TestValidate_AcceptsEasyWetherWidget` (valid EWW config with interval 30–120 passes), `TestValidate_RejectsEWWIntervalBelow30`, `TestValidate_RejectsOWMIntervalBelow120`, `TestValidate_AcceptsOWMInterval120`, `TestValidate_RejectsIntervalAbove120`
    - _Requirements: 1.1, 6.6, 6.7_

- [x] 2. Implement EasyWetherWidget remote API adapter
  - [x] 2.1 Add EWW response struct, `fetchEWW`, `testEWW`, and `mapEWWFreeTextToIcon` to the remote API adapter
    - In `internal/weather/remoteapi/adapter.go`:
    - Add `const defaultEWWBaseURL = "https://wagemaker.uk:8043"`
    - Update `NewRemoteAPIAdapter` to set `BaseURL` to `defaultEWWBaseURL` when provider is `"EasyWetherWidget"`
    - Add `case "EasyWetherWidget"` to `FetchWeather` and `TestConnection` switch statements
    - Define `ewwResponse` struct with fields: `Temp float64`, `Neighborhood string`, `Country string`, `FreeText string`, `ObsTimeLocal string`
    - Implement `fetchEWW`: construct URL as `{BaseURL}/api/v1/weather/key={apiKey}/{cityName},{region}`, parse JSON, round `Temp` with `math.Round`, map `FreeText` to icon via `mapEWWFreeTextToIcon`, return `WeatherData` using `city.Name` and `city.Region` (consistent with OWM pattern)
    - Implement `testEWW`: test request to `{BaseURL}/api/v1/weather/key={apiKey}/London,GB`, return `"invalid API key"` on 401, descriptive error on other failures
    - Implement `mapEWWFreeTextToIcon`: case-insensitive keyword matching with priority order: storm/thunder → snow → rain/drizzle → fog/mist/haze → cloud → clear → default partly_cloudy
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 2.9, 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 4.1, 4.2, 4.3_

  - [x] 2.2 Add unit tests for EWW adapter
    - In `internal/weather/remoteapi/adapter_test.go`:
    - Add `newEWWServer` helper to create mock EWW HTTP server
    - Add `TestFetchWeather_EWW_Success`: mock server returns valid `ewwResponse`, verify `CityName`, `Region`, `Temperature` (rounded), `Description`, `IconCode`
    - Add `TestFetchWeather_EWW_URLConstruction`: verify the request URL format `key={apiKey}/{city},{region}`
    - Add `TestFetchWeather_EWW_APIError`: mock 401 response, verify error returned
    - Add `TestFetchWeather_EWW_MalformedJSON`: mock server returns invalid JSON, verify parse error
    - Add `TestTestConnection_EWW_Success`: mock 200 response, verify no error
    - Add `TestTestConnection_EWW_InvalidKey`: mock 401 response, verify `"invalid API key"` error
    - Add `TestMapEWWFreeTextToIcon`: table-driven test covering all keyword categories and the default case
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 2.9, 3.1–3.8, 4.1, 4.2, 4.3_

- [x] 3. Checkpoint
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Update Settings UI for provider selection and dynamic interval slider
  - [x] 4.1 Add provider dropdown, display-to-value mapping, and dynamic interval slider to settings UI
    - In `internal/ui/settings.go`:
    - Add `providerDisplayToValue` and `providerValueToDisplay` package-level maps: `"OpenWeatherMap (Free)" ↔ "openweathermap"`, `"EasyWetherWidget (Pro)" ↔ "EasyWetherWidget"`
    - Replace `widget.NewSelect([]string{"OpenWeatherMap"}, nil)` with `widget.NewSelect([]string{"OpenWeatherMap (Free)", "EasyWetherWidget (Pro)"}, nil)`
    - Set initial provider selection from current config using `providerValueToDisplay`
    - Add `providerSelect.OnChanged` handler: for OWM set slider min=120, max=120, value=120; for EWW set slider min=30, max=120, clamp value if below 30
    - Apply initial slider constraints based on current provider when dialog opens
    - Update `buildConfigFromUI` to use `providerDisplayToValue[providerSelect.Selected]` instead of hardcoded `"openweathermap"`
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 6.1, 6.2, 6.3, 6.4, 6.5_

  - [x] 4.2 Add unit tests for provider display-to-value mapping
    - Create `internal/ui/settings_test.go` if it doesn't exist
    - Test that `providerDisplayToValue` maps both display strings to correct internal values
    - Test that `providerValueToDisplay` maps both internal values to correct display strings
    - Test round-trip: display → value → display for both providers
    - _Requirements: 5.2, 5.3_

- [x] 5. Update existing property test generators to include EasyWetherWidget
  - [x] 5.1 Update `config_property_test.go` generators for the new provider
    - In `internal/config/config_property_test.go`:
    - Add `"EasyWetherWidget"` to the `providers` slice
    - Update `genConfig`: when `DataSource` is `remote_api`, generate provider-appropriate refresh intervals (OWM: 120, EWW: 30–120) instead of the current 1–60 range
    - This ensures the existing Property 1 (Config Serialization Round-Trip) covers the new provider
    - _Requirements: 1.2_

  - [x] 5.2 Update `validation_property_test.go` generators for the new provider and interval rules
    - In `internal/config/validation_property_test.go`:
    - Update `genMaybeInvalidConfig`: when `DataSource` is `remote_api`, generate refresh intervals valid for the chosen provider (OWM: 120, EWW: 30–120) for valid configs, and out-of-range values for invalid configs
    - Update `genMaybeInvalidAPIConfig`: include `"EasyWetherWidget"` in the valid provider set so it is not accidentally treated as invalid
    - Ensure the invalid provider generator excludes both `"openweathermap"` and `"EasyWetherWidget"`
    - _Requirements: 1.1, 1.3_

- [x] 6. Checkpoint
  - Ensure all tests pass, ask the user if questions arise.

- [x] 7. Write property-based tests for EWW adapter
  - [x] 7.1 Write property test: EWW Response Parsing Consistency (Property 1)
    - Create `internal/weather/remoteapi/adapter_property_test.go`
    - **Property 1: EWW Response Parsing Consistency**
    - **Validates: Requirements 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 7.1, 7.3**
    - Generate arbitrary `ewwResponse` structs (random `Temp` float, `Neighborhood` string, `Country` string, `FreeText` string, `ObsTimeLocal` string), serve via `httptest.Server`, call `FetchWeather`, assert: `Temperature == int(math.Round(Temp))`, `CityName == city.Name`, `Region == city.Region`, `Description == FreeText`, `IconCode ∈ AllIconCodes`

  - [x] 7.2 Write property test: FreeText-to-Icon Totality (Property 2)
    - In `internal/weather/remoteapi/adapter_property_test.go`
    - **Property 2: FreeText-to-Icon Totality**
    - **Validates: Requirements 2.6, 7.2**
    - Generate arbitrary strings, call `mapEWWFreeTextToIcon`, assert result is a member of `weather.AllIconCodes`

  - [x] 7.3 Write property test: FreeText Keyword Mapping Correctness (Property 3)
    - In `internal/weather/remoteapi/adapter_property_test.go`
    - **Property 3: FreeText Keyword Mapping Correctness**
    - **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7**
    - For each keyword tier, generate strings containing the target keyword but no higher-priority keywords, assert the correct icon code is returned

  - [x] 7.4 Write property test: FreeText Case-Insensitive Matching (Property 4)
    - In `internal/weather/remoteapi/adapter_property_test.go`
    - **Property 4: FreeText Case-Insensitive Matching**
    - **Validates: Requirements 3.8**
    - Generate arbitrary strings, assert `mapEWWFreeTextToIcon(s) == mapEWWFreeTextToIcon(strings.ToUpper(s)) == mapEWWFreeTextToIcon(strings.ToLower(s))`

  - [x] 7.5 Write property test: Provider-Dependent Interval Validation (Property 5)
    - In `internal/config/validation_property_test.go`
    - **Property 5: Provider-Dependent Interval Validation**
    - **Validates: Requirements 6.6, 6.7**
    - Generate valid configs with `DataSource=remote_api`: for OWM with interval < 120, assert `Validate` returns `refreshInterval` error; for EWW with interval < 30, assert error; for EWW with interval 30–120, assert no `refreshInterval` error

- [x] 8. Final checkpoint
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- The project uses Go with `pgregory.net/rapid` for property-based testing and `net/http/httptest` for mock HTTP servers
- All new code follows existing patterns in the codebase (provider dispatch via switch, validation via allowedProviders map)
