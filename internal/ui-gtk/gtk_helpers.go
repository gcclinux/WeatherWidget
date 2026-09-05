//go:build linux

package uitk

import (
	"fmt"
	"log"
	"math"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

// cairoContext is an alias used in draw signal callbacks.
// gotk3 passes *cairo.Context as the second argument.
type cairoContext = cairo.Context

// enableRGBA requests an RGBA visual for the window so the compositor can
// show through fully transparent pixels. Falls back gracefully if unavailable.
func enableRGBA(win *gtk.Window) {
	// gtk.Window.GetScreen() returns *gdk.Screen (no error).
	screen := win.GetScreen()
	if screen == nil {
		log.Println("GTK: could not get screen for RGBA visual")
		return
	}
	visual, err := screen.GetRGBAVisual()
	if err != nil || visual == nil {
		log.Println("GTK: RGBA visual not available — transparency may not work")
		return
	}
	// gtk.Window embeds gtk.Widget through Bin/Container, so SetVisual is available.
	win.SetVisual(visual)
	win.SetAppPaintable(true)
}

// paintTransparent clears the cairo context with a fully transparent fill,
// allowing the compositor to show the desktop through the window background.
func paintTransparent(cr *cairo.Context) {
	cr.SetSourceRGBA(0, 0, 0, 0)
	cr.SetOperator(cairo.OPERATOR_SOURCE)
	cr.Paint()
}

// panelAlpha maps the opacity percent (and no-background flag) to the alpha
// used for the card background. Kept in one place so the CSS and the manual
// card painter (paintCard) agree exactly.
func panelAlpha(opacity int, noBackground bool) float64 {
	if noBackground {
		return 0.0
	}
	switch {
	case opacity >= 100:
		return 0.85
	case opacity >= 75:
		return 0.70
	case opacity >= 50:
		return 0.55
	default: // 25%
		return 0.40
	}
}

// paintRoundedRect fills a rounded rectangle at (x, y, w, h) with the given
// RGBA using the cairo context, matching the card's border radius. The path is
// built with an explicit MoveTo + four corner arcs (this gotk3 cairo binding
// does not expose NewSubPath).
func paintRoundedRect(cr *cairo.Context, x, y, w, h, radius, r, g, b, a float64) {
	const degrees = math.Pi / 180.0
	if radius > w/2 {
		radius = w / 2
	}
	if radius > h/2 {
		radius = h / 2
	}
	cr.NewPath()
	cr.MoveTo(x+radius, y)
	cr.Arc(x+w-radius, y+radius, radius, -90*degrees, 0*degrees)   // top-right
	cr.Arc(x+w-radius, y+h-radius, radius, 0*degrees, 90*degrees)  // bottom-right
	cr.Arc(x+radius, y+h-radius, radius, 90*degrees, 180*degrees)  // bottom-left
	cr.Arc(x+radius, y+radius, radius, 180*degrees, 270*degrees)   // top-left
	cr.ClosePath()
	cr.SetSourceRGBA(r, g, b, a)
	cr.Fill()
}

// buildCSS returns the CSS string for the widget panels based on opacity,
// no-background settings, and the three user-configurable font sizes:
//   - fsCityTime:   city name and time labels (px)
//   - fsTempIcon:   temperature label (px)
//   - fsConditions: description, humidity/wind, and all info rows below temp (px)
//
// Pass 0 for any size to use the built-in defaults (14 / 32 / 10).
func buildCSS(opacity int, noBackground bool, fsCityTime, fsTempIcon, fsConditions int) string {
	// Apply defaults for zero values (handles old configs and first run).
	if fsCityTime <= 0 {
		fsCityTime = 14
	}
	if fsTempIcon <= 0 {
		fsTempIcon = 32
	}
	if fsConditions <= 0 {
		fsConditions = 10
	}

	// The time label is scaled proportionally: it is ~14% larger than the
	// city label in the default theme (16px vs 14px).  We maintain that ratio
	// so both grow/shrink together when the user adjusts the City & Time size.
	fsTime := int(float64(fsCityTime) * (16.0 / 14.0))
	if fsTime < 1 {
		fsTime = 1
	}

	// The card background itself is painted manually in manager.paintCards
	// (see panelAlpha); the CSS below only styles text, tiles, and borders.

	return fmt.Sprintf(`
window {
    background-color: transparent;
}
#panelbox {
    background-color: transparent;
}
.city-panel {
    /* Background is painted manually in manager.paintCards so it sits behind
       the weather icon's transparent pixels too; keep the CSS fill clear to
       avoid double-tinting. */
    background-color: transparent;
    border-radius: 16px;
    padding: 12px 14px;
    margin: 2px;
    color: white;
}
/* The weather icon sits directly on the card background — no extra fill, so
   its transparent PNG pixels show the same card tint as the rest of the card. */
.icon-bg {
    background-color: transparent;
    border-radius: 12px;
}
.city-label {
    font-size: %dpx;
    font-weight: bold;
    color: white;
}
.temp-label {
    font-size: %dpx;
    font-weight: bold;
    color: white;
}
.desc-label {
    font-size: %dpx;
    font-style: italic;
    color: #dddddd;
}
.time-label {
    font-size: %dpx;
    font-weight: bold;
    color: white;
}
.date-label {
    font-size: %dpx;
    color: #cccccc;
}
.info-label {
    font-size: %dpx;
    color: #eeeeee;
}
/* Right-hand metrics grid: transparent tiles separated by thin borders that
   read as a grid, matching the design mockup. */
.metrics-grid {
    border: 1px solid rgba(255, 255, 255, 0.14);
    border-radius: 8px;
}
.metric-tile {
    background-color: transparent;
    border: 1px solid rgba(255, 255, 255, 0.14);
    padding: 6px 10px;
}
.metric-emoji {
    font-size: %dpx;
}
.metric-name {
    font-size: %dpx;
    color: #dddddd;
}
.metric-value {
    font-size: %dpx;
    font-weight: bold;
    color: white;
}
/* Thin divider between the top region and the air-quality row. */
.card-separator {
    background-color: rgba(255, 255, 255, 0.15);
    min-height: 1px;
    margin: 6px 0;
}
/* Air-quality tiles along the bottom of the card. */
.air-tile {
    padding: 4px 2px;
}
.air-name {
    font-size: %dpx;
    color: #bbbbbb;
}
.air-label {
    font-size: %dpx;
    font-weight: bold;
    color: #ffffff;
}
.error-label {
    font-size: 11px;
    color: #ff8888;
    font-style: italic;
}
`, fsCityTime, fsTempIcon, fsConditions, fsTime, fsConditions, fsConditions,
		fsConditions+2, fsConditions, fsConditions+2, fsConditions-1, fsConditions)
}

// applyCSSToScreen loads the given CSS string into the default screen's
// style context. Previous providers from this package are replaced.
var currentCSSProvider *gtk.CssProvider

func applyCSSToScreen(css string) {
	provider, err := gtk.CssProviderNew()
	if err != nil {
		log.Printf("GTK: failed to create CSS provider: %v", err)
		return
	}
	if err := provider.LoadFromData(css); err != nil {
		log.Printf("GTK: failed to load CSS: %v", err)
		return
	}
	screen, err := gdk.ScreenGetDefault()
	if err != nil {
		log.Printf("GTK: failed to get default screen: %v", err)
		return
	}
	// Remove previous provider if any.
	if currentCSSProvider != nil {
		gtk.RemoveProviderForScreen(screen, currentCSSProvider)
	}
	gtk.AddProviderForScreen(screen, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	currentCSSProvider = provider
}

// cornerToXY computes the top-left window coordinate for a given corner
// position and monitor index. Falls back to bottom-right on the primary
// monitor if the corner or monitor is invalid.
func cornerToXY(corner string, monitorIndex int, winW, winH int) (int, int) {
	screen, err := gdk.ScreenGetDefault()
	if err != nil {
		return 0, 0
	}
	display, err := screen.GetDisplay()
	if err != nil {
		return 0, 0
	}
	nMon := display.GetNMonitors()
	if monitorIndex < 0 || monitorIndex >= nMon {
		monitorIndex = 0
	}
	mon, err := display.GetMonitor(monitorIndex)
	if err != nil || mon == nil {
		return 0, 0
	}
	geom := mon.GetGeometry()
	mx := geom.GetX()
	my := geom.GetY()
	mw := geom.GetWidth()
	mh := geom.GetHeight()

	switch corner {
	case "top-left":
		return mx, my
	case "top-right":
		return mx + mw - winW, my
	case "bottom-left":
		return mx, my + mh - winH
	default: // bottom-right
		return mx + mw - winW, my + mh - winH
	}
}

// removeDecorations removes the window title bar by replacing GTK's built-in
// CSD titlebar with an empty widget. This works on both X11 and Wayland:
// on X11 it suppresses the WM-drawn decorations, on Wayland it removes the
// GTK client-side title bar that GNOME Mutter would otherwise draw.
func removeDecorations(win *gtk.Window) {
	// An empty Box as the titlebar removes all title bar chrome.
	emptyBar, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	if err != nil {
		log.Printf("GTK: failed to create empty titlebar: %v", err)
		return
	}
	emptyBar.SetSizeRequest(0, 0) // zero height — completely invisible
	emptyBar.ShowAll()
	win.SetTitlebar(emptyBar)
	win.SetDecorated(false)
}

// restoreDecorations restores the default GTK title bar.
func restoreDecorations(win *gtk.Window) {
	win.SetTitlebar(nil)
	win.SetDecorated(true)
}

// enableDrag wires button-press and motion events for manual drag-to-reposition.
// moveFunc is called with the computed (x, y) to actually reposition the window;
// on X11 this is typically win.Move(), on Wayland it updates layer-shell margins.
// onMove is called with the new (x,y) after every position change so callers
// can auto-save without user action.
func enableDrag(win *gtk.Window, moveFunc func(x, y int), onMove func(x, y int)) {
	var dragging bool
	var startRootX, startRootY int // pointer position at drag start
	var startWinX, startWinY int   // window position at drag start

	win.Connect("button-press-event", func(_ *gtk.Window, ev *gdk.Event) bool {
		btn := gdk.EventButtonNewFromEvent(ev)
		if btn.Button() != 1 {
			return false
		}
		dragging = true
		startRootX = int(btn.XRoot())
		startRootY = int(btn.YRoot())
		startWinX, startWinY = win.GetPosition()
		return false
	})

	win.Connect("motion-notify-event", func(_ *gtk.Window, ev *gdk.Event) bool {
		if !dragging {
			return false
		}
		motion := gdk.EventMotionNewFromEvent(ev)
		rx, ry := motion.MotionValRoot()
		newX := startWinX + int(rx) - startRootX
		newY := startWinY + int(ry) - startRootY
		moveFunc(newX, newY)
		if onMove != nil {
			onMove(newX, newY)
		}
		return false
	})

	win.Connect("button-release-event", func(_ *gtk.Window, _ *gdk.Event) bool {
		dragging = false
		return false
	})
}
