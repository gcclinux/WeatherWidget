package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"golang.org/x/image/font/sfnt"
)

func TestSetLocaleFont_TamilCoverage(t *testing.T) {
	SetLocaleFont("ta-IN")

	th := NewWidgetTheme(theme.DefaultTheme())
	res := th.Font(fyne.TextStyle{})
	if res == nil {
		t.Fatal("expected non-nil font resource for ta-IN")
	}

	data := res.Content()
	if len(data) == 0 {
		t.Fatal("empty font content for ta-IN")
	}

	f, err := sfnt.Parse(data)
	if err != nil {
		t.Fatalf("failed to parse font: %v", err)
	}

	testRunes := []rune("தமிழ் WeatherWidget Settings 0123456789 °C % km/h -")
	var b sfnt.Buffer
	for _, r := range testRunes {
		if r == ' ' {
			continue
		}
		idx, err := f.GlyphIndex(&b, r)
		if idx == 0 || err != nil {
			t.Errorf("missing glyph for rune %q (U+%04X)", r, r)
		}
	}
}

func TestSetLocaleFont_LatinCoverage(t *testing.T) {
	SetLocaleFont("en-GB")

	th := NewWidgetTheme(theme.DefaultTheme())
	res := th.Font(fyne.TextStyle{})
	if res == nil {
		t.Fatal("expected non-nil font resource for en-GB")
	}

	data := res.Content()
	if len(data) == 0 {
		t.Fatal("empty font content for en-GB")
	}

	f, err := sfnt.Parse(data)
	if err != nil {
		t.Fatalf("failed to parse font: %v", err)
	}

	testRunes := []rune("WeatherWidget Settings Display Widget Data Provider Locations 0123456789 °C % km/h")
	var b sfnt.Buffer
	for _, r := range testRunes {
		if r == ' ' {
			continue
		}
		idx, err := f.GlyphIndex(&b, r)
		if idx == 0 || err != nil {
			t.Errorf("missing glyph for rune %q (U+%04X)", r, r)
		}
	}
}
