//go:build windows

package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// TestWindowsWidgetTheme_BackgroundTransparencyKey verifies that on Windows,
// widgetTheme returns transparencyKey (RGB 1,1,1) for ColorNameBackground
// regardless of whether VariantDark or VariantLight is requested.
func TestWindowsWidgetTheme_BackgroundTransparencyKey(t *testing.T) {
	wt := NewWidgetTheme(theme.DefaultTheme())

	for _, v := range []fyne.ThemeVariant{theme.VariantDark, theme.VariantLight} {
		got := wt.Color(theme.ColorNameBackground, v)
		nrgba, ok := got.(color.NRGBA)
		if !ok {
			t.Fatalf("expected color.NRGBA, got %T", got)
		}
		if nrgba != transparencyKey {
			t.Errorf("variant %v: expected transparencyKey %+v, got %+v", v, transparencyKey, nrgba)
		}
	}
}

// TestWindowsWidgetTheme_ForegroundDarkOnWindows verifies that on Windows,
// widgetTheme returns dark text for ColorNameForeground, ensuring that settings
// tabs, dialogs, checkboxes, and radio buttons render readable dark text on
// white backgrounds, while weather cards use explicit white text on dark cards.
func TestWindowsWidgetTheme_ForegroundDarkOnWindows(t *testing.T) {
	wt := NewWidgetTheme(theme.DefaultTheme())

	for _, v := range []fyne.ThemeVariant{theme.VariantDark, theme.VariantLight} {
		got := wt.Color(theme.ColorNameForeground, v)
		nrgba, ok := got.(color.NRGBA)
		if !ok {
			t.Fatalf("expected color.NRGBA, got %T", got)
		}
		expected := color.NRGBA{R: 34, G: 34, B: 34, A: 255}
		if nrgba != expected {
			t.Errorf("variant %v: expected dark text %+v, got %+v", v, expected, nrgba)
		}
	}
}

// TestWindowsSettingsTheme_LightModeContrast verifies that NewSettingsTheme
// always returns a clean light theme (white background, dark foreground)
// even if the system or caller passes VariantDark.
func TestWindowsSettingsTheme_LightModeContrast(t *testing.T) {
	st := NewSettingsTheme(theme.DefaultTheme())

	for _, v := range []fyne.ThemeVariant{theme.VariantDark, theme.VariantLight} {
		bg := st.Color(theme.ColorNameBackground, v)
		bgNRGBA, ok := bg.(color.NRGBA)
		if !ok {
			t.Fatalf("expected color.NRGBA, got %T", bg)
		}
		if bgNRGBA.R != 255 || bgNRGBA.G != 255 || bgNRGBA.B != 255 {
			t.Errorf("variant %v: expected white background, got %+v", v, bgNRGBA)
		}

		fg := st.Color(theme.ColorNameForeground, v)
		fgNRGBA, ok := fg.(color.NRGBA)
		if !ok {
			t.Fatalf("expected color.NRGBA, got %T", fg)
		}
		// Foreground should be dark text
		if fgNRGBA.R > 50 || fgNRGBA.G > 50 || fgNRGBA.B > 50 {
			t.Errorf("variant %v: expected dark text foreground, got %+v", v, fgNRGBA)
		}
	}
}
