//go:build windows

package uiwindows

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
	"testing"
	"time"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func TestFontLoadingAndDrawing(t *testing.T) {
	fontBytes, err := assets.Fonts.ReadFile("fonts/segoeui.ttf")
	if err != nil {
		t.Fatalf("failed to read segoeui.ttf: %v", err)
	}

	otf, err := opentype.Parse(fontBytes)
	if err != nil {
		t.Fatalf("failed to parse font: %v", err)
	}

	face, err := opentype.NewFace(otf, &opentype.FaceOptions{
		Size:    16,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		t.Fatalf("failed to create font face: %v", err)
	}
	defer face.Close()

	img := image.NewRGBA(image.Rect(0, 0, 380, 250))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{18, 20, 24, 200}}, image.Point{}, draw.Src)

	d := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{255, 255, 255, 255}),
		Face: face,
		Dot:  fixed.P(20, 40),
	}
	d.DrawString("London 21°C")

	// Verify some pixels were drawn (non-zero alpha)
	pixel := img.RGBAAt(25, 35)
	t.Logf("Sample pixel at (25, 35): R=%d G=%d B=%d A=%d", pixel.R, pixel.G, pixel.B, pixel.A)
	if pixel.A == 0 {
		t.Error("expected non-zero alpha on text pixel")
	}
}

func TestSimpleCardGridModeTodayAndTomorrow(t *testing.T) {
	drawer := NewWidgetDrawer()
	cfg := config.DefaultConfig()
	cfg.DisplayFields = config.DefaultDisplayFields()
	cfg.ViewMode = config.ViewModeSimple
	city := config.CityConfig{Name: "London", Region: "GB", Timezone: "UTC"}

	todayMax := 22.0
	todayMin := 14.0
	tomorrowMax := 25.0
	tomorrowMin := 17.0
	data := &weather.WeatherData{
		CityName:    "London",
		Temperature: 19,
		Description: "Current Clouds",
		IconCode:    weather.IconCloudy,
		Forecast: &weather.ForecastData{
			TodayTempMax:        &todayMax,
			TodayTempMin:        &todayMin,
			TodaySummary:        "Sunny Intervals",
			TodayWeatherCode:    1, // few clouds -> partly cloudy
			TomorrowTempMax:     &tomorrowMax,
			TomorrowTempMin:     &tomorrowMin,
			TomorrowSummary:     "Rain Showers",
			TomorrowWeatherCode: 61, // rain
		},
	}
	results := []weather.WeatherResult{{Data: data}}

	// Live mode
	cfg.DisplayFields.GridMode = "live"
	icon, temp, _, desc := simpleCardWeather(data, cfg.DisplayFields, cfg, nil)
	if temp != "19°C" || desc != "Current Clouds" || icon != weather.IconCloudy {
		t.Fatalf("unexpected live card weather: temp=%s, desc=%s, icon=%s", temp, desc, icon)
	}
	liveMetrics := simpleMetrics(cfg.DisplayFields, data, cfg, nil)
	if len(liveMetrics) == 0 {
		t.Fatal("expected live metrics to be non-empty")
	}

	// Today mode
	cfg.DisplayFields.GridMode = "today"
	icon, temp, feels, desc := simpleCardWeather(data, cfg.DisplayFields, cfg, nil)
	if temp != "22°C" || feels != "14°C" || desc != "Sunny Intervals" || icon != weather.IconPartlyCloudy {
		t.Fatalf("unexpected today card weather: temp=%s, feels=%s, desc=%s, icon=%s", temp, feels, desc, icon)
	}
	todayMetrics := simpleMetrics(cfg.DisplayFields, data, cfg, nil)
	if len(todayMetrics) != 6 {
		t.Fatalf("expected 6 today metrics, got %d", len(todayMetrics))
	}
	if todayMetrics[0].Value != "22°C" || todayMetrics[1].Value != "14°C" {
		t.Fatalf("unexpected today metric values: %v, %v", todayMetrics[0], todayMetrics[1])
	}

	// Tomorrow mode
	cfg.DisplayFields.GridMode = "tomorrow"
	icon, temp, feels, desc = simpleCardWeather(data, cfg.DisplayFields, cfg, nil)
	if temp != "25°C" || feels != "17°C" || desc != "Rain Showers" || icon != weather.IconRain {
		t.Fatalf("unexpected tomorrow card weather: temp=%s, feels=%s, desc=%s, icon=%s", temp, feels, desc, icon)
	}
	tomorrowMetrics := simpleMetrics(cfg.DisplayFields, data, cfg, nil)
	if len(tomorrowMetrics) != 6 {
		t.Fatalf("expected 6 tomorrow metrics, got %d", len(tomorrowMetrics))
	}
	if tomorrowMetrics[0].Value != "25°C" || tomorrowMetrics[1].Value != "17°C" {
		t.Fatalf("unexpected tomorrow metric values: %v, %v", tomorrowMetrics[0], tomorrowMetrics[1])
	}

	// Verify render succeeds in all modes
	now := time.Now()
	for _, mode := range []string{"live", "today", "tomorrow"} {
		cfg.DisplayFields.GridMode = mode
		img := drawer.Render([]config.CityConfig{city}, results, cfg, nil, now)
		if img == nil || img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0 {
			t.Fatalf("render failed for gridMode %s", mode)
		}
	}
}

func TestWrapDescription(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxChars int
		expected []string
	}{
		{
			name:     "empty input",
			input:    "",
			maxChars: 18,
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    "   \t  \n  ",
			maxChars: 18,
			expected: nil,
		},
		{
			name:     "sample forecast 1 - expect a day of partly cloudy with clear spells",
			input:    "Expect A Day Of Partly Cloudy With Clear Spells",
			maxChars: 18,
			expected: []string{
				"Expect A Day Of",
				"Partly Cloudy With",
				"Clear Spells",
			},
		},
		{
			name:     "sample forecast 2 - there will be partly cloudy today",
			input:    "There Will Be Partly Cloudy Today",
			maxChars: 18,
			expected: []string{
				"There Will Be",
				"Partly Cloudy",
				"Today",
			},
		},
		{
			name:     "word longer than maxChars is not broken up",
			input:    "Supercalifragilistic is very long",
			maxChars: 18,
			expected: []string{
				"Supercalifragilistic",
				"is very long",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lines := wrapDescription(tc.input, tc.maxChars)
			if len(lines) != len(tc.expected) {
				t.Fatalf("expected %d lines, got %d: %v", len(tc.expected), len(lines), lines)
			}
			for i, exp := range tc.expected {
				if lines[i] != exp {
					t.Errorf("line %d: expected %q, got %q", i, exp, lines[i])
				}
				// Verify rule 1 & rule 2: no line exceeds maxChars unless a single word exceeds it
				if len([]rune(lines[i])) > tc.maxChars && len(strings.Fields(lines[i])) > 1 {
					t.Errorf("line %d exceeds maxChars %d with multiple words: %q", i, tc.maxChars, lines[i])
				}
			}
		})
	}
}

func TestSimpleCardForecastDescriptionWrappingAndCardWidth(t *testing.T) {
	drawer := NewWidgetDrawer()
	cfg := config.DefaultConfig()
	cfg.DisplayFields = config.DefaultDisplayFields()
	cfg.DisplayFields.GridMode = "today"
	cfg.ViewMode = config.ViewModeSimple

	cities := []config.CityConfig{
		{Name: "London", Region: "GB", Timezone: "UTC"},
		{Name: "New York", Region: "US", Timezone: "America/New_York"},
		{Name: "Tokyo", Region: "JP", Timezone: "Asia/Tokyo"},
	}

	todayMax1 := 9.0
	todayMin1 := 6.0
	todayMax2 := 15.0
	todayMin2 := 9.0
	todayMax3 := 7.0
	todayMin3 := 4.0

	results := []weather.WeatherResult{
		{
			Data: &weather.WeatherData{
				CityName: "London", Temperature: 9,
				Forecast: &weather.ForecastData{
					TodayTempMax: &todayMax1, TodayTempMin: &todayMin1,
					TodaySummary: "Expect A Day Of Partly Cloudy With Clear Spells",
				},
			},
		},
		{
			Data: &weather.WeatherData{
				CityName: "New York", Temperature: 15,
				Forecast: &weather.ForecastData{
					TodayTempMax: &todayMax2, TodayTempMin: &todayMin2,
					TodaySummary: "Expect A Day Of Partly Cloudy With Clear Spells",
				},
			},
		},
		{
			Data: &weather.WeatherData{
				CityName: "Tokyo", Temperature: 7,
				Forecast: &weather.ForecastData{
					TodayTempMax: &todayMax3, TodayTempMin: &todayMin3,
					TodaySummary: "There Will Be Partly Cloudy Today",
				},
			},
		},
	}

	img := drawer.Render(cities, results, cfg, nil, time.Now())
	if img == nil {
		t.Fatal("Render returned nil")
	}

	// Verify all wrapped description lines are compact (<= 120px)
	for _, l := range []string{"Expect A Day Of", "Partly Cloudy With", "Clear Spells", "There Will Be", "Partly Cloudy", "Today"} {
		wDesc := measureText(l, drawer.getFace(false, float64(cfg.GetFontSizeConditions()+2))) + 16
		if wDesc > 120 {
			t.Errorf("expected wrapped line %q to be <= 120px, got %d", l, wDesc)
		}
	}

	// 3 cities with width 160 each and gap 6 between them: 3*160 + 2*6 = 492
	expectedWidth := 3*160 + 2*6
	if img.Bounds().Dx() != expectedWidth {
		t.Errorf("expected wrapped compact width %d, got %d (card expanded excessively)", expectedWidth, img.Bounds().Dx())
	}
}


