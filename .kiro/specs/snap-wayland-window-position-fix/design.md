# Snap Wayland Window Position Fix — Design (gtk-layer-shell)

## Overview

The GTK weather widget cannot position itself on Wayland because `xdg_toplevel`
windows (normal GTK windows) are compositor-positioned — no client-side API can
override the compositor's placement decision.

The fix uses **gtk-layer-shell** to transform the window from an `xdg_toplevel`
surface into a `layer_surface`. Layer surfaces (used by panels, docks, desktop
widgets) are positioned by the client via anchors and margins, not by the
compositor. This is the standard approach for desktop widgets on Wayland.

## Glossary

- **gtk-layer-shell**: A library that provides GTK3 bindings for the
  `wlr-layer-shell` Wayland protocol. Allows apps to create layer surfaces.
- **wlr-layer-shell**: A Wayland protocol extension for surfaces that should be
  rendered as part of the desktop environment (panels, docks, widgets).
  Supported by GNOME Mutter 42+, KDE KWin, Sway, Hyprland.
- **layer_surface**: A Wayland surface type that the client can position and
  size independently of the compositor's window management.
- **xdg_toplevel**: The standard Wayland surface type for application windows.
  Position is controlled exclusively by the compositor.
- **anchors**: In layer-shell, specify which screen edges the surface attaches to.
  For absolute positioning, anchor TOP|LEFT and use margins as coordinates.
- **margins**: Pixel offsets from anchored edges. With TOP|LEFT anchor,
  margin_left = X coordinate, margin_top = Y coordinate.

## Architecture

### Session Detection

The existing `isWayland()` function detects Wayland sessions. On Wayland, the
layer-shell path is taken. On X11, the existing positioning logic runs unchanged.

### Positioning Strategy

**On Wayland (layer-shell):**
1. After `gtk.WindowNew()`, call `gtk_layer_shell_init_for_window(window)` to
   convert it from xdg_toplevel to a layer_surface.
2. Set layer to `GTK_LAYER_SHELL_LAYER_OVERLAY` (above normal windows, which
   matches the existing `SetKeepBelow(false)` desktop widget behavior — actually
   we want BOTTOM layer to stay behind normal windows, matching SetKeepBelow).
3. Set anchors: `TOP=true, LEFT=true, RIGHT=false, BOTTOM=false`
4. Set margins: `left=customX, top=customY` (or computed corner position)
5. The compositor renders the surface at exactly those coordinates.
6. For drag: update margins via `gtk_layer_shell_set_margin()` and the window
   moves immediately.

**On X11 (unchanged):**
1. Existing two-phase positioning: USPosition hint pre-map + _NET_MOVERESIZE post-map
2. Drag via enableDrag() with auto-save — unchanged.

### Layer Choice

Use `GTK_LAYER_SHELL_LAYER_BOTTOM` — this positions the window below normal
windows but above the desktop background, matching the current `SetKeepBelow(true)`
behavior. The widget stays on the desktop, never obscures application windows.

### CGo Integration

Create `internal/ui-gtk/gtk_layer_shell.go` with CGo bindings:
- `#cgo pkg-config: gtk-layer-shell-0`
- Wrap: `gtk_layer_shell_init_for_window`, `gtk_layer_shell_set_layer`,
  `gtk_layer_shell_set_anchor`, `gtk_layer_shell_set_margin`,
  `gtk_layer_shell_set_exclusive_zone` (set to -1 for no exclusive zone)

### Snap Environment

- Remove `GDK_BACKEND: x11` from `snap/snapcraft.yaml` — GDK must use the
  native Wayland backend for layer-shell to work.
- Remove the `GDK_BACKEND` override in `cmd/weatherwidget-gtk/main.go`.
- Add `libgtk-layer-shell-dev` to build-packages.
- Add `libgtk-layer-shell0` to stage-packages.
- Remove `wmctrl`, `xdotool`, `x11-utils`, `x11-xserver-utils`, `libxdo3`
  from stage-packages (no longer needed).
- Keep the `x11` and `wayland` plugs (app needs both).

### Drag Behavior on Wayland

Layer-shell surfaces don't receive normal window move events from the compositor.
For drag on Wayland:
1. Use the same button-press + motion-notify approach as X11.
2. On each motion event, compute new (x, y) from pointer delta.
3. Call `gtk_layer_shell_set_margin(win, GTK_LAYER_SHELL_EDGE_LEFT, x)` and
   `gtk_layer_shell_set_margin(win, GTK_LAYER_SHELL_EDGE_TOP, y)`.
4. The window repositions immediately (layer-shell margins are live).
5. Auto-save with 300ms debounce — same as X11 path.

### Fallback

If gtk-layer-shell is not available at runtime (library missing):
- Log a warning.
- Fall back to `gtk_window_move()` (won't work on Wayland, but won't crash).
- Use `dlopen("libgtk-layer-shell.so.0")` to check availability, OR
  compile unconditionally and handle gracefully if the compositor doesn't
  support the protocol.

Actually, since we control the Snap package and include the library, the
fallback is only needed for non-Snap builds on systems without the library.
For simplicity, compile with gtk-layer-shell unconditionally (it's always
available in the Snap). For development builds, check `pkg-config` availability.

## Changes Required

### File 1: `snap/snapcraft.yaml`

- Remove `environment: GDK_BACKEND: x11` from `apps.weatherwidget`
- Add `libgtk-layer-shell-dev` to `build-packages`
- Add `libgtk-layer-shell0` to `stage-packages`
- Remove from `stage-packages`: `wmctrl`, `xdotool`, `x11-utils`,
  `x11-xserver-utils`, `libxdo3`

### File 2: `cmd/weatherwidget-gtk/main.go`

- Remove the `GDK_BACKEND` override block:
  ```go
  // DELETE:
  if os.Getenv("GDK_BACKEND") == "" {
      os.Setenv("GDK_BACKEND", "x11")
  }
  ```

### File 3: `internal/ui-gtk/gtk_layer_shell.go` (NEW)

- CGo file with `//go:build linux`
- `#cgo pkg-config: gtk-layer-shell-0`
- Go wrappers for:
  - `layerShellInitForWindow(win *gtk.Window)`
  - `layerShellSetLayer(win *gtk.Window, layer int)`
  - `layerShellSetAnchor(win *gtk.Window, edge int, anchor bool)`
  - `layerShellSetMargin(win *gtk.Window, edge int, margin int)`
  - `layerShellSetExclusiveZone(win *gtk.Window, zone int)`
  - `layerShellSetNamespace(win *gtk.Window, ns string)`
- Constants: `LAYER_BOTTOM`, `EDGE_TOP`, `EDGE_LEFT`, `EDGE_RIGHT`, `EDGE_BOTTOM`

### File 4: `internal/ui-gtk/manager.go`

- In `buildWindow()`, after window creation, add Wayland branch:
  ```go
  if isWayland() {
      layerShellInitForWindow(win)
      layerShellSetLayer(win, LAYER_BOTTOM)
      layerShellSetAnchor(win, EDGE_TOP, true)
      layerShellSetAnchor(win, EDGE_LEFT, true)
      layerShellSetAnchor(win, EDGE_RIGHT, false)
      layerShellSetAnchor(win, EDGE_BOTTOM, false)
      layerShellSetExclusiveZone(win, -1) // don't reserve screen space
      layerShellSetNamespace(win, "weatherwidget")
      layerShellSetMargin(win, EDGE_LEFT, posX)
      layerShellSetMargin(win, EDGE_TOP, posY)
  }
  ```
- The existing `win.Realize()` + `x11SetPositionHint` + `x11NetMoveWindow` stays
  inside the `if !isWayland()` block (already guarded).
- Update `enableDrag` callback for Wayland: when `isWayland()`, call
  `layerShellSetMargin` instead of `win.Move()`.
- Remove `SetKeepBelow(true)` on Wayland (layer-shell BOTTOM layer already
  handles this).
- Remove `SetSkipTaskbarHint`, `SetSkipPagerHint` on Wayland (layer surfaces
  don't appear in taskbar by default).

### File 5: `internal/ui-gtk/gtk_helpers.go`

- Remove or simplify `moveWindowForced()` (no longer needed — wmctrl/xdotool removed)

## Testing Strategy

Since the positioning now uses compositor-level protocols (layer-shell on Wayland,
X11 hints on X11), full integration testing requires a running compositor.
Unit/property tests verify:
1. The correct branch is taken based on session type
2. The margin values computed match the config coordinates
3. The X11 path is unchanged for X11 sessions
4. Drag updates margins correctly on Wayland
5. Corner-to-margin conversion is correct
