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
// On Wayland, direct window-move interception is not possible through the
// protocol. Instead we poll the window position (via wmctrl or xdotool)
// and detect when the user has moved the window using the compositor's
// built-in drag mechanism (Super+drag or Alt+drag on GNOME).
//
// On X11, the same polling approach is used with xdotool.
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

	if isWayland() {
		log.Println("Linux/Wayland: drag position poller started (use Super+drag or Alt+drag to reposition)")
	} else {
		log.Println("Linux/X11: drag position poller started")
	}
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
					log.Printf("Linux: position changed from (%d,%d) to (%d,%d), saving", lastX, lastY, x, y)
					cb()
				}
			}
		}
	}
}
