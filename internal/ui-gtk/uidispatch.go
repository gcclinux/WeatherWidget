//go:build linux

package uitk

// This file implements a centralized, cgo-safe dispatcher for running work on
// the GTK main thread.
//
// Background — why this exists:
//
// gotk3's glib.IdleAdd(f) wraps the supplied function in a reflect.Value inside
// a FuncStack and then makes a cgo call (C.g_idle_add_full). During that cgo
// call the Go runtime executes cgoCheckPointer, which scans the pointer graph
// of the argument. If the *calling goroutine's* stack is grown or shrunk by the
// garbage collector at that exact moment (runtime.morestack / shrinkstack /
// adjustframe), a pointer inside the frame can fail adjustment and the runtime
// aborts with:
//
//     runtime: bad pointer in frame github.com/gotk3/gotk3/glib.idleAdd ...
//     fatal error: invalid pointer found on stack
//
// This is triggered by calling glib.IdleAdd repeatedly from long-lived worker
// goroutines (e.g. the per-panel clock goroutine) whose stacks are subject to
// GC-driven growth/shrinking. Forcing captured variables to the heap (the
// earlier "IIFE" attempt) does not help, because the crash is in gotk3's own
// idleAdd frame, not in our captured data.
//
// The fix: never call glib.IdleAdd from worker goroutines. Instead, install a
// SINGLE persistent GTK idle source ONCE (from the main thread, using a stable
// package-level function value), and have worker goroutines hand plain func()
// closures to it over a buffered channel. The idle source drains the channel
// and runs each closure directly on the GTK main thread. Only one cgo-crossing
// idle registration ever happens, and it is performed from the main thread
// which is not subject to the same concurrent stack adjustment during the call.

import (
	"sync"

	"github.com/gotk3/gotk3/glib"
)

// uiWorkQueue carries closures to be executed on the GTK main thread. It is
// buffered so that worker goroutines rarely block; if it fills up the sender
// blocks briefly, which provides natural backpressure.
var uiWorkQueue = make(chan func(), 1024)

// installDispatcherOnce guards installation of the persistent idle source.
var installDispatcherOnce sync.Once

// InitUIDispatcher installs the persistent GTK sources that drain the UI work
// queue. It MUST be called exactly once from the GTK main thread, after
// gtk.Init and before entering gtk.Main. Subsequent calls are no-ops.
//
// Two sources are installed:
//
//  1. An idle source (glib.IdleAdd). This drains the queue promptly whenever
//     the main loop is otherwise idle, giving low-latency UI updates in the
//     normal top-level gtk.Main() loop.
//
//  2. A periodic timeout source (glib.TimeoutAdd, ~50ms). This is the critical
//     one for correctness inside nested main loops such as the one spun up by
//     gtk_dialog_run() (used by the Settings dialog). A plain idle source is
//     not reliably serviced while such a nested loop is running, which is why
//     an async result enqueued from a worker goroutine (e.g. the "Search API"
//     result in the Settings > Locations tab) could sit undrained and leave
//     the UI stuck on "Searching..." until the dialog was dismissed. Timeout
//     sources ARE dispatched by nested loops, so this guarantees the queue is
//     drained regardless of which main loop is currently running.
//
// Both sources call the same package-level drain function and are registered
// once, on the main thread, so neither is subject to the concurrent
// stack-adjustment cgo crash that repeated worker-goroutine IdleAdd calls
// caused.
func InitUIDispatcher() {
	installDispatcherOnce.Do(func() {
		glib.IdleAdd(drainUIWorkQueue)
		glib.TimeoutAdd(uint(50), drainUIWorkQueue)
	})
}

// drainUIWorkQueue runs pending UI closures on the GTK main thread. It is
// installed as a persistent idle source: returning true keeps it registered so
// it is invoked again on the next idle cycle.
//
// It drains a bounded number of items per invocation so a flood of work cannot
// starve GTK's own event processing (rendering, input) within a single idle
// callback.
func drainUIWorkQueue() bool {
	const maxPerCycle = 64
	for i := 0; i < maxPerCycle; i++ {
		select {
		case fn := <-uiWorkQueue:
			if fn != nil {
				fn()
			}
		default:
			return true // queue empty for now; stay registered
		}
	}
	return true // keep the idle source alive
}

// runOnUI schedules fn to run on the GTK main thread. It is safe to call from
// any goroutine. Unlike glib.IdleAdd, it does not perform a cgo call on the
// caller's goroutine, so it is immune to the cgoCheckPointer stack-adjustment
// crash. fn and everything it references are ordinary heap values owned by the
// channel until executed.
func runOnUI(fn func()) {
	if fn == nil {
		return
	}
	uiWorkQueue <- fn
}
