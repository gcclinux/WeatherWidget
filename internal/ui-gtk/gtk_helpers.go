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
	// Map the opacity percentage (25–100) linearly onto an alpha range of
	// 0.40–0.85 so the whole card darkens smoothly as the slider moves,
	// instead of jumping between four fixed steps.
	if opacity < 25 {
		opacity = 25
	}
	if opacity > 100 {
		opacity = 100
	}
	const (
		minAlpha = 0.40
		maxAlpha = 0.85
	)
	frac := float64(opacity-25) / float64(100-25) // 0.0 at 25%, 1.0 at 100%
	return minAlpha + frac*(maxAlpha-minAlpha)
}

// paintRoundedRect fills a rounded rectangle at (x, y, w, h) with the given
// RGBA using the cairo context, matching the card's border radius. The path is
// built with an explicit MoveTo + four corner arcs (this gotk3 cairo binding
// does not expose NewSubPath).
func paintRoundedRect(cr *cairo.Context, x, y, w, h, radius, r, g, b, a float64) {
	roundedRectPath(cr, x, y, w, h, radius)
	cr.SetSourceRGBA(r, g, b, a)
	cr.Fill()
}

// strokeRoundedRect draws a rounded-rectangle outline (no fill) at
// (x, y, w, h) with the given RGBA and line width. It is the border companion
// to paintRoundedRect and is used to frame each city panel when the card
// background is removed (no-background mode), matching the Fyne backend's
// faint white rounded border.
func strokeRoundedRect(cr *cairo.Context, x, y, w, h, radius, lineWidth, r, g, b, a float64) {
	roundedRectPath(cr, x, y, w, h, radius)
	cr.SetSourceRGBA(r, g, b, a)
	cr.SetLineWidth(lineWidth)
	cr.Stroke()
}

// roundedRectPath builds a rounded-rectangle path on the cairo context using an
// explicit MoveTo + four corner arcs (this gotk3 cairo binding does not expose
// NewSubPath). It sets the current path but does not paint; callers Fill() or
// Stroke() afterwards.
func roundedRectPath(cr *cairo.Context, x, y, w, h, radius float64) {
	const degrees = math.Pi / 180.0
	if radius > w/2 {
		radius = w / 2
	}
	if radius > h/2 {
		radius = h / 2
	}
	cr.NewPath()
	cr.MoveTo(x+radius, y)
	cr.Arc(x+w-radius, y+radius, radius, -90*degrees, 0*degrees)  // top-right
	cr.Arc(x+w-radius, y+h-radius, radius, 0*degrees, 90*degrees) // bottom-right
	cr.Arc(x+radius, y+h-radius, radius, 90*degrees, 180*degrees) // bottom-left
	cr.Arc(x+radius, y+radius, radius, 180*degrees, 270*degrees)  // top-left
	cr.ClosePath()
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
	// (see panelAlpha); the CSS below styles text, and the tile/grid/separator
	// backgrounds and borders that make up the interior "boxes".
	//
	// The metric grid, metric tiles and the separator are drawn by GTK CSS
	// (not Cairo), so their opacity must be derived from the same opacity
	// value here — otherwise they stay a fixed shade while the Cairo-painted
	// card behind them darkens, which is exactly the "only the icon
	// backgrounds change" symptom.
	cardAlpha := panelAlpha(opacity, noBackground)

	// Derive interior alphas from the card alpha so the whole widget darkens
	// together. When noBackground is set (cardAlpha == 0) the interior boxes
	// disappear as well.
	//   - lineAlpha: the tile/grid border and separator lines
	//
	// The metric tiles themselves keep a fully transparent fill so the
	// Cairo-painted card (manager.paintCards) shows through them directly.
	// This makes the six metric cells (Humidity, Wind, Wind Gust, Dew Point,
	// Pressure, UV Index) track the transparency slider exactly like the rest
	// of the card, instead of layering a fixed dark fill that reads as a
	// separate static block.
	lineAlpha := 0.10 + cardAlpha*0.18 // ~0.17 at 25%, ~0.25 at 100%
	if noBackground {
		lineAlpha = 0.0
	}

	// Tile fill is transparent so the card tint behind it is what the user
	// sees; only the grid/tile borders and the separator scale with opacity.
	tileFillCSS := "transparent"
	lineCSS := fmt.Sprintf("rgba(255, 255, 255, %.3f)", lineAlpha)
	sepCSS := lineCSS

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
    border: 1px solid %s;
    border-radius: 8px;
}
.metric-tile {
    background-color: %s;
    border: 1px solid %s;
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
    background-color: %s;
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
/* ── Simple / classic view ──────────────────────────────────────────────
   A narrow vertical column per city, laid out side-by-side. The card fill is
   painted manually in manager.paintCards (same rounded, tinted background as
   the enhanced card), so these classes only style text. */
.simple-city-panel {
    background-color: transparent;
    border-radius: 16px;
    padding: 12px 10px;
    margin: 2px;
    color: white;
}
.simple-city-label {
    font-size: %dpx;
    font-weight: bold;
    color: white;
}
.simple-time-label {
    font-size: %dpx;
    font-weight: bold;
    color: white;
}
.simple-date-label {
    font-size: %dpx;
    color: #cccccc;
}
.simple-temp-label {
    font-size: %dpx;
    font-weight: bold;
    color: white;
}
.simple-desc-label {
    font-size: %dpx;
    font-style: italic;
    color: #dddddd;
}
.simple-info-label {
    font-size: %dpx;
    color: #eeeeee;
}
`, fsCityTime, fsTempIcon, fsConditions, fsTime, fsConditions, fsConditions,
		lineCSS, tileFillCSS, lineCSS,
		fsConditions+2, fsConditions, fsConditions+2,
		sepCSS,
		fsConditions-1, fsConditions,
		fsCityTime, fsTime, fsConditions, fsTempIcon, fsConditions, fsConditions)
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
