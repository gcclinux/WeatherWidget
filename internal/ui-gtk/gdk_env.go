//go:build linux

package uitk

// #cgo pkg-config: x11
// #include <stdlib.h>
// #include <X11/Xlib.h>
//
// static void force_gdk_backend_x11(void) {
//     setenv("GDK_BACKEND", "x11", 1);
// }
//
// static int x11_display_available(const char *name) {
//     Display *display = XOpenDisplay(name);
//     if (!display) return 0;
//     XCloseDisplay(display);
//     return 1;
// }
import "C"

import (
	"os"
	"strings"
	"unsafe"
)

// ensureGDKBackend selects an available backend before gtk.Init().
func ensureGDKBackend() {
	preset := os.Getenv("GDK_BACKEND")
	if shouldTryX11(preset, isWayland(), layerShellBuilt, looksLikeGNOME()) &&
		x11DisplayAvailable() {
		forceX11()
	}
}

func x11DisplayAvailable() bool {
	name := os.Getenv("DISPLAY")
	if name == "" {
		return false
	}
	nameC := C.CString(name)
	defer C.free(unsafe.Pointer(nameC))
	return C.x11_display_available(nameC) != 0
}

func shouldTryX11(preset string, wayland, layerShell, gnome bool) bool {
	first := strings.ToLower(strings.TrimSpace(strings.Split(preset, ",")[0]))
	if first != "" && first != "wayland" {
		return false
	}
	return !wayland || !layerShell || gnome
}

// forceX11 sets GDK_BACKEND=x11 in both the C runtime environment (which GTK
// reads at init) and Go's os environment (used for logging and the buildWindow
// backend split).
func forceX11() {
	C.force_gdk_backend_x11()
	os.Setenv("GDK_BACKEND", "x11")
}

// looksLikeGNOME reports whether the current session is a GNOME/Mutter desktop.
//
// Detection is heuristic because it must run BEFORE gtk.Init() — we cannot yet
// query the compositor via GDK. GNOME reliably advertises itself through the
// standard freedesktop session variables, so we match those:
//
//   - XDG_CURRENT_DESKTOP: colon-separated list, e.g. "ubuntu:GNOME" or "GNOME".
//   - XDG_SESSION_DESKTOP: e.g. "gnome" or "ubuntu".
//   - GNOME_DESKTOP_SESSION_ID: legacy marker still exported by some sessions.
func looksLikeGNOME() bool {
	if os.Getenv("GNOME_DESKTOP_SESSION_ID") != "" {
		return true
	}
	for _, v := range []string{
		os.Getenv("XDG_CURRENT_DESKTOP"),
		os.Getenv("XDG_SESSION_DESKTOP"),
	} {
		if strings.Contains(strings.ToUpper(v), "GNOME") {
			return true
		}
	}
	return false
}
