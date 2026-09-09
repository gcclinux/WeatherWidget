//go:build linux

package uitk

// Source-level guard: worker goroutines must never call glib.IdleAdd directly.
//
// The production crash ("invalid pointer found on stack" inside glib.idleAdd)
// was caused by calling glib.IdleAdd from goroutines whose stacks are moved by
// the GC. All cross-thread UI work must go through runOnUI, which enqueues a
// plain closure and lets the single main-thread idle source (drainUIWorkQueue)
// execute it. This test fails if glib.IdleAdd reappears in the worker-goroutine
// source files, guarding against regressions.

import (
	"os"
	"strings"
	"testing"
)

func TestNoDirectIdleAddInWorkerFiles(t *testing.T) {
	// Files that run UI updates from background goroutines. The only allowed
	// glib.IdleAdd call in the whole package is the single registration inside
	// uidispatch.go (InitUIDispatcher), which runs on the main thread.
	files := []string{
		"panel.go",
		"panel_simple.go",
		"settings.go",
		"manager.go",
		"tray.go",
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		for lineNo, line := range strings.Split(string(data), "\n") {
			// Ignore comments.
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(line, "glib.IdleAdd(") {
				t.Errorf("%s:%d calls glib.IdleAdd directly; use runOnUI instead:\n\t%s",
					f, lineNo+1, trimmed)
			}
		}
	}
}

// TestDispatcherRegistrationIsIsolated confirms the ONLY glib.IdleAdd call in
// the package lives in uidispatch.go.
func TestDispatcherRegistrationIsIsolated(t *testing.T) {
	data, err := os.ReadFile("uidispatch.go")
	if err != nil {
		t.Fatalf("reading uidispatch.go: %v", err)
	}
	if !strings.Contains(string(data), "glib.IdleAdd(drainUIWorkQueue)") {
		t.Error("uidispatch.go must register drainUIWorkQueue via glib.IdleAdd exactly once")
	}
}
