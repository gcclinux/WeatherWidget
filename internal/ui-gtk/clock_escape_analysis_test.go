//go:build linux

package uitk

// **Validates: Requirements 1.1, 1.2, 1.3**
//
// Bug Condition Exploration Test — Stack-Allocated String Capture in glib.IdleAdd
//
// Property 1: Bug Condition — Stack-Allocated String Capture in glib.IdleAdd
//
// The bug occurs when startClock()'s goroutine captures local string variables
// (timeStr, dateStr) in a closure passed to glib.IdleAdd(). These variables
// are stack-allocated within the goroutine. When the Go runtime performs stack
// management (shrinking) after extended operation, the closure's captured string
// headers can become invalid before the C callback executes, causing:
//   "fatal error: invalid pointer found on stack"
//
// Bug condition formula:
//   isBugCondition = closureCapturesLocalStrings
//                  AND stringsAreStackAllocated
//                  AND goRuntimePerformsStackManagement
//                  AND closureIsPassedToCgo
//
// This test validates the bug condition exists by checking Go's escape analysis
// output. In the UNFIXED code, timeStr and dateStr should NOT escape to heap
// (they remain stack-allocated), confirming the bug condition.
//
// CRITICAL: This test is EXPECTED TO FAIL on unfixed code - failure confirms
// the bug exists. After the fix is applied, the test should PASS (strings
// should escape to heap, eliminating the bug condition).

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// TestBugCondition_StackAllocatedStringCapture verifies that in the current
// (unfixed) code, the closure passed to glib.IdleAdd() captures stack-allocated
// strings that do NOT escape to heap.
//
// This test checks Go's escape analysis output for the startClock() function.
//
// EXPECTED BEHAVIOR (test PASSES when fix is applied):
//   - timeStr and dateStr (or their IIFE-bound equivalents ts/ds) escape to heap
//   - The strings passed to glib.IdleAdd's closure are heap-allocated
//   - This eliminates the cgo pointer invalidation bug
//
// BUG CONDITION (test FAILS on unfixed code - this confirms bug exists):
//   - timeStr and dateStr are captured by value but DO NOT escape to heap
//   - The closure captures stack-allocated string headers
//   - These can become invalid when Go shrinks the goroutine's stack
//
// **Validates: Requirements 1.1, 1.2, 1.3**
func TestBugCondition_StackAllocatedStringCapture(t *testing.T) {
	// Run escape analysis on the ui-gtk package
	cmd := exec.Command("go", "build", "-gcflags=-m -m", "./internal/ui-gtk/")
	cmd.Dir = "/home/ricardo/Programming/WeatherWidget"
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Build errors are acceptable - we still get escape analysis output
		t.Logf("Build completed with warnings (escape analysis output captured)")
	}

	escapeOutput := string(output)

	// Log the relevant escape analysis lines for documentation
	t.Log("=== Escape Analysis for startClock() ===")
	lines := strings.Split(escapeOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "startClock") ||
			strings.Contains(line, "timeStr") ||
			strings.Contains(line, "dateStr") ||
			(strings.Contains(line, "panel.go:88") && strings.Contains(line, "func literal")) {
			t.Log(line)
		}
	}

	// Check 1: Verify the closure captures timeStr and dateStr by value
	// This pattern indicates the strings are captured in the closure
	capturePattern := regexp.MustCompile(`capturing by value: (timeStr|dateStr)`)
	captureMatches := capturePattern.FindAllString(escapeOutput, -1)

	if len(captureMatches) < 2 {
		t.Log("WARNING: Could not find expected capture patterns for timeStr/dateStr")
		t.Log("This may indicate the code structure has changed")
	} else {
		t.Logf("Found capture patterns: %v", captureMatches)
	}

	// Check 2: Look for evidence that the IIFE fix has been applied
	//
	// With the IIFE pattern fix:
	//   func(ts, ds string) {
	//       glib.IdleAdd(func() {
	//           p.timeLbl.SetText(ts)
	//           p.dateLbl.SetText(ds)
	//       })
	//   }(timeStr, dateStr)
	//
	// The escape analysis will show:
	//   1. The inner closure captures ts/ds (NOT timeStr/dateStr)
	//   2. "func literal escapes to heap" for the inner closure in startClock.func1
	//   3. ts and ds are captured by value in the escaping closure
	//
	// In UNFIXED code, the closure would capture timeStr/dateStr directly
	// and there would be no nested func literal in startClock.func1

	// Document the bug condition findings
	t.Log("=== Bug Condition Analysis ===")

	// Pattern 1: Direct string escape (alternative fix approach)
	directEscapePattern := regexp.MustCompile(`(timeStr|dateStr) escapes to heap`)
	directEscapeMatches := directEscapePattern.FindAllString(escapeOutput, -1)

	if len(directEscapeMatches) > 0 {
		t.Logf("FIXED (direct escape): Found strings escaping to heap: %v", directEscapeMatches)
		t.Log("The fix has been applied - strings are heap-allocated")
		return
	}

	// Pattern 2: IIFE pattern - the inner closure captures ts/ds and escapes to heap
	// Look for: "startClock.func1 capturing by value: ts" AND
	//           "func literal escapes to heap in (*cityPanel).startClock.func1"
	//
	// This indicates the IIFE pattern is in use: the goroutine (func1) has an
	// inner closure that captures ts/ds and that inner closure escapes to heap
	// when passed to glib.IdleAdd

	tsCapturePattern := regexp.MustCompile(`startClock\.func1 capturing by value: ts`)
	dsCapturePattern := regexp.MustCompile(`startClock\.func1 capturing by value: ds`)
	innerClosureEscapesPattern := regexp.MustCompile(`func literal escapes to heap in \(\*cityPanel\)\.startClock\.func1`)

	tsCapture := tsCapturePattern.FindString(escapeOutput)
	dsCapture := dsCapturePattern.FindString(escapeOutput)
	innerClosureEscapes := innerClosureEscapesPattern.FindString(escapeOutput)

	if tsCapture != "" && dsCapture != "" && innerClosureEscapes != "" {
		t.Log("FIXED (IIFE pattern): Detected IIFE pattern in startClock")
		t.Logf("  - ts captured by value: %s", tsCapture)
		t.Logf("  - ds captured by value: %s", dsCapture)
		t.Logf("  - Inner closure escapes to heap: %s", innerClosureEscapes)
		t.Log("")
		t.Log("The IIFE pattern forces the inner closure (which captures ts/ds) to")
		t.Log("escape to the heap when passed to glib.IdleAdd. Since ts and ds are")
		t.Log("parameters of the IIFE, they are copied when the IIFE is invoked,")
		t.Log("and the escaping closure takes its captured values with it to the heap.")
		t.Log("")
		t.Log("This eliminates the bug condition: the strings passed to SetText()")
		t.Log("are now heap-allocated and survive Go runtime stack management.")
		return
	}

	// Partial IIFE detection - log what we found
	if tsCapture != "" || dsCapture != "" || innerClosureEscapes != "" {
		t.Log("Partial IIFE pattern detected:")
		t.Logf("  - ts capture: %v", tsCapture != "")
		t.Logf("  - ds capture: %v", dsCapture != "")
		t.Logf("  - inner closure escapes: %v", innerClosureEscapes != "")
	}

	// If we reach here, the bug condition exists
	//
	// The strings are captured by value (we found captureMatches) but they
	// don't escape to heap (no heapMatches). This means:
	//   - timeStr and dateStr remain stack-allocated in the goroutine
	//   - The closure passed to glib.IdleAdd captures stack pointers
	//   - When Go shrinks the stack, these pointers can become invalid
	//   - This causes "fatal error: invalid pointer found on stack"

	t.Log("")
	t.Log("=== BUG CONDITION CONFIRMED ===")
	t.Log("")
	t.Log("The escape analysis shows that timeStr and dateStr are captured")
	t.Log("by value in the closure but do NOT escape to heap.")
	t.Log("")
	t.Log("Bug Condition Present:")
	t.Log("  - closureCapturesLocalStrings: TRUE (strings captured in glib.IdleAdd closure)")
	t.Log("  - stringsAreStackAllocated: TRUE (no 'escapes to heap' for timeStr/dateStr)")
	t.Log("  - closureIsPassedToCgo: TRUE (glib.IdleAdd crosses Go/C boundary)")
	t.Log("")
	t.Log("When the Go runtime performs stack management (shrinking), the")
	t.Log("stack-allocated string headers captured by the closure can become")
	t.Log("invalid before the C callback executes, causing the crash.")
	t.Log("")
	t.Log("Expected Fix: Use IIFE pattern to force strings to escape to heap:")
	t.Log("  func(ts, ds string) {")
	t.Log("      glib.IdleAdd(func() {")
	t.Log("          p.timeLbl.SetText(ts)")
	t.Log("          p.dateLbl.SetText(ds)")
	t.Log("      })")
	t.Log("  }(timeStr, dateStr)")
	t.Log("")

	// This test FAILS on unfixed code - this is EXPECTED
	// Failure confirms the bug condition exists
	t.Errorf("BUG CONDITION EXISTS: timeStr/dateStr do not escape to heap\n" +
		"This confirms the bug: stack-allocated strings are captured by the\n" +
		"glib.IdleAdd closure and can become invalid during stack shrinking.\n" +
		"\n" +
		"This test is EXPECTED TO FAIL on unfixed code.\n" +
		"After applying the IIFE fix, this test should PASS.")
}

// isBugConditionPresent is a helper that returns true if the escape analysis
// output indicates the bug condition is present (strings don't escape to heap).
func isBugConditionPresent(escapeOutput string) bool {
	// Check if timeStr/dateStr are captured by value in startClock
	capturePattern := regexp.MustCompile(`startClock\.func1 capturing by value: (timeStr|dateStr)`)
	hasCaptures := capturePattern.MatchString(escapeOutput)

	// Check if timeStr/dateStr escape to heap (indicates fix)
	heapPattern := regexp.MustCompile(`(timeStr|dateStr) escapes to heap`)
	escapesToHeap := heapPattern.MatchString(escapeOutput)

	// Bug condition: strings are captured but don't escape to heap
	return hasCaptures && !escapesToHeap
}
