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

// setWindowOpacity applies whole-window transparency on Linux/X11 by setting
// the _NET_WM_WINDOW_OPACITY property via xprop. This requires a compositing
// manager (Picom, Mutter, KWin, etc.) to take effect.
//
// Unlike Windows (which supports background-only color-key transparency),
// X11 _NET_WM_WINDOW_OPACITY affects the entire window including content.
// To keep text and icons readable, the user-facing opacity values are mapped
// to less aggressive X11 values:
//
//	User 100% → X11 100% (fully opaque)
//	User  75% → X11  85%
//	User  50% → X11  70%
//	User  25% → X11  55%
func setWindowOpacity(opacityPercent int) {
	// Map user-facing opacity to X11 whole-window opacity so content stays
	// readable. The window background dims while text/icons remain legible.
	x11Percent := opacityPercent
	switch {
	case opacityPercent >= 100:
		x11Percent = 100
	case opacityPercent >= 75:
		x11Percent = 85
	case opacityPercent >= 50:
		x11Percent = 70
	case opacityPercent >= 25:
		x11Percent = 55
	default:
		x11Percent = 55
	}

	go func() {
		// Give the window a moment to be mapped if called at startup.
		time.Sleep(600 * time.Millisecond)

		if x11Percent >= 100 {
			// Remove the property entirely — fully opaque.
			cmd := exec.Command("xprop", "-name", widgetTitle, "-remove", "_NET_WM_WINDOW_OPACITY")
			if err := cmd.Run(); err != nil {
				log.Printf("Linux: setWindowOpacity: failed to remove _NET_WM_WINDOW_OPACITY: %v", err)
			}
			return
		}

		// _NET_WM_WINDOW_OPACITY is a 32-bit cardinal where 0xFFFFFFFF = fully opaque.
		opacity := uint64(x11Percent) * 0xFFFFFFFF / 100
		val := strconv.FormatUint(opacity, 10)

		cmd := exec.Command("xprop", "-name", widgetTitle,
			"-f", "_NET_WM_WINDOW_OPACITY", "32c",
			"-set", "_NET_WM_WINDOW_OPACITY", val)
		if err := cmd.Run(); err != nil {
			log.Printf("Linux: setWindowOpacity: xprop failed (is x11-utils installed? is a compositor running?): %v", err)
			return
		}
		log.Printf("Linux: set window opacity to %d%% (user: %d%%, _NET_WM_WINDOW_OPACITY=%s)", x11Percent, opacityPercent, val)
	}()
}

// getMonitorCount returns the number of display monitors using xrandr.
// Falls back to 1 if xrandr is unavailable.
func getMonitorCount() int {
	out, err := exec.Command("xrandr", "--listmonitors").Output()
	if err != nil {
		return 1
	}
	// First line is "Monitors: N", remaining lines are one per monitor.
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) <= 1 {
		return 1
	}
	return len(lines) - 1
}

// getMonitorBounds returns the work-area rectangle (left, top, width, height)
// for the monitor at the given 0-based index using xrandr.
// Falls back to primary screen dimensions if parsing fails.
func getMonitorBounds(index int) (int, int, int, int) {
	out, err := exec.Command("xrandr", "--listmonitors").Output()
	if err != nil {
		w, h := getScreenSize()
		return 0, 0, w, h
	}
	// Example output:
	//   Monitors: 2
	//    0: +*HDMI-1 1920/527x1080/296+0+0  HDMI-1
	//    1: +DP-1 2560/597x1440/336+1920+0  DP-1
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if index < 0 || index >= len(lines)-1 {
		index = 0
	}
	// Skip the header line.
	line := lines[index+1]
	// Parse the geometry: WIDTHxHEIGHT+X+Y (ignoring physical size /NNN parts).
	// Find the resolution part after the colon.
	parts := strings.Fields(line)
	if len(parts) < 3 {
		w, h := getScreenSize()
		return 0, 0, w, h
	}
	geom := parts[2] // e.g. "1920/527x1080/296+0+0"
	// Strip physical size info (everything between / and next delimiter).
	// Simplify: replace /NNN with nothing.
	cleaned := ""
	skip := false
	for _, ch := range geom {
		if ch == '/' {
			skip = true
			continue
		}
		if skip && (ch == 'x' || ch == '+') {
			skip = false
		}
		if !skip {
			cleaned += string(ch)
		}
	}
	// Now cleaned is like "1920x1080+0+0"
	// Split on 'x' and '+'
	cleaned = strings.ReplaceAll(cleaned, "x", "+")
	nums := strings.Split(cleaned, "+")
	if len(nums) < 4 {
		w, h := getScreenSize()
		return 0, 0, w, h
	}
	mw, _ := strconv.Atoi(nums[0])
	mh, _ := strconv.Atoi(nums[1])
	mx, _ := strconv.Atoi(nums[2])
	my, _ := strconv.Atoi(nums[3])
	if mw == 0 || mh == 0 {
		w, h := getScreenSize()
		return 0, 0, w, h
	}
	return mx, my, mw, mh
}
