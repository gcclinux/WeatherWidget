package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
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

func TestLanguageCardFont_TamilCoverage(t *testing.T) {
	res := LanguageCardFont("ta-IN")
	if res == nil {
		t.Fatal("expected non-nil resource for ta-IN")
	}

	data := res.Content()
	if len(data) == 0 {
		t.Fatal("empty font content for ta-IN card font")
	}

	f, err := sfnt.Parse(data)
	if err != nil {
		t.Fatalf("failed to parse font: %v", err)
	}

	testRunes := []rune("தமிழ்")
	var b sfnt.Buffer
	for _, r := range testRunes {
		idx, err := f.GlyphIndex(&b, r)
		if idx == 0 || err != nil {
			t.Errorf("missing glyph for rune %q (U+%04X)", r, r)
		}
	}
}

func TestLanguageCardFont_CJKCoverage(t *testing.T) {
	for _, code := range []string{"zh-CN", "ja-JP"} {
		res := LanguageCardFont(code)
		if res == nil {
			t.Fatalf("expected non-nil resource for %s", code)
		}
		data := res.Content()
		if len(data) == 0 {
			t.Fatalf("empty font content for %s card font", code)
		}
	}
}

func TestLanguageCardFont_LatinReturnsNil(t *testing.T) {
	latinLocales := []string{"en-GB", "es-ES", "fr-FR", "de-DE", "it-IT", "pt-BR", "nl-NL", "pl-PL", "tr-TR"}
	for _, loc := range latinLocales {
		if res := LanguageCardFont(loc); res != nil {
			t.Errorf("expected nil card font for Latin locale %s, got %v", loc, res.Name())
		}
	}
}

func TestTamilCanvasTextWithLanguageCardFont(t *testing.T) {
	test.NewApp()
	// Ensure Latin theme font is active (simulates English or other language selected)
	SetLocaleFont("en-GB")

	cardFont := LanguageCardFont("ta-IN")
	if cardFont == nil {
		t.Fatal("expected non-nil card font for ta-IN")
	}

	txt := canvas.NewText("தமிழ்", color.White)
	txt.FontSource = cardFont
	txt.TextSize = 14
	txt.TextStyle = fyne.TextStyle{Bold: true}

	minSize := txt.MinSize()
	if minSize.Width <= 0 || minSize.Height <= 0 {
		t.Errorf("expected positive dimensions for Tamil text, got %v", minSize)
	}
}




