//go:build linux && layershell

// This file is compiled only when the "layershell" build tag is set. It links
// libgtk-layer-shell (pkg-config: gtk-layer-shell-0), which is an ADDITIONAL
// build/runtime dependency. Default builds (all current packages: bin, deb,
// rpm, appimage, snap, flatpak) do NOT set this tag and therefore do NOT need
// the library — see gtk_layer_shell_stub.go for the no-op fallback.
//
// To build with native Wayland layer-shell positioning:
//
//	go build -tags layershell ./cmd/weatherwidget-gtk/
//
// and ensure libgtk-layer-shell(-dev) is installed / staged / bundled.

package uitk

// #cgo pkg-config: gtk+-3.0 gtk-layer-shell-0
// #include <gtk/gtk.h>
// #include <gtk-layer-shell/gtk-layer-shell.h>
//
// // ws_layer_is_supported reports whether the running compositor implements
// // the wlr-layer-shell protocol. Requires GDK to be initialised (call after
// // gtk_init). Returns 1 when supported, 0 otherwise. Compositors that support
// // it: KWin (KDE), sway, Hyprland, wayfire, river. Mutter (GNOME) does not.
// static int ws_layer_is_supported(void) {
//     return gtk_layer_is_supported() ? 1 : 0;
// }
//
// // ws_layer_init initialises layer-shell for the window. Must be called BEFORE
// // the window is realized/shown.
// static void ws_layer_init(GtkWindow *win) {
//     gtk_layer_init_for_window(win);
//     gtk_layer_set_layer(win, GTK_LAYER_SHELL_LAYER_TOP);
//     // Do not reserve exclusive space — the widget must not push other windows.
//     gtk_layer_set_exclusive_zone(win, 0);
// }
//
// // ws_layer_set_anchors anchors the window to a pair of edges (a corner). Each
// // argument is a boolean (0/1). Anchoring two adjacent edges pins the window to
// // that corner; margins then offset it inward.
// static void ws_layer_set_anchors(GtkWindow *win, int left, int right, int top, int bottom) {
//     gtk_layer_set_anchor(win, GTK_LAYER_SHELL_EDGE_LEFT,   left   ? TRUE : FALSE);
//     gtk_layer_set_anchor(win, GTK_LAYER_SHELL_EDGE_RIGHT,  right  ? TRUE : FALSE);
//     gtk_layer_set_anchor(win, GTK_LAYER_SHELL_EDGE_TOP,    top    ? TRUE : FALSE);
//     gtk_layer_set_anchor(win, GTK_LAYER_SHELL_EDGE_BOTTOM, bottom ? TRUE : FALSE);
// }
//
// // ws_layer_set_margins sets the per-edge margin (in px) that offsets the
// // window from each anchored edge.
// static void ws_layer_set_margins(GtkWindow *win, int left, int right, int top, int bottom) {
//     gtk_layer_set_margin(win, GTK_LAYER_SHELL_EDGE_LEFT,   left);
//     gtk_layer_set_margin(win, GTK_LAYER_SHELL_EDGE_RIGHT,  right);
//     gtk_layer_set_margin(win, GTK_LAYER_SHELL_EDGE_TOP,    top);
//     gtk_layer_set_margin(win, GTK_LAYER_SHELL_EDGE_BOTTOM, bottom);
// }
import "C"

import (
	"unsafe"

	"github.com/gotk3/gotk3/gtk"
)

// layerShellBuilt is true in this build variant so callers can prefer the
// layer-shell path when the compositor also supports it at runtime.
const layerShellBuilt = true

// layerShellSupported reports whether the running compositor implements
// wlr-layer-shell. Must be called after gtk.Init().
func layerShellSupported() bool {
	return C.ws_layer_is_supported() == 1
}

// nativeWindow returns the underlying GtkWindow pointer, or nil if unavailable.
func nativeWindow(win *gtk.Window) *C.GtkWindow {
	ptr := win.Native()
	if ptr == 0 {
		return nil
	}
	return (*C.GtkWindow)(unsafe.Pointer(ptr))
}

// layerShellInit initialises layer-shell for the window (TOP layer, no
// exclusive zone). Must be called before the window is realized/shown.
func layerShellInit(win *gtk.Window) {
	if w := nativeWindow(win); w != nil {
		C.ws_layer_init(w)
	}
}

// layerShellPosition anchors the window to the corner implied by (anchorLeft,
// anchorTop) and offsets it inward by (marginX, marginY) px. Only the two
// anchored edges receive a margin; the opposite edges are anchored off and
// their margins are zero.
func layerShellPosition(win *gtk.Window, anchorLeft, anchorTop bool, marginX, marginY int) {
	w := nativeWindow(win)
	if w == nil {
		return
	}
	left := b2i(anchorLeft)
	right := b2i(!anchorLeft)
	top := b2i(anchorTop)
	bottom := b2i(!anchorTop)
	C.ws_layer_set_anchors(w, C.int(left), C.int(right), C.int(top), C.int(bottom))

	var ml, mr, mt, mb int
	if anchorLeft {
		ml = marginX
	} else {
		mr = marginX
	}
	if anchorTop {
		mt = marginY
	} else {
		mb = marginY
	}
	C.ws_layer_set_margins(w, C.int(ml), C.int(mr), C.int(mt), C.int(mb))
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
