//go:build windows

package panel

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// windowsColorKey is the Win32 LWA_COLORKEY color used by applyToolWindowStyle.
// Any Fyne pixel painted this exact color becomes fully transparent on screen.
var windowsColorKey = color.NRGBA{R: 1, G: 1, B: 1, A: 255}

// newCornerMask returns a canvas.Raster that paints windowsColorKey over the
// four corner regions that fall outside the card's rounded rectangle (radius =
// cardCornerRadius). Because the window uses LWA_COLORKEY transparency those
// pixels become fully transparent, giving the card rounded edges that match
// macOS exactly — without relying on DWM window rounding (which adds an
// unwanted system border ring).
func newCornerMask() fyne.CanvasObject {
	r := float64(cardCornerRadius)
	return canvas.NewRasterWithPixels(func(x, y, w, h int) color.Color {
		fx := float64(x) + 0.5
		fy := float64(y) + 0.5
		fw := float64(w)
		fh := float64(h)

		// Only the four corner quadrants need checking; the edges and center
		// are always inside the rounded rect.
		inLeftZone := fx < r
		inRightZone := fx > fw-r
		inTopZone := fy < r
		inBottomZone := fy > fh-r

		if !((inLeftZone || inRightZone) && (inTopZone || inBottomZone)) {
			// Not a corner zone — leave transparent so content shows through.
			return color.Transparent
		}

		// Determine the centre of the nearest arc.
		var cx, cy float64
		if inLeftZone {
			cx = r
		} else {
			cx = fw - r
		}
		if inTopZone {
			cy = r
		} else {
			cy = fh - r
		}

		dx := fx - cx
		dy := fy - cy
		if dx*dx+dy*dy > r*r {
			// Outside the rounded rect — paint the color key so it becomes transparent.
			return windowsColorKey
		}
		return color.Transparent
	})
}
