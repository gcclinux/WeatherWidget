//go:build windows

package uiwindows

import (
	"image"
	"image/color"
	"testing"
	"time"

	"weatherwidget/internal/config"
	"weatherwidget/internal/weather"
)

func TestWidgetLifecycle(t *testing.T) {
	var posChangedX, posChangedY int
	var contextMenuCalled bool

	w, err := NewWidget(func(x, y int) {
		posChangedX = x
		posChangedY = y
	}, func() {
		contextMenuCalled = true
	})
	if err != nil {
		t.Fatalf("NewWidget failed: %v", err)
	}
	if w.hwnd == 0 {
		t.Fatal("expected non-zero hwnd for Widget")
	}

	// Test UpdateSurface with a 380x250 test image
	testImg := image.NewRGBA(image.Rect(0, 0, 380, 250))
	for y := 0; y < 250; y++ {
		for x := 0; x < 380; x++ {
			testImg.SetRGBA(x, y, color.RGBA{R: 20, G: 30, B: 40, A: 200})
		}
	}
	w.UpdateSurface(testImg)

	if w.width != 380 || w.height != 250 {
		t.Errorf("expected w.width=380, w.height=250, got (%d, %d)", w.width, w.height)
	}

	// Test SetPosition
	w.SetPosition(100, 200)
	if w.posX != 100 || w.posY != 200 {
		t.Errorf("expected pos (100, 200), got (%d, %d)", w.posX, w.posY)
	}

	// Test ApplyPlacement
	customX, customY := 350, 450
	w.ApplyPlacement(&customX, &customY, "bottom-right", 0)
	if w.posX != 350 || w.posY != 450 {
		t.Errorf("expected custom placement (350, 450), got (%d, %d)", w.posX, w.posY)
	}

	// Test Show and Hide
	w.Show()
	w.Hide()

	// Test Destroy
	w.Destroy()
	if w.hwnd != 0 {
		t.Errorf("expected hwnd=0 after Destroy, got %#x", w.hwnd)
	}
	_ = posChangedX
	_ = posChangedY
	_ = contextMenuCalled
}

func TestWidgetDrawerRender(t *testing.T) {
	drawer := NewWidgetDrawer()
	if drawer == nil {
		t.Fatal("NewWidgetDrawer returned nil")
	}

	cities := []config.CityConfig{
		{Name: "London", Region: "UK", Timezone: "Europe/London"},
		{Name: "Tokyo", Region: "Japan", Timezone: "Asia/Tokyo"},
	}

	temp1 := 18
	feels1 := 16.5
	aqi1 := 2
	pm25_1 := 12.4
	results := []weather.WeatherResult{
		{
			Data: &weather.WeatherData{
				CityName:    "London",
				Temperature: temp1,
				FeelsLike:   &feels1,
				Description: "Partly Cloudy",
				IconCode:    "partly_cloudy",
				Humidity:    68,
				WindSpeed:   15.5,
				WindGust:    22.0,
				DewPoint:    11.2,
				Pressure:    1014,
				UVIndex:     3.5,
				AQI:         &aqi1,
				PM25:        &pm25_1,
			},
		},
		{
			Data: &weather.WeatherData{
				CityName:    "Tokyo",
				Temperature: 24,
				Description: "Clear Sky",
				IconCode:    "clear",
				Humidity:    55,
				WindSpeed:   8.2,
			},
		},
	}

	cfg := config.DefaultConfig()
	cfg.Cities = cities

	// Test Enhanced View Mode
	cfg.ViewMode = config.ViewModeEnhanced
	imgEnhanced := drawer.Render(cities, results, cfg, nil, time.Now())
	if imgEnhanced == nil {
		t.Fatal("drawer.Render returned nil in enhanced mode")
	}
	if imgEnhanced.Bounds().Dx() != 380 {
		t.Errorf("expected enhanced width 380, got %d", imgEnhanced.Bounds().Dx())
	}
	expectedEnhancedH := 2*250 + 8
	if imgEnhanced.Bounds().Dy() != expectedEnhancedH {
		t.Errorf("expected enhanced height %d, got %d", expectedEnhancedH, imgEnhanced.Bounds().Dy())
	}

	// Test Simple View Mode
	cfg.ViewMode = config.ViewModeSimple
	imgSimple := drawer.Render(cities, results, cfg, nil, time.Now())
	if imgSimple == nil {
		t.Fatal("drawer.Render returned nil in simple mode")
	}
	expectedSimpleW := 2*140 + 8
	if imgSimple.Bounds().Dx() != expectedSimpleW {
		t.Errorf("expected simple width %d, got %d", expectedSimpleW, imgSimple.Bounds().Dx())
	}
	if imgSimple.Bounds().Dy() != 560 {
		t.Errorf("expected simple height 560, got %d", imgSimple.Bounds().Dy())
	}
}
