//go:build linux

package uitk

import (
	"fmt"
	"log"

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

	// Map opacity percent to an alpha value for the panel background.
	var alpha float64
	if noBackground {
		alpha = 0.0
	} else {
		switch {
		case opacity >= 100:
			alpha = 0.85
		case opacity >= 75:
			alpha = 0.70
		case opacity >= 50:
			alpha = 0.55
		default: // 25%
			alpha = 0.40
		}
	}

	panelBg := fmt.Sprintf("rgba(20, 20, 20, %.2f)", alpha)

	return fmt.Sprintf(`
window {
    background-color: transparent;
}
#panelbox {
    background-color: transparent;
}
.city-panel {
    background-color: %s;
    border-radius: 10px;
    padding: 10px;
    margin: 2px;
    color: white;
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
    color: #cccccc;
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
    color: #cccccc;
}
.error-label {
    font-size: 11px;
    color: #ff8888;
    font-style: italic;
}
`, panelBg, fsCityTime, fsTempIcon, fsConditions, fsTime, fsConditions, fsConditions)
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
