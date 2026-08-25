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
	// rain has rain_day.png, no gif
	anim, staticData, path, err := loadIconAsset("rain")
	if err != nil {
		t.Fatalf("loadIconAsset(rain) returned error: %v", err)
	}
	if anim != nil {
		t.Errorf("expected static asset for rain, got anim=%v", anim)
	}
	if len(staticData) == 0 {
		t.Errorf("expected staticData to be non-empty for rain")
	}
	if path != "icons/day/rain_day.png" && path != "icons/original/rain.png" {
		t.Errorf("expected path icons/day/rain_day.png, got %s", path)
	}

	// Test day and night explicit paths
	_, dayData, dayPath, dayErr := loadIconAsset("day/clear_day")
	if dayErr != nil || len(dayData) == 0 || dayPath != "icons/day/clear_day.png" {
		t.Errorf("loadIconAsset(day/clear_day) failed: err=%v, path=%s", dayErr, dayPath)
	}

	_, nightData, nightPath, nightErr := loadIconAsset("night/clear_night")
	if nightErr != nil || len(nightData) == 0 || nightPath != "icons/night/clear_night.png" {
		t.Errorf("loadIconAsset(night/clear_night) failed: err=%v, path=%s", nightErr, nightPath)
	}
}

func TestCityPanel_AnimatedIconLifecycle(t *testing.T) {
	test.NewApp()
	p := NewCityPanel(nil)

	// Update with moon icon from original
	p.updateIcon("moon")
	if p.iconWidget.Image == nil && p.iconWidget.Resource == nil {
		t.Errorf("expected iconWidget.Image or Resource to be set for moon icon")
	}

	// Allow animation loop to run briefly
	time.Sleep(150 * time.Millisecond)

	// Switch to static day icon
	p.updateIcon("day/rain_day")
	if p.iconWidget.Resource == nil {
		t.Errorf("expected iconWidget.Resource to be set for static rain icon")
	}

	// Switch to nighttime weather data
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
	if p.iconWidget.Resource == nil {
		t.Errorf("expected static night icon resource on iconWidget")
	}

	// Stop animation & clock
	p.StopClock()
	p.StopAnimation()
}
