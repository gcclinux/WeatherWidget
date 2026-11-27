//go:build windows

package uiwindows

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
	"time"

	xdraw "golang.org/x/image/draw"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"
)

func filledCanvas(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = uint8(i), uint8(i/7), uint8(i/13), 255
	}
	return img
}

func TestRoundedRectMatchesReference(t *testing.T) {
	sizes := [][2]int{{1, 1}, {2, 2}, {3, 9}, {9, 3}, {10, 10}, {24, 24}, {25, 40}, {100, 30}, {300, 120}, {600, 200}}
	radii := []int{0, 1, 2, 5, 10, 12, 30, 80}
	colors := []color.RGBA{{255, 255, 255, 100}, {0, 0, 0, 120}, {10, 200, 250, 255}}
	for _, sz := range sizes {
		for _, r := range radii {
			for _, c := range colors {
				for _, off := range [][2]int{{0, 0}, {5, 3}, {-4, -2}} {
					want, got := filledCanvas(sz[0]+20, sz[1]+20), filledCanvas(sz[0]+20, sz[1]+20)
					refStrokeRoundedRect(want, off[0], off[1], sz[0], sz[1], r, c, 1)
					strokeRoundedRect(got, off[0], off[1], sz[0], sz[1], r, c, 1)
					if string(want.Pix) != string(got.Pix) {
						t.Fatalf("stroke differs: size %v radius %d color %v offset %v", sz, r, c, off)
					}
					want, got = filledCanvas(sz[0]+20, sz[1]+20), filledCanvas(sz[0]+20, sz[1]+20)
					refFillRoundedRect(want, off[0], off[1], sz[0], sz[1], r, c)
					fillRoundedRect(got, off[0], off[1], sz[0], sz[1], r, c)
					if string(want.Pix) != string(got.Pix) {
						t.Fatalf("fill differs: size %v radius %d color %v offset %v", sz, r, c, off)
					}
				}
			}
		}
	}
}

func TestDrawImageUsesCachedPrescaleWithSameLook(t *testing.T) {
	drawer := NewWidgetDrawer()
	icon := drawer.loadIcon("icons/thermometer.png")
	rgba, ok := icon.(*image.RGBA)
	if !ok {
		t.Fatalf("loadIcon returned %T, want *image.RGBA", icon)
	}
	for _, size := range [][2]int{{10, 18}, {14, 35}, {64, 64}, {84, 84}, {126, 126}} {
		got := filledCanvas(200, 200)
		want := filledCanvas(200, 200)
		drawImage(got, icon, 7, 9, size[0], size[1])
		xdraw.BiLinear.Scale(want, image.Rect(7, 9, 7+size[0], 9+size[1]), rgba, rgba.Bounds(), draw.Over, nil)
		for i := range got.Pix {
			d := int(got.Pix[i]) - int(want.Pix[i])
			if d < -2 || d > 2 {
				t.Fatalf("size %v: pixel byte %d differs by %d", size, i, d)
			}
		}
		if scaledIcon(rgba, size[0], size[1]) != scaledIcon(rgba, size[0], size[1]) {
			t.Fatalf("size %v: scaled icon not cached", size)
		}
	}
	if scaledIcon(rgba, 10, 18) == scaledIcon(rgba, 20, 36) {
		t.Fatal("different target sizes must not share a cache entry")
	}
	if b := scaledIcon(rgba, 20, 36).Bounds(); b.Dx() != 20 || b.Dy() != 36 {
		t.Fatalf("scaled bounds = %v", b)
	}
}

func TestNextClockDelay(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 250_000_000, time.UTC)
	if got := nextClockDelay(true, now); got != animationFrameInterval {
		t.Fatalf("animating delay = %v, want %v", got, animationFrameInterval)
	}
	if got := nextClockDelay(false, now); got != 755*time.Millisecond {
		t.Fatalf("idle delay = %v, want 755ms", got)
	}
	if animationFrameInterval != 100*time.Millisecond {
		t.Fatalf("animation interval = %v, want 100ms (10 fps)", animationFrameInterval)
	}
}

func animatingFor(t *testing.T, mutate func(cfg *config.Config), data *weather.WeatherData) bool {
	t.Helper()
	drawer := NewWidgetDrawer()
	cfg := config.DefaultConfig()
	cfg.NoBackground = true
	cfg.DisplayFields = &config.DisplayFields{ShowCity: true, ShowIcon: true, ShowTemp: true}
	cfg.PollutionFields = &config.PollutionFields{}
	mutate(cfg)
	cities := []config.CityConfig{{Name: "London", Timezone: "UTC"}}
	results := []weather.WeatherResult{{Data: data}}
	drawer.Render(cities, results, cfg, nil, time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))
	return drawer.Animating()
}

func TestAnimatingOnlyWhenSomethingMoves(t *testing.T) {
	aqi := 2
	clear := &weather.WeatherData{IconCode: weather.IconClear, AQI: &aqi}
	rain := &weather.WeatherData{IconCode: weather.IconRain, AQI: &aqi}
	for _, view := range []config.ViewMode{config.ViewModeSimple, config.ViewModeEnhanced} {
		view := view
		setView := func(cfg *config.Config) { cfg.ViewMode = view }
		if animatingFor(t, setView, clear) {
			t.Errorf("%v: static PNG theme must not animate", view)
		}
		original := func(cfg *config.Config) { setView(cfg); cfg.IconTheme = config.IconThemeOriginal }
		if !animatingFor(t, original, clear) {
			t.Errorf("%v: Original theme clear GIF should animate", view)
		}
		if animatingFor(t, original, rain) {
			t.Errorf("%v: Original theme rain is a PNG and must not animate", view)
		}
		if animatingFor(t, func(cfg *config.Config) { original(cfg); cfg.DisplayFields.ShowIcon = false }, clear) {
			t.Errorf("%v: hidden icon must not animate", view)
		}
	}
	ticker := func(cfg *config.Config) {
		cfg.ViewMode = config.ViewModeEnhanced
		cfg.PollutionFields = &config.PollutionFields{ShowAQI: true, ShowTicker: true}
	}
	if !animatingFor(t, ticker, rain) {
		t.Error("ticker with data should animate")
	}
	if animatingFor(t, ticker, &weather.WeatherData{IconCode: weather.IconRain}) {
		t.Error("ticker with nothing to scroll must not animate")
	}
	if animatingFor(t, ticker, nil) {
		t.Error("ticker without weather data must not animate")
	}
	if animatingFor(t, func(cfg *config.Config) { ticker(cfg); cfg.PollutionFields.ShowTicker = false }, rain) {
		t.Error("ticker disabled must not animate")
	}
}

func BenchmarkRenderFiveCities(b *testing.B) {
	for _, view := range []config.ViewMode{config.ViewModeEnhanced, config.ViewModeSimple} {
		b.Run(string(view), func(b *testing.B) {
			drawer := NewWidgetDrawer()
			cfg := config.DefaultConfig()
			cfg.ViewMode = view
			cfg.PollutionFields = &config.PollutionFields{ShowAQI: true, ShowTicker: true}
			zones := []string{"Europe/London", "Asia/Tokyo", "America/New_York", "Australia/Sydney", "America/Sao_Paulo"}
			var cities []config.CityConfig
			var results []weather.WeatherResult
			aqi := 2
			for i, z := range zones {
				cities = append(cities, config.CityConfig{Name: "City" + string(rune('A'+i)), Timezone: z})
				results = append(results, weather.WeatherResult{Data: &weather.WeatherData{IconCode: weather.IconClear, AQI: &aqi}})
			}
			now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
			drawer.Render(cities, results, cfg, nil, now)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				drawer.Render(cities, results, cfg, nil, now.Add(time.Duration(i)*time.Millisecond))
			}
		})
	}
}
