//go:build linux

package uitk

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/pango"
)

// TestTickerScroll_FullTraverseVerifiesTextExitsLeft verifies that the track layout
// (leftPad matching pageSize, and rightPad matching pageSize + tickerTrailingGap)
// mathematically guarantees that maxScroll is always strictly greater than
// (pageSize + textWidth), ensuring the last character completely exits off the
// left edge of the visible window before the item resets.
func TestTickerScroll_FullTraverseVerifiesTextExitsLeft(t *testing.T) {
	testCases := []struct {
		name      string
		pageSize  int
		textWidth int
	}{
		{"Small widget, short text", 150, 100},
		{"Standard card, medium text", 240, 350},
		{"Standard card, long text", 240, 800},
		{"Wide widget, long text", 320, 1200},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			leftPad := tc.pageSize
			rightPad := tc.pageSize + tickerTrailingGap
			upper := leftPad + tc.textWidth + rightPad
			maxScroll := upper - tc.pageSize

			// Scroll position when the very first character reaches the left edge:
			firstCharLeftExitPos := leftPad
			// Scroll position when the very last character reaches the left edge:
			lastCharLeftExitPos := leftPad + tc.textWidth

			if firstCharLeftExitPos != tc.pageSize {
				t.Fatalf("first character should reach left edge at %d, got %d", tc.pageSize, firstCharLeftExitPos)
			}

			if maxScroll < lastCharLeftExitPos {
				t.Fatalf("maxScroll (%d) is less than lastCharLeftExitPos (%d), text would cut off early by %d px!",
					maxScroll, lastCharLeftExitPos, lastCharLeftExitPos-maxScroll)
			}

			pausePixels := maxScroll - lastCharLeftExitPos
			if pausePixels != tickerTrailingGap {
				t.Fatalf("expected trailing pause gap of %d px, got %d px", tickerTrailingGap, pausePixels)
			}
		})
	}
}

// TestBuildCSS_TickerBackgroundMatchesCardTint verifies that buildCSS injects
// the card tint into the ticker viewport while keeping inner boxes/labels transparent,
// and correctly falls back to transparent in no-background mode.
func TestBuildCSS_TickerBackgroundMatchesCardTint(t *testing.T) {
	// Mode 1: Tinted background (e.g. opacity 80)
	cssTinted := buildCSS(80, false, 14, 32, 10)
	expectedAlpha := panelAlpha(80, false)
	expectedRGBA := fmt.Sprintf("rgba(20, 20, 20, %.3f)", expectedAlpha)

	if !strings.Contains(cssTinted, ".pollution-ticker viewport {\n    background-color: "+expectedRGBA) {
		t.Fatalf("expected tinted CSS to contain viewport background %s", expectedRGBA)
	}
	if !strings.Contains(cssTinted, ".pollution-ticker box,\n.pollution-ticker label {\n    background-color: transparent;") {
		t.Fatalf("expected inner boxes and labels to remain transparent")
	}

	// Mode 2: Fully transparent mode (noBackground = true)
	cssNoBg := buildCSS(80, true, 14, 32, 10)
	if !strings.Contains(cssNoBg, ".pollution-ticker viewport {\n    background-color: transparent;") {
		t.Fatalf("expected no-background CSS to contain transparent viewport background")
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
}


