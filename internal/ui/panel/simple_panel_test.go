package panel

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"
)

func TestNewSimpleCityPanel(t *testing.T) {
	test.NewApp()
	p := NewSimpleCityPanel(nil)
	if p == nil {
		t.Fatal("NewSimpleCityPanel returned nil")
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
	if p.dateText == nil {
		t.Error("dateText is nil")
	}
	if p.humidityText == nil {
		t.Error("humidityText is nil")
	}
	if p.windText == nil {
		t.Error("windText is nil")
	}
	if p.windDirText == nil {
		t.Error("windDirText is nil")
	}
	if p.pressureText == nil {
		t.Error("pressureText is nil")
	}
	if p.errorIcon == nil {
		t.Error("errorIcon is nil")
	}

	// Verify column min width is at least 160 dip
	minSize := p.Container().MinSize()
	if minSize.Width < 160 {
		t.Errorf("expected min width >= 160, got %f", minSize.Width)
	}

	// Placeholder content checks
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

func TestSimpleCityPanel_Update(t *testing.T) {
	test.NewApp()
	p := NewSimpleCityPanel(nil)

	data := &weather.WeatherData{
		CityName:      "Broxburn",
		Region:        "GB",
		Temperature:   16,
		Description:   "Overcast Clouds",
		Humidity:      74,
		WindSpeed:     7.7,
		WindDirection: 240,
		WindGust:      0.0,
		DewPoint:      11.8,
		Pressure:      1010,
		UVIndex:       2.6,
		IconCode:      weather.IconCloudy,
		LocalTime:     time.Now(),
		FetchedAt:     time.Now(),
	}

	p.Update(data, config.TemperatureUnitCelsius, config.WindSpeedUnitKmh)

	if p.tempText.Text != "16°C" {
		t.Errorf("tempText = %q, want %q", p.tempText.Text, "16°C")
	}
	if p.descText.Text != "Overcast Clouds" {
		t.Errorf("descText = %q, want %q", p.descText.Text, "Overcast Clouds")
	}
	if p.cityText.Text != "Broxburn, GB" {
		t.Errorf("cityText = %q, want %q", p.cityText.Text, "Broxburn, GB")
	}
	if p.humidityText.Text != "💧 Hum 74%" {
		t.Errorf("humidityText = %q, want %q", p.humidityText.Text, "💧 Hum 74%")
	}
	if p.pressureText.Text != "🌡 1010 hPa" {
		t.Errorf("pressureText = %q, want %q", p.pressureText.Text, "🌡 1010 hPa")
	}
	if p.uvIndexText.Text != "☀ UV 2.6" {
		t.Errorf("uvIndexText = %q, want %q", p.uvIndexText.Text, "☀ UV 2.6")
	}
}

func TestSimpleCityPanel_ApplyDisplayFieldsAndPollution(t *testing.T) {
	test.NewApp()
	p := NewSimpleCityPanel(nil)

	df := &config.DisplayFields{
		ShowCity:     true,
		ShowIcon:     true,
		ShowTemp:     true,
		ShowDesc:     true,
		ShowHumidity: false,
		ShowWind:     false,
		ShowTime:     true,
		ShowDate:     false,
	}

	p.ApplyDisplayFields(df)
	p.ApplyPollutionFields(&config.PollutionFields{ShowAQI: true}) // should not panic

	if p.container == nil {
		t.Fatal("container should not be nil after ApplyDisplayFields")
	}

	// Test pollution data population
	aqiVal := 2
	coVal := 12.34
	no2Val := 8.2
	o3Val := 45.1
	pm25Val := 15.0
	pm10Val := 22.5
	data := &weather.WeatherData{
		CityName:    "London",
		Temperature: 20,
		AQI:         &aqiVal,
		CO:          &coVal,
		NO2:         &no2Val,
		O3:          &o3Val,
		PM25:        &pm25Val,
		PM10:        &pm10Val,
	}

	pf := &config.PollutionFields{
		ShowAQI:  true,
		ShowCO:   true,
		ShowNO2:  true,
		ShowO3:   true,
		ShowPM25: true,
		ShowPM10: true,
	}
	p.ApplyPollutionFields(pf)
	p.Update(data, config.TemperatureUnitCelsius, config.WindSpeedUnitKmh)

	if !p.pollutionBox.Visible() {
		t.Error("pollutionBox should be visible when metrics are enabled and data is present")
	}

	aqiCell := p.pollutionCells[weather.MetricAQI]
	if !aqiCell.container.Visible() {
		t.Error("AQI cell container should be visible")
	}
	if aqiCell.value.Text != "AQI: 2 (Fair)" {
		t.Errorf("AQI value = %q, want %q", aqiCell.value.Text, "AQI: 2 (Fair)")
	}

	coCell := p.pollutionCells[weather.MetricCO]
	if !coCell.container.Visible() {
		t.Error("CO cell container should be visible")
	}
	if coCell.value.Text != "CO: 12.3 µg/m³" {
		t.Errorf("CO value = %q, want %q", coCell.value.Text, "CO: 12.3 µg/m³")
	}

	// Disable all pollution fields
	p.ApplyPollutionFields(&config.PollutionFields{})
	if aqiCell.container.Visible() {
		t.Error("AQI cell should be hidden when ShowAQI is false")
	}
	if p.pollutionBox.Visible() {
		t.Error("pollutionBox should be hidden when no pollution metrics are active")
	}
}

func TestSimpleCityPanel_TextColors(t *testing.T) {
	test.NewApp()
	p := NewSimpleCityPanel(nil)

	expectedTextColor := currentSimpleTextColor()
	expectedSepColor := currentSimpleSeparatorColor()

	if p.tempText.Color != expectedTextColor {
		t.Errorf("tempText.Color = %v, want %v", p.tempText.Color, expectedTextColor)
	}
	if p.cityText.Color != expectedTextColor {
		t.Errorf("cityText.Color = %v, want %v", p.cityText.Color, expectedTextColor)
	}
	if p.descText.Color != expectedTextColor {
		t.Errorf("descText.Color = %v, want %v", p.descText.Color, expectedTextColor)
	}
	if p.timeText.Color != expectedTextColor {
		t.Errorf("timeText.Color = %v, want %v", p.timeText.Color, expectedTextColor)
	}
	if p.separatorLine == nil {
		t.Fatal("separatorLine should not be nil")
	}
	if p.separatorLine.FillColor != expectedSepColor {
		t.Errorf("separatorLine.FillColor = %v, want %v", p.separatorLine.FillColor, expectedSepColor)
	}

	// Update with weather and pollution
	aqiVal := 1
	data := &weather.WeatherData{
		CityName:    "Edinburgh",
		Temperature: 15,
		AQI:         &aqiVal,
	}
	p.ApplyPollutionFields(&config.PollutionFields{ShowAQI: true})
	p.Update(data, config.TemperatureUnitCelsius, config.WindSpeedUnitKmh)

	aqiCell := p.pollutionCells[weather.MetricAQI]
	if aqiCell.value.Color != expectedTextColor {
		t.Errorf("pollution cell value color = %v, want %v", aqiCell.value.Color, expectedTextColor)
	}

	// Verify updateTextColors refreshes all elements
	p.updateTextColors()
	if p.tempText.Color != expectedTextColor {
		t.Errorf("after updateTextColors, tempText.Color = %v, want %v", p.tempText.Color, expectedTextColor)
	}
}
