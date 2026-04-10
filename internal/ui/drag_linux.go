//go:build linux

package ui

import (
	"log"
	"sync"
	"time"
)

var (
	linuxDragMu       sync.Mutex
	linuxDragStop     chan struct{}
	linuxDragCallback func()
	linuxLastX        int
	linuxLastY        int
	linuxMovedByUs    bool // true briefly after we call moveWindow
)

// enableWindowDrag enables drag-to-reposition detection on Linux.
//
// Since the title bar is removed, most window managers still allow moving
// via Alt+left-click drag or Super+drag. This function starts a background
// poller that detects when the window position changes and calls onDragEnd
// so the caller can persist the new coordinates.
//
// Requires xdotool to be installed.
func enableWindowDrag(onDragEnd func()) {
	linuxDragMu.Lock()
	defer linuxDragMu.Unlock()

	// Stop any existing poller.
	if linuxDragStop != nil {
		close(linuxDragStop)
	}

	linuxDragCallback = onDragEnd
	linuxDragStop = make(chan struct{})
	linuxLastX, linuxLastY = getWindowPosition()

	go pollWindowPosition(linuxDragStop)
	log.Println("Linux drag: position poller started")
}

// notifyLinuxMoveByUs should be called before programmatic moves so the
// poller doesn't treat them as user drags.
func notifyLinuxMoveByUs() {
	linuxDragMu.Lock()
	linuxMovedByUs = true
	linuxDragMu.Unlock()
}

// pollWindowPosition checks the window position every 500ms and fires the
// callback when it detects a change not initiated by the application.
func pollWindowPosition(stop chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			x, y := getWindowPosition()

			linuxDragMu.Lock()
			movedByUs := linuxMovedByUs
			linuxMovedByUs = false
			lastX, lastY := linuxLastX, linuxLastY
			cb := linuxDragCallback
			linuxDragMu.Unlock()

			if movedByUs {
				// Update last known position but don't fire callback.
				linuxDragMu.Lock()
				linuxLastX = x
				linuxLastY = y
				linuxDragMu.Unlock()
				continue
			}

			if x != lastX || y != lastY {
				linuxDragMu.Lock()
				linuxLastX = x
				linuxLastY = y
				linuxDragMu.Unlock()

				if cb != nil {
					log.Printf("Linux drag: position changed from (%d,%d) to (%d,%d), saving", lastX, lastY, x, y)
					cb()
				}
			}
		}
	}
}
