# Snap Wayland Window Position Fix — Bugfix Design

## Overview

The GTK weather widget fails to honour `customX`/`customY` position settings
when installed via Snap on Ubuntu Wayland sessions. The Snap forces
`GDK_BACKEND=x11`, which makes GDK use XWayland even on a native Wayland
desktop. Both X11 positioning mechanisms the app currently uses
(`WM_NORMAL_HINTS USPosition` pre-map and `_NET_MOVERESIZE_WINDOW` post-map)
are relayed through the XWayland bridge, but GNOME Mutter — as the Wayland
compositor — discards all client-initiated position requests for
`xdg_toplevel` surfaces. The window therefore always opens at (0, 0).

The fix has two parts:
1. Remove `GDK_BACKEND=x11` from `snap/snapcraft.yaml` so GDK auto-selects the
   Wayland backend on Wayland sessions.
2. Add session-type detection at runtime; when a Wayland session is detected,
   use GTK's native `gtk_window_move()` through the GDK Wayland backend (which
   maps to `xdg_popup`/layer-shell positioning or compositor hints rather than
   XWayland relays). The existing X11 code path is kept as a fallback for native
   X11 sessions.

## Glossary

- **Bug_Condition (C)**: The condition that triggers the positioning bug —
  the app is running under XWayland (GDK forced to x11 while the session is
  Wayland) AND a configured position (`customX`, `customY`) exists.
- **Property (P)**: The desired post-fix behaviour — the window opens at
  the configured coordinates regardless of whether the session is Wayland or X11.
- **Preservation**: All non-Wayland-positioning behaviours (X11 positioning,
  corner-based positioning, drag auto-save, multi-platform behaviour) that must
  remain exactly as before.
- **isBugCondition(env)**: Pseudocode function that returns `true` when the
  environment matches the bug trigger.
- **GDK_BACKEND**: Environment variable that overrides GDK's display-backend
  selection. Currently hard-coded to `x11` in `snap/snapcraft.yaml`.
- **XWayland**: An X11 compatibility server that runs inside a Wayland session,
  bridging X11 apps. Client-initiated window moves are silently discarded by
  the Wayland compositor.
- **xdg_toplevel**: The Wayland protocol surface type used for normal top-level
  windows. Position is compositor-controlled; clients cannot request a position.
- **x11SetPositionHint / x11NetMoveWindow**: Functions in
  `internal/ui-gtk/gtk_x11_move.go` that send X11 position hints via Xlib.
  Effective only when GDK is using a real X11 display server.
- **applyPosition**: Method in `internal/ui-gtk/manager.go` that calls
  `win.Move(x, y)`. With the Wayland backend active, `gtk_window_move` is
  honoured by GDK's Wayland layer; with XWayland it is silently dropped.
- **buildWindow**: Method in `internal/ui-gtk/manager.go` that orchestrates
  window creation, CSS, panel layout, and initial positioning.

## Bug Details

### Bug Condition

The bug manifests when the Snap sets `GDK_BACKEND=x11`, forcing GDK to use the
XWayland bridge instead of the native Wayland backend on a Wayland desktop
session. All three X11 positioning mechanisms (`WM_NORMAL_HINTS USPosition`,
`_NET_MOVERESIZE_WINDOW`, and `gtk.Window.Move`) are then either ignored or
silently discarded by the Wayland compositor (GNOME Mutter), which controls
`xdg_toplevel` placement exclusively.

**Formal Specification:**

```
FUNCTION isBugCondition(env)
  INPUT: env — runtime environment (env vars, GDK backend in use)
  OUTPUT: boolean

  RETURN (env.GDK_BACKEND == "x11" OR env.GDK_BACKEND_ACTIVE == "x11")
         AND (env.WAYLAND_DISPLAY != "" OR env.XDG_SESSION_TYPE == "wayland")
         AND (env.configuredCustomX != nil AND env.configuredCustomY != nil)
END FUNCTION
```

The condition can also hold when `GDK_BACKEND` is unset but XWayland is chosen
as a fallback — however the Snap case is the confirmed trigger.

### Examples

- **Confirmed bug**: Snap installed on Ubuntu 24.04 (Wayland session), config
  has `customX=440, customY=440`. Window opens at (0, 0) instead of (440, 440).
- **Confirmed bug**: Same setup with corner-based positioning
  (`cornerPosition="top-right"`). Window appears at (0, 0) instead of the
  computed corner coordinates.
- **Not affected (X11 session)**: Same Snap installed on an X11 session
  (`XDG_SESSION_TYPE=x11`). Positioning works correctly because the X11 hints
  are handled natively by the WM.
- **Edge case**: `WAYLAND_DISPLAY` is set but `GDK_BACKEND=x11` is overridden
  by the user at launch. Should still trigger the bug condition because GDK
  will use XWayland.

## Expected Behavior

### Preservation Requirements

**Unchanged Behaviors:**
- Native X11 session positioning via `WM_NORMAL_HINTS USPosition` and
  `_NET_MOVERESIZE_WINDOW` must continue to work exactly as before.
- Corner-based positioning (`cornerPosition` + `monitorIndex`) must continue to
  place the window at the correct corner when no `customX`/`customY` is set.
- Drag-to-reposition with 300 ms auto-save must continue to work on both X11
  and Wayland sessions.
- Position restoration from config after `rebuildPanels` (e.g. settings change)
  must continue to work.
- macOS and Windows positioning code must be completely unaffected.
- Running the app outside the Snap (direct binary on Wayland) must also benefit
  from the fix — the fix must not be Snap-specific.

**Scope:**
All inputs where `isBugCondition` returns `false` (native X11 session, or no
configured custom position, or non-Linux platform) must be completely unaffected
by the change. This includes:
- All mouse / drag interactions
- Settings dialog save flow
- Corner-position computation
- CSS / transparency / panel rebuild flows

## Hypothesized Root Cause

Based on bug analysis and code review of `gtk_x11_move.go` and `manager.go`:

1. **Forced XWayland via `GDK_BACKEND=x11`**: `snap/snapcraft.yaml` sets
   `GDK_BACKEND: x11` unconditionally in the app environment. On a Wayland
   session this causes GDK to use the XWayland bridge instead of the native
   Wayland backend, bypassing all Wayland-native positioning support.

2. **`WM_NORMAL_HINTS USPosition` is Wayland-invisible**: `x11SetPositionHint`
   in `gtk_x11_move.go` writes pre-map X11 hints. The XWayland bridge relays
   these to the Wayland compositor, but Mutter's `xdg_toplevel` protocol does
   not expose a way for clients to request an initial position, so the hint is
   silently dropped.

3. **`_NET_MOVERESIZE_WINDOW` is discarded by Mutter for XWayland surfaces**:
   `x11NetMoveWindow` sends a post-map client message. Mutter handles this for
   native X11 windows, but for XWayland surfaces the compositor ignores it
   because `xdg_toplevel` position is compositor-authoritative.

4. **`gtk.Window.Move()` is a no-op over XWayland**: GDK's Wayland backend
   (when properly active) maps `gtk_window_move` to compositor-specific
   mechanisms. But when GDK is forced to the X11 backend (XWayland), the move
   goes through Xlib, which XWayland relays, and Mutter again discards.

## Correctness Properties

Property 1: Bug Condition — Wayland Session Window Positioning

_For any_ runtime environment where `isBugCondition` returns `true` (GDK is
using XWayland while the desktop session is Wayland, and a custom position is
configured), the fixed application SHALL open the window at the configured
`(customX, customY)` coordinates, or at the computed corner position if no
custom coordinates are set, rather than at (0, 0).

**Validates: Requirements 2.1, 2.2, 2.3**

Property 2: Preservation — X11 and Non-Wayland Session Behavior

_For any_ runtime environment where `isBugCondition` returns `false` (native
X11 session, or no custom position configured, or non-Linux platform), the
fixed application SHALL produce exactly the same positioning behavior as the
original application, preserving all X11 hint logic, corner computation, drag
auto-save, and multi-platform positioning paths.

**Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6**

## Fix Implementation

### Changes Required

Assuming the root cause analysis is correct:

**File 1**: `snap/snapcraft.yaml`

**Change**: Remove `GDK_BACKEND: x11` from the app environment block.

**Rationale**: With this line removed, GDK will auto-detect the display
backend at runtime. On Wayland sessions GDK will select the Wayland backend;
on X11 sessions it will select the X11 backend. The Snap's `wayland` plug
already grants access to the Wayland socket.

**Specific Change**:
```yaml
# Remove these lines from apps.weatherwidget.environment:
environment:
  GDK_BACKEND: x11   # DELETE THIS
```

---

**File 2**: `internal/ui-gtk/manager.go`

**Function**: `buildWindow()`

**Specific Changes**:

1. **Add session-type detection**: Before applying position, detect whether the
   session is Wayland by checking `WAYLAND_DISPLAY` and `XDG_SESSION_TYPE`
   environment variables.

2. **Conditional positioning path**: Call `x11SetPositionHint` and
   `x11NetMoveWindow` only on native X11 sessions. On Wayland sessions, rely
   solely on `win.Move(x, y)` (which GDK's Wayland backend handles correctly
   without XWayland interference).

3. **Remove or guard XWayland-broken logic**: The `x11SetPositionHint` pre-map
   call and the 400 ms `x11NetMoveWindow` post-map timeout must not run on
   Wayland sessions since they are no-ops at best and misleading at worst.

Pseudocode for the new positioning logic in `buildWindow`:

```
isWayland := os.Getenv("WAYLAND_DISPLAY") != ""
             OR os.Getenv("XDG_SESSION_TYPE") == "wayland"

win.Realize()
IF NOT isWayland THEN
  x11SetPositionHint(win, posX, posY)   // X11 only: pre-map hint
END IF
m.applyPosition()   // gtk.Window.Move() — works on native Wayland GDK backend

win.Connect("map-event", func() {
  win.SetKeepBelow(true)
  IF NOT isWayland THEN
    glib.TimeoutAdd(400, func() {
      x11NetMoveWindow(win, posX, posY)  // X11 only: post-map override
    })
  END IF
  glib.TimeoutAdd(1000, func() {
    m.positioned = true
  })
})
```

---

**File 3 (optional)**: `internal/ui-gtk/gtk_wayland_move.go` (new file)

If runtime `win.Move()` proves insufficient for certain Wayland compositors,
a separate file with the build tag `//go:build linux` can implement a
`waylandMove(win, x, y)` function using GTK layer-shell or GDK-Wayland-specific
APIs via CGo. This is the escape hatch if Property 1 is not satisfied by the
`snapcraft.yaml` change + removing the XWayland-specific X11 calls alone.

**Stage-packages cleanup**: Once `GDK_BACKEND=x11` is removed, `wmctrl`,
`xdotool`, `x11-utils`, `x11-xserver-utils`, and `libxdo3` may no longer be
needed for positioning. They should be reviewed and removed from
`snap/snapcraft.yaml` `stage-packages` if unused elsewhere.

## Testing Strategy

### Validation Approach

Testing follows a two-phase approach: first run exploratory tests on the
**unfixed** code to confirm the bug condition and root cause, then verify the
fix satisfies Property 1 (window is positioned correctly) and Property 2
(all existing behaviour is unchanged).

### Exploratory Bug Condition Checking

**Goal**: Surface counterexamples demonstrating the bug on unfixed code.
Confirm or refute the root cause analysis. If refuted, re-hypothesize.

**Test Plan**: Mock or stub the GDK backend and environment variables to
simulate the XWayland case. Assert that after `buildWindow()`, the window's
reported position equals `(customX, customY)`. Run on the **unfixed** code to
observe failure and confirm the root cause.

**Test Cases**:

1. **Snap Wayland Positioning Test**: Set `WAYLAND_DISPLAY=/run/user/1000/wayland-0`
   and `GDK_BACKEND=x11` (simulating the Snap environment), configure
   `customX=440, customY=440`, call `buildWindow()`, assert window position is
   `(440, 440)`. Will **fail** on unfixed code — window lands at `(0, 0)`.

2. **Corner Position on Wayland Test**: Same environment, no `customX`/`customY`,
   `cornerPosition="bottom-right"`. Assert window position equals computed
   bottom-right corner. Will **fail** on unfixed code.

3. **`_NET_MOVERESIZE_WINDOW` No-Op Test**: Inject a mock XWayland display,
   send `x11NetMoveWindow`, assert the compositor-reported position is unchanged.
   Confirms the Mutter discard behaviour.

4. **`WM_NORMAL_HINTS` No-Op Test**: Set `USPosition` hint via
   `x11SetPositionHint` on an XWayland surface, map the window, assert reported
   position is not `(customX, customY)`. Confirms pre-map hint is ignored.

**Expected Counterexamples**:
- Window position remains `(0, 0)` despite `customX=440, customY=440`.
- Possible causes confirmed: `GDK_BACKEND=x11` forces XWayland; Mutter discards
  X11 position requests for `xdg_toplevel` surfaces.

### Fix Checking

**Goal**: Verify that for all inputs where `isBugCondition` is true, the fixed
app places the window at the configured position.

**Pseudocode:**

```
FOR ALL env WHERE isBugCondition(env) DO
  result := buildWindow_fixed(env)
  ASSERT result.windowPosition == (env.configuredCustomX, env.configuredCustomY)
         OR (if no custom coords) result.windowPosition == computedCornerPosition(env)
END FOR
```

**Test Cases**:

1. **Fixed Snap Wayland Test**: With `GDK_BACKEND` removed from env and
   `WAYLAND_DISPLAY` set, configure `customX=440, customY=440`. Assert window
   opens at `(440, 440)`.
2. **Fixed Corner Position Test**: Same Wayland env, `cornerPosition="top-right"`.
   Assert window opens at the correct top-right coordinates.
3. **Rebuild Window Test**: Call `rebuildPanels()` on Wayland, assert the
   rebuilt window is positioned correctly.

### Preservation Checking

**Goal**: Verify that for all inputs where `isBugCondition` is false, the fixed
app behaves identically to the original.

**Pseudocode:**

```
FOR ALL env WHERE NOT isBugCondition(env) DO
  ASSERT buildWindow_original(env).windowPosition
       = buildWindow_fixed(env).windowPosition
END FOR
```

**Testing Approach**: Property-based testing is recommended for preservation
checking because:
- It generates many env/config combinations automatically.
- It catches edge cases (missing env vars, nil custom coords, various corner
  values) that manual tests would miss.
- It provides a strong guarantee that the X11 path and other platform paths are
  unchanged.

**Test Plan**: Run existing X11 positioning tests on both original and fixed
code and assert identical outcomes.

**Test Cases**:

1. **X11 Session Preservation**: `XDG_SESSION_TYPE=x11`, `WAYLAND_DISPLAY` unset.
   `customX=100, customY=200`. Assert window position is `(100, 200)` on both
   original and fixed code.
2. **Corner Position on X11 Preservation**: Same X11 env, no custom coords.
   Assert computed corner position is identical to original.
3. **Drag Auto-Save Preservation**: Simulate drag on X11, assert `cfgSvc.Save`
   is called with correct coordinates.
4. **Rebuild on X11 Preservation**: Call `rebuildPanels()` on X11 env, assert
   window position equals saved config coordinates.
5. **No Wayland Display Preservation**: `WAYLAND_DISPLAY=""`,
   `XDG_SESSION_TYPE=x11`. Assert full X11 code path runs as before (both
   `x11SetPositionHint` and `x11NetMoveWindow` are called).

### Unit Tests

- Test `isBugCondition` logic: various combinations of `WAYLAND_DISPLAY`,
  `XDG_SESSION_TYPE`, and `GDK_BACKEND` values return correct boolean.
- Test that `x11SetPositionHint` and `x11NetMoveWindow` are **not** called
  when a Wayland session is detected.
- Test that `x11SetPositionHint` and `x11NetMoveWindow` **are** called when an
  X11 session is detected.
- Test `applyPosition` with and without `customX`/`customY` set.
- Test `cornerToXY` output is unchanged by the fix.

### Property-Based Tests

- Generate random `(customX, customY)` values in screen-coordinate range and
  verify the window is placed at exactly those coordinates on both X11 and
  Wayland (after fix) sessions.
- Generate random `cornerPosition` and `monitorIndex` values and verify
  `cornerToXY` produces the same result on original and fixed code paths.
- Generate random environment variable combinations where
  `isBugCondition = false` and verify the fixed code calls the same X11
  positioning functions as the original.

### Integration Tests

- Full Snap launch simulation on a Wayland compositor (or headless Wayland via
  `weston` in CI): verify window appears at configured position.
- Switch from X11 session to Wayland session (env var change) mid-test and
  verify the session-type detection path is exercised correctly.
- Settings save flow on Wayland: change `customX`/`customY` via the settings
  dialog, restart the app, verify position is restored at the new coordinates.
