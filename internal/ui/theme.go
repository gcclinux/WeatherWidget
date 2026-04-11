package ui

import (
	"image/color"
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

// SetTransparencyActive switches the background color key on or off.
func SetTransparencyActive(active bool) {
	if active {
		transparencyActive.Store(1)
	} else {
		transparencyActive.Store(0)
	}
}

// widgetTheme is a Fyne theme that replaces the window background with the
// color key when transparency is active, leaving all other colors unchanged.
type widgetTheme struct {
	base fyne.Theme
}

// NewWidgetTheme wraps the given base theme.
func NewWidgetTheme(base fyne.Theme) fyne.Theme {
	return &widgetTheme{base: base}
}

func (t *widgetTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if transparencyActive.Load() == 1 {
		switch name {
		case theme.ColorNameBackground, theme.ColorNameOverlayBackground:
			return transparencyKey
		}
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
