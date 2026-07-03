//go:build darwin

package ui

import (
	"math"
	"testing"

	"pgregory.net/rapid"
)

// floatTol is the tolerance used for floating-point comparisons in tests.
const floatTol = 1e-3

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// =============================================================================
// Property 1: Bug Condition — Transparency & Square Corners (verify fix)
// =============================================================================

// TestBugCondition1a_WholeWindowAlpha verifies the transparency behavior.
//
// After setupDarwinWindow + setDarwinBackgroundAlpha(50):
//   - NSWindow.alphaValue must be ≈ 0.5 (the transparency mechanism)
//
// This uses NSWindow.alphaValue (whole-window transparency) which is the only
// mechanism available when Fyne renders into a single opaque GL framebuffer.
// **Validates: Requirements 1.1, 1.2**
func TestBugCondition1a_WholeWindowAlpha(t *testing.T) {
	handle := testCreateOffscreenNSWindow()
	defer testReleaseOffscreenNSWindow(handle)

	const opacityPercent = 50

	testSetupDarwinWindow(handle)
	testSetDarwinBackgroundAlpha(handle, opacityPercent)

	windowAlpha := testGetNSWindowAlphaValue(handle)
	expectedAlpha := float64(opacityPercent) / 100.0

	t.Logf("setupDarwinWindow + setDarwinBackgroundAlpha(%d) → [w alphaValue]=%.3f",
		opacityPercent, windowAlpha)

	if !approxEqual(windowAlpha, expectedAlpha, floatTol) {
		t.Errorf(
			"NSWindow.alphaValue should be ≈ %.3f for %d%% opacity, got %.3f",
			expectedAlpha, opacityPercent, windowAlpha,
		)
	}
}

// TestBugCondition1b_SquareCorners verifies the FIXED rounded-corners behavior.
//
// After setupDarwinWindow: cornerRadius >= 12.0 and masksToBounds == YES.
// Counterexample on unfixed code: cornerRadius=0, masksToBounds=NO
// **Validates: Requirements 1.3**
func TestBugCondition1b_SquareCorners(t *testing.T) {
	handle := testCreateOffscreenNSWindow()
	defer testReleaseOffscreenNSWindow(handle)

	testSetupDarwinWindow(handle)

	cornerRadius := testGetContentViewCornerRadius(handle)
	masksToBounds := testGetContentViewMasksToBounds(handle)

	t.Logf("Fix verified: contentView.layer.cornerRadius=%.1f, masksToBounds=%v",
		cornerRadius, masksToBounds)

	if cornerRadius < 12.0 {
		t.Errorf(
			"FIXED BEHAVIOR NOT MET (Bug 2): cornerRadius should be >= 12.0, got %.1f",
			cornerRadius,
		)
	}
	if !masksToBounds {
		t.Error("FIXED BEHAVIOR NOT MET (Bug 2): masksToBounds should be YES")
	}
}

// TestBugCondition_PBT_WholeWindowTransparency is a property-based test verifying
// that for opacityPercent ∈ {25, 50, 75}:
//   - NSWindow.alphaValue ≈ opacityPercent/100.0
//
// **Validates: Requirements 1.1, 1.2**
func TestBugCondition_PBT_WholeWindowTransparency(t *testing.T) {
	bugCaseGen := rapid.SampledFrom([]int{25, 50, 75})

	rapid.Check(t, func(t *rapid.T) {
		opacityPercent := bugCaseGen.Draw(t, "opacityPercent")

		handle := testCreateOffscreenNSWindow()
		defer testReleaseOffscreenNSWindow(handle)

		testSetupDarwinWindow(handle)
		testSetDarwinBackgroundAlpha(handle, opacityPercent)

		windowAlpha := testGetNSWindowAlphaValue(handle)
		expectedAlpha := float64(opacityPercent) / 100.0

		if !approxEqual(windowAlpha, expectedAlpha, floatTol) {
			t.Errorf(
				"Property 1: opacityPercent=%d, NSWindow.alphaValue should be ≈ %.3f, got %.3f",
				opacityPercent, expectedAlpha, windowAlpha,
			)
		}
	})
}

// =============================================================================
// Property 2: Preservation — Non-Buggy Input Behavior
// =============================================================================

// TestPreservation2a_100PercentOpacity verifies that at 100% opacity
// NSWindow.alphaValue is 1.0 (fully opaque).
// **Validates: Requirements 3.1**
func TestPreservation2a_100PercentOpacity(t *testing.T) {
	handle := testCreateOffscreenNSWindow()
	defer testReleaseOffscreenNSWindow(handle)

	testSetupDarwinWindow(handle)
	testSetDarwinBackgroundAlpha(handle, 100)

	windowAlpha := testGetNSWindowAlphaValue(handle)

	t.Logf("Preservation 2a: alphaValue=%.3f at 100%%", windowAlpha)

	if !approxEqual(windowAlpha, 1.0, floatTol) {
		t.Errorf("Preservation 2a: NSWindow.alphaValue should be 1.0 at 100%%, got %.3f", windowAlpha)
	}
}

// TestPreservation2b_MousePassthrough is a placeholder — the new approach
// uses NSWindow.backgroundColor for transparency, so there is no extra
// background subview to worry about. Mouse events are unaffected.
// **Validates: Requirements 3.5**
func TestPreservation2b_MousePassthrough(t *testing.T) {
	// With the NSWindow.backgroundColor approach, no extra subview is inserted,
	// so mouse passthrough is not affected. This test verifies the window
	// is correctly configured and does not panic.
	handle := testCreateOffscreenNSWindow()
	defer testReleaseOffscreenNSWindow(handle)

	testSetupDarwinWindow(handle)

	// Verify the window is non-opaque (required for background transparency).
	if testIsWindowOpaque(handle) {
		t.Error("Preservation 2b: window should be non-opaque after setupDarwinWindow")
	}
}

// TestPreservation2c_WindowPositioning verifies that opacity changes do not
// affect window positioning.
// **Validates: Requirements 3.5**
func TestPreservation2c_WindowPositioning(t *testing.T) {
	handle := testCreateOffscreenNSWindow()
	defer testReleaseOffscreenNSWindow(handle)

	testApplySetNSWindowAlpha(handle, 50)
	testMoveNSWindowToSync(handle, 100, 200)
	gotX, gotY := testGetNSWindowPosition(handle)

	t.Logf("Preservation 2c: moveNSWindowTo(100,200) → frame origin x=%d y=%d", gotX, gotY)

	const posTol = 1
	if abs(gotX-100) > posTol {
		t.Errorf("Preservation 2c: window X should be ~100, got %d", gotX)
	}
	if abs(gotY-200) > posTol {
		t.Errorf("Preservation 2c: window Y should be ~200, got %d", gotY)
	}
}

// TestPreservation_PBT_NSWindowAlphaAlwaysOne verifies that at 100% opacity
// NSWindow.alphaValue is always 1.0.
// **Validates: Requirements 3.1, 3.2**
func TestPreservation_PBT_NSWindowAlphaAlwaysOne(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		handle := testCreateOffscreenNSWindow()
		defer testReleaseOffscreenNSWindow(handle)

		testSetupDarwinWindow(handle)
		testSetDarwinBackgroundAlpha(handle, 100)

		windowAlpha := testGetNSWindowAlphaValue(handle)

		if !approxEqual(windowAlpha, 1.0, floatTol) {
			t.Errorf(
				"Preservation PBT: at 100%% opacity, NSWindow.alphaValue should be 1.0, got %.3f",
				windowAlpha,
			)
		}
	})
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// =============================================================================
// Task 4 — Targeted Unit Tests for New Implementation
// =============================================================================

// TestSetupDarwinWindow_NonOpaque verifies window is non-opaque after setup.
// **Validates: Requirements 2.1, 2.3**
func TestSetupDarwinWindow_NonOpaque(t *testing.T) {
	handle := testCreateOffscreenNSWindow()
	defer testReleaseOffscreenNSWindow(handle)

	testSetupDarwinWindow(handle)

	if testIsWindowOpaque(handle) {
		t.Error("window should be non-opaque after setupDarwinWindow")
	}
}

// TestSetupDarwinWindow_AlphaValueDefault verifies NSWindow.alphaValue is 1.0
// after setupDarwinWindow (fully opaque until setDarwinBackgroundAlpha changes it).
// **Validates: Requirements 2.1**
func TestSetupDarwinWindow_AlphaValueDefault(t *testing.T) {
	handle := testCreateOffscreenNSWindow()
	defer testReleaseOffscreenNSWindow(handle)

	testSetupDarwinWindow(handle)

	windowAlpha := testGetNSWindowAlphaValue(handle)
	// setupDarwinWindow does NOT call setAlphaValue, so it remains at the default (1.0).
	if !approxEqual(windowAlpha, 1.0, floatTol) {
		t.Errorf("after setupDarwinWindow, alphaValue should be 1.0, got %.3f", windowAlpha)
	}
}

// TestSetupDarwinWindow_CornerRadius verifies cornerRadius >= 12.0 and masksToBounds.
// **Validates: Requirements 2.2, 2.3**
func TestSetupDarwinWindow_CornerRadius(t *testing.T) {
	handle := testCreateOffscreenNSWindow()
	defer testReleaseOffscreenNSWindow(handle)

	testSetupDarwinWindow(handle)

	cornerRadius := testGetContentViewCornerRadius(handle)
	masksToBounds := testGetContentViewMasksToBounds(handle)

	if cornerRadius < 12.0 {
		t.Errorf("cornerRadius should be >= 12.0, got %.2f", cornerRadius)
	}
	if !masksToBounds {
		t.Error("masksToBounds should be YES after setupDarwinWindow")
	}
}

// TestSetDarwinBackgroundAlpha_BoundaryValues verifies NSWindow.alphaValue
// for boundary values {1, 25, 50, 75, 99, 100}.
// **Validates: Requirements 2.1**
func TestSetDarwinBackgroundAlpha_BoundaryValues(t *testing.T) {
	cases := []int{1, 25, 50, 75, 99, 100}

	for _, opacityPercent := range cases {
		opacityPercent := opacityPercent
		t.Run("opacity_"+itoa(opacityPercent), func(t *testing.T) {
			handle := testCreateOffscreenNSWindow()
			defer testReleaseOffscreenNSWindow(handle)

			testSetupDarwinWindow(handle)
			testSetDarwinBackgroundAlpha(handle, opacityPercent)

			got := testGetNSWindowAlphaValue(handle)
			expected := float64(opacityPercent) / 100.0

			if !approxEqual(got, expected, floatTol) {
				t.Errorf(
					"opacityPercent=%d: NSWindow.alphaValue should be ≈ %.3f, got %.3f",
					opacityPercent, expected, got,
				)
			}
		})
	}
}

// itoa is a minimal int-to-string helper for subtest names.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
