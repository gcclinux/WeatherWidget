# Bugfix Requirements Document

## Introduction

On macOS, the WeatherWidget applies transparency using `NSWindow.setAlphaValue()`, which sets opacity at the window level. This causes all window contents — background, text labels, and weather icons — to fade together, making the widget unreadable at lower opacity levels. The fix must isolate transparency to the background layer only, leaving text and icons at full opacity. A secondary issue is that the widget window has square corners, which is inconsistent with macOS design conventions.

## Bug Analysis

### Current Behavior (Defect)

1.1 WHEN the user sets widget transparency to any value below 100% THEN the system applies `NSWindow.setAlphaValue()`, making the entire window — background, text, and icons — uniformly transparent.

1.2 WHEN the user sets transparency to 25% THEN the system renders all UI elements (temperature, city name, weather icon) at 25% opacity, making the widget content barely visible and unreadable.

1.3 WHEN the widget window is displayed on macOS THEN the system renders it with hard square corners, inconsistent with macOS visual conventions.

### Expected Behavior (Correct)

2.1 WHEN the user sets widget transparency to any value below 100% THEN the system SHALL apply transparency only to the window background layer, leaving text and icon elements at full (100%) opacity.

2.2 WHEN the user sets transparency to 25% THEN the system SHALL render the background at ~25% opacity while all text labels and weather icons remain fully opaque and readable.

2.3 WHEN the widget window is displayed on macOS THEN the system SHALL render it with rounded corners (radius ≈ 12pt) matching macOS design conventions.

### Unchanged Behavior (Regression Prevention)

3.1 WHEN transparency is set to 100% (fully opaque) THEN the system SHALL CONTINUE TO display the widget with a fully opaque dark background and fully visible content.

3.2 WHEN the user changes the opacity setting THEN the system SHALL CONTINUE TO immediately reflect the new transparency level without requiring an app restart.

3.3 WHEN the widget is displayed on Windows THEN the system SHALL CONTINUE TO use the existing Win32 color-key transparency mechanism, unaffected by this change.

3.4 WHEN the widget is displayed on Linux THEN the system SHALL CONTINUE TO use the existing `_NET_WM_WINDOW_OPACITY` transparency mechanism, unaffected by this change.

3.5 WHEN the widget window is moved, resized, or repositioned on macOS THEN the system SHALL CONTINUE TO function correctly (drag, corner snapping, position persistence).

---

## Bug Condition Pseudocode

**Bug Condition Function** — identifies inputs that trigger the bug:

```pascal
FUNCTION isBugCondition(X)
  INPUT: X of type OpacityInput { platform: string, opacityPercent: int }
  OUTPUT: boolean

  // Bug occurs when running on macOS AND transparency is active
  RETURN X.platform = "darwin" AND X.opacityPercent < 100
END FUNCTION
```

**Property Specification** — defines correct behavior for buggy inputs:

```pascal
// Property: Fix Checking — Background-Only Transparency
FOR ALL X WHERE isBugCondition(X) DO
  result ← setWindowOpacity'(X)
  ASSERT backgroundAlpha(result) ≈ X.opacityPercent / 100
  ASSERT contentAlpha(result) = 1.0   // text and icons fully opaque
  ASSERT windowAlphaValue(result) = 1.0  // NSWindow.alphaValue is NOT used
END FOR

// Property: Fix Checking — Rounded Corners
FOR ALL W WHERE platform(W) = "darwin" DO
  ASSERT cornerRadius(contentView(W)) ≥ 12.0
  ASSERT contentView(W).wantsLayer = true
END FOR
```

**Preservation Goal:**

```pascal
// Property: Preservation Checking
FOR ALL X WHERE NOT isBugCondition(X) DO
  ASSERT setWindowOpacity(X) = setWindowOpacity'(X)
END FOR
```
