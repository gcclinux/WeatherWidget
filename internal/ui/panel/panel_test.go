package panel

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"weatherwidget/internal/weather"
)

func TestNewCityPanel(t *testing.T) {
	test.NewApp()
	p := NewCityPanel()
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
	if p.descLabel == nil {
		t.Error("descLabel is nil")
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

	// Placeholder content checks.
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
	p := NewCityPanel()
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
	p := NewCityPanel()

	data := &weather.WeatherData{
		CityName:    "Holambra",
		Region:      "SP",
		Temperature: 24,
		Description: "Partial Sunny",
		IconCode:    weather.IconClear,
		LocalTime:   time.Now(),
		FetchedAt:   time.Now(),
	}

	p.Update(data)

	if p.tempText.Text != "24°C" {
		t.Errorf("tempText = %q, want %q", p.tempText.Text, "24°C")
	}
	if p.descLabel.Text != "Partial Sunny" {
		t.Errorf("descLabel = %q, want %q", p.descLabel.Text, "Partial Sunny")
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
	p := NewCityPanel()
	// Should not panic.
	p.Update(nil)
	if p.tempText.Text != "--°C" {
		t.Errorf("tempText should remain placeholder after nil update, got %q", p.tempText.Text)
	}
}

func TestCityPanel_ShowError(t *testing.T) {
	test.NewApp()
	p := NewCityPanel()

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
	if p.descLabel.Text != "Data may be stale" {
		t.Errorf("descLabel = %q, want %q", p.descLabel.Text, "Data may be stale")
	}
}

func TestCityPanel_ShowErrorThenUpdate(t *testing.T) {
	test.NewApp()
	p := NewCityPanel()
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
	p.Update(data)
	if p.errorIcon.Visible() {
		t.Error("errorIcon should be hidden after successful Update")
	}
}

func TestCityPanel_StartStopClock(t *testing.T) {
	test.NewApp()
	p := NewCityPanel()

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
	p := NewCityPanel()

	p.StartClock("UTC")
	time.Sleep(100 * time.Millisecond)

	// Starting a new clock should stop the old one without panic.
	p.StartClock("America/New_York")
	time.Sleep(100 * time.Millisecond)

	p.StopClock()
}
