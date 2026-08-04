# Bugfix Requirements Document

## Introduction

When the GTK weather widget is installed via Snap on Ubuntu, the window ignores
the user-configured position (`customX`/`customY`) and always opens at the
top-left corner of the desktop (0, 0). The position is read from config
correctly and the X11 hints are written, but the window is ultimately placed
at the wrong location.

The root cause is a layering mismatch: Ubuntu defaults to a Wayland session,
and the Snap package sets `GDK_BACKEND=x11` to force XWayland. Under XWayland,
the two positioning mechanisms used by the app — `WM_NORMAL_HINTS USPosition`
(pre-map X11 hint) and `_NET_MOVERESIZE_WINDOW` (post-map client message) —
are relayed through the XWayland bridge, but the Wayland compositor (GNOME
Mutter) controls window placement exclusively and silently discards both
client-side requests. As a result the window lands at the compositor's chosen
position (top-left) regardless of what the app requests.

## Bug Analysis

### Current Behavior (Defect)

1.1 WHEN the app is launched as a Snap on a Wayland session AND `customX` and
`customY` are set in config THEN the system opens the window at position (0, 0)
instead of the configured coordinates.

1.2 WHEN the app applies `WM_NORMAL_HINTS USPosition` via Xlib before the
window is mapped AND the display server is XWayland THEN the system ignores the
hint and the Wayland compositor places the window at its own chosen position.

1.3 WHEN the app sends `_NET_MOVERESIZE_WINDOW` 400 ms after the map-event AND
the display server is XWayland THEN the system fails to move the window to the
requested coordinates because the Wayland compositor does not honour
client-initiated X11 window moves for `xdg_toplevel` surfaces.

1.4 WHEN the app calls `gtk.Window.Move(x, y)` AND GDK is running over XWayland
THEN the system silently discards the move because the Wayland protocol has no
client-side window position API.

### Expected Behavior (Correct)

2.1 WHEN the app is launched as a Snap on a Wayland session AND `customX` and
`customY` are set in config THEN the system SHALL open the window at the
configured coordinates (customX, customY).

2.2 WHEN the session is Wayland AND the window needs to be positioned THEN the
system SHALL use a GTK/GDK Wayland-native positioning strategy (e.g.
`gtk_window_move` with the `GDK_BACKEND=wayland` backend, or a
`wl_surface`-based approach) rather than X11-only Xlib calls.

2.3 WHEN the session type cannot be determined or the Wayland positioning
attempt fails THEN the system SHALL fall back to the existing X11 positioning
path so that native X11 sessions are unaffected.

2.4 WHEN the window is repositioned via drag on a Wayland session AND the new
position is saved to config THEN the system SHALL restore the saved position
correctly on the next launch.

### Unchanged Behavior (Regression Prevention)

3.1 WHEN the app is running on a native X11 session (not XWayland) THEN the
system SHALL CONTINUE TO position the window using `WM_NORMAL_HINTS USPosition`
and `_NET_MOVERESIZE_WINDOW` as before.

3.2 WHEN `customX` and `customY` are not set THEN the system SHALL CONTINUE TO
compute the window position from `cornerPosition` and `monitorIndex` and place
the window at the correct corner.

3.3 WHEN the user drags the window to a new position on X11 THEN the system
SHALL CONTINUE TO auto-save the new coordinates and restore them on next launch.

3.4 WHEN the app is running on macOS or Windows THEN the system SHALL CONTINUE
TO use the existing platform-specific positioning logic without any change.

3.5 WHEN the app is launched without the Snap confinement on a Wayland session
(e.g. built and run directly) THEN the system SHALL CONTINUE TO apply the same
Wayland-aware positioning logic so the fix is not Snap-specific.

3.6 WHEN the window position is restored after a `rebuildPanels` call (e.g.
after a settings change) THEN the system SHALL CONTINUE TO position the new
window at the saved coordinates.
