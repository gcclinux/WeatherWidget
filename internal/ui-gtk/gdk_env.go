//go:build linux

package uitk

// #include <stdlib.h>
//
// static void force_gdk_backend_x11(void) {
//     setenv("GDK_BACKEND", "x11", 1);
// }
import "C"

import (
	"os"
	"strings"
)

// ensureGDKBackend selects the GDK backend before GTK reads the environment.
//
// WeatherWidget is a *positioned* desktop widget: the user picks an absolute
// on-screen location. Native Wayland forbids a client from setting its own
// absolute window position, and GNOME's Mutter does not implement the
// wlr-layer-shell protocol that would let a widget anchor to a corner. So on a
// GNOME Wayland session the ONLY way to honour the saved position is to run on
// XWayland (GDK_BACKEND=x11). Compositors that DO implement layer-shell
// (KDE/KWin, sway, Hyprland, wayfire, river) can position natively, so there we
// leave the native Wayland backend in place and buildWindow() uses layer-shell.
//
// The decision, in order:
//
//   - If GDK_BACKEND is already set to a value OTHER THAN a lone "wayland"
//     (e.g. the Snap and Flatpak force GDK_BACKEND=x11 in their environment),
//     it is left untouched. That covers the packaging x11 override and any
//     deliberate user choice such as "x11" or "broadway".
//   - EXCEPTION: if the pre-set backend is exactly "wayland" AND the session is
//     GNOME/Mutter, override it to x11. A positioned widget cannot work under
//     native Wayland on GNOME (no absolute placement, no layer-shell), so a
//     stray `export GDK_BACKEND=wayland` in the user's shell must not be
//     allowed to break positioning. This is the case that made a freshly built
//     AppImage/bin land on the left and refuse to move.
//   - Otherwise, on X11 sessions (native deb/rpm/appimage/bin), force
//     GDK_BACKEND=x11 so the direct X11/XWayland positioning path works — the
//     original, always-worked behaviour.
//   - Otherwise, on a Wayland session that looks like GNOME/Mutter, force
//     GDK_BACKEND=x11 so the widget runs on XWayland and its saved position is
//     honoured.
//   - Otherwise, on a non-GNOME Wayland session (layer-shell capable, or a
//     confined runtime such as Flatpak with no X socket), leave GDK_BACKEND
//     unset so GDK auto-selects its native Wayland backend. The app can open a
//     display, and buildWindow() positions via layer-shell where supported or
//     win.Move() otherwise.
//
// Must be called before gtk.Init().
func ensureGDKBackend() {
	preset := os.Getenv("GDK_BACKEND")
	if preset != "" {
		// A pre-set backend normally wins (Snap/Flatpak force x11). The one
		// exception: a lone "wayland" on GNOME cannot position the widget, so
		// override it to XWayland rather than ship a broken window.
		if isLoneWayland(preset) && looksLikeGNOME() {
			forceX11()
		}
		return
	}
	if !isWayland() {
		// X11 (or no display-server info): force x11 for XWayland positioning.
		forceX11()
		return
	}
	if looksLikeGNOME() {
		// GNOME/Mutter on Wayland: no absolute positioning and no layer-shell,
		// so fall back to XWayland to keep the saved position working.
		forceX11()
		return
	}
	// Non-GNOME Wayland (layer-shell capable, or a confined runtime with no X
	// socket): leave GDK_BACKEND unset so GDK auto-selects native Wayland.
}

// isLoneWayland reports whether a pre-set GDK_BACKEND value selects ONLY the
// Wayland backend. GDK accepts a comma-separated priority list (e.g.
// "wayland,x11" or "x11,wayland"); if x11 appears anywhere GDK can still fall
// back to XWayland, so we only treat a value whose entries are all "wayland" as
// a lone-Wayland selection worth overriding on GNOME.
func isLoneWayland(v string) bool {
	sawWayland := false
	for _, part := range strings.Split(v, ",") {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case "wayland":
			sawWayland = true
		case "":
			// skip empty entries from stray commas
		default:
			// Any non-wayland backend (e.g. x11) means GDK can use it instead.
			return false
		}
	}
	return sawWayland
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
//
// If GNOME is somehow not detected here, the runtime layer-shell probe in
// buildWindow() is the backstop: a real GNOME session reports no layer-shell
// support and falls through to win.Move() (the pre-fix behaviour), never worse.
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
