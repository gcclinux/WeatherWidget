//go:build linux

package uitk

// #cgo pkg-config: gtk+-3.0 gdk-x11-3.0
// #cgo LDFLAGS: -lX11
// #include <string.h>
// #include <gtk/gtk.h>
// #include <gdk/gdkx.h>
// #include <X11/Xlib.h>
// #include <X11/Xatom.h>
// #include <X11/Xutil.h>
//
// // x11_set_position_hint sets WM_NORMAL_HINTS with USPosition|PPosition before
// // the window is mapped. The WM reads these during MapWindow and places the
// // window at (x,y). USPosition is the highest-priority X11 position hint.
// // Call after gtk_widget_realize() and before gtk_widget_show_all().
// static void x11_set_position_hint(GtkWidget *widget, int x, int y) {
//     GdkWindow *gdk_win = gtk_widget_get_window(widget);
//     if (!gdk_win || !GDK_IS_X11_WINDOW(gdk_win)) return;
//     Display *dpy  = gdk_x11_get_default_xdisplay();
//     Window   xwin = gdk_x11_window_get_xid(gdk_win);
//     XSizeHints hints;
//     long supplied = 0;
//     XGetWMNormalHints(dpy, xwin, &hints, &supplied);
//     hints.flags |= USPosition | PPosition;
//     hints.x = x;
//     hints.y = y;
//     XSetWMNormalHints(dpy, xwin, &hints);
//     XFlush(dpy);
// }
//
// // x11_net_moveresize sends _NET_MOVERESIZE_WINDOW to the root window as a
// // post-map override. Mutter handles this at the WM level (same as wmctrl).
// // Flags: gravity=0 (NW), x-present=0x100, y-present=0x200 => 0x300
// static void x11_net_moveresize(GtkWidget *widget, int x, int y) {
//     GdkWindow *gdk_win = gtk_widget_get_window(widget);
//     if (!gdk_win || !GDK_IS_X11_WINDOW(gdk_win)) return;
//     Display *dpy  = gdk_x11_get_default_xdisplay();
//     Window   xwin = gdk_x11_window_get_xid(gdk_win);
//     Window   root = DefaultRootWindow(dpy);
//     XEvent ev;
//     memset(&ev, 0, sizeof(ev));
//     ev.xclient.type         = ClientMessage;
//     ev.xclient.window       = xwin;
//     ev.xclient.message_type = XInternAtom(dpy, "_NET_MOVERESIZE_WINDOW", False);
//     ev.xclient.format       = 32;
//     ev.xclient.data.l[0]   = 0x300;
//     ev.xclient.data.l[1]   = (long)x;
//     ev.xclient.data.l[2]   = (long)y;
//     ev.xclient.data.l[3]   = 0;
//     ev.xclient.data.l[4]   = 0;
//     XSendEvent(dpy, root, False,
//                SubstructureRedirectMask | SubstructureNotifyMask, &ev);
//     XFlush(dpy);
// }
import "C"

import (
	"log"
	"unsafe"

	"github.com/gotk3/gotk3/gtk"
)

// x11SetPositionHint sets WM_NORMAL_HINTS with USPosition|PPosition so the WM
// places the window at (x, y) during the initial MapWindow request.
// Must be called after win.Realize() and before win.ShowAll().
func x11SetPositionHint(win *gtk.Window, x, y int) {
	ptr := win.Native()
	if ptr == 0 {
		log.Println("x11SetPositionHint: nil pointer")
		return
	}
	C.x11_set_position_hint((*C.GtkWidget)(unsafe.Pointer(ptr)), C.int(x), C.int(y))
	log.Printf("x11SetPositionHint: WM_NORMAL_HINTS USPosition set to (%d,%d)", x, y)
}

// x11NetMoveWindow sends _NET_MOVERESIZE_WINDOW via GDK's existing X11
// connection — same protocol as wmctrl but in-process (no subprocess needed).
func x11NetMoveWindow(win *gtk.Window, x, y int) {
	ptr := win.Native()
	if ptr == 0 {
		log.Println("x11NetMoveWindow: nil pointer")
		return
	}
	C.x11_net_moveresize((*C.GtkWidget)(unsafe.Pointer(ptr)), C.int(x), C.int(y))
	log.Printf("x11NetMoveWindow: sent _NET_MOVERESIZE_WINDOW (%d,%d)", x, y)
}
