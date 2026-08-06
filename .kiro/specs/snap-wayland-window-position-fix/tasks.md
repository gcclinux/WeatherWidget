# Implementation Plan

- [x] 1. Update `snap/snapcraft.yaml` for gtk-layer-shell
  - Remove the `environment:` block (specifically `GDK_BACKEND: x11`) from `apps.weatherwidget`
  - Add `libgtk-layer-shell-dev` to `parts.weatherwidget.build-packages`
  - Add `libgtk-layer-shell0` to `parts.weatherwidget.stage-packages`
  - Remove from `stage-packages`: `wmctrl`, `xdotool`, `x11-utils`, `x11-xserver-utils`, `libxdo3`
  - Keep `x11` and `wayland` plugs (both needed)
  - _Requirements: 2.1, 2.2_

- [x] 2. Remove GDK_BACKEND override from `cmd/weatherwidget-gtk/main.go`
  - Delete the block that sets `GDK_BACKEND=x11` when unset:
    ```go
    if os.Getenv("GDK_BACKEND") == "" {
        os.Setenv("GDK_BACKEND", "x11")
    }
    ```
  - Without this, GDK auto-detects: Wayland backend on Wayland sessions, X11 on X11
  - _Requirements: 2.1, 2.2_

- [x] 3. Create `internal/ui-gtk/gtk_layer_shell.go` with CGo bindings
  - Build tag: `//go:build linux`
  - Package: `uitk`
  - CGo directive: `#cgo pkg-config: gtk-layer-shell-0`
  - Include `<gtk-layer-shell/gtk-layer-shell.h>` and `<gtk/gtk.h>`
  - Define Go constants matching the C enums:
    - `layerBackground = 0`, `layerBottom = 1`, `layerTop = 2`, `layerOverlay = 3`
    - `edgeTop = 0`, `edgeBottom = 1`, `edgeLeft = 2`, `edgeRight = 3`
  - Implement Go wrapper functions that take `*gtk.Window` and call via unsafe.Pointer:
    - `func layerShellInitForWindow(win *gtk.Window)` — calls `gtk_layer_shell_init_for_window`
    - `func layerShellSetLayer(win *gtk.Window, layer int)` — calls `gtk_layer_shell_set_layer`
    - `func layerShellSetAnchor(win *gtk.Window, edge int, anchor bool)` — calls `gtk_layer_shell_set_anchor`
    - `func layerShellSetMargin(win *gtk.Window, edge int, margin int)` — calls `gtk_layer_shell_set_margin`
    - `func layerShellSetExclusiveZone(win *gtk.Window, zone int)` — calls `gtk_layer_shell_set_exclusive_zone`
    - `func layerShellSetNamespace(win *gtk.Window, ns string)` — calls `gtk_layer_shell_set_namespace`
  - Each function should log on failure (nil window pointer)
  - _Requirements: 2.2_

- [x] 4. Update `buildWindow()` in `internal/ui-gtk/manager.go` for layer-shell positioning
  - After creating the window (`gtk.WindowNew`) and BEFORE `win.SetTitle(...)` etc, add the Wayland layer-shell initialization:
    ```go
    if isWayland() {
        layerShellInitForWindow(win)
        layerShellSetLayer(win, layerBottom)
        layerShellSetAnchor(win, edgeTop, true)
        layerShellSetAnchor(win, edgeLeft, true)
        layerShellSetAnchor(win, edgeRight, false)
        layerShellSetAnchor(win, edgeBottom, false)
        layerShellSetExclusiveZone(win, -1)
        layerShellSetNamespace(win, "weatherwidget")
    }
    ```
  - After computing `posX, posY`, add the Wayland margin-based positioning:
    ```go
    if isWayland() {
        layerShellSetMargin(win, edgeLeft, posX)
        layerShellSetMargin(win, edgeTop, posY)
        log.Printf("layer-shell: positioned at margins left=%d top=%d", posX, posY)
    }
    ```
  - The X11 path (`x11SetPositionHint`, `x11NetMoveWindow`, `applyPosition`) stays inside `if !isWayland()` (already guarded)
  - On Wayland, skip `SetKeepBelow(true)` (layer BOTTOM already handles stacking)
  - On Wayland, skip `SetSkipTaskbarHint(true)` and `SetSkipPagerHint(true)` (layer surfaces don't appear in taskbar)
  - Keep `SetDecorated(false)`, `SetResizable(false)`, `SetAppPaintable(true)` for both paths
  - _Requirements: 2.1, 2.2, 2.4, 3.1_

- [x] 5. Update drag behavior for Wayland in `internal/ui-gtk/manager.go`
  - In the `enableDrag` callback (or in the inline drag closure in `buildWindow`), detect Wayland session
  - On Wayland: instead of calling `win.Move(newX, newY)`, call:
    ```go
    layerShellSetMargin(win, edgeLeft, newX)
    layerShellSetMargin(win, edgeTop, newY)
    ```
  - The auto-save logic (300ms debounce writing customX/customY to config) stays unchanged
  - The `m.positioned` guard stays unchanged (prevents startup spurious saves)
  - Option A: Modify `enableDrag` to accept an `onReposition` callback that does the actual move
  - Option B: Check `isWayland()` inline in the drag handler in `buildWindow()`
  - Choose option B (simpler, keeps the change localized to `buildWindow()`)
  - _Requirements: 2.3, 3.3_

- [x] 6. Remove `moveWindowForced()` from `internal/ui-gtk/gtk_helpers.go`
  - Delete the `moveWindowForced` function (uses wmctrl/xdotool which are removed)
  - Verify no other code references it (grep for `moveWindowForced`)
  - If referenced elsewhere, remove those call sites too
  - _Requirements: 2.1_

- [x] 7. Update existing tests for the new positioning model
  - Update `wayland_position_bug_property_test.go`:
    - The test should model layer-shell positioning: on Wayland, margins are set to (customX, customY)
    - Update `simulateCurrentBuildWindowPositioning` to reflect the layer-shell path
    - The `isBugCondition` function may need updating since GDK_BACKEND is no longer relevant
  - Update `x11_preservation_property_test.go`:
    - X11 tests should still pass unchanged (X11 path is preserved)
    - Verify that `simulateX11BuildWindowPositioning` still works
  - Run `go test ./internal/ui-gtk/...` and ensure all tests pass
  - _Requirements: 2.1, 3.1, 3.2, 3.3_

- [x] 8. Build verification
  - Run `go build ./cmd/weatherwidget-gtk/` to verify the CGo layer-shell bindings compile
  - If `libgtk-layer-shell-dev` is not installed locally, install it: `sudo apt install libgtk-layer-shell-dev`
  - Fix any compilation errors
  - Run `go test ./internal/ui-gtk/...` — all tests must pass
  - Run `go vet ./internal/ui-gtk/...` — no warnings
  - _Requirements: 2.1, 2.2, 3.1_
