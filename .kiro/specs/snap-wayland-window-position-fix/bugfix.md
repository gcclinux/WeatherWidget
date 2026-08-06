# Bugfix Requirements Document

## Introduction

When the GTK weather widget is installed via Snap on Ubuntu (Wayland session),
the window ignores the user-configured position (`customX`/`customY`) and always
opens at the compositor's default position. Position is saved correctly to config
but cannot be restored on next launch.

Previous fix attempts (removing GDK_BACKEND=x11 and relying on gtk_window_move)
proved insufficient because `gtk_window_move()` is a no-op for `xdg_toplevel`
windows on Wayland — the compositor controls placement exclusively.

The correct solution is to use `gtk-layer-shell`, which creates a "layer surface"
via the `wlr-layer-shell` Wayland protocol. Layer surfaces allow the application
to control its own position using margins from screen edges, bypassing the
compositor's exclusive placement control over regular `xdg_toplevel` windows.

## Bug Analysis

### Current Behavior (Defect)

1.1 WHEN the app is launched as a Snap on a Wayland session AND `customX` and
`customY` are set in config THEN the window opens at the compositor-chosen
position (typically 0,0 or centered) instead of the configured coordinates.

1.2 WHEN the app calls `gtk_window_move(x, y)` on a Wayland session THEN the
call is silently ignored because `xdg_toplevel` windows cannot be positioned
by the client under the Wayland protocol.

1.3 WHEN the app sends X11 positioning hints (WM_NORMAL_HINTS, _NET_MOVERESIZE_WINDOW)
via XWayland THEN GNOME Mutter discards them because it controls xdg_toplevel
placement exclusively.

1.4 WHEN the user drags the widget to a position and restarts the app THEN the
saved position is not restored — the window appears at the compositor's default.

### Expected Behavior (Correct)

2.1 WHEN the app is launched on a Wayland session AND `customX` and `customY`
are set in config THEN the window SHALL appear at the exact configured coordinates.

2.2 WHEN the session is Wayland THEN the app SHALL use gtk-layer-shell to create
a layer surface with TOP|LEFT anchors and margins equal to (customX, customY),
giving the app control over its own position.

2.3 WHEN the user drags the widget to a new position on Wayland THEN the app
SHALL update the layer-shell margins dynamically and save the new coordinates.

2.4 WHEN no `customX`/`customY` is configured on Wayland THEN the app SHALL
compute the appropriate corner position and apply it as layer-shell margins.

2.5 WHEN gtk-layer-shell is not available (older compositor or missing library)
THEN the app SHALL fall back to `gtk_window_move()` gracefully.

### Unchanged Behavior (Regression Prevention)

3.1 WHEN the app is running on a native X11 session THEN the system SHALL
CONTINUE TO position the window using `WM_NORMAL_HINTS USPosition` and
`_NET_MOVERESIZE_WINDOW` as before.

3.2 WHEN `customX` and `customY` are not set THEN the system SHALL CONTINUE TO
compute the window position from `cornerPosition` and `monitorIndex`.

3.3 WHEN the user drags the window on X11 THEN the auto-save with 300ms debounce
SHALL CONTINUE TO work as before.

3.4 WHEN the app is launched outside Snap confinement on Wayland THEN the
layer-shell positioning SHALL also work (the fix is not Snap-specific).

3.5 WHEN the window is rebuilt via `rebuildPanels()` THEN positioning SHALL be
correctly re-applied on both Wayland and X11.
