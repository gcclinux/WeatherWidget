package panel

import (
	"testing"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"

	"fyne.io/fyne/v2/test"
)

func TestNewCityPanel(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)
	if p == nil {
		t.Fatal("NewCityPanel returned nil")
	}
	if p.container == nil {
		t.Error("container is nil")
	}
	if p.iconWidget == nil {
		t.Error("iconWidget is nil")
	}
	if p.tempText == nil {
		t.Error("tempText is nil")
	}
	if p.descText == nil {
		t.Error("descText is nil")
	}
	if p.cityText == nil {
		t.Error("cityText is nil")
	}
	if p.timeText == nil {
		t.Error("timeText is nil")
	}
	if p.errorIcon == nil {
		t.Error("errorIcon is nil")
	}

	// Placeholder content checks (nil lm uses hardcoded defaults).
	if p.tempText.Text != "--°C" {
		t.Errorf("tempText placeholder = %q, want %q", p.tempText.Text, "--°C")
	}
	if p.cityText.Text != "City, RG" {
		t.Errorf("cityText placeholder = %q, want %q", p.cityText.Text, "City, RG")
	}
	if p.errorIcon.Visible() {
		t.Error("errorIcon should be hidden initially")
	}
}

func TestCityPanel_Container(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)
	c := p.Container()
	if c == nil {
		t.Fatal("Container() returned nil")
	}
	if c != p.container {
		t.Error("Container() should return the internal container")
	}
}

func TestCityPanel_Update(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)

	data := &weather.WeatherData{
		CityName:    "Holambra",
		Region:      "SP",
		Temperature: 24,
		Description: "Partial Sunny",
		IconCode:    weather.IconClear,
		LocalTime:   time.Now(),
		FetchedAt:   time.Now(),
	}

	p.Update(data, "celsius", config.WindSpeedUnitKmh)

	if p.tempText.Text != "24°C" {
		t.Errorf("tempText = %q, want %q", p.tempText.Text, "24°C")
	}
	if p.descText.Text != "Partial Sunny" {
		t.Errorf("descText = %q, want %q", p.descText.Text, "Partial Sunny")
	}
	if p.cityText.Text != "Holambra, SP" {
		t.Errorf("cityText = %q, want %q", p.cityText.Text, "Holambra, SP")
	}
	if p.errorIcon.Visible() {
		t.Error("errorIcon should be hidden after successful update")
	}
}

func TestCityPanel_UpdateNil(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)
	// Should not panic.
	p.Update(nil, "celsius", config.WindSpeedUnitKmh)
	if p.tempText.Text != "--°C" {
		t.Errorf("tempText should remain placeholder after nil update, got %q", p.tempText.Text)
	}
}

func TestCityPanel_ShowError(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)

	// Non-stale error.
	p.ShowError(false)
	if !p.errorIcon.Visible() {
		t.Error("errorIcon should be visible after ShowError(false)")
	}

	// Stale error.
	p.ShowError(true)
	if !p.errorIcon.Visible() {
		t.Error("errorIcon should be visible after ShowError(true)")
	}
	if p.descText.Text != "Data may be stale" {
		t.Errorf("descText = %q, want %q", p.descText.Text, "Data may be stale")
	}
}

func TestCityPanel_ShowErrorThenUpdate(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)
	p.ShowError(false)
	if !p.errorIcon.Visible() {
		t.Error("errorIcon should be visible after ShowError")
	}

	data := &weather.WeatherData{
		CityName:    "Tokyo",
		Region:      "JP",
		Temperature: 18,
		Description: "Cloudy",
		IconCode:    weather.IconCloudy,
		LocalTime:   time.Now(),
		FetchedAt:   time.Now(),
	}
	p.Update(data, "celsius", config.WindSpeedUnitKmh)
	if p.errorIcon.Visible() {
		t.Error("errorIcon should be hidden after successful Update")
	}
}

func TestCityPanel_StartStopClock(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)

	p.StartClock("America/Sao_Paulo")

	// Give the ticker a moment to fire.
	time.Sleep(1200 * time.Millisecond)

	text := p.timeText.Text
	if text == "--/--/---- - --:--:--" {
		t.Error("timeText should have been updated by the clock")
	}

	p.StopClock()

	// Verify double-stop doesn't panic.
	p.StopClock()
}

func TestCityPanel_StartClockReplacesExisting(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)

	p.StartClock("UTC")
	time.Sleep(100 * time.Millisecond)

	// Starting a new clock should stop the old one without panic.
	p.StartClock("America/New_York")
	time.Sleep(100 * time.Millisecond)

	p.StopClock()
}

func TestCityPanel_Rerender_NilData(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)
	// Should not panic and should be a no-op when no data has been cached.
	p.Rerender(config.TemperatureUnitFahrenheit, config.WindSpeedUnitKmh)
	if p.tempText.Text != "--°C" {
		t.Errorf("tempText should remain placeholder after Rerender with nil data, got %q", p.tempText.Text)
	}
}

func TestCityPanel_Rerender_WithCachedData(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)

	data := &weather.WeatherData{
		CityName:    "London",
		Region:      "UK",
		Temperature: 20, // 20°C = 68°F
		Description: "Sunny",
		IconCode:    weather.IconClear,
		LocalTime:   time.Now(),
		FetchedAt:   time.Now(),
	}

	// First update with Celsius.
	p.Update(data, config.TemperatureUnitCelsius, config.WindSpeedUnitKmh)
	if p.tempText.Text != "20°C" {
		t.Errorf("after Update with celsius, tempText = %q, want %q", p.tempText.Text, "20°C")
	}

	// Rerender with Fahrenheit — should use cached data without a new fetch.
	p.Rerender(config.TemperatureUnitFahrenheit, config.WindSpeedUnitKmh)
	if p.tempText.Text != "68°F" {
		t.Errorf("after Rerender with fahrenheit, tempText = %q, want %q", p.tempText.Text, "68°F")
	}
}
