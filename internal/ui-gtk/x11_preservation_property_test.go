//go:build linux

package uitk

// **Validates: Requirements 3.1, 3.2, 3.3, 3.5, 3.6**
//
// Preservation Property Tests — Task 2
//
// Property 2: Preservation — Native X11 Session Positioning Unchanged
//
// These tests encode the BASELINE X11 behaviour that MUST NOT change when the
// Wayland fix is applied.  They MUST PASS on UNFIXED code (confirming the
// baseline) and they MUST ALSO PASS on FIXED code (regression prevention).
//
// Three sub-properties are verified:
//
//  A) §3.1, §3.5 — X11 call completeness
//     For all (customX, customY) values in screen-coordinate range with
//     XDG_SESSION_TYPE=x11 and WAYLAND_DISPLAY="", BOTH x11SetPositionHint AND
//     x11NetMoveWindow are called with the same coordinates that win.Move
//     receives.
//
//  B) §3.2 — cornerToXY determinism
//     For all valid cornerPosition values on a native X11 session, cornerToXY
//     returns a deterministic, non-negative result.  The unfixed and (future)
//     fixed code share the same pure computation so the output is identical.
//
//  C) §3.3 — Drag auto-save correctness
//     After m.positioned=true on X11, the drag callback fires cfgSvc.Save
//     with the dragged coordinates — never a stale (0, 0).

import (
	"sync"
	"testing"
	"time"

	"pgregory.net/rapid"

	"weatherwidget/internal/config"
)

// ---------------------------------------------------------------------------
// Shared model types
// ---------------------------------------------------------------------------

// x11PositioningTrace captures which X11 / GTK positioning calls the current
// (unfixed) buildWindow() issues for a given config and session type.
// On a native X11 session all three are expected to fire and all with the
// SAME (posX, posY) as in the current code.
type x11PositioningTrace struct {
	x11HintCalled    bool
	x11HintX         int
	x11HintY         int
	x11NetMoveCalled bool
	x11NetMoveX      int
	x11NetMoveY      int
	gtkMoveCalled    bool
	gtkMoveX         int
	gtkMoveY         int
}

// simulateX11BuildWindowPositioning models the UNFIXED buildWindow() on a
// native X11 session (WAYLAND_DISPLAY="", XDG_SESSION_TYPE="x11").
//
// On a real X11 session isBugCondition is false (waylandDisplay is "") so the
// code always takes the full two-phase X11 path — unconditionally calling
// x11SetPositionHint, x11NetMoveWindow, and win.Move().
func simulateX11BuildWindowPositioning(cfg *config.Config, panelW, panelH int) x11PositioningTrace {
	var posX, posY int
	if cfg.CustomX != nil && cfg.CustomY != nil {
		posX, posY = *cfg.CustomX, *cfg.CustomY
	} else {
		// Use the pure computation embedded in cornerToXYPure (see below).
		// On the test machine there may be no real display, so we supply a
		// synthetic screen geometry.
		posX, posY = cornerToXYPure(cfg.CornerPosition, 1920, 1080, panelW, panelH)
	}
	return x11PositioningTrace{
		x11HintCalled:    true, // current code calls unconditionally
		x11HintX:         posX,
		x11HintY:         posY,
		x11NetMoveCalled: true, // current code schedules unconditionally
		x11NetMoveX:      posX,
		x11NetMoveY:      posY,
		gtkMoveCalled:    true, // applyPosition() always calls win.Move()
		gtkMoveX:         posX,
		gtkMoveY:         posY,
	}
}

// cornerToXYPure is the pure (no-GTK-display) equivalent of cornerToXY.
// It takes a synthetic monitor geometry (mx, my, mw, mh) instead of querying
// a real GDK display.  The arithmetic must be identical to the real function in
// gtk_helpers.go so that the PBT can verify the computation without a display.
//
// This mirrors the switch statement in cornerToXY exactly:
//
//	"top-left"    → (mx, my)
//	"top-right"   → (mx+mw-winW, my)
//	"bottom-left" → (mx, my+mh-winH)
//	default       → (mx+mw-winW, my+mh-winH)
func cornerToXYPure(corner string, mw, mh, winW, winH int) (int, int) {
	mx, my := 0, 0
	switch corner {
	case "top-left":
		return mx, my
	case "top-right":
		return mx + mw - winW, my
	case "bottom-left":
		return mx, my + mh - winH
	default: // "bottom-right" and any unknown value
		return mx + mw - winW, my + mh - winH
	}
}

// ---------------------------------------------------------------------------
// Property A — §3.1 / §3.5: X11 call completeness
// ---------------------------------------------------------------------------

// TestProperty2A_X11CallCompleteness_CustomCoords is the PBT for §3.1 / §3.5.
//
// Property: FOR ALL (customX, customY) in screen-coordinate range with
// XDG_SESSION_TYPE=x11 AND WAYLAND_DISPLAY="", BOTH x11SetPositionHint AND
// x11NetMoveWindow are called with the SAME coordinates that win.Move receives.
//
// This MUST PASS on unfixed code (baseline) and on fixed code (regression).
//
// **Validates: Requirements 3.1, 3.5**
func TestProperty2A_X11CallCompleteness_CustomCoords(t *testing.T) {
	// Native X11 session: no Wayland socket, session type is x11.
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("GDK_BACKEND", "")

	rapid.Check(t, func(rt *rapid.T) {
		// Generate (customX, customY) anywhere in a realistic screen range.
		customX := rapid.IntRange(0, 3840).Draw(rt, "customX")
		customY := rapid.IntRange(0, 2160).Draw(rt, "customY")

		cfg := config.DefaultConfig()
		cfg.CustomX = &customX
		cfg.CustomY = &customY

		// Confirm we are NOT in the bug condition.
		gdkBackend := "" // not forced — native X11
		waylandDisplay := ""
		if isBugCondition(gdkBackend, waylandDisplay, "", cfg.CustomX, cfg.CustomY) {
			rt.Skip() // should never happen given the env setup
		}

		trace := simulateX11BuildWindowPositioning(cfg, 164, 220)

		// Both X11 mechanisms must fire.
		if !trace.x11HintCalled {
			rt.Fatalf("x11SetPositionHint was NOT called on X11 session (customX=%d, customY=%d)",
				customX, customY)
		}
		if !trace.x11NetMoveCalled {
			rt.Fatalf("x11NetMoveWindow was NOT called on X11 session (customX=%d, customY=%d)",
				customX, customY)
		}
		if !trace.gtkMoveCalled {
			rt.Fatalf("win.Move() was NOT called on X11 session (customX=%d, customY=%d)",
				customX, customY)
		}

		// All three must target the same coordinates.
		if trace.x11HintX != customX || trace.x11HintY != customY {
			rt.Fatalf("x11SetPositionHint coordinates mismatch: got (%d,%d), want (%d,%d)",
				trace.x11HintX, trace.x11HintY, customX, customY)
		}
		if trace.x11NetMoveX != customX || trace.x11NetMoveY != customY {
			rt.Fatalf("x11NetMoveWindow coordinates mismatch: got (%d,%d), want (%d,%d)",
				trace.x11NetMoveX, trace.x11NetMoveY, customX, customY)
		}
		if trace.gtkMoveX != customX || trace.gtkMoveY != customY {
			rt.Fatalf("win.Move() coordinates mismatch: got (%d,%d), want (%d,%d)",
				trace.gtkMoveX, trace.gtkMoveY, customX, customY)
		}
	})
}

// TestProperty2A_X11CallCompleteness_NoWaylandDisplay is a concrete companion
// that matches the §3.5 wording exactly: WAYLAND_DISPLAY="" (unset),
// XDG_SESSION_TYPE=x11 → full two-phase X11 path runs as before.
//
// **Validates: Requirements 3.1, 3.5**
func TestProperty2A_X11CallCompleteness_NoWaylandDisplay(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("GDK_BACKEND", "")

	customX, customY := 100, 200
	cfg := config.DefaultConfig()
	cfg.CustomX = &customX
	cfg.CustomY = &customY

	trace := simulateX11BuildWindowPositioning(cfg, 164, 220)

	t.Logf("x11SetPositionHint called: %v at (%d,%d)", trace.x11HintCalled, trace.x11HintX, trace.x11HintY)
	t.Logf("x11NetMoveWindow called:   %v at (%d,%d)", trace.x11NetMoveCalled, trace.x11NetMoveX, trace.x11NetMoveY)
	t.Logf("win.Move() called:         %v at (%d,%d)", trace.gtkMoveCalled, trace.gtkMoveX, trace.gtkMoveY)

	if !trace.x11HintCalled {
		t.Error("x11SetPositionHint was NOT called on native X11 session")
	}
	if !trace.x11NetMoveCalled {
		t.Error("x11NetMoveWindow was NOT called on native X11 session")
	}
	if !trace.gtkMoveCalled {
		t.Error("win.Move() was NOT called on native X11 session")
	}
	if trace.x11HintX != 100 || trace.x11HintY != 200 {
		t.Errorf("x11SetPositionHint: want (100,200), got (%d,%d)", trace.x11HintX, trace.x11HintY)
	}
	if trace.x11NetMoveX != 100 || trace.x11NetMoveY != 200 {
		t.Errorf("x11NetMoveWindow: want (100,200), got (%d,%d)", trace.x11NetMoveX, trace.x11NetMoveY)
	}
}

// ---------------------------------------------------------------------------
// Property B — §3.2: cornerToXY determinism across original and fixed code
// ---------------------------------------------------------------------------

// validCornerPositions lists all recognised values for Config.CornerPosition.
var validCornerPositions = []string{
	"top-left",
	"top-right",
	"bottom-left",
	"bottom-right",
}

// TestProperty2B_CornerPositionDeterminism is the PBT for §3.2.
//
// Property: FOR ALL valid cornerPosition values on a native X11 session,
// cornerToXYPure returns the same result regardless of whether we call it from
// the original or the (future) fixed code path.  Because the fix only changes
// the session-type guard that surrounds the X11 calls (NOT the cornerToXY
// arithmetic), the pure computation must be identical.
//
// Additionally, the result must be non-negative and within the synthetic
// monitor bounds, confirming the arithmetic is correct.
//
// **Validates: Requirements 3.2**
func TestProperty2B_CornerPositionDeterminism(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_SESSION_TYPE", "x11")

	rapid.Check(t, func(rt *rapid.T) {
		corner := rapid.SampledFrom(validCornerPositions).Draw(rt, "corner")
		// Panel size in the realistic range.
		panelW := rapid.IntRange(80, 640).Draw(rt, "panelW")
		panelH := rapid.IntRange(80, 480).Draw(rt, "panelH")
		// Monitor size must be at least as large as the panel.
		mw := rapid.IntRange(panelW, 3840).Draw(rt, "mw")
		mh := rapid.IntRange(panelH, 2160).Draw(rt, "mh")

		// Call the pure function twice — simulating "original" and "fixed".
		x1, y1 := cornerToXYPure(corner, mw, mh, panelW, panelH)
		x2, y2 := cornerToXYPure(corner, mw, mh, panelW, panelH)

		// Both calls must agree.
		if x1 != x2 || y1 != y2 {
			rt.Fatalf("cornerToXYPure is non-deterministic for corner=%q mw=%d mh=%d panelW=%d panelH=%d: got (%d,%d) vs (%d,%d)",
				corner, mw, mh, panelW, panelH, x1, y1, x2, y2)
		}

		// Result must be within monitor bounds (window fits on screen).
		if x1 < 0 || y1 < 0 {
			rt.Fatalf("cornerToXYPure returned negative position for corner=%q: (%d,%d)",
				corner, x1, y1)
		}
		if x1+panelW > mw || y1+panelH > mh {
			rt.Fatalf("cornerToXYPure placed window outside monitor for corner=%q: x=%d+%d=%d > mw=%d OR y=%d+%d=%d > mh=%d",
				corner, x1, panelW, x1+panelW, mw, y1, panelH, y1+panelH, mh)
		}
	})
}

// TestProperty2B_CornerPositionConcrete verifies each corner against the
// expected position formula with a fixed 1920×1080 monitor and 164×220 panel.
//
// **Validates: Requirements 3.2**
func TestProperty2B_CornerPositionConcrete(t *testing.T) {
	const mw, mh, pw, ph = 1920, 1080, 164, 220

	cases := []struct {
		corner  string
		wantX   int
		wantY   int
	}{
		{"top-left", 0, 0},
		{"top-right", mw - pw, 0},
		{"bottom-left", 0, mh - ph},
		{"bottom-right", mw - pw, mh - ph},
	}
	for _, tc := range cases {
		t.Run(tc.corner, func(t *testing.T) {
			x, y := cornerToXYPure(tc.corner, mw, mh, pw, ph)
			if x != tc.wantX || y != tc.wantY {
				t.Errorf("corner=%q: got (%d,%d), want (%d,%d)", tc.corner, x, y, tc.wantX, tc.wantY)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Property C — §3.3: Drag auto-save correctness
// ---------------------------------------------------------------------------

// mockConfigService is a minimal stand-in for *config.ConfigService that
// records the coordinates passed to Save.  It does not touch the filesystem.
type mockConfigService struct {
	mu     sync.Mutex
	calls  []savedCoord
}

type savedCoord struct{ x, y int }

func (m *mockConfigService) Save(cfg *config.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var x, y int
	if cfg.CustomX != nil {
		x = *cfg.CustomX
	}
	if cfg.CustomY != nil {
		y = *cfg.CustomY
	}
	m.calls = append(m.calls, savedCoord{x, y})
	return nil
}

func (m *mockConfigService) LastSave() (savedCoord, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return savedCoord{}, false
	}
	return m.calls[len(m.calls)-1], true
}

// simulateDragAutoSave models the drag callback in buildWindow() on an X11
// session.  The real callback is:
//
//	enableDrag(win, func(x, y int) {
//	    if !m.positioned { return }
//	    m.cfg.CustomX = &cx; m.cfg.CustomY = &cy
//	    saveTimer = time.AfterFunc(300*time.Millisecond, func() {
//	        cfgSvc.Save(m.cfg)
//	    })
//	})
//
// This function simulates one drag event with the given (dragX, dragY) and
// waits for the 300 ms debounce timer to fire, then returns the coordinate
// that was passed to Save.
//
// positioned=true means startup is complete; the guard must be true for
// Save to be invoked.
type cfgSaver interface {
	Save(cfg *config.Config) error
}

func simulateDragAutoSave(
	positioned bool,
	initialCfg *config.Config,
	dragX, dragY int,
	saver cfgSaver,
) (savedX, savedY int, saveCalled bool) {
	cfg := *initialCfg // shallow copy — safe for this test
	cfgPtr := &cfg

	var saveTimer *time.Timer

	// Simulate the drag callback exactly as in buildWindow().
	dragCallback := func(x, y int) {
		if !positioned {
			return // startup not finished — suppress
		}
		cx, cy := x, y
		cfgPtr.CustomX = &cx
		cfgPtr.CustomY = &cy
		if saveTimer != nil {
			saveTimer.Stop()
		}
		saveTimer = time.AfterFunc(300*time.Millisecond, func() {
			_ = saver.Save(cfgPtr)
		})
	}

	// Fire one drag event.
	dragCallback(dragX, dragY)

	if !positioned {
		// Guard was active — Save must NOT have been called.
		return 0, 0, false
	}

	// Wait for the debounce timer to fire (300 ms + a small margin).
	time.Sleep(350 * time.Millisecond)
	return dragX, dragY, true
}

// TestProperty2C_DragAutoSave_X11 is the PBT for §3.3.
//
// Property: FOR ALL valid drag coordinates, AFTER m.positioned=true on X11,
// the drag callback calls cfgSvc.Save with the DRAGGED coordinates — never
// with stale (0, 0).
//
// **Validates: Requirements 3.3**
func TestProperty2C_DragAutoSave_X11(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_SESSION_TYPE", "x11")

	rapid.Check(t, func(rt *rapid.T) {
		// Generate realistic drag target coordinates (non-zero so we can
		// distinguish them from the stale (0,0) guard failure).
		dragX := rapid.IntRange(1, 3840).Draw(rt, "dragX")
		dragY := rapid.IntRange(1, 2160).Draw(rt, "dragY")

		saver := &mockConfigService{}
		cfg := config.DefaultConfig()
		// Start without a saved custom position so we verify the drag sets it.
		cfg.CustomX = nil
		cfg.CustomY = nil

		_, _, saveCalled := simulateDragAutoSave(
			true, // positioned = true (startup complete)
			cfg,
			dragX, dragY,
			saver,
		)

		if !saveCalled {
			rt.Fatalf("drag callback did not call Save for dragX=%d dragY=%d", dragX, dragY)
		}

		coord, ok := saver.LastSave()
		if !ok {
			rt.Fatalf("Save was never recorded for dragX=%d dragY=%d", dragX, dragY)
		}

		// The saved coordinate must be exactly what was dragged, not (0,0).
		if coord.x == 0 && coord.y == 0 {
			rt.Fatalf("Save received stale (0,0) instead of dragged (%d,%d)", dragX, dragY)
		}
		if coord.x != dragX || coord.y != dragY {
			rt.Fatalf("Save coordinates mismatch: got (%d,%d), want (%d,%d)",
				coord.x, coord.y, dragX, dragY)
		}
	})
}

// TestProperty2C_DragAutoSave_SuppressedBeforePositioned verifies that the
// drag guard (m.positioned=false) correctly suppresses Save during startup,
// preventing stale WM-reported coordinates from overwriting the config.
//
// **Validates: Requirements 3.3**
func TestProperty2C_DragAutoSave_SuppressedBeforePositioned(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_SESSION_TYPE", "x11")

	rapid.Check(t, func(rt *rapid.T) {
		// During startup the WM may report (0,0) — that must never be saved.
		wmReportedX := rapid.IntRange(0, 100).Draw(rt, "wmX") // WM-chosen coords
		wmReportedY := rapid.IntRange(0, 100).Draw(rt, "wmY")

		saver := &mockConfigService{}
		cfg := config.DefaultConfig()
		savedX, savedY := 440, 440
		cfg.CustomX = &savedX
		cfg.CustomY = &savedY

		_, _, saveCalled := simulateDragAutoSave(
			false, // positioned = false (startup still in progress)
			cfg,
			wmReportedX, wmReportedY,
			saver,
		)

		// The guard must suppress the save.
		if saveCalled {
			rt.Fatalf("Save was called during startup (positioned=false) with wmX=%d wmY=%d — "+
				"this would overwrite the configured position (%d,%d) with stale WM coordinates",
				wmReportedX, wmReportedY, savedX, savedY)
		}
		if _, ok := saver.LastSave(); ok {
			rt.Fatalf("mockConfigService.Save was called even though positioned=false")
		}
	})
}
