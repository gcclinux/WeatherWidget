package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
)

func TestProviderDisplayToValue(t *testing.T) {
	tests := []struct {
		display string
		value   string
	}{
		{"OpenWeatherMap (Free)", "openweathermap"},
		{"EasyWeatherWidget (Pro)", "easyweatherwidget"},
	}
	for _, tt := range tests {
		t.Run(tt.display, func(t *testing.T) {
			got, ok := providerDisplayToValue[tt.display]
			if !ok {
				t.Fatalf("providerDisplayToValue missing key %q", tt.display)
			}
			if got != tt.value {
				t.Errorf("providerDisplayToValue[%q] = %q, want %q", tt.display, got, tt.value)
			}
		})
	}
}

func TestProviderValueToDisplay(t *testing.T) {
	tests := []struct {
		value   string
		display string
	}{
		{"openweathermap", "OpenWeatherMap (Free)"},
		{"easyweatherwidget", "EasyWeatherWidget (Pro)"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, ok := providerValueToDisplay[tt.value]
			if !ok {
				t.Fatalf("providerValueToDisplay missing key %q", tt.value)
			}
			if got != tt.display {
				t.Errorf("providerValueToDisplay[%q] = %q, want %q", tt.value, got, tt.display)
			}
		})
	}
}

func TestProviderMappingRoundTrip(t *testing.T) {
	// display → value → display
	for display, value := range providerDisplayToValue {
		t.Run(display, func(t *testing.T) {
			backToDisplay, ok := providerValueToDisplay[value]
			if !ok {
				t.Fatalf("providerValueToDisplay missing key %q (mapped from display %q)", value, display)
			}
			if backToDisplay != display {
				t.Errorf("round-trip failed: %q → %q → %q, want %q", display, value, backToDisplay, display)
			}
		})
	}
}

func TestUIManagerT_WithLocaleManager(t *testing.T) {
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		t.Fatalf("failed to create LocaleManager: %v", err)
	}

	um := &UIManager{lm: lm}

	got := um.t("settings.title")
	if got != "WeatherWidget Settings" {
		t.Errorf("t(settings.title) = %q, want %q", got, "WeatherWidget Settings")
	}
}

func TestUIManagerT_WithoutLocaleManager(t *testing.T) {
	um := &UIManager{}

	got := um.t("settings.title")
	if got != "settings.title" {
		t.Errorf("t(settings.title) without lm = %q, want %q", got, "settings.title")
	}
}

func TestUIManagerTArgs_WithLocaleManager(t *testing.T) {
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		t.Fatalf("failed to create LocaleManager: %v", err)
	}

	um := &UIManager{lm: lm}

	got := um.tFmt("settings.interval.format", 30)
	if got != "30 min" {
		t.Errorf("tFmt(settings.interval.format, 30) = %q, want %q", got, "30 min")
	}
}

func TestUIManagerTArgs_WithoutLocaleManager(t *testing.T) {
	um := &UIManager{}

	got := um.tFmt("settings.interval.format", 30)
	// Without a locale manager, tFmt falls back to fmt.Sprintf with the key as format
	// The key "settings.interval.format" doesn't contain %d, so Sprintf returns it with extra args
	if got == "" {
		t.Error("tFmt should return a non-empty string even without locale manager")
	}
}

func TestBuildConfigFromUI_IncludesLocale(t *testing.T) {
	test.NewApp()

	providerSelect := widget.NewSelect([]string{"OpenWeatherMap (Free)"}, nil)
	providerSelect.SetSelected("OpenWeatherMap (Free)")

	apiKeyEntry := widget.NewEntry()
	apiKeyEntry.SetText("test-key")

	intervalSlider := widget.NewSlider(10, 120)
	intervalSlider.SetValue(30)

	state := &settingsState{
		cities: []config.CityConfig{
			{Name: "London", Region: "UK", Timezone: "Europe/London"},
		},
		selectedLang: "pt-BR",
	}

	positionValueMap := map[string]string{
		"Top-Left": "top-left", "Top-Right": "top-right",
		"Bottom-Left": "bottom-left", "Bottom-Right": "bottom-right",
	}
	positionRadio := widget.NewRadioGroup([]string{"Bottom-Right"}, nil)
	positionRadio.SetSelected("Bottom-Right")

	monitorSelect := widget.NewSelect([]string{"Monitor 1"}, nil)
	monitorSelect.SetSelected("Monitor 1")

	opacityRadio := widget.NewRadioGroup([]string{"100%"}, nil)
	opacityRadio.SetSelected("100%")
	opacityMap := map[string]int{"100%": 100}

	current := config.DefaultConfig()

	cfg := buildConfigFromUI(
		providerSelect, apiKeyEntry, intervalSlider,
		state, positionValueMap, positionRadio, monitorSelect,
		opacityRadio, opacityMap, current,
	)

	if cfg.Locale != "pt-BR" {
		t.Errorf("buildConfigFromUI Locale = %q, want %q", cfg.Locale, "pt-BR")
	}
}

func TestBuildConfigFromUI_DefaultLocale(t *testing.T) {
	test.NewApp()

	providerSelect := widget.NewSelect([]string{"OpenWeatherMap (Free)"}, nil)
	providerSelect.SetSelected("OpenWeatherMap (Free)")

	apiKeyEntry := widget.NewEntry()

	intervalSlider := widget.NewSlider(10, 120)
	intervalSlider.SetValue(120)

	state := &settingsState{
		cities:       []config.CityConfig{{Name: "Test", Region: "US"}},
		selectedLang: "", // empty should default to en-GB
	}

	positionValueMap := map[string]string{
		"Bottom-Right": "bottom-right",
	}
	positionRadio := widget.NewRadioGroup([]string{"Bottom-Right"}, nil)
	positionRadio.SetSelected("Bottom-Right")

	monitorSelect := widget.NewSelect([]string{"Monitor 1"}, nil)
	monitorSelect.SetSelected("Monitor 1")

	opacityRadio := widget.NewRadioGroup([]string{"100%"}, nil)
	opacityRadio.SetSelected("100%")
	opacityMap := map[string]int{"100%": 100}

	current := config.DefaultConfig()

	cfg := buildConfigFromUI(
		providerSelect, apiKeyEntry, intervalSlider,
		state, positionValueMap, positionRadio, monitorSelect,
		opacityRadio, opacityMap, current,
	)

	if cfg.Locale != "en-GB" {
		t.Errorf("buildConfigFromUI Locale = %q, want %q", cfg.Locale, "en-GB")
	}
}

// buildConfigFromUIHelper is a test helper that calls buildConfigFromUI with
// sensible defaults, overriding only the settingsState so tests can focus on
// the selectedUnit field.
func buildConfigFromUIHelper(t *testing.T, state *settingsState) *config.Config {
	t.Helper()
	test.NewApp()

	providerSelect := widget.NewSelect([]string{"OpenWeatherMap (Free)"}, nil)
	providerSelect.SetSelected("OpenWeatherMap (Free)")

	apiKeyEntry := widget.NewEntry()
	apiKeyEntry.SetText("test-key")

	intervalSlider := widget.NewSlider(10, 120)
	intervalSlider.SetValue(120)

	positionValueMap := map[string]string{
		"Bottom-Right": "bottom-right",
	}
	positionRadio := widget.NewRadioGroup([]string{"Bottom-Right"}, nil)
	positionRadio.SetSelected("Bottom-Right")

	monitorSelect := widget.NewSelect([]string{"Monitor 1"}, nil)
	monitorSelect.SetSelected("Monitor 1")

	opacityRadio := widget.NewRadioGroup([]string{"100%"}, nil)
	opacityRadio.SetSelected("100%")
	opacityMap := map[string]int{"100%": 100}

	current := config.DefaultConfig()

	return buildConfigFromUI(
		providerSelect, apiKeyEntry, intervalSlider,
		state, positionValueMap, positionRadio, monitorSelect,
		opacityRadio, opacityMap, current,
	)
}

// TestBuildConfigFromUI_TemperatureUnit_Celsius verifies that selecting Celsius
// writes TemperatureUnitCelsius to the returned Config.
// Validates: Requirements 2.3
func TestBuildConfigFromUI_TemperatureUnit_Celsius(t *testing.T) {
	state := &settingsState{
		cities:       []config.CityConfig{{Name: "London", Region: "UK"}},
		selectedLang: "en-GB",
		selectedUnit: config.TemperatureUnitCelsius,
	}

	cfg := buildConfigFromUIHelper(t, state)

	if cfg.TemperatureUnit != config.TemperatureUnitCelsius {
		t.Errorf("TemperatureUnit = %q, want %q", cfg.TemperatureUnit, config.TemperatureUnitCelsius)
	}
}

// TestBuildConfigFromUI_TemperatureUnit_Fahrenheit verifies that selecting
// Fahrenheit writes TemperatureUnitFahrenheit to the returned Config.
// Validates: Requirements 2.3
func TestBuildConfigFromUI_TemperatureUnit_Fahrenheit(t *testing.T) {
	state := &settingsState{
		cities:       []config.CityConfig{{Name: "New York", Region: "US"}},
		selectedLang: "en-GB",
		selectedUnit: config.TemperatureUnitFahrenheit,
	}

	cfg := buildConfigFromUIHelper(t, state)

	if cfg.TemperatureUnit != config.TemperatureUnitFahrenheit {
		t.Errorf("TemperatureUnit = %q, want %q", cfg.TemperatureUnit, config.TemperatureUnitFahrenheit)
	}
}

// TestBuildConfigFromUI_TemperatureUnit_InvalidNormalizesToCelsius verifies
// that an invalid/empty selectedUnit is normalized to Celsius in the returned Config.
// Validates: Requirements 2.3
func TestBuildConfigFromUI_TemperatureUnit_InvalidNormalizesToCelsius(t *testing.T) {
	state := &settingsState{
		cities:       []config.CityConfig{{Name: "Paris", Region: "FR"}},
		selectedLang: "en-GB",
		selectedUnit: config.TemperatureUnit(""), // empty / invalid
	}

	cfg := buildConfigFromUIHelper(t, state)

	if cfg.TemperatureUnit != config.TemperatureUnitCelsius {
		t.Errorf("TemperatureUnit = %q, want %q (empty should normalize to celsius)", cfg.TemperatureUnit, config.TemperatureUnitCelsius)
	}
}
