//go:build linux

package uitk

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"

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

// buildCSS returns the CSS string for the widget panels based on opacity
// and no-background settings.
func buildCSS(opacity int, noBackground bool) string {
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

	return `
window {
    background-color: transparent;
}
#panelbox {
    background-color: transparent;
}
.city-panel {
    background-color: ` + panelBg + `;
    border-radius: 10px;
    padding: 10px;
    margin: 2px;
    color: white;
}
.city-label {
    font-size: 14px;
    font-weight: bold;
    color: white;
}
.temp-label {
    font-size: 32px;
    font-weight: bold;
    color: white;
}
.desc-label {
    font-size: 10px;
    font-style: italic;
    color: #cccccc;
}
.time-label {
    font-size: 16px;
    font-weight: bold;
    color: white;
}
.date-label {
    font-size: 10px;
    color: #cccccc;
}
.info-label {
    font-size: 10px;
    color: #cccccc;
}
.error-label {
    font-size: 11px;
    color: #ff8888;
    font-style: italic;
}
`
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
// Works on X11 and XWayland (GDK_BACKEND=x11).
// onMove is called with the new (x,y) after every position change so callers
// can auto-save without user action.
func enableDrag(win *gtk.Window, onMove func(x, y int)) {
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
		win.Move(newX, newY)
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

// moveWindowForced moves the window to (x, y) by sending _NET_MOVERESIZE_WINDOW
// to the WM via wmctrl. This bypasses XWayland's limitation where XMoveWindow()
// (used by gtk.Window.Move) is silently discarded for xdg_toplevel surfaces —
// the Wayland compositor owns their position and ignores client-side X11 moves.
// wmctrl sends a WM-level client message that GNOME Mutter honours directly.
// Falls back to xdotool if wmctrl is not available.
// Must be called from a goroutine (runs external processes).
func moveWindowForced(title string, x, y int) {
	// wmctrl: -r <title> -e <gravity,x,y,w,h>  (-1 = unchanged for w/h)
	arg := fmt.Sprintf("0,%d,%d,-1,-1", x, y)
	cmd := exec.Command("wmctrl", "-r", title, "-e", arg)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("wmctrl move failed (%v: %s), trying xdotool...", err, string(out))
		// xdotool fallback
		xdo := exec.Command("xdotool", "search", "--name", title,
			"windowmove", strconv.Itoa(x), strconv.Itoa(y))
		if out2, err2 := xdo.CombinedOutput(); err2 != nil {
			log.Printf("xdotool move also failed (%v: %s)", err2, string(out2))
		} else {
			log.Printf("xdotool moved '%s' to (%d,%d)", title, x, y)
		}
	} else {
		log.Printf("wmctrl moved '%s' to (%d,%d)", title, x, y)
	}
}
