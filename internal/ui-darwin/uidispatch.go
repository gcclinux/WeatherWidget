//go:build darwin

package uidarwin

// uidispatch.go — main-thread work queue for the Darwin native UI.
//
// Cocoa requires that all UI objects be created and mutated on the main thread.
// Go goroutines (scheduler callbacks, clock tickers, etc.) use RunOnMain() to
// hand closures to this queue.  The queue is drained by the runMainLoop()
// function which runs on the main OS thread inside manager.start().
//
// Design mirrors internal/ui-gtk/uidispatch.go but without glib: instead of a
// GTK idle source we use a ticker goroutine that sends a dummy byte over a
// Unix pipe, and the main thread's CFRunLoop monitors the pipe's read end via a
// CFRunLoopSource so the main thread wakes up promptly.
//
// For simplicity the initial implementation uses a different approach: the
// main loop simply polls the channel on a short timer.  This avoids the
// complexity of CFRunLoopSource registration and is sufficient for a desktop
// widget where millisecond latency doesn't matter.

import (
	"sync"
)

// mainQueue is the buffered channel that worker goroutines push closures onto.
// It is drained on the Cocoa main thread inside pumpMainQueue().
var mainQueue = make(chan func(), 1024)

// installOnce guards the pump goroutine installation.
var installOnce sync.Once

// RunOnMain schedules fn to run on the Cocoa/AppKit main thread.
// Safe to call from any goroutine. Panics if fn is nil.
// Note: the actual execution happens inside pumpMainQueue which must be called
// repeatedly from the main thread (see manager.go runMainLoop).
func init() {
	// Ensure the package-level mainQueue is ready before any goroutine uses it.
	// Nothing else needed here; pumpMainQueue is called from the run loop.
}

// pumpMainQueue drains up to maxPerCycle pending closures on the caller's
// goroutine (which must be the Cocoa main thread).  Returns the number of
// closures executed.
func pumpMainQueue() int {
	const maxPerCycle = 64
	n := 0
	for i := 0; i < maxPerCycle; i++ {
		select {
		case fn := <-mainQueue:
			if fn != nil {
				fn()
			}
			n++
		default:
			return n
		}
	}
	return n
}
