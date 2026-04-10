//go:build linux

package ui

import (
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
)

// applyToolWindowStyle attempts to remove the title bar on Linux/X11
// by setting the _MOTIF_WM_HINTS property via xprop.
func applyToolWindowStyle(_ string) {
	// We need to wait a brief moment for the window to be managed by the WM
	// so that xprop can find it by its title.
	go func() {
		time.Sleep(500 * time.Millisecond)

		// Use xprop to remove decorations.
		// _MOTIF_WM_HINTS: 2 = functions/decorations, 0 = no decorations
		cmd := exec.Command("xprop", "-name", widgetTitle, "-f", "_MOTIF_WM_HINTS", "32c", "-set", "_MOTIF_WM_HINTS", "0x2, 0x0, 0x0, 0x0, 0x0")
		if err := cmd.Run(); err != nil {
			log.Printf("Linux: failed to remove title bar via xprop (is x11-utils installed?): %v", err)
			return
		}
		log.Printf("Linux: successfully requested title bar removal via xprop")
	}()
}

// getScreenSize returns the primary screen dimensions using xdotool.
// Falls back to 1920×1080 if xdotool is unavailable or fails.
func getScreenSize() (int, int) {
	out, err := exec.Command("xdotool", "getdisplaygeometry").Output()
	if err != nil {
		log.Printf("Linux: getScreenSize: xdotool failed: %v; using fallback 1920x1080", err)
		return 1920, 1080
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) < 2 {
		log.Printf("Linux: getScreenSize: unexpected xdotool output %q; using fallback 1920x1080", string(out))
		return 1920, 1080
	}
	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		log.Printf("Linux: getScreenSize: parse error (%v, %v); using fallback 1920x1080", err1, err2)
		return 1920, 1080
	}
	return w, h
}

// moveWindow repositions the widget window on Linux using xdotool.
// It finds the window by title and moves it to (x, y).
func moveWindow(_ fyne.Window, x, y int) {
	// Mark this as a programmatic move so the drag poller ignores it.
	notifyLinuxMoveByUs()

	// Give Fyne a moment to finish rendering before moving, then move in background.
	go func() {
		time.Sleep(100 * time.Millisecond)
		// Search for the window ID by name.
		idOut, err := exec.Command("xdotool", "search", "--name", widgetTitle).Output()
		if err != nil || len(strings.TrimSpace(string(idOut))) == 0 {
			log.Printf("Linux: moveWindow: could not find window %q via xdotool: %v", widgetTitle, err)
			return
		}
		// xdotool may return multiple IDs; take the last one (the most recently mapped).
		lines := strings.Fields(strings.TrimSpace(string(idOut)))
		wid := lines[len(lines)-1]

		cmd := exec.Command("xdotool", "windowmove", wid,
			strconv.Itoa(x), strconv.Itoa(y))
		if err := cmd.Run(); err != nil {
			log.Printf("Linux: moveWindow: xdotool windowmove failed: %v", err)
			return
		}
		log.Printf("Linux: moved window %q (id=%s) to (%d, %d)", widgetTitle, wid, x, y)
	}()
}

// getWindowPosition returns the current top-left position of the widget window
// using xdotool on Linux. Returns (0, 0) if the position cannot be determined.
func getWindowPosition() (int, int) {
	idOut, err := exec.Command("xdotool", "search", "--name", widgetTitle).Output()
	if err != nil || len(strings.TrimSpace(string(idOut))) == 0 {
		return 0, 0
	}
	lines := strings.Fields(strings.TrimSpace(string(idOut)))
	wid := lines[len(lines)-1]

	posOut, err := exec.Command("xdotool", "getwindowgeometry", "--shell", wid).Output()
	if err != nil {
		return 0, 0
	}
	var x, y int
	for _, line := range strings.Split(string(posOut), "\n") {
		if strings.HasPrefix(line, "X=") {
			x, _ = strconv.Atoi(strings.TrimPrefix(line, "X="))
		}
		if strings.HasPrefix(line, "Y=") {
			y, _ = strconv.Atoi(strings.TrimPrefix(line, "Y="))
		}
	}
	return x, y
}
