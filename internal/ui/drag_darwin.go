//go:build darwin

package ui

import (
	"log"
	"sync"
	"time"
)

var (
	darwinDragMu       sync.Mutex
	darwinDragStop     chan struct{}
	darwinDragCallback func()
	darwinLastX        int
	darwinLastY        int
	darwinMovedByUs    bool
)

// enableWindowDrag enables drag-to-reposition detection on macOS.
//
// Since the widget uses a borderless splash window, there is no native
// title-bar drag. We poll the window position and detect when the user
// has moved the window (e.g. via scripting or accessibility tools).
func enableWindowDrag(onDragEnd func()) {
	darwinDragMu.Lock()
	defer darwinDragMu.Unlock()

	if darwinDragStop != nil {
		close(darwinDragStop)
	}

	darwinDragCallback = onDragEnd
	darwinDragStop = make(chan struct{})
	darwinLastX, darwinLastY = getWindowPosition()

	go darwinPollPosition(darwinDragStop)
	log.Println("macOS: drag position poller started")
}

// notifyDarwinMoveByUs should be called before programmatic moves so the
// poller doesn't treat them as user drags.
func notifyDarwinMoveByUs() {
	darwinDragMu.Lock()
	darwinMovedByUs = true
	darwinDragMu.Unlock()
}

// darwinPollPosition checks the window position periodically and fires the
// callback when it detects a change not initiated by the application.
func darwinPollPosition(stop chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			x, y := getWindowPosition()

			darwinDragMu.Lock()
			movedByUs := darwinMovedByUs
			darwinMovedByUs = false
			lastX, lastY := darwinLastX, darwinLastY
			cb := darwinDragCallback
			darwinDragMu.Unlock()

			if movedByUs {
				darwinDragMu.Lock()
				darwinLastX = x
				darwinLastY = y
				darwinDragMu.Unlock()
				continue
			}

			if x != lastX || y != lastY {
				darwinDragMu.Lock()
				darwinLastX = x
				darwinLastY = y
				darwinDragMu.Unlock()

				if cb != nil {
					log.Printf("macOS: position changed from (%d,%d) to (%d,%d), saving", lastX, lastY, x, y)
					cb()
				}
			}
		}
	}
}
