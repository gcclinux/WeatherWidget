package weather

import (
	"strings"
	"testing"
	"time"

	"weatherwidget/internal/config"
)

func TestPlanPollutionTickerWithWeather_Advice(t *testing.T) {
	pm25 := 5.0
	fields := &config.PollutionFields{ShowPM25: true, ShowTicker: true, TickerProfile: config.TickerProfileEveryday}
	feels := 3.0
	night := time.Date(2026, 10, 22, 20, 12, 0, 0, time.UTC)
	day := time.Date(2026, 10, 22, 13, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		w    WeatherData
		want []string // each must appear in the tail
		not  []string // none may appear in the tail
	}{
		{"rain", WeatherData{CityName: "Holambra", Temperature: 26, IconCode: IconRain, Description: "light rain", LocalTime: day, PM25: &pm25},
			[]string{"umbrella"}, []string{"torch", "warm"}},
		{"cold night", WeatherData{CityName: "Lundie", Temperature: 5, FeelsLike: &feels, IconCode: IconCloudy, LocalTime: night, PM25: &pm25},
			[]string{"3°C", "warm", "and take a torch"}, []string{"umbrella"}},
		{"dark but mild", WeatherData{CityName: "Lundie", Temperature: 12, IconCode: IconClear, LocalTime: night, PM25: &pm25},
			[]string{"torch"}, []string{"umbrella", "warm"}},
	}
	for _, c := range cases {
		w := c.w
		items := PlanPollutionTickerWithWeather(fields, &w, config.TemperatureUnitCelsius)
		if len(items) == 0 || items[0].Kind != TickerItemOpportunity {
			t.Fatalf("%s: items = %+v, want opportunity first", c.name, items)
		}
		tail := items[0].SourcesText
		if strings.Contains(tail, "Everyday") {
			t.Errorf("%s: %q should not repeat the profile name", c.name, tail)
		}
		for _, s := range c.want {
			if !strings.Contains(tail, s) {
				t.Errorf("%s: %q missing %q", c.name, tail, s)
			}
		}
		for _, s := range c.not {
			if strings.Contains(tail, s) {
				t.Errorf("%s: %q unexpectedly contains %q", c.name, tail, s)
			}
		}
	}

	// Mild, bright weather falls back to a friendly "good to go" line.
	mild := WeatherData{CityName: "Broxburn", Temperature: 18, IconCode: IconClear, LocalTime: day, PM25: &pm25}
	items := PlanPollutionTickerWithWeather(fields, &mild, config.TemperatureUnitCelsius)
	if len(items) == 0 || items[0].SourcesText == "" || strings.Contains(items[0].SourcesText, "torch") {
		t.Fatalf("mild day items = %+v, want friendly fallback line", items)
	}
}

func TestPlanPollutionTicker_AllProfilesAndLevels(t *testing.T) {
	profiles := []config.TickerProfile{
		config.TickerProfileEveryday,
		config.TickerProfileActive,
		config.TickerProfileFamily,
		config.TickerProfileSensitive,
		config.TickerProfileVentilation,
	}

	for _, prof := range profiles {
		for level := 1; level <= 5; level++ {
			msg := tickerGuidance(prof, level, nil)
			if msg == "" {
				t.Errorf("missing guidance for profile=%s level=%d", prof, level)
			}
		}
	}

	for _, m := range PollutionMetricOrder {
		tip := pollutantTipText(m, nil)
		if tip == "" {
			t.Errorf("missing tip for metric=%v", m)
		}
	}
}

func TestStripEmoji(t *testing.T) {
	input := "Fresh, clean air 🌿 • Feels like 3°C, so wrap up warm 🧥 and take a torch 🔦"
	got := StripEmoji(input)
	want := "Fresh, clean air • Feels like 3°C, so wrap up warm and take a torch"
	if got != want {
		t.Errorf("StripEmoji() = %q, want %q", got, want)
	}
}

