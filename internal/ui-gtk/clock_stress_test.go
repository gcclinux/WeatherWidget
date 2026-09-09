//go:build linux

package uitk

// Stress test for the centralized UI dispatcher (runOnUI) under aggressive GC.
//
// Background: the production crash was
//
//     runtime: bad pointer in frame github.com/gotk3/gotk3/glib.idleAdd ...
//     fatal error: invalid pointer found on stack
//
// It occurred because the clock goroutines called glib.IdleAdd directly. That
// crosses cgo (cgoCheckPointer) on a worker goroutine whose stack is grown and
// shrunk by the GC, corrupting gotk3's idleAdd frame. The fix routes all
// cross-thread UI work through runOnUI, which only enqueues a plain heap
// closure onto a channel and never crosses cgo on the worker goroutine.
//
// These tests drive runOnUI exactly as the clock goroutines do, drain the queue
// the same way the installed idle source does (drainUIWorkQueue), and hammer the
// GC concurrently. If runOnUI ever performed an unsafe cgo call on the caller
// goroutine, this would surface instability.

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"weatherwidget/internal/weather"
)

// drainQueueUntil repeatedly runs drainUIWorkQueue until `want` items have been
// executed or the deadline passes. It stands in for the GTK idle source, which
// in production calls drainUIWorkQueue on the main thread.
func drainQueueUntil(counter *int64, want int64, deadline time.Time) {
	for atomic.LoadInt64(counter) < want && time.Now().Before(deadline) {
		drainUIWorkQueue()
		time.Sleep(time.Millisecond)
	}
	// Final drain to catch stragglers.
	drainUIWorkQueue()
}

// TestDispatcherStress_UnderAggressiveGC enqueues the same clock-update closure
// pattern from many goroutines while forcing GC, and drains via the real
// drainUIWorkQueue. It verifies every closure runs and nothing panics.
func TestDispatcherStress_UnderAggressiveGC(t *testing.T) {
	// Enlarge the queue drain loop to keep up; use a local counter.
	var ran int64

	const goroutines = 10
	const iterations = 1000
	total := int64(goroutines * iterations)

	// Drainer goroutine mimics the GTK main-thread idle source.
	done := make(chan struct{})
	go func() {
		drainQueueUntil(&ran, total, time.Now().Add(30*time.Second))
		close(done)
	}()

	// GC stress goroutine.
	stopGC := make(chan struct{})
	go func() {
		for {
			select {
			case <-stopGC:
				return
			default:
				runtime.GC()
				time.Sleep(time.Millisecond)
			}
		}
	}()

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loc, _ := time.LoadLocation("America/New_York")
			for i := 0; i < iterations; i++ {
				localT := time.Now().In(loc)
				timeStr := weather.FormatTime(localT, "America/New_York", nil)
				dateStr := weather.FormatDate(localT, "America/New_York", nil)
				// Same pattern as startClock: bind locals, enqueue heap closure.
				ts, ds := timeStr, dateStr
				runOnUI(func() {
					_ = ts
					_ = ds
					atomic.AddInt64(&ran, 1)
				})
				if i%50 == 0 {
					runtime.GC()
				}
			}
		}()
	}

	wg.Wait()
	<-done
	close(stopGC)

	if got := atomic.LoadInt64(&ran); got != total {
		t.Errorf("expected %d closures to run, got %d", total, got)
	}
}

// TestDispatcher_RunsInFIFOOrderPerProducer verifies that closures enqueued by a
// single producer run in the order they were submitted when drained.
func TestDispatcher_RunsInFIFOOrderPerProducer(t *testing.T) {
	const n = 500
	var mu sync.Mutex
	var order []int

	for i := 0; i < n; i++ {
		idx := i
		runOnUI(func() {
			mu.Lock()
			order = append(order, idx)
			mu.Unlock()
		})
	}

	deadline := time.Now().Add(5 * time.Second)
	for len(order) < n && time.Now().Before(deadline) {
		drainUIWorkQueue()
	}

	if len(order) != n {
		t.Fatalf("expected %d items, drained %d", n, len(order))
	}
	for i := 0; i < n; i++ {
		if order[i] != i {
			t.Fatalf("out of order at %d: got %d", i, order[i])
		}
	}
}

// TestDispatcher_NilClosureIgnored verifies runOnUI tolerates a nil closure.
func TestDispatcher_NilClosureIgnored(t *testing.T) {
	runOnUI(nil)
	// Draining must not panic on the nil entry (runOnUI drops nils, so the
	// queue should simply be empty).
	drainUIWorkQueue()
}

// BenchmarkRunOnUI measures the enqueue + drain cost of the dispatcher.
func BenchmarkRunOnUI(b *testing.B) {
	var ran int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runOnUI(func() { atomic.AddInt64(&ran, 1) })
		drainUIWorkQueue()
	}
}
