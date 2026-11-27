//go:build windows

package uiwindows

import (
	"runtime"
	"testing"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"
)

func fiveCityInputs() ([]config.CityConfig, []weather.WeatherResult) {
	zones := []string{"Europe/London", "Asia/Tokyo", "America/New_York", "Australia/Sydney", "America/Sao_Paulo"}
	var cities []config.CityConfig
	var results []weather.WeatherResult
	aqi := 2
	for i, z := range zones {
		cities = append(cities, config.CityConfig{Name: "City" + string(rune('A'+i)), Timezone: z})
		results = append(results, weather.WeatherResult{Data: &weather.WeatherData{IconCode: weather.IconClear, AQI: &aqi}})
	}
	return cities, results
}

func heapStats() (alloc, inuse uint64) {
	runtime.GC()
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc, ms.HeapInuse
}

// TestMemoryHarness logs HeapAlloc/HeapInuse after startup and after N frames.
// Run with: go test ./internal/ui-windows -run TestMemoryHarness -v
func TestMemoryHarness(t *testing.T) {
	const frames = 300
	a0, i0 := heapStats()
	drawer := NewWidgetDrawer()
	a1, i1 := heapStats()
	t.Logf("startup (NewWidgetDrawer): HeapAlloc %+.2f MB, HeapInuse %+.2f MB", (float64(a1)-float64(a0))/1e6, (float64(i1)-float64(i0))/1e6)
	cities, results := fiveCityInputs()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	for _, view := range []config.ViewMode{config.ViewModeEnhanced, config.ViewModeSimple} {
		cfg := config.DefaultConfig()
		cfg.ViewMode = view
		cfg.PollutionFields = &config.PollutionFields{ShowAQI: true, ShowTicker: true}
		drawer.Render(cities, results, cfg, nil, now)
		b0, _ := heapStats()
		var ms0, ms1 runtime.MemStats
		runtime.ReadMemStats(&ms0)
		for i := 0; i < frames; i++ {
			drawer.Render(cities, results, cfg, nil, now.Add(time.Duration(i*33)*time.Millisecond))
		}
		runtime.ReadMemStats(&ms1)
		b1, j1 := heapStats()
		t.Logf("%s after %d frames: live HeapAlloc %.2f MB (delta %+.2f MB), HeapInuse %.2f MB, allocated %.2f MB/frame, GCs %d",
			view, frames, float64(b1)/1e6, (float64(b1)-float64(b0))/1e6, float64(j1)/1e6,
			float64(ms1.TotalAlloc-ms0.TotalAlloc)/frames/1e6, ms1.NumGC-ms0.NumGC)
	}
	for _, l := range drawer.fallbackFonts {
		t.Logf("fallback font %s loaded: %v", l.name, l.font != nil)
	}
}
