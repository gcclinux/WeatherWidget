//go:build linux

package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
)

// sessionType returns "wayland", "x11", or "unknown" based on the current
// display server session. Ubuntu 22.04+ defaults to Wayland.
func sessionType() string {
	if st := os.Getenv("XDG_SESSION_TYPE"); st != "" {
		return strings.ToLower(st)
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return "wayland"
	}
	if os.Getenv("DISPLAY") != "" {
		return "x11"
	}
	return "unknown"
}

// isWayland returns true when the session is running under a Wayland compositor.
func isWayland() bool {
	return sessionType() == "wayland"
}

// ---------------------------------------------------------------------------
// applyToolWindowStyle removes window decorations and sets window hints.
//
// On Wayland the window is already undecorated via CreateSplashWindow.
// We use wmctrl (if available) to set skip_taskbar and below hints.
//
// On X11 we use xprop to remove decorations via _MOTIF_WM_HINTS.
// ---------------------------------------------------------------------------

func applyToolWindowStyle(_ string) {
	go func() {
		if isWayland() {
			// On Wayland, the window is already undecorated via CreateSplashWindow.
			// Wait for it to be mapped, then apply skip_taskbar/below hints.
			for _, delay := range []time.Duration{
				500 * time.Millisecond,
				1000 * time.Millisecond,
				2000 * time.Millisecond,
			} {
				time.Sleep(delay)
				if applyWaylandWindowHints() {
					return
				}
			}
			log.Println("Linux/Wayland: window hints could not be applied (wmctrl may not be installed)")
		} else {
			time.Sleep(500 * time.Millisecond)
			applyX11WindowStyle()
		}
	}()
}

// applyWaylandWindowHints uses wmctrl to set skip_taskbar and below hints.
// Returns true if at least one hint was applied successfully.
func applyWaylandWindowHints() bool {
	if _, err := exec.LookPath("wmctrl"); err != nil {
		return false
	}

	success := false

	cmd := exec.Command("wmctrl", "-r", widgetTitle, "-b", "add,skip_taskbar,skip_pager")
	if err := cmd.Run(); err != nil {
		log.Printf("Linux/Wayland: wmctrl skip_taskbar failed: %v", err)
	} else {
		log.Println("Linux/Wayland: set skip_taskbar,skip_pager via wmctrl")
		success = true
	}

	cmd = exec.Command("wmctrl", "-r", widgetTitle, "-b", "add,below")
	if err := cmd.Run(); err != nil {
		log.Printf("Linux/Wayland: wmctrl below failed: %v", err)
	} else {
		log.Println("Linux/Wayland: set window to below via wmctrl")
		success = true
	}

	return success
}

// applyX11WindowStyle uses xprop to remove the title bar on X11 sessions.
func applyX11WindowStyle() {
	if _, err := exec.LookPath("xprop"); err != nil {
		log.Printf("Linux/X11: xprop not found, skipping decoration removal")
		return
	}
	cmd := exec.Command("xprop", "-name", widgetTitle,
		"-f", "_MOTIF_WM_HINTS", "32c",
		"-set", "_MOTIF_WM_HINTS", "0x2, 0x0, 0x0, 0x0, 0x0")
	if err := cmd.Run(); err != nil {
		log.Printf("Linux/X11: failed to remove title bar via xprop: %v", err)
		return
	}
	log.Println("Linux/X11: successfully removed title bar via xprop")
}

// ---------------------------------------------------------------------------
// Screen / monitor queries
// ---------------------------------------------------------------------------

// getScreenSize returns the primary screen dimensions.
func getScreenSize() (int, int) {
	// xrandr works on both X11 and XWayland.
	if w, h, ok := getScreenSizeXrandr(); ok {
		return w, h
	}
	if w, h, ok := getScreenSizeXdotool(); ok {
		return w, h
	}
	log.Println("Linux: getScreenSize: all methods failed, using fallback 1920x1080")
	return 1920, 1080
}

// getScreenSizeXrandr parses xrandr --current output for screen dimensions.
func getScreenSizeXrandr() (int, int, bool) {
	out, err := exec.Command("xrandr", "--current").Output()
	if err != nil {
		return 0, 0, false
	}
	firstLine := strings.Split(string(out), "\n")[0]
	if idx := strings.Index(firstLine, "current "); idx >= 0 {
		rest := firstLine[idx+8:]
		parts := strings.Fields(rest)
		if len(parts) >= 3 {
			w, e1 := strconv.Atoi(parts[0])
			hStr := strings.TrimRight(parts[2], ",")
			h, e2 := strconv.Atoi(hStr)
			if e1 == nil && e2 == nil && w > 0 && h > 0 {
				return w, h, true
			}
		}
	}
	// Fallback: parse connected output lines for active resolution.
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "*") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				res := strings.Split(fields[0], "x")
				if len(res) == 2 {
					w, e1 := strconv.Atoi(res[0])
					h, e2 := strconv.Atoi(res[1])
					if e1 == nil && e2 == nil && w > 0 && h > 0 {
						return w, h, true
					}
				}
			}
		}
	}
	return 0, 0, false
}

// getScreenSizeXdotool is the legacy X11 fallback.
func getScreenSizeXdotool() (int, int, bool) {
	out, err := exec.Command("xdotool", "getdisplaygeometry").Output()
	if err != nil {
		return 0, 0, false
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) < 2 {
		return 0, 0, false
	}
	w, e1 := strconv.Atoi(parts[0])
	h, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil {
		return 0, 0, false
	}
	return w, h, true
}

// ---------------------------------------------------------------------------
// Window positioning
// ---------------------------------------------------------------------------

// moveWindow repositions the widget window.
// On Wayland, uses wmctrl with retry logic (window must be mapped first).
// On X11, uses xdotool or wmctrl.
func moveWindow(_ fyne.Window, x, y int) {
	notifyLinuxMoveByUs()

	go func() {
		if isWayland() {
			for _, delay := range []time.Duration{
				300 * time.Millisecond,
				700 * time.Millisecond,
				1500 * time.Millisecond,
			} {
				time.Sleep(delay)
				if moveWindowWmctrl(x, y) {
					return
				}
			}
			if moveWindowXdotool(x, y) {
				return
			}
			log.Printf("Linux/Wayland: window positioning failed after retries")
		} else {
			time.Sleep(100 * time.Millisecond)
			if moveWindowXdotool(x, y) {
				return
			}
			moveWindowWmctrl(x, y)
		}
	}()
}

// moveWindowWmctrl uses wmctrl to position the window. Returns true if successful.
func moveWindowWmctrl(x, y int) bool {
	if _, err := exec.LookPath("wmctrl"); err != nil {
		return false
	}
	cmd := exec.Command("wmctrl", "-r", widgetTitle, "-e",
		fmt.Sprintf("0,%d,%d,-1,-1", x, y))
	if err := cmd.Run(); err != nil {
		return false
	}
	log.Printf("Linux: moved window to (%d, %d) via wmctrl", x, y)
	return true
}

// moveWindowXdotool uses xdotool to position the window. Returns true if successful.
func moveWindowXdotool(x, y int) bool {
	if _, err := exec.LookPath("xdotool"); err != nil {
		return false
	}
	idOut, err := exec.Command("xdotool", "search", "--name", widgetTitle).Output()
	if err != nil || len(strings.TrimSpace(string(idOut))) == 0 {
		return false
	}
	lines := strings.Fields(strings.TrimSpace(string(idOut)))
	wid := lines[len(lines)-1]
	cmd := exec.Command("xdotool", "windowmove", wid,
		strconv.Itoa(x), strconv.Itoa(y))
	if err := cmd.Run(); err != nil {
		return false
	}
	log.Printf("Linux: moved window to (%d, %d) via xdotool (wid=%s)", x, y, wid)
	return true
}

// ---------------------------------------------------------------------------
// Window position queries
// ---------------------------------------------------------------------------

// getWindowPosition returns the current top-left position of the widget window.
func getWindowPosition() (int, int) {
	if x, y, ok := getWindowPositionWmctrl(); ok {
		return x, y
	}
	return getWindowPositionXdotool()
}

// getWindowPositionWmctrl uses wmctrl -lG to find the window position.
func getWindowPositionWmctrl() (int, int, bool) {
	if _, err := exec.LookPath("wmctrl"); err != nil {
		return 0, 0, false
	}
	out, err := exec.Command("wmctrl", "-lG").Output()
	if err != nil {
		return 0, 0, false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, widgetTitle) {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				x, e1 := strconv.Atoi(fields[2])
				y, e2 := strconv.Atoi(fields[3])
				if e1 == nil && e2 == nil {
					return x, y, true
				}
			}
		}
	}
	return 0, 0, false
}

// getWindowPositionXdotool uses xdotool on X11 sessions.
func getWindowPositionXdotool() (int, int) {
	if _, err := exec.LookPath("xdotool"); err != nil {
		return 0, 0
	}
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

// ---------------------------------------------------------------------------
// Window opacity / transparency
// ---------------------------------------------------------------------------

// setWindowOpacity controls the widget background appearance.
// On Wayland: adjusts the theme background shade (true transparency not possible).
// On X11: uses _NET_WM_WINDOW_OPACITY via xprop.
func setWindowOpacity(opacityPercent int) {
	if isWayland() {
		setWindowOpacityWayland(opacityPercent)
	} else {
		setWindowOpacityX11(opacityPercent)
	}
}

// setWindowOpacityWayland controls the background shade on Linux/Wayland.
// Since true see-through transparency is not possible with Fyne on Wayland,
// the opacity setting controls the background darkness level:
//
//	100% → dark background (RGB 30) — content most prominent
//	 75% → slightly lighter (RGB 50)
//	 50% → medium grey (RGB 70)
//	 25% → lighter grey (RGB 90)
func setWindowOpacityWayland(opacityPercent int) {
	SetLinuxBackgroundShade(opacityPercent)
	log.Printf("Linux/Wayland: background shade set for opacity %d%%", opacityPercent)
}

// setWindowOpacityX11 applies whole-window transparency on X11 via xprop.
func setWindowOpacityX11(opacityPercent int) {
	x11Percent := mapOpacityForDisplay(opacityPercent)

	go func() {
		time.Sleep(600 * time.Millisecond)

		if _, err := exec.LookPath("xprop"); err != nil {
			// Fall back to theme shade if xprop not available.
			SetLinuxBackgroundShade(opacityPercent)
			log.Printf("Linux/X11: xprop not found, using theme shade for opacity")
			return
		}

		if x11Percent >= 100 {
			cmd := exec.Command("xprop", "-name", widgetTitle, "-remove", "_NET_WM_WINDOW_OPACITY")
			cmd.Run()
			return
		}

		opacity := uint64(x11Percent) * 0xFFFFFFFF / 100
		val := strconv.FormatUint(opacity, 10)

		cmd := exec.Command("xprop", "-name", widgetTitle,
			"-f", "_NET_WM_WINDOW_OPACITY", "32c",
			"-set", "_NET_WM_WINDOW_OPACITY", val)
		if err := cmd.Run(); err != nil {
			log.Printf("Linux/X11: xprop opacity failed: %v", err)
			return
		}
		log.Printf("Linux/X11: set window opacity to %d%% (user: %d%%)", x11Percent, opacityPercent)
	}()
}

// mapOpacityForDisplay maps user-facing opacity percentages to display
// values that keep content readable when using whole-window opacity.
func mapOpacityForDisplay(opacityPercent int) int {
	switch {
	case opacityPercent >= 100:
		return 100
	case opacityPercent >= 75:
		return 85
	case opacityPercent >= 50:
		return 70
	default:
		return 55
	}
}

// ---------------------------------------------------------------------------
// Monitor enumeration
// ---------------------------------------------------------------------------

// getMonitorCount returns the number of display monitors.
func getMonitorCount() int {
	out, err := exec.Command("xrandr", "--listmonitors").Output()
	if err != nil {
		return 1
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) <= 1 {
		return 1
	}
	return len(lines) - 1
}

// getMonitorBounds returns the work-area rectangle for the given monitor.
func getMonitorBounds(index int) (int, int, int, int) {
	out, err := exec.Command("xrandr", "--listmonitors").Output()
	if err != nil {
		w, h := getScreenSize()
		return 0, 0, w, h
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if index < 0 || index >= len(lines)-1 {
		index = 0
	}
	line := lines[index+1]
	parts := strings.Fields(line)
	if len(parts) < 3 {
		w, h := getScreenSize()
		return 0, 0, w, h
	}
	geom := parts[2]
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
