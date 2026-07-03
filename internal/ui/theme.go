package ui

import (
	"image/color"
	"runtime"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)


// transparencyKey is the color Windows will make invisible via LWA_COLORKEY.
// Chosen to be near-black but distinct from pure black so it doesn't clash
// with any real UI element color.
var transparencyKey = color.NRGBA{R: 1, G: 1, B: 1, A: 255}

// transparencyActive is 1 when a color-key background should be used.
var transparencyActive atomic.Int32

// linuxBgShade stores the background RGB value for Linux (0=black, 255=white).
// Controlled by the opacity setting: 100% = fully dark (30), 25% = lighter (90).
var linuxBgShade atomic.Int32

func init() {
	linuxBgShade.Store(30) // default: dark background
}

// SetTransparencyActive switches the background color key on or off.
func SetTransparencyActive(active bool) {
	if active {
		transparencyActive.Store(1)
	} else {
		transparencyActive.Store(0)
	}
}

// SetLinuxBackgroundShade sets the Linux background darkness level.
// opacityPercent maps to background shade:
//
//	100% → RGB(30,30,30)  — fully dark, content most visible
//	 75% → RGB(50,50,50)  — slightly lighter
//	 50% → RGB(70,70,70)  — medium grey
//	 25% → RGB(90,90,90)  — lighter grey
func SetLinuxBackgroundShade(opacityPercent int) {
	var shade int
	switch {
	case opacityPercent >= 100:
		shade = 30
	case opacityPercent >= 75:
		shade = 50
	case opacityPercent >= 50:
		shade = 70
	default:
		shade = 90
	}
	linuxBgShade.Store(int32(shade))
}

// widgetTheme is a Fyne theme that:
//   - On Windows: replaces the background with a color-key when transparency is active
//   - On Linux: always uses a dark background (ignoring system light/dark preference)
//     to ensure the widget looks consistent and native
type widgetTheme struct {
	base fyne.Theme
}

// NewWidgetTheme wraps the given base theme.
func NewWidgetTheme(base fyne.Theme) fyne.Theme {
	return &widgetTheme{base: base}
}

func (t *widgetTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Windows: color-key transparency when active.
	if transparencyActive.Load() == 1 {
		switch name {
		case theme.ColorNameBackground, theme.ColorNameOverlayBackground:
			return transparencyKey
		}
	}

	// Linux: always use dark variant colors for a consistent widget appearance,
	// regardless of the system theme (light/dark). The background shade is
	// controlled by the opacity setting.
	if runtime.GOOS == "linux" {
		switch name {
		case theme.ColorNameBackground, theme.ColorNameOverlayBackground:
			shade := uint8(linuxBgShade.Load())
			return color.NRGBA{R: shade, G: shade, B: shade, A: 255}
		case theme.ColorNameForeground:
			return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		case theme.ColorNameDisabled:
			return color.NRGBA{R: 180, G: 180, B: 180, A: 255}
		case theme.ColorNameSeparator:
			return color.NRGBA{R: 80, G: 80, B: 80, A: 255}
		}
		// For all other colors, force dark variant.
		return t.base.Color(name, theme.VariantDark)
	}

	// macOS: use a dark opaque background so the Fyne GL renderer clears to
	// a visible color. Transparency is achieved by setting NSWindow.backgroundColor
	// with the desired alpha via setDarwinBackgroundAlpha — the window is
	// non-opaque so the desktop shows through the background while Fyne-rendered
	// content (text, icons) remains fully opaque.
	if runtime.GOOS == "darwin" {
		switch name {
		case theme.ColorNameBackground, theme.ColorNameOverlayBackground:
			return color.NRGBA{R: 30, G: 30, B: 30, A: 255}
		case theme.ColorNameForeground:
			return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		case theme.ColorNameDisabled:
			return color.NRGBA{R: 180, G: 180, B: 180, A: 255}
		case theme.ColorNameSeparator:
			return color.NRGBA{R: 80, G: 80, B: 80, A: 255}
		}
		return t.base.Color(name, theme.VariantDark)
	}

	return t.base.Color(name, variant)
}

func (t *widgetTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *widgetTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *widgetTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}
