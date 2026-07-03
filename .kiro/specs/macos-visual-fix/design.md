# macOS Visual Fix — Bugfix Design

## Overview

Two macOS-specific visual defects affect the WeatherWidget:

1. **Whole-window transparency**: `NSWindow.setAlphaValue()` scales the opacity of every pixel in the window uniformly, including text labels and weather icons. Users who set transparency below 100% lose readability.
2. **Square window corners**: The borderless splash window has hard rectangular corners, inconsistent with macOS's rounded-corner aesthetic.

The fix replaces `NSWindow.setAlphaValue()` with a native `NSView` subclass (`WWidgetBackgroundView`) that sits behind the Fyne GL canvas and carries the translucent background color. The Fyne canvas itself remains at full opacity. Rounded corners are achieved by making the `contentView` layer-backed and setting `cornerRadius = 12`.

Both changes are confined to the darwin build (`//go:build darwin`) and do not touch Windows or Linux code paths.

---

## Glossary

- **Bug_Condition (C)**: The condition that triggers the bug — `platform = "darwin" AND opacityPercent < 100`, or `platform = "darwin"` for the rounded-corners defect.
- **Property (P)**: The desired behavior when the bug condition holds — background layer alpha reflects the opacity setting, content alpha remains 1.0, and NSWindow.alphaValue is never set below 1.0.
- **Preservation**: All behaviors that must remain unchanged — 100% opaque rendering, Windows/Linux transparency paths, window drag/positioning.
- **`setWindowOpacity`**: The Go function in `internal/ui/win32_darwin.go` called by `UIManager.SetOpacity()`. Currently calls `C.setNSWindowAlpha()` (to be removed).
- **`setNSWindowAlpha`**: The Objective-C CGo helper (to be removed) that calls `[w setAlphaValue:]`.
- **`setupDarwinWindow`**: New Objective-C CGo function that makes the NSWindow non-opaque, inserts `WWidgetBackgroundView`, and configures rounded corners on the content view.
- **`setDarwinBackgroundAlpha`**: New Objective-C CGo function called on every opacity change to update the background view's CALayer color.
- **`WWidgetBackgroundView`**: A minimal `NSView` subclass with `wantsLayer = YES` whose CALayer carries the semi-transparent dark background color.
- **`opacityPercent`**: Integer in `[1, 100]` representing the user's chosen transparency level. Maps to CALayer alpha via `opacityPercent / 100.0`.
- **`darwinBgAlpha`**: Go-side `atomic.Int32` storing the current background alpha in the range `[1, 100]`, replacing the existing `darwinBgShade` pattern for macOS.

---

## Bug Details

### Bug Condition

**Bug 1 — Whole-window transparency** manifests whenever the user applies any opacity below 100% on macOS. The current `setWindowOpacity` calls `C.setNSWindowAlpha(handle, opacityPercent)`, which calls `[w setAlphaValue: alpha]` on the `NSWindow`. `setAlphaValue:` composites the *entire* window — Fyne GL surface, text, icons — at the given alpha before blending with the desktop. There is no mechanism to exempt individual subviews.

**Bug 2 — Square corners** manifests unconditionally on macOS. The `NSWindow` is created as a borderless splash window but the `contentView` is not configured with a layer or corner radius. No CALayer masking means the GL surface renders to a hard rectangle.

**Formal Specification:**

```
FUNCTION isBugCondition(X)
  INPUT: X of type { platform: string, opacityPercent: int }
  OUTPUT: boolean

  IF X.platform ≠ "darwin" THEN RETURN false END IF

  // Bug 1: whole-window transparency is triggered whenever opacity < 100
  IF X.opacityPercent < 100 THEN RETURN true END IF

  // Bug 2: square corners are always present on darwin regardless of opacity
  // (modelled as opacityPercent = 100 branch still being buggy for corners)
  RETURN true
END FUNCTION
```

For the transparency property alone:

```
FUNCTION isBugCondition_transparency(X)
  INPUT: X of type { platform: string, opacityPercent: int }
  OUTPUT: boolean
  RETURN X.platform = "darwin" AND X.opacityPercent < 100
END FUNCTION
```

### Examples

- `opacity=50%, darwin` → **Bug 1**: entire widget (background + "22°C" label + cloud icon) renders at 50% opacity; desktop bleeds through text making it unreadable. Expected: background at 50% alpha, text and icon at 100% opacity.
- `opacity=25%, darwin` → **Bug 1**: widget is barely visible; temperature label is nearly invisible against the desktop. Expected: background at 25% alpha, content fully opaque.
- `opacity=100%, darwin` → **No Bug 1**, but **Bug 2** still present: window has hard square corners.
- `opacity=75%, darwin` → **Both bugs**: transparent content + square corners.
- `opacity=50%, windows` → **Not a bug condition**: Win32 color-key path, unaffected.

---

## Expected Behavior

### Preservation Requirements

**Unchanged Behaviors:**

- Mouse interaction (drag, click) on the widget window must continue to work exactly as before — `WWidgetBackgroundView` must not intercept mouse events (`acceptsFirstResponder` returns NO, `hitTest:` returns nil for background view touches).
- Window positioning, corner snapping, and position persistence (move/retry logic in `moveWindow`) must remain unaffected — the fix does not alter `NSWindow` frame or `setFrameOrigin:`.
- At `opacity=100%`, the widget must display with a fully opaque dark background and fully opaque content, visually identical to current behavior.
- The Windows transparency mechanism (Win32 color-key via `LWA_COLORKEY`) must remain unchanged — this fix is darwin-only via build tags.
- The Linux transparency mechanism (`_NET_WM_WINDOW_OPACITY`) must remain unchanged.
- `UIManager.SetOpacity()` → `setWindowOpacity()` call chain must remain intact; only the implementation of `setWindowOpacity` changes.

**Scope:**

All inputs where `platform ≠ "darwin"` are completely unaffected. On darwin, all behaviors not involving background rendering (drag, positioning, sizing, settings window, tray icon) must be preserved.

---

## Hypothesized Root Cause

### Bug 1 — Whole-window transparency

1. **`NSWindow.setAlphaValue:` is window-level**: Apple's compositing pipeline applies the window alpha *after* all subviews are rendered into the window's backing buffer. There is no API to exempt subviews from this alpha multiplication. The only correct approach is to leave `alphaValue = 1.0` and control alpha at the view/layer level instead.

2. **Fyne GL canvas is a flat surface**: Fyne renders all content (background + widgets) into a single OpenGL framebuffer. There is no layer hierarchy that can be targeted for alpha independently. The background must therefore be a *separate native NSView* sitting below the Fyne surface.

3. **Current theme color is opaque**: `darwinBgShade` in `theme.go` returns `color.NRGBA{R: shade, G: shade, B: shade, A: 255}` — a fully opaque color. This was designed to let `setAlphaValue:` handle transparency, which is the root of Bug 1. After the fix, this must return `color.NRGBA{A: 0}` (fully transparent) so Fyne paints nothing over the native background view.

### Bug 2 — Square corners

4. **No layer configuration after window creation**: `createWidgetWindow` creates the window and `initPlatformWindow` calls `registerDarwinWindow`. Neither function sets `wantsLayer`, `cornerRadius`, or `masksToBounds` on the `contentView`. The CALayer properties are never configured.

---

## Correctness Properties

Property 1: Bug Condition — Background-Only Transparency

_For any_ darwin input where `opacityPercent ∈ [1, 99]` (isBugCondition_transparency returns true), the fixed `setWindowOpacity` / `setDarwinBackgroundAlpha` SHALL:
- set the `WWidgetBackgroundView` CALayer background color to `rgba(0.12, 0.12, 0.12, opacityPercent/100.0)`,
- leave `NSWindow.alphaValue = 1.0` (never call `[w setAlphaValue:]`),
- leave all Fyne-rendered content (text, icons) at full (100%) opacity.

**Validates: Requirements 2.1, 2.2**

Property 2: Preservation — Non-Buggy Input Behavior

_For any_ input where `isBugCondition` does NOT hold (i.e., `platform ≠ "darwin"`, or `opacityPercent = 100`), the fixed code SHALL produce exactly the same observable behavior as the original code:
- Windows: color-key path unchanged.
- Linux: `_NET_WM_WINDOW_OPACITY` path unchanged.
- darwin at 100%: fully opaque dark background, fully opaque content, window appearance identical to current `opacity=100%` behavior.

**Validates: Requirements 3.1, 3.3, 3.4, 3.5**

Property 3: Rounded Corners

_For any_ macOS widget window after `setupDarwinWindow()` completes, the fixed code SHALL configure the `contentView` layer with `wantsLayer = YES`, `cornerRadius ≥ 12.0`, and `masksToBounds = YES`, ensuring all subviews are clipped to a rounded rectangle.

**Validates: Requirements 2.3**

---

## Fix Implementation

### Changes Required

#### File: `internal/ui/win32_darwin.go`

**Remove** the `setNSWindowAlpha` Objective-C function and its Go call site in `setWindowOpacity`.

**Add** the following Objective-C functions in the CGo block:

1. **`WWidgetBackgroundView` (NSView subclass)**
   - `wantsLayer` returns `YES`
   - `acceptsFirstResponder` returns `NO`
   - `hitTest:` returns `nil` so mouse events pass through to the Fyne surface

2. **`setupDarwinWindow(uintptr_t winHandle)`** — called once after window creation:
   ```objc
   NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
   [w setOpaque:NO];
   [w setBackgroundColor:[NSColor clearColor]];

   NSView *contentView = [w contentView];
   contentView.wantsLayer = YES;
   contentView.layer.cornerRadius = 12.0;
   contentView.layer.masksToBounds = YES;

   WWidgetBackgroundView *bgView = [[WWidgetBackgroundView alloc]
       initWithFrame:[contentView bounds]];
   bgView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
   [contentView addSubview:bgView positioned:NSWindowBelow relativeTo:nil];

   bgView.layer.backgroundColor = [[NSColor colorWithRed:0.12
       green:0.12 blue:0.12 alpha:1.0] CGColor];
   ```

3. **`setDarwinBackgroundAlpha(uintptr_t winHandle, int opacityPercent)`** — called on every opacity change:
   ```objc
   NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
   NSView *contentView = [w contentView];
   // Find the WWidgetBackgroundView (first subview by insertion order)
   for (NSView *sub in [contentView subviews]) {
       if ([sub isKindOfClass:[WWidgetBackgroundView class]]) {
           CGFloat alpha = (CGFloat)opacityPercent / 100.0;
           sub.layer.backgroundColor = [[NSColor colorWithRed:0.12
               green:0.12 blue:0.12 alpha:alpha] CGColor];
           break;
       }
   }
   ```

**Update** the Go `setWindowOpacity` function:
- Remove the `C.setNSWindowAlpha(handle, ...)` call.
- Call `C.setDarwinBackgroundAlpha(handle, C.int(opacityPercent))` instead.
- Remove `SetDarwinBackgroundShade(opacityPercent)` (the shade system is superseded).

#### File: `internal/ui/platform_darwin.go`

Update `initPlatformWindow` to call `setupDarwinWindow` after registering the window:

```go
func initPlatformWindow(w fyne.Window) {
    registerDarwinWindow(w)
    setupDarwinWindow()   // configures transparency + rounded corners
}
```

`setupDarwinWindow()` is a thin Go wrapper that retrieves the NSWindow handle and calls `C.setupDarwinWindow(handle)`. Because the handle may not be available until after `Show()`, the setup should be deferred or retried (same pattern as `moveWindow`).

#### File: `internal/ui/theme.go`

Update the darwin branch in `widgetTheme.Color`:

```go
if runtime.GOOS == "darwin" {
    switch name {
    case theme.ColorNameBackground, theme.ColorNameOverlayBackground:
        // Return fully transparent: native WWidgetBackgroundView
        // handles the background color at the CALayer level.
        return color.NRGBA{A: 0}
    case theme.ColorNameForeground:
        return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
    case theme.ColorNameDisabled:
        return color.NRGBA{R: 180, G: 180, B: 180, A: 255}
    case theme.ColorNameSeparator:
        return color.NRGBA{R: 80, G: 80, B: 80, A: 255}
    }
    return t.base.Color(name, theme.VariantDark)
}
```

The `SetDarwinBackgroundShade` function and `darwinBgShade` atomic can be removed; they are no longer used.

---

## Testing Strategy

### Validation Approach

Testing follows a two-phase approach:

1. **Exploratory / Bug Condition Checking**: Run tests against the *unfixed* code to confirm the bug manifests as hypothesized and to understand the root cause.
2. **Fix Checking + Preservation Checking**: After the fix, verify the correctness properties hold and no regression is introduced.

Because the native NSWindow/NSView APIs can only be exercised at runtime, CGo unit tests require a running macOS environment. The test suite uses CGo test helpers that create a headless `NSWindow` (off-screen) to exercise the functions without a visible display.

---

### Exploratory Bug Condition Checking

**Goal**: Surface counterexamples demonstrating the bug on *unfixed* code. Confirm or refute the root cause hypothesis.

**Test Plan**: Create an off-screen `NSWindow`, call `setNSWindowAlpha(handle, 50)`, and inspect `[w alphaValue]`. Also inspect any subview alpha values to confirm the entire window is affected.

**Test Cases**:

1. **Whole-window alpha test** (will demonstrate bug on unfixed code): Call `setNSWindowAlpha(handle, 50)` → assert `[w alphaValue] == 0.5` AND that the window-level alpha is the *only* transparency mechanism active (no layer-level alpha on subviews). This confirms the root cause.
2. **Content alpha leakage test** (will demonstrate bug): After `setNSWindowAlpha(handle, 25)`, simulate rendering and confirm no mechanism exists to render subviews at full opacity — demonstrating the fundamental limitation.
3. **Square corner test** (will demonstrate bug): Create a window via `createWidgetWindow`, inspect `contentView.layer` → assert `cornerRadius == 0` and `masksToBounds == NO`.

**Expected Counterexamples**:
- `[w alphaValue]` is `0.5` after calling `setNSWindowAlpha(handle, 50)` — confirms window-level alpha is being set.
- `contentView.layer.cornerRadius == 0` — confirms square corners.
- Possible causes: `setAlphaValue:` is the wrong API for background-only transparency; CALayer is not configured for contentView.

---

### Fix Checking

**Goal**: After applying the fix, verify that for all `opacityPercent ∈ [1, 99]` on darwin, the correctness properties hold.

**Pseudocode:**
```
FOR ALL opacityPercent IN [1, 99] DO
  handle ← createOffscreenNSWindow()
  setupDarwinWindow(handle)
  setDarwinBackgroundAlpha(handle, opacityPercent)

  w ← nsWindowFromHandle(handle)
  ASSERT [w alphaValue] = 1.0                          // NSWindow alpha untouched
  ASSERT [w isOpaque] = false                          // window is non-opaque
  bgView ← findWWidgetBackgroundView(contentView(w))
  ASSERT bgView ≠ nil                                  // background view exists
  ASSERT layerAlpha(bgView) ≈ opacityPercent / 100.0   // correct layer alpha
  ASSERT [w contentView].layer.cornerRadius ≥ 12.0     // rounded corners
  ASSERT [w contentView].layer.masksToBounds = true
END FOR
```

---

### Preservation Checking

**Goal**: Verify that inputs where the bug condition does NOT hold produce identical behavior before and after the fix.

**Pseudocode:**
```
FOR ALL X WHERE NOT isBugCondition(X) DO
  ASSERT setWindowOpacity_original(X) = setWindowOpacity_fixed(X)
END FOR
```

**Testing Approach**: Property-based testing is recommended because it generates many `opacityPercent` values (edge cases: 0, 1, 99, 100, negative, >100) and platform combinations, catching boundary failures that manual tests miss.

**Test Cases**:

1. **100% opacity preservation** (edge case): Call `setDarwinBackgroundAlpha(handle, 100)` → assert `layerAlpha(bgView) == 1.0` and `[w alphaValue] == 1.0` — fully opaque widget identical to current behavior.
2. **Mouse passthrough preservation**: After `setupDarwinWindow`, simulate a click on the `WWidgetBackgroundView` bounds → assert the background view does not become first responder and the event reaches the Fyne content view.
3. **Subview ordering preservation**: After `setupDarwinWindow`, confirm `WWidgetBackgroundView` is the *bottom-most* subview and does not obscure any Fyne-added subviews above it.
4. **Platform isolation — Windows**: Build and run the Windows code path (`//go:build windows`) → assert `setWindowOpacity` continues to call `setWin32LayeredWindowColor` and does not call any darwin-specific function.
5. **Platform isolation — Linux**: Build and run the Linux code path → assert `setWindowOpacity` continues to call `setLinuxWindowOpacity` unchanged.

---

### Unit Tests

- Test `setupDarwinWindow` creates `WWidgetBackgroundView` as the bottom-most subview of `contentView`.
- Test `setupDarwinWindow` sets `[w isOpaque] = false` and `[w backgroundColor] = clearColor`.
- Test `setupDarwinWindow` sets `contentView.layer.cornerRadius = 12` and `masksToBounds = YES`.
- Test `setDarwinBackgroundAlpha` with boundary values: `1`, `25`, `50`, `75`, `99`, `100`.
- Test that `WWidgetBackgroundView.hitTest:` returns `nil` for points within its bounds.
- Test that `theme.go` darwin branch returns `color.NRGBA{A: 0}` for `ColorNameBackground`.

### Property-Based Tests

- **Fix property** (Property 1): Generate random `opacityPercent ∈ [1, 99]` → for each, assert `layerAlpha(bgView) ≈ opacityPercent / 100.0` and `[w alphaValue] == 1.0`.
- **Preservation property** (Property 2): Generate random `opacityPercent ∈ [1, 100]` → for `opacityPercent = 100`, assert `layerAlpha(bgView) == 1.0`; for all values, assert `[w alphaValue] == 1.0`.
- **Corner property** (Property 3): After `setupDarwinWindow`, assert `contentView.layer.cornerRadius ≥ 12.0` regardless of window size or position.

### Integration Tests

- Launch the full widget on macOS at each opacity level (25%, 50%, 75%, 100%) and visually verify background alpha vs. content opacity.
- Drag the widget across the screen after the fix and verify positioning is unchanged.
- Switch opacity settings at runtime and verify the background view updates without requiring a restart.
- Verify rounded corners persist after the window is moved or resized.
