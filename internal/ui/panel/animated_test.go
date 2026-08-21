package panel

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"
)

func TestLoadIconAsset_AnimatedGIF(t *testing.T) {
	// Test loading moon.gif
	anim, staticData, path, err := loadIconAsset("moon")
	if err != nil {
		t.Fatalf("loadIconAsset(moon) returned error: %v", err)
	}
	if anim == nil {
		t.Fatalf("expected animatedFrames for moon.gif, got nil (staticData=%v, path=%q)", staticData != nil, path)
	}
	if len(anim.frames) <= 1 {
		t.Errorf("expected moon.gif to have > 1 frames, got %d", len(anim.frames))
	}
	if len(anim.delays) != len(anim.frames) {
		t.Errorf("frames and delays length mismatch: %d vs %d", len(anim.frames), len(anim.delays))
	}

	// Test loading cloudy_moon.gif
	anim2, staticData2, path2, err2 := loadIconAsset("cloudy_moon")
	if err2 != nil {
		t.Fatalf("loadIconAsset(cloudy_moon) returned error: %v", err2)
	}
	if anim2 == nil {
		t.Fatalf("expected animatedFrames for cloudy_moon.gif, got nil (staticData=%v, path=%q)", staticData2 != nil, path2)
	}
	if len(anim2.frames) <= 1 {
		t.Errorf("expected cloudy_moon.gif to have > 1 frames, got %d", len(anim2.frames))
	}
}

func TestLoadIconAsset_StaticFallback(t *testing.T) {
	// rain has rain.png, no gif
	anim, staticData, path, err := loadIconAsset("rain")
	if err != nil {
		t.Fatalf("loadIconAsset(rain) returned error: %v", err)
	}
	if anim != nil {
		t.Errorf("expected static asset for rain, got anim=%v", anim)
	}
	if len(staticData) == 0 {
		t.Errorf("expected staticData to be non-empty for rain.png")
	}
	if path != "icons/rain.png" {
		t.Errorf("expected path icons/rain.png, got %s", path)
	}
}

func TestCityPanel_AnimatedIconLifecycle(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)

	// Update with moon icon
	p.updateIcon("moon")
	if p.iconWidget.Image == nil {
		t.Errorf("expected iconWidget.Image to be set for animated moon icon")
	}

	// Allow animation loop to run briefly
	time.Sleep(150 * time.Millisecond)

	// Switch to static icon
	p.updateIcon("rain")
	if p.iconWidget.Resource == nil {
		t.Errorf("expected iconWidget.Resource to be set for static rain icon")
	}

	// Switch back to cloudy_moon
	nightTime := time.Date(2026, 8, 20, 22, 0, 0, 0, time.UTC)
	data := &weather.WeatherData{
		CityName:    "London",
		Region:      "UK",
		Temperature: 15,
		Description: "Cloudy",
		IconCode:    weather.IconCloudy,
		LocalTime:   nightTime,
		FetchedAt:   time.Now(),
	}
	p.Update(data, config.TemperatureUnitCelsius, config.WindSpeedUnitKmh)

	time.Sleep(150 * time.Millisecond)
	if p.iconWidget.Image == nil {
		t.Errorf("expected animated cloudy_moon frame on iconWidget.Image")
	}

	// Stop animation & clock
	p.StopClock()
	p.StopAnimation()
}
