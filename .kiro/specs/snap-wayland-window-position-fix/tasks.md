# Implementation Plan

- [x] 1. Write bug condition exploration test
  - **Property 1: Bug Condition** - XWayland Window Positioning Failure
  - **CRITICAL**: This test MUST FAIL on unfixed code — failure confirms the bug exists
  - **DO NOT attempt to fix the test or the code when it fails**
  - **NOTE**: This test encodes the expected behavior — it will validate the fix when it passes after implementation
  - **GOAL**: Surface counterexamples that demonstrate the window lands at (0, 0) instead of the configured position
  - **Scoped PBT Approach**: Scope the property to the concrete failing case: `WAYLAND_DISPLAY` set + `GDK_BACKEND=x11` + `customX=440, customY=440`
  - Simulate the Snap Wayland environment by setting `WAYLAND_DISPLAY=/run/user/1000/wayland-0` and `GDK_BACKEND=x11` in the test process environment
  - Set config `customX=440, customY=440` and call `buildWindow()`
  - Assert that after `buildWindow()`, `win.Move(x, y)` was called with `(440, 440)` AND that neither `x11SetPositionHint` nor `x11NetMoveWindow` short-circuits the result to (0, 0)
  - The `isBugCondition` pseudocode from the design: `GDK_BACKEND_ACTIVE == "x11" AND WAYLAND_DISPLAY != "" AND configuredCustomX != nil`
  - Run test on UNFIXED code — because `GDK_BACKEND=x11` forces XWayland and all X11 position hints are discarded by Mutter, the window reports (0, 0)
  - **EXPECTED OUTCOME**: Test FAILS (this is correct — it proves the bug exists)
  - Document counterexamples found: e.g. `buildWindow(env{GDK_BACKEND=x11, WAYLAND_DISPLAY=set, customX=440})` → window position = (0, 0), expected (440, 440)
  - Mark task complete when test is written, run, and failure is documented
  - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [x] 2. Write preservation property tests (BEFORE implementing fix)
  - **Property 2: Preservation** - Native X11 Session Positioning Unchanged
  - **IMPORTANT**: Follow observation-first methodology
  - Observe on UNFIXED code: with `XDG_SESSION_TYPE=x11` and `WAYLAND_DISPLAY` unset, `buildWindow()` calls `x11SetPositionHint(win, 100, 200)` and schedules `x11NetMoveWindow(win, 100, 200)` — observe both calls happen
  - Observe on UNFIXED code: with `cornerPosition="bottom-right"` and no `customX`/`customY`, `cornerToXY` returns a non-zero position that is passed to `x11SetPositionHint` and `win.Move`
  - Write property-based test: for all `(customX, customY)` values in valid screen-coordinate range AND `XDG_SESSION_TYPE=x11` AND `WAYLAND_DISPLAY=""`, BOTH `x11SetPositionHint` and `x11NetMoveWindow` are called with the same coordinates that `win.Move` receives (from Preservation Requirements §3.1, §3.3)
  - Write property-based test: for all valid `cornerPosition` values on X11 session, `cornerToXY` output is identical on original and fixed code (from §3.2)
  - Write property-based test: after a drag event on X11 (`m.positioned=true`), the drag callback fires `cfgSvc.Save` with the dragged coordinates — not a stale (0,0) (from §3.3)
  - Run tests on UNFIXED code
  - **EXPECTED OUTCOME**: Tests PASS (this confirms baseline X11 behavior to preserve)
  - Mark task complete when tests are written, run, and passing on unfixed code
  - _Requirements: 3.1, 3.2, 3.3, 3.5, 3.6_

- [x] 3. Fix for XWayland window position ignored in Snap on Wayland sessions

  - [x] 3.1 Remove `GDK_BACKEND: x11` from `snap/snapcraft.yaml`
    - Delete the `environment:` block (or just the `GDK_BACKEND: x11` line) under `apps.weatherwidget` in `snap/snapcraft.yaml`
    - This allows GDK to auto-detect the display backend at runtime: Wayland backend on Wayland sessions, X11 backend on X11 sessions
    - The Snap `wayland` plug already grants access to the Wayland socket, so no new plugs are needed
    - _Bug_Condition: `isBugCondition(env)` where `env.GDK_BACKEND == "x11" AND env.WAYLAND_DISPLAY != "" AND env.configuredCustomX != nil`_
    - _Expected_Behavior: GDK selects the native Wayland backend; `gtk_window_move` is honoured by GDK's Wayland layer_
    - _Requirements: 2.1, 2.2_

  - [x] 3.2 Add `isWayland()` session-type detection in `internal/ui-gtk/manager.go`
    - Add a package-level (or local) helper: `func isWayland() bool { return os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland" }`
    - Import `"os"` if not already present
    - This implements the runtime guard that separates the Wayland and X11 positioning paths
    - _Bug_Condition: `env.WAYLAND_DISPLAY != "" OR env.XDG_SESSION_TYPE == "wayland"`_
    - _Requirements: 2.2, 2.3_

  - [x] 3.3 Guard X11-only positioning calls in `buildWindow()` with `!isWayland()`
    - In `buildWindow()`, wrap the `x11SetPositionHint(win, posX, posY)` call with `if !isWayland() { ... }`
    - In the `"map-event"` handler, wrap the 400 ms `x11NetMoveWindow` timeout with `if !isWayland() { ... }`
    - The `m.applyPosition()` call (which calls `win.Move(x, y)`) stays unconditional — it works correctly on both Wayland and X11
    - _Bug_Condition: `isBugCondition(env)` — X11 hints are no-ops under XWayland; guarding them prevents misleading log noise and potential interference_
    - _Expected_Behavior: On Wayland sessions, only `win.Move(x, y)` runs; on X11 sessions, the full two-phase hint + move logic runs as before_
    - _Preservation: §3.1 — native X11 path (both `x11SetPositionHint` and `x11NetMoveWindow`) must still run when `!isWayland()`_
    - _Requirements: 2.2, 2.3, 3.1, 3.5_

  - [x] 3.4 Review and clean up `stage-packages` in `snap/snapcraft.yaml`
    - With `GDK_BACKEND=x11` removed, `wmctrl`, `xdotool`, `x11-utils`, `x11-xserver-utils`, and `libxdo3` are no longer needed for window positioning
    - Check whether any other part of the app uses these packages; remove them from `stage-packages` if unused
    - Keeps the Snap image lean and removes unnecessary X11 tools from a Wayland-capable package
    - _Requirements: 2.1_

  - [x] 3.5 Verify bug condition exploration test now passes
    - **Property 1: Expected Behavior** - XWayland Window Positioning Failure (resolved)
    - **IMPORTANT**: Re-run the SAME test from task 1 — do NOT write a new test
    - The test from task 1 encodes the expected behavior: window placed at `(440, 440)` on a simulated Wayland env
    - When this test passes it confirms: GDK auto-selects the Wayland backend, `win.Move` is honoured, and `(440, 440)` is the actual position
    - Run bug condition exploration test from task 1
    - **EXPECTED OUTCOME**: Test PASSES (confirms bug is fixed)
    - _Requirements: 2.1, 2.2, 2.3_

  - [x] 3.6 Verify preservation tests still pass
    - **Property 2: Preservation** - Native X11 Session Positioning Unchanged
    - **IMPORTANT**: Re-run the SAME tests from task 2 — do NOT write new tests
    - Run preservation property tests from task 2
    - **EXPECTED OUTCOME**: Tests PASS (confirms no regressions on X11 sessions, corner positioning, drag auto-save, and rebuild flow)
    - Confirm all X11 path functions (`x11SetPositionHint`, `x11NetMoveWindow`) are still called on X11 sessions
    - _Requirements: 3.1, 3.2, 3.3, 3.5, 3.6_

- [x] 4. Checkpoint — Ensure all tests pass
  - Run the full test suite (`go test ./internal/ui-gtk/...`)
  - Verify Property 1 (bug condition) test passes
  - Verify Property 2 (preservation) tests pass
  - Verify no compilation errors (both the CGo X11 file and the updated manager compile cleanly)
  - Ensure all tests pass; ask the user if questions arise
