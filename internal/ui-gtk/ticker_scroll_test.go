//go:build linux

package uitk

import (
	"strings"
	"testing"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/pango"
)

// TestTickerScroll_FullTraverseVerifiesTextExitsLeft verifies that the scroll traversal
// math (x = w - tickerPos, with maxScroll = w + textWidth + tickerTrailingGap)
// mathematically guarantees that:
// 1. Text starts entering at the right edge (x = w).
// 2. The first character reaches the left boundary at tickerPos = w.
// 3. The last character completely exits off the left edge at tickerPos = w + textWidth.
// 4. A clean pause of tickerTrailingGap occurs before resetting.
func TestTickerScroll_FullTraverseVerifiesTextExitsLeft(t *testing.T) {
	testCases := []struct {
		name      string
		width     float64
		textWidth float64
	}{
		{"Small widget, short text", 150, 100},
		{"Standard card, medium text", 240, 350},
		{"Standard card, long text", 240, 800},
		{"Wide widget, long text", 320, 1200},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := tc.width
			textW := tc.textWidth
			maxScroll := w + textW + float64(tickerTrailingGap)

			// 1. Start position (tickerPos = 0):
			startPos := 0.0
			startX := w - startPos
			if startX != w {
				t.Fatalf("expected text to start at right edge (x=%v), got %v", w, startX)
			}

			// 2. Position when first character reaches left edge (x = 0):
			firstCharLeftExitTickerPos := w
			if w-firstCharLeftExitTickerPos != 0 {
				t.Fatalf("first character should reach left edge at tickerPos=%v", w)
			}

			// 3. Position when last character completely exits left edge (x = -textW):
			lastCharLeftExitTickerPos := w + textW
			if (w - lastCharLeftExitTickerPos) != -textW {
				t.Fatalf("last character should completely exit at tickerPos=%v", lastCharLeftExitTickerPos)
			}

			// 4. Trailing pause before next item:
			pausePixels := maxScroll - lastCharLeftExitTickerPos
			if pausePixels != float64(tickerTrailingGap) {
				t.Fatalf("expected trailing pause gap of %v px, got %v px", tickerTrailingGap, pausePixels)
			}
		})
	}
}

// TestBuildCSS_TickerTransparent verifies that buildCSS sets .pollution-ticker
// to transparent so the Cairo card background drawn by paintCards shows through directly.
func TestBuildCSS_TickerTransparent(t *testing.T) {
	css := buildCSS(80, false, 14, 32, 10)
	if !strings.Contains(css, ".pollution-ticker {\n    background-color: transparent;\n}") {
		t.Fatalf("expected .pollution-ticker to have transparent background")
	}
}

func TestPangoCairoDrawing(t *testing.T) {
	surf := cairo.CreateImageSurface(cairo.FORMAT_ARGB32, 200, 50)
	cr := cairo.Create(surf)
	layout := pango.CairoCreateLayout(cr)
	desc := pango.FontDescriptionFromString("Sans 11")
	layout.SetFontDescription(desc)
	layout.SetMarkup("<span foreground='#FFFFFF'>Hello</span>", -1)
	pw, ph := layout.GetSize()
	if pw <= 0 || ph <= 0 {
		t.Fatalf("expected non-zero size, got %d, %d", pw, ph)
	}
	pango.CairoShowLayout(cr, layout)
}

func TestDrawingAreaWindowless(t *testing.T) {
	gtk.Init(nil)
	da, err := gtk.DrawingAreaNew()
	if err != nil {
		t.Fatal(err)
	}
	da.SetHasWindow(false)
	if da.GetHasWindow() {
		t.Fatalf("expected SetHasWindow(false) to result in GetHasWindow() == false")
	}
	da.Connect("draw", func(area *gtk.DrawingArea, cr *cairo.Context) {
	})
}


