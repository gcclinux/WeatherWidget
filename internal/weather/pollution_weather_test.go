package weather

import (
	"strings"
	"testing"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
)

func TestPlanPollutionTickerWithWeather_Advice(t *testing.T) {
	pm25 := 5.0
	fields := &config.PollutionFields{ShowPM25: true, ShowTicker: true, TickerProfile: config.TickerProfileEveryday, TickerWeatherTips: true}
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

func TestPlanPollutionTicker_ProTickerWithoutSelectedPollutants(t *testing.T) {
	pm25 := 5.0
	aqi := 1
	dataWithPollution := PollutionData{
		AQI:  &aqi,
		PM25: &pm25,
	}
	dataFree := PollutionData{} // Free / Demo: no pollution data from provider

	// 1. FREE / Demo = No pollution & No ticker
	fieldsNoPollutionSelected := &config.PollutionFields{
		ShowTicker:    true,
		TickerProfile: config.TickerProfileEveryday,
		// All ShowXXX are false
	}
	if got := PlanPollutionTicker(fieldsNoPollutionSelected, dataFree); got != nil {
		t.Fatalf("Free/Demo: expected nil ticker, got %+v", got)
	}

	// 2. PRO with Ticker ON, but NO individual pollutions selected:
	// should emit ONLY Opportunity and Context (no additional pollutant items)
	itemsNoPollutants := PlanPollutionTicker(fieldsNoPollutionSelected, dataWithPollution)
	if len(itemsNoPollutants) != 2 {
		t.Fatalf("Pro with no pollutants selected: want 2 items (Opp & Context), got %d: %+v", len(itemsNoPollutants), itemsNoPollutants)
	}
	if itemsNoPollutants[0].Kind != TickerItemOpportunity {
		t.Errorf("item 0 kind = %v, want Opportunity", itemsNoPollutants[0].Kind)
	}
	if itemsNoPollutants[1].Kind != TickerItemContext {
		t.Errorf("item 1 kind = %v, want Context", itemsNoPollutants[1].Kind)
	}

	// 3. PRO with Ticker ON, and additional ticker pollutions selected (e.g. PM2.5):
	// should emit Opportunity, Context, PLUS the selected ticker pollutant definitions,
	// independent of card Data Settings (Show*).
	fieldsWithPM25 := &config.PollutionFields{
		ShowTicker:    true,
		ShowAQI:       true, // card data setting should not appear in ticker unless TickerAQI is set
		TickerPM25:    true,
		TickerProfile: config.TickerProfileEveryday,
	}
	itemsWithPM25 := PlanPollutionTicker(fieldsWithPM25, dataWithPollution)
	if len(itemsWithPM25) != 3 {
		t.Fatalf("Pro with PM2.5 selected: want 3 items (Opp, Context, PM2.5), got %d: %+v", len(itemsWithPM25), itemsWithPM25)
	}
	if itemsWithPM25[2].Metric != MetricPM25 {
		t.Errorf("item 2 metric = %v, want PM25", itemsWithPM25[2].Metric)
	}
	rows := PlanPollutionRows(fieldsWithPM25, dataWithPollution)
	if len(rows) != 1 || rows[0].Metric != MetricAQI {
		t.Fatalf("PlanPollutionRows should only reflect Data Settings (ShowAQI), got %+v", rows)
	}
}

func TestPlanPollutionTicker_LocalizedTitles(t *testing.T) {
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		t.Fatalf("locale manager: %v", err)
	}

	fields := &config.PollutionFields{
		ShowTicker:         true,
		TickerNewsToday:    true,
		TickerNewsPrecip:   true,
		TickerNewsTomorrow: true,
		TickerAlert:        true,
	}

	maxTemp, minTemp := 24.0, 16.0
	precipSum := 1.5
	w := &WeatherData{
		CityName: "Holambra",
		Forecast: &ForecastData{
			TodayTempMax:       &maxTemp,
			TodayTempMin:       &minTemp,
			TodayPrecipProbMax: 80,
			TodayPrecipSumMM:   &precipSum,
			TomorrowTempMax:    &maxTemp,
			TomorrowTempMin:    &minTemp,
		},
		NewsItems: []NewsItem{
			{Category: "alert", Symbol: "Alert", StatusText: "Severe Storm"},
			{Category: "today_outlook", Symbol: "Today's Outlook", StatusText: "High 24.0°C / Low 16.0°C"},
			{Category: "precipitation", Symbol: "Precipitation Watch", StatusText: "80% Chance of Rain"},
			{Category: "tomorrow_outlook", Symbol: "Tomorrow's Forecast", StatusText: "High 24.0°C / Low 16.0°C"},
		},
	}

	// 1. Dutch (nl-NL) specific check matching the user's report
	if err := lm.SetLocale("nl-NL"); err != nil {
		t.Fatalf("set nl-NL: %v", err)
	}
	items := PlanPollutionTickerWithWeather(fields, w, config.TemperatureUnitCelsius, lm)
	if len(items) < 4 {
		t.Fatalf("expected at least 4 items, got %d", len(items))
	}

	symbols := make(map[string]string)
	for _, item := range items {
		symbols[string(item.Kind)+":"+item.Symbol] = item.Symbol
	}

	wantDutchToday := "Verwachting vandaag"
	foundToday := false
	for _, it := range items {
		if it.Symbol == wantDutchToday {
			foundToday = true
			if !strings.Contains(it.StatusText, "Max temp.") || !strings.Contains(it.StatusText, "Min temp.") {
				t.Errorf("Dutch today outlook statusText = %q, want localized Max/Min temp", it.StatusText)
			}
		}
	}
	if !foundToday {
		t.Errorf("Dutch today outlook title not found, got items: %+v", items)
	}

	// 2. Test across all 12 locales that titles and headings exist and are not empty
	for _, locale := range lm.AvailableLocales() {
		if err := lm.SetLocale(locale.Code); err != nil {
			t.Errorf("%s set locale: %v", locale.Code, err)
			continue
		}
		locItems := PlanPollutionTickerWithWeather(fields, w, config.TemperatureUnitCelsius, lm)
		for _, item := range locItems {
			if item.Symbol == "" {
				t.Errorf("%s: item symbol is empty", locale.Code)
			}
			// When locale is not en-GB, English fallback "Today's Outlook" should not appear as symbol
			if locale.Code != "en-GB" && item.Symbol == "Today's Outlook" {
				t.Errorf("%s: item symbol still has untranslated English 'Today\\'s Outlook'", locale.Code)
			}
		}
	}
}



