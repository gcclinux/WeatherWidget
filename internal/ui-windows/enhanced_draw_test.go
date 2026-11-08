//go:build windows

package uiwindows

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"
	"unsafe"

	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"
)

func TestEnhancedBackgroundAndTransparency(t *testing.T) {
	drawer := NewWidgetDrawer()
	cfg := config.DefaultConfig()
	cities := []config.CityConfig{{Name: "London", Region: "GB", Timezone: "UTC"}}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	cfg.NoBackground = true
	transparent := drawer.Render(cities, nil, cfg, nil, now)
	if got := transparent.RGBAAt(10, 160).A; got != 0 {
		t.Fatalf("transparent card interior alpha = %d, want 0", got)
	}
	cfg.NoBackground = false
	cfg.Opacity = 100
	day := drawer.Render(cities, nil, cfg, nil, now)
	night := drawer.Render(cities, nil, cfg, nil, now.Add(12*time.Hour))
	if got := day.RGBAAt(10, 160).A; got != 255 {
		t.Fatalf("background alpha = %d, want 255", got)
	}
	if bytes.Equal(day.Pix, night.Pix) {
		t.Fatal("day and night cards should differ")
	}
	cfg.Opacity = 50
	faded := drawer.Render(cities, nil, cfg, nil, now)
	if got := faded.RGBAAt(10, 160).A; got < 126 || got > 128 {
		t.Fatalf("50%% background alpha = %d, want approximately 127", got)
	}
}

func TestEnhancedSelectedFieldsAndPollution(t *testing.T) {
	drawer := NewWidgetDrawer()
	cfg := config.DefaultConfig()
	cfg.NoBackground = true
	cfg.DisplayFields = &config.DisplayFields{}
	cfg.PollutionFields = &config.PollutionFields{}
	cities := []config.CityConfig{{Name: "London", Region: "GB", Timezone: "UTC"}}
	aqi, value := 2, 0.0
	data := &weather.WeatherData{AQI: &aqi, CO: &value, NO: &value, NO2: &value, O3: &value, SO2: &value, NH3: &value, PM25: &value, PM10: &value}
	results := []weather.WeatherResult{{Data: data}}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	empty := drawer.Render(cities, results, cfg, nil, now)
	if empty.Bounds().Dx() != 550 || empty.Bounds().Dy() != 172 {
		t.Fatalf("empty card size = %v", empty.Bounds())
	}
	cfg.DisplayFields.ShowHumidity = true
	withHumidity := drawer.Render(cities, results, cfg, nil, now)
	if bytes.Equal(empty.Pix, withHumidity.Pix) {
		t.Fatal("enabling humidity did not change rendering")
	}
	cfg.PollutionFields = &config.PollutionFields{ShowAQI: true, ShowCO: true, ShowNO: true, ShowNO2: true, ShowO3: true, ShowSO2: true, ShowNH3: true, ShowPM25: true, ShowPM10: true}
	rows := weather.PlanPollutionRows(cfg.PollutionFields, weather.PollutionOf(data), nil)
	if len(rows) != 9 {
		t.Fatalf("got %d pollution rows, want 9", len(rows))
	}
	full := drawer.Render(cities, results, cfg, nil, now)
	if full.Bounds().Dx() != 822 || full.Bounds().Dy() != 236 {
		t.Fatalf("nine-metric card size = %v", full.Bounds())
	}
	for i := 0; i < 9; i++ {
		x := 6 + i*(full.Bounds().Dx()-12)/9
		r := image.Rect(x, enhancedPollutionTop, x+90, 232).Intersect(full.Bounds())
		found := false
		for y := r.Min.Y; y < r.Max.Y && !found; y++ {
			for px := r.Min.X; px < r.Max.X; px++ {
				if full.RGBAAt(px, y).A != 0 {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("pollution cell %d was empty", i)
		}
	}
}

func TestEnhancedInfoDoesNotOverlapMetrics(t *testing.T) {
	drawer := NewWidgetDrawer()
	city := config.CityConfig{Name: "Broxburn", Region: "GB", Timezone: "UTC"}
	now := time.Date(2026, 11, 6, 13, 57, 23, 0, time.UTC)
	feels := 10.0
	lm, err := i18n.NewLocaleManager(i18n.LocaleFS)
	if err != nil {
		t.Fatal(err)
	}
	if err := lm.SetLocale("de-DE"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		description string
		fontSize    int
		locale      *i18n.LocaleManager
	}{
		{"screenshot", "overcast clouds", 32, nil},
		{"long description", "exceptionally long weather conditions", 32, nil},
		{"large temperature", "overcast clouds", 60, nil},
		{"translated", "overcast clouds", 32, lm},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.NoBackground = true
			cfg.FontSizeTempIcon = tc.fontSize
			cfg.DisplayFields = &config.DisplayFields{
				ShowTime: true, ShowDate: true, ShowTemp: true, ShowDesc: true,
			}
			result := weather.WeatherResult{Data: &weather.WeatherData{
				Temperature: 10, FeelsLike: &feels, Description: tc.description,
			}}
			infoWidth := drawer.enhancedInfoWidth(city, &result, cfg, tc.locale, now)
			gridLeft := enhancedInfoLeft + infoWidth + enhancedInfoGap
			img := drawer.Render([]config.CityConfig{city}, []weather.WeatherResult{result}, cfg, tc.locale, now)
			if img.Bounds().Dx() < gridLeft+enhancedMetricsWidth {
				t.Fatalf("card width %d leaves insufficient space for metrics at %d", img.Bounds().Dx(), gridLeft)
			}
			for y := 30; y < 165; y++ {
				for x := gridLeft - enhancedInfoGap; x < img.Bounds().Dx()-5; x++ {
					if img.RGBAAt(x, y).A != 0 {
						t.Fatalf("information content entered metric gap or grid at (%d,%d), grid starts at %d", x, y, gridLeft)
					}
				}
			}
			if tc.name == "screenshot" && enhancedInfoLeft+infoWidth/2 >= 167 {
				t.Fatal("weather information was not moved left")
			}
		})
	}
}

func TestPollutionIconsKeepAspectRatio(t *testing.T) {
	drawer := NewWidgetDrawer()
	for _, metric := range weather.PollutionMetricOrder {
		icon := drawer.loadIcon("air/" + weather.AirIconFile(metric))
		if icon == nil {
			t.Fatalf("missing icon for metric %d", metric)
		}
		box := image.Rect(4, 3, 34, 33)
		actual := image.NewRGBA(image.Rect(0, 0, 40, 40))
		expected := image.NewRGBA(actual.Bounds())
		drawImageContained(actual, icon, box)
		drawImage(expected, icon, 4, 7, 30, 21)
		if !bytes.Equal(actual.Pix, expected.Pix) {
			t.Errorf("metric %d was not rendered at its 365:250 aspect ratio", metric)
		}
	}
}

func TestPollutionRowKeepsTextWithinShorterCard(t *testing.T) {
	drawer := NewWidgetDrawer()
	cfg := config.DefaultConfig()
	cfg.PollutionFields = &config.PollutionFields{ShowAQI: true, ShowCO: true}
	aqi, co := 2, 215.1
	data := &weather.WeatherData{AQI: &aqi, CO: &co}
	rows := weather.PlanPollutionRows(cfg.PollutionFields, weather.PollutionOf(data), nil)
	_, height := enhancedCardSize(rows)
	rect := image.Rect(6, enhancedPollutionTop, enhancedWidth-6, height-4)
	actual := image.NewRGBA(image.Rect(0, 0, enhancedWidth, height))
	drawer.drawEnhancedPollution(actual, rect, cfg, data, nil)
	face := drawer.getFace(true, 12)
	slots := 7
	cell := rect.Dx() / slots
	for i, row := range rows {
		left := rect.Max.X - (len(rows)-i)*cell
		right := left + cell
		if row.Metric == weather.MetricAQI {
			left, right = rect.Min.X, rect.Min.X+110
		}
		center := (left + right) / 2
		expected := image.NewRGBA(image.Rect(0, 0, enhancedWidth, height+20))
		drawCentered(expected, row.ValueText, center, rect.Min.Y+61, face, enhancedWhite)
		for y := rect.Min.Y + 48; y < expected.Bounds().Max.Y; y++ {
			for x := left; x < right; x++ {
				want := expected.RGBAAt(x, y)
				if want.A != 0 && (y >= rect.Max.Y || actual.RGBAAt(x, y) != want) {
					t.Fatalf("metric %d value is clipped or missing at (%d,%d)", row.Metric, x, y)
				}
			}
		}
	}
}

func TestLayeredWindowUsesPremultipliedPixels(t *testing.T) {
	w, err := NewWidget(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Destroy()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	w.UpdateSurface(img)
	if w.pBits == nil {
		t.Fatal("DIB was not allocated")
	}
	pixel := unsafe.Slice((*byte)(w.pBits), 4)
	want := []byte{25, 50, 100, 128}
	for i := range want {
		if pixel[i] != want[i] {
			t.Fatalf("BGRA = %v, want %v", pixel, want)
		}
	}
}

func TestWeatherIconThemesAndAnimation(t *testing.T) {
	drawer := NewWidgetDrawer()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	day := "icons/" + weather.MapConditionToIconWithTheme(weather.IconClear, now, config.IconThemeNew)
	night := "icons/" + weather.MapConditionToIconWithTheme(weather.IconClear, now.Add(12*time.Hour), config.IconThemeNew)
	if day == night || drawer.weatherIcon(day, now) == nil || drawer.weatherIcon(night, now) == nil {
		t.Fatal("expected distinct embedded day and night icons")
	}
	original := "icons/" + weather.MapConditionToIconWithTheme(weather.IconClear, now, config.IconThemeOriginal)
	if drawer.weatherIcon(original, now) == nil {
		t.Fatal("original weather icon is missing")
	}
	anim := drawer.animations[original]
	if anim == nil || len(anim.frames) < 2 || anim.total <= 0 {
		t.Fatal("original clear icon should provide an animated GIF")
	}
}

func TestTickerRespectsSelectionAndMoves(t *testing.T) {
	drawer := NewWidgetDrawer()
	cfg := config.DefaultConfig()
	cfg.NoBackground = true
	cfg.DisplayFields = &config.DisplayFields{}
	cfg.PollutionFields = &config.PollutionFields{ShowAQI: true}
	aqi := 2
	data := &weather.WeatherData{AQI: &aqi}
	rect := image.Rect(0, 0, 400, 25)
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	hidden := image.NewRGBA(rect)
	drawer.drawEnhancedTicker(hidden, rect, cfg, data, nil, now, 0)
	if !bytes.Equal(hidden.Pix, make([]byte, len(hidden.Pix))) {
		t.Fatal("disabled ticker drew content")
	}
	cfg.PollutionFields.ShowTicker = true
	found := false
	for i := 0; i < 30; i++ {
		current := image.NewRGBA(rect)
		drawer.drawEnhancedTicker(current, rect, cfg, data, nil, now.Add(time.Duration(i)*time.Second), 0)
		if !bytes.Equal(current.Pix, hidden.Pix) {
			found = true
			next := image.NewRGBA(rect)
			drawer.drawEnhancedTicker(next, rect, cfg, data, nil, now.Add(time.Duration(i)*time.Second+time.Second), 0)
			if bytes.Equal(current.Pix, next.Pix) {
				t.Fatal("ticker did not move")
			}
			break
		}
	}
	if !found {
		t.Fatal("enabled ticker did not draw a message")
	}
}

func TestFallbackFontContainsCJKAndTamil(t *testing.T) {
	drawer := NewWidgetDrawer()
	face, ok := drawer.getFace(false, 14).(*fallbackFace)
	if !ok {
		t.Fatal("expected a fallback font face")
	}
	for _, r := range []rune{'漢', 'あ', 'அ'} {
		if face.faceFor(r) == face.faces[0] {
			t.Errorf("no fallback selected for %q", r)
		}
		found := false
		for _, source := range face.fonts[1:] {
			if glyph, err := source.GlyphIndex(&face.buf, r); err == nil && glyph != 0 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no embedded font contains %q", r)
		}
	}
}

func TestSimpleCardRespectsFieldAndPollutionSelection(t *testing.T) {
	drawer := NewWidgetDrawer()
	cfg := config.DefaultConfig()
	cfg.ViewMode = config.ViewModeSimple
	cfg.DisplayFields = &config.DisplayFields{}
	cfg.PollutionFields = &config.PollutionFields{}
	value := 0.0
	cities := []config.CityConfig{{Name: "London", Timezone: "UTC"}}
	results := []weather.WeatherResult{{Data: &weather.WeatherData{CO: &value}}}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	empty := drawer.Render(cities, results, cfg, nil, now)
	cfg.DisplayFields.ShowWindGust = true
	cfg.PollutionFields.ShowCO = true
	visible := drawer.Render(cities, results, cfg, nil, now)
	if visible.Bounds().Dy() != empty.Bounds().Dy()+22+23 {
		t.Fatalf("simple card height = %d, want %d", visible.Bounds().Dy(), empty.Bounds().Dy()+22+23)
	}
}

func TestComputeEnhancedMetricEdges(t *testing.T) {
	// 1. Forecast mode: small text width -> should shrink to minMid (112, which is 140 - 20%)
	// and outer columns expand equally from 88 to 102.
	forecastEdges := computeEnhancedMetricEdges(60, 316)
	expectedForecast := [4]int{0, 102, 214, 316}
	if forecastEdges != expectedForecast {
		t.Errorf("forecast edges = %v, want %v", forecastEdges, expectedForecast)
	}
	midW := forecastEdges[2] - forecastEdges[1]
	if midW != 112 {
		t.Errorf("forecast middle column width = %d, want 112 (-20%% of 140)", midW)
	}

	// 2. Real-time mode with long wind string (e.g. 150px) -> should cap at currentMid (140)
	liveEdges := computeEnhancedMetricEdges(150, 316)
	expectedLive := [4]int{0, 88, 228, 316}
	if liveEdges != expectedLive {
		t.Errorf("live edges = %v, want %v", liveEdges, expectedLive)
	}
	midWLive := liveEdges[2] - liveEdges[1]
	if midWLive != 140 {
		t.Errorf("live middle column width = %d, want 140", midWLive)
	}

	// 3. Intermediate width (e.g. needed 126 in 316-scale) -> should adjust smoothly
	intermediateEdges := computeEnhancedMetricEdges(126, 316)
	midWInter := intermediateEdges[2] - intermediateEdges[1]
	if midWInter != 126 {
		t.Errorf("intermediate middle column width = %d, want 126", midWInter)
	}
}

func TestEnhancedMiddleGridAutoResizeForecastVsLive(t *testing.T) {
	drawer := NewWidgetDrawer()
	cfg := config.DefaultConfig()
	cfg.ViewMode = config.ViewModeEnhanced
	cfg.DisplayFields = config.DefaultDisplayFields()

	city := config.CityConfig{Name: "London", Region: "GB", Timezone: "UTC"}
	todayMax, todayMin, uvMax, gustMax := 11.0, 5.0, 1.1, 7.0
	precipProb, precipSum := 0, 0.0
	now := time.Date(2026, 11, 8, 11, 2, 22, 0, time.UTC)

	data := &weather.WeatherData{
		CityName:    "London",
		Temperature: 9,
		Forecast: &weather.ForecastData{
			TodayTempMax:       &todayMax,
			TodayTempMin:       &todayMin,
			TodayUVMax:         &uvMax,
			TodayGustMax:       &gustMax,
			TodayPrecipProbMax: precipProb,
			TodayPrecipSumMM:   &precipSum,
		},
	}
	results := []weather.WeatherResult{{Data: data}}

	// Today's forecast: short strings in col 1 ("Low Temp 5°C", "Peak UV 1.1")
	cfg.DisplayFields.GridMode = "today"
	visibleForecast := enhancedVisibleMetrics(cfg.DisplayFields, data, cfg, nil)
	neededForecast := drawer.enhancedMiddleColumnNeededWidth(visibleForecast, data)
	edgesForecast := computeEnhancedMetricEdges(neededForecast, 325)
	if edgesForecast[2]-edgesForecast[1] != 112 {
		t.Errorf("expected forecast middle column width 112 (-20%%), got %d", edgesForecast[2]-edgesForecast[1])
	}

	// Render succeeds and produces non-nil image
	imgForecast := drawer.Render([]config.CityConfig{city}, results, cfg, nil, now)
	if imgForecast == nil {
		t.Fatal("Render returned nil in today forecast mode")
	}

	// Real-time mode: long wind with direction arrow in col 1 ("15.2 km/h ↗ WNW")
	cfg.DisplayFields.GridMode = "live"
	dataLive := &weather.WeatherData{
		CityName:      "London",
		Temperature:   9,
		WindSpeed:     15.2,
		WindDirection: 290, // WNW
		Pressure:      1013.2,
	}
	resultsLive := []weather.WeatherResult{{Data: dataLive}}
	visibleLive := enhancedVisibleMetrics(cfg.DisplayFields, dataLive, cfg, nil)
	neededLive := drawer.enhancedMiddleColumnNeededWidth(visibleLive, dataLive)
	edgesLive := computeEnhancedMetricEdges(neededLive, 325)
	if edgesLive[2]-edgesLive[1] != 140 {
		t.Errorf("expected live middle column width 140, got %d", edgesLive[2]-edgesLive[1])
	}

	imgLive := drawer.Render([]config.CityConfig{city}, resultsLive, cfg, nil, now)
	if imgLive == nil {
		t.Fatal("Render returned nil in live mode")
	}
}

