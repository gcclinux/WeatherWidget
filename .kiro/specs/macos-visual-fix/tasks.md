# Implementation Plan

## Overview

Fix two macOS-only visual defects: (1) replace `NSWindow.setAlphaValue()` with a `WWidgetBackgroundView` NSView subclass so transparency applies only to the background layer, and (2) configure `contentView.layer.cornerRadius = 12` for rounded corners. All changes are confined to darwin build-tagged files.

## Tasks

- [x] 1. Write bug condition exploration test
  - **Property 1: Bug Condition** - Whole-Window Transparency & Square Corners
  - **CRITICAL**: This test MUST FAIL on unfixed code — failure confirms the bug exists
  - **DO NOT attempt to fix the test or the code when it fails**
  - **NOTE**: This test encodes the expected behavior — it will validate the fix when it passes after implementation
  - **GOAL**: Surface counterexamples that demonstrate both bugs exist on unfixed code
  - **Scoped PBT Approach**: Scope to concrete failing cases: `opacityPercent ∈ {25, 50, 75}` (bug condition: darwin + opacity < 100)
  - Create `internal/ui/win32_darwin_test.go` (build tag `//go:build darwin`)
  - Test 1a — Whole-window alpha: call `setNSWindowAlpha(handle, 50)` on an off-screen NSWindow; assert `[w alphaValue] == 0.5` AND `[w contentView].layer.backgroundColor alpha == 1.0` — this demonstrates the entire window (not just background) is being composited at 50%, confirming Bug 1
  - Test 1b — Square corners: create widget window, inspect `contentView.layer`; assert `cornerRadius == 0` and `masksToBounds == NO` — confirms Bug 2
  - For the property-based variant: generate `opacityPercent ∈ [1, 99]`, for each assert that the FIXED behavior (backgroundAlpha ≈ opacityPercent/100, NSWindow.alphaValue == 1.0) does NOT hold on unfixed code
  - Run test on UNFIXED code
  - **EXPECTED OUTCOME**: Test FAILS (this is correct — it proves both bugs exist)
  - Document counterexamples found, e.g. `"setNSWindowAlpha(handle, 50) → [w alphaValue]=0.5; contentView.layer.cornerRadius=0"`
  - Mark task complete when test is written, run, and failure is documented
  - _Requirements: 1.1, 1.2, 1.3_

- [x] 2. Write preservation property tests (BEFORE implementing fix)
  - **Property 2: Preservation** - Non-Buggy Input Behavior
  - **IMPORTANT**: Follow observation-first methodology — observe UNFIXED code behavior for inputs where `isBugCondition` does NOT hold, then encode as property tests
  - Create `internal/ui/win32_darwin_test.go` (or extend it); create `internal/ui/theme_darwin_test.go`
  - **Observation phase (on unfixed code):**
    - Observe: `setWindowOpacity(100)` on darwin — `[w alphaValue] == 1.0`, dark background visible, content fully opaque
    - Observe: no darwin-specific code is invoked when `GOOS != darwin` (build-tag isolation)
    - Observe: drag/move calls (`moveNSWindowTo`) are unaffected by opacity changes
  - **Preservation property tests to write:**
    - Property 2a — 100% opacity: for `opacityPercent = 100`, assert `[w alphaValue] == 1.0` and background appears fully opaque dark (`layerAlpha == 1.0`) — must hold on BOTH unfixed and fixed code
    - Property 2b — Mouse passthrough: after `setupDarwinWindow` (once written), assert `WWidgetBackgroundView.hitTest:` returns `nil` for any point in its bounds — background does not capture mouse events
    - Property 2c — Window positioning unaffected: `moveNSWindowTo` still repositions the window correctly after the fix; frame origin matches expected values
    - Property 2d — Platform isolation: confirm the darwin code path is not compiled/called on non-darwin builds (build tag check — no direct assertion needed beyond compilation)
  - Write property-based test: for `opacityPercent ∈ [1, 100]`, for all values assert `[w alphaValue] == 1.0` (NSWindow alpha is never changed — this tests the preservation invariant)
  - Run tests on UNFIXED code
  - **EXPECTED OUTCOME**: Tests PASS (confirms baseline behaviors to preserve)
  - Mark task complete when tests are written, run, and passing on unfixed code
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

- [x] 3. Fix macOS visual bugs (background-only transparency + rounded corners)

  - [x] 3.1 Add `WWidgetBackgroundView` NSView subclass and new CGo functions in `internal/ui/win32_darwin.go`
    - Remove the `setNSWindowAlpha` Objective-C CGo function entirely
    - Add `WWidgetBackgroundView` NSView subclass in the CGo block:
      - `wantsLayer` returns `YES`
      - `acceptsFirstResponder` returns `NO`
      - `hitTest:withEvent:` returns `nil` so all mouse events pass through to the Fyne surface
    - Add `setupDarwinWindow(uintptr_t winHandle)` CGo function:
      - `[w setOpaque:NO]` and `[w setBackgroundColor:[NSColor clearColor]]`
      - `contentView.wantsLayer = YES`, `cornerRadius = 12.0`, `masksToBounds = YES`
      - Create and insert `WWidgetBackgroundView` as bottom-most subview of `contentView` with `NSViewWidthSizable | NSViewHeightSizable` autoresizing
      - Set initial layer background color `rgba(0.12, 0.12, 0.12, 1.0)`
    - Add `setDarwinBackgroundAlpha(uintptr_t winHandle, int opacityPercent)` CGo function:
      - Walk `contentView.subviews`, find the `WWidgetBackgroundView` instance
      - Set `sub.layer.backgroundColor` to `rgba(0.12, 0.12, 0.12, opacityPercent/100.0)`
    - Add Go wrapper `applyDarwinWindowSetup()` that calls `getNSWindowHandle()` + `C.setupDarwinWindow(handle)`; use a retry pattern (same as `moveWindow`) if handle is not yet available
    - Update Go `setWindowOpacity` function:
      - Remove `SetDarwinBackgroundShade(opacityPercent)` call
      - Remove `C.setNSWindowAlpha(handle, ...)` call
      - Call `C.setDarwinBackgroundAlpha(handle, C.int(opacityPercent))` instead
    - _Bug_Condition: isBugCondition(X) where X.platform = "darwin" AND X.opacityPercent < 100 (transparency bug); X.platform = "darwin" unconditionally (corners bug)_
    - _Expected_Behavior: backgroundAlpha(result) ≈ opacityPercent/100.0; contentAlpha(result) = 1.0; windowAlphaValue(result) = 1.0; cornerRadius(contentView) ≥ 12.0_
    - _Preservation: NSWindow.alphaValue is never set below 1.0; WWidgetBackgroundView.hitTest returns nil; moveNSWindowTo is unaffected; Windows/Linux code paths unchanged_
    - _Requirements: 2.1, 2.2, 2.3, 3.1, 3.2, 3.5_

  - [x] 3.2 Update `internal/ui/platform_darwin.go` to call `applyDarwinWindowSetup`
    - In `initPlatformWindow`, call `applyDarwinWindowSetup()` after `registerDarwinWindow(w)`:
      ```go
      func initPlatformWindow(w fyne.Window) {
          registerDarwinWindow(w)
          applyDarwinWindowSetup()
      }
      ```
    - _Requirements: 2.1, 2.3_

  - [x] 3.3 Update `internal/ui/theme.go` to return transparent background on macOS
    - In the darwin branch of `widgetTheme.Color`, change `ColorNameBackground` and `ColorNameOverlayBackground` to return `color.NRGBA{A: 0}` (fully transparent) so the Fyne GL surface paints nothing over the native `WWidgetBackgroundView`
    - Remove `SetDarwinBackgroundShade` function
    - Remove `darwinBgShade` atomic variable
    - Remove the `darwinBgShade.Store(30)` line from `init()`
    - _Bug_Condition: darwin theme returning opaque background color was masking the native background view_
    - _Expected_Behavior: theme.Color returns color.NRGBA{A:0} for ColorNameBackground on darwin_
    - _Preservation: Linux and Windows theme paths are unchanged; foreground/disabled/separator colors on darwin are unchanged_
    - _Requirements: 2.1, 3.3, 3.4_

  - [x] 3.4 Verify bug condition exploration test now passes
    - **Property 1: Expected Behavior** - Whole-Window Transparency & Square Corners
    - **IMPORTANT**: Re-run the SAME test from task 1 — do NOT write a new test
    - The test from task 1 encodes the expected behavior (backgroundAlpha ≈ opacityPercent/100, NSWindow.alphaValue = 1.0, cornerRadius ≥ 12.0)
    - When this test passes, it confirms the expected behavior is satisfied for all darwin inputs where `isBugCondition` is true
    - Run bug condition exploration test from step 1
    - **EXPECTED OUTCOME**: Test PASSES (confirms both bugs are fixed)
    - _Requirements: 2.1, 2.2, 2.3_

  - [x] 3.5 Verify preservation tests still pass
    - **Property 2: Preservation** - Non-Buggy Input Behavior
    - **IMPORTANT**: Re-run the SAME tests from task 2 — do NOT write new tests
    - Run all preservation property tests from step 2
    - **EXPECTED OUTCOME**: Tests PASS (confirms no regressions in 100% opacity, mouse passthrough, window positioning, and platform isolation)
    - Confirm all tests still pass after fix (no regressions)

- [x] 4. Write targeted unit tests for the new implementation
  - Create/extend `internal/ui/win32_darwin_test.go` with unit tests (all under `//go:build darwin`):
    - `TestSetupDarwinWindow_NonOpaque`: assert `[w isOpaque] == false` and `[w backgroundColor] == clearColor` after `setupDarwinWindow`
    - `TestSetupDarwinWindow_BgViewInserted`: assert `WWidgetBackgroundView` exists as the first (bottom-most) subview of `contentView`
    - `TestSetupDarwinWindow_CornerRadius`: assert `contentView.layer.cornerRadius >= 12.0` and `masksToBounds == YES`
    - `TestSetDarwinBackgroundAlpha_BoundaryValues`: call `setDarwinBackgroundAlpha` with `opacityPercent ∈ {1, 25, 50, 75, 99, 100}`; for each assert `layerAlpha(bgView) ≈ opacityPercent / 100.0` (within 0.001 tolerance)
    - `TestWWidgetBackgroundView_HitTestReturnsNil`: create a `WWidgetBackgroundView`, call `hitTest:` with a point inside its bounds, assert return value is `nil`
  - Create `internal/ui/theme_darwin_test.go` with:
    - `TestDarwinThemeBackground_Transparent`: assert `widgetTheme.Color(ColorNameBackground, VariantDark)` returns `color.NRGBA{A: 0}` on darwin
    - `TestDarwinThemeOverlayBackground_Transparent`: same assertion for `ColorNameOverlayBackground`
  - _Requirements: 2.1, 2.2, 2.3_

- [x] 5. Checkpoint — Ensure all tests pass
  - Run full darwin test suite: `go test ./internal/ui/... -tags darwin`
  - Ensure all property-based tests (Property 1 and Property 2) pass
  - Ensure all unit tests added in task 4 pass
  - Ensure no compilation errors on darwin build (`go build -o /dev/null ./...`)
  - Ensure no compilation errors on non-darwin builds (Windows/Linux) — darwin-specific code is gated by `//go:build darwin`
  - Ask the user if any questions arise or manual visual verification is needed

## Task Dependency Graph

```json
{
  "waves": [
    { "wave": 1, "tasks": ["1", "2"] },
    { "wave": 2, "tasks": ["3.1"] },
    { "wave": 3, "tasks": ["3.2", "3.3"] },
    { "wave": 4, "tasks": ["3.4", "3.5"] },
    { "wave": 5, "tasks": ["4"] },
    { "wave": 6, "tasks": ["5"] }
  ]
}
```

- Task 1 and Task 2 are independent and can be written in parallel (wave 1)
- Task 3 depends on Tasks 1 and 2 being complete (counterexamples documented, baselines established)
- Tasks 3.1, 3.2, 3.3 can be worked sequentially (each file change is independent but 3.1 must precede 3.2/3.3)
- Tasks 3.4 and 3.5 depend on all of 3.1–3.3 being complete
- Task 4 depends on 3.1 (functions must exist to unit-test)
- Task 5 depends on all prior tasks

## Notes

- All new code is darwin-only: files must carry `//go:build darwin` tags; CGo functions must be in the existing CGo block in `win32_darwin.go`
- Off-screen NSWindow creation for tests: use `NSWindow` with `NSWindowStyleMaskBorderless` and `NSBackingStoreNonretained` initialized to a zero-sized rect off-screen; no display connection is required for unit tests on macOS
- `applyDarwinWindowSetup` should use the same retry-after-delay pattern as `moveWindow` (150 ms, 400 ms, 900 ms) because the NSWindow handle may not be populated until after `Show()` is called
- `SetDarwinBackgroundShade` removal: grep for all call sites before deleting; the only call site is in the old `setWindowOpacity` function in `win32_darwin.go`
- Property-based testing library: use `gopkg.in/check.v1` or `pgregory.net/rapid` (whichever is already in go.mod); if neither is present, add `pgregory.net/rapid` as it integrates cleanly with `testing.T`
