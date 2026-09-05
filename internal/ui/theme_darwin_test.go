//go:build darwin

package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2/theme"
)

// TestDarwinThemeBackground_Transparent verifies the darwin theme returns a
// fully transparent background. This allows the desktop to show through
// everywhere except the individual city card backgrounds (which have their own
// opaque fill). NSWindow is made non-opaque via setupDarwinWindow.
// **Validates: Requirements 2.1**
func TestDarwinThemeBackground_Transparent(t *testing.T) {
	wt := NewWidgetTheme(theme.DefaultTheme())

	got := wt.Color(theme.ColorNameBackground, theme.VariantDark)

	nrgba, ok := got.(color.NRGBA)
	if !ok {
		r, g, b, a := got.RGBA()
		nrgba = color.NRGBA{
			R: uint8(r >> 8),
			G: uint8(g >> 8),
			B: uint8(b >> 8),
			A: uint8(a >> 8),
		}
	}

	t.Logf("Darwin theme background: R=%d G=%d B=%d A=%d", nrgba.R, nrgba.G, nrgba.B, nrgba.A)

	if nrgba.A != 0 {
		t.Errorf(
			"Darwin theme background should be fully transparent (A=0) so desktop shows through, got A=%d.",
			nrgba.A,
		)
	}
}

// TestDarwinThemeOverlayBackground_Transparent mirrors TestDarwinThemeBackground_Transparent
// for ColorNameOverlayBackground.
// **Validates: Requirements 2.1**
func TestDarwinThemeOverlayBackground_Transparent(t *testing.T) {
	wt := NewWidgetTheme(theme.DefaultTheme())

	got := wt.Color(theme.ColorNameOverlayBackground, theme.VariantDark)

	nrgba, ok := got.(color.NRGBA)
	if !ok {
		r, g, b, a := got.RGBA()
		nrgba = color.NRGBA{
			R: uint8(r >> 8),
			G: uint8(g >> 8),
			B: uint8(b >> 8),
			A: uint8(a >> 8),
		}
	}

	t.Logf("Darwin theme overlay background: R=%d G=%d B=%d A=%d", nrgba.R, nrgba.G, nrgba.B, nrgba.A)

	if nrgba.A != 0 {
		t.Errorf(
			"Darwin theme overlay background should be fully transparent (A=0), got A=%d.",
			nrgba.A,
		)
	}
}

// TestDarwinThemeForeground_AlwaysWhite verifies the foreground color is white.
// **Validates: Requirements 3.1**
func TestDarwinThemeForeground_AlwaysWhite(t *testing.T) {
	wt := NewWidgetTheme(theme.DefaultTheme())

	got := wt.Color(theme.ColorNameForeground, theme.VariantDark)

	nrgba, ok := got.(color.NRGBA)
	if !ok {
		r, g, b, a := got.RGBA()
		nrgba = color.NRGBA{
			R: uint8(r >> 8),
			G: uint8(g >> 8),
			B: uint8(b >> 8),
			A: uint8(a >> 8),
		}
	}

	t.Logf("Darwin theme foreground: R=%d G=%d B=%d A=%d", nrgba.R, nrgba.G, nrgba.B, nrgba.A)

	if nrgba.R != 255 || nrgba.G != 255 || nrgba.B != 255 || nrgba.A != 255 {
		t.Errorf(
			"Darwin theme foreground should be pure white (255,255,255,255), got (%d,%d,%d,%d).",
			nrgba.R, nrgba.G, nrgba.B, nrgba.A,
		)
	}
}
