//go:build linux

package uitk

// #include <stdlib.h>
// #include <stdio.h>
//
// static void force_gdk_backend_x11(void) {
//     setenv("GDK_BACKEND", "x11", 1);
//     // Debug: verify from C side
//     fprintf(stderr, "C-level GDK_BACKEND=%s\n", getenv("GDK_BACKEND"));
// }
import "C"

import "os"

// ensureGDKBackend selects the GDK backend before GTK reads the environment.
//
// It preserves the exact behaviour of every existing package and only relaxes
// the one case that cannot work — forcing x11 on a Wayland desktop inside a
// runtime that exposes no X11 socket (the Flatpak-on-Wayland "cannot open
// display" failure):
//
//   - If GDK_BACKEND is already set (e.g. the Snap sets GDK_BACKEND=x11 in its
//     environment), it is left untouched. Whatever the packaging/user chose
//     wins. This keeps the Snap on its proven x11 path on every desktop.
//   - Otherwise, on X11 sessions (native deb/rpm/appimage), it forces
//     GDK_BACKEND=x11 so the direct-XWayland positioning path works reliably —
//     unchanged from before.
//   - Otherwise, on Wayland sessions with no backend forced (the Flatpak
//     case), it leaves GDK_BACKEND unset so GDK auto-selects its native
//     Wayland backend and the app can actually open a display. Positioning
//     then uses win.Move() before ShowAll() (see manager.buildWindow).
//
// Must be called before gtk.Init().
func ensureGDKBackend() {
	// Respect an explicitly-forced backend (Snap sets GDK_BACKEND=x11).
	if os.Getenv("GDK_BACKEND") != "" {
		return
	}
	if isWayland() {
		// Native Wayland backend: leave GDK_BACKEND unset so GDK auto-selects.
		return
	}
	// X11 (or no display server info): force x11 for XWayland positioning.
	C.force_gdk_backend_x11()
	os.Setenv("GDK_BACKEND", "x11")
}
