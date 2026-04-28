//go:build windows

package ui

import (
	"unsafe"

	"golang.org/x/sys/windows"

	"fyne.io/fyne/v2"
)

var (
	user32                         = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW                = user32.NewProc("FindWindowW")
	procGetWindowLongW             = user32.NewProc("GetWindowLongW")
	procSetWindowLongW             = user32.NewProc("SetWindowLongW")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procEnumDisplayMonitors        = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW            = user32.NewProc("GetMonitorInfoW")
)

const (
	wsExToolWindow  = 0x00000080
	wsExAppWindow   = 0x00040000
	wsExLayered     = 0x00080000
	wsCaption       = 0x00C00000
	wsSysMenu       = 0x00080000
	wsThickFrame    = 0x00040000
	wsMinimizeBox   = 0x00020000
	wsMaximizeBox   = 0x00010000
	hwndTopMost     = ^uintptr(0) // (HWND)-1 == HWND_TOPMOST (unused, kept for reference)
	hwndBottom      = uintptr(1)  // (HWND)1  == HWND_BOTTOM — behind all other windows
	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
	swpShowWindow   = 0x0040
	smCxScreen      = 0
	smCyScreen      = 1
	lwaColorKey     = 0x00000001
	lwaAlpha        = 0x00000002
)

// findHWND locates the window handle by its title.
func findHWND(title string) uintptr {
	titlePtr, _ := windows.UTF16PtrFromString(title)
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	return hwnd
}

// applyToolWindowStyle sets WS_EX_TOOLWINDOW (removes from taskbar),
// removes the title bar (WS_CAPTION), and makes the window always-on-top.
func applyToolWindowStyle(title string) {
	hwnd := findHWND(title)
	if hwnd == 0 {
		return
	}

	// GWL_STYLE = -16; GWL_EXSTYLE = -20
	const (
		gwlStyle   = ^uintptr(15) // -16
		gwlExStyle = ^uintptr(19) // -20
	)

	// 1. Remove title bar and borders from basic style.
	style, _, _ := procGetWindowLongW.Call(hwnd, gwlStyle)
	newStyle := style &^ (wsCaption | wsThickFrame | wsSysMenu | wsMinimizeBox | wsMaximizeBox)
	procSetWindowLongW.Call(hwnd, gwlStyle, newStyle)

	// 2. Get current extended style and set tool-window behavior.
	exStyle, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyle)
	newExStyle := (exStyle | wsExToolWindow) &^ wsExAppWindow
	procSetWindowLongW.Call(hwnd, gwlExStyle, newExStyle)

	// 3. Set HWND_BOTTOM (behind all other windows) and force frame refresh.
	procSetWindowPos.Call(hwnd, hwndBottom, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate|swpShowWindow|swpFrameChanged)
}

// getScreenSize returns the primary monitor resolution in pixels.
func getScreenSize() (int, int) {
	cx, _, _ := procGetSystemMetrics.Call(smCxScreen)
	cy, _, _ := procGetSystemMetrics.Call(smCyScreen)
	if cx == 0 || cy == 0 {
		return 1920, 1080 // fallback
	}
	return int(cx), int(cy)
}

// moveWindow repositions the Fyne window to the given screen coordinates
// using the Win32 SetWindowPos API.
func moveWindow(_ fyne.Window, x, y int) {
	hwnd := findHWND(widgetTitle)
	if hwnd == 0 {
		return
	}
	// SetWindowPos with SWP_NOSIZE to move without resizing.
	procSetWindowPos.Call(hwnd, hwndBottom,
		uintptr(x), uintptr(y), 0, 0,
		swpNoSize|swpNoActivate)
}

// rect matches the Win32 RECT structure.
type rect struct {
	Left, Top, Right, Bottom int32
}

// getWindowPosition returns the current top-left position of the widget window.
func getWindowPosition() (int, int) {
	hwnd := findHWND(widgetTitle)
	if hwnd == 0 {
		return 0, 0
	}
	var r rect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return int(r.Left), int(r.Top)
}

// setWindowOpacity applies background-only transparency using LWA_COLORKEY.
// When opacityPercent < 100 the window background color (#010101) is made
// invisible by Windows; all other pixels (text, icons) remain fully opaque.
// At 100% the layered style is removed and the window is fully opaque.
func setWindowOpacity(opacityPercent int) {
	hwnd := findHWND(widgetTitle)
	if hwnd == 0 {
		return
	}

	const gwlExStyle = ^uintptr(19) // GWL_EXSTYLE = -20

	exStyle, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyle)

	if opacityPercent >= 100 {
		// Remove layered style — fully opaque, normal rendering.
		procSetWindowLongW.Call(hwnd, gwlExStyle, exStyle&^wsExLayered)
		SetTransparencyActive(false)
		return
	}

	// Enable layered window and set the color key transparent.
	procSetWindowLongW.Call(hwnd, gwlExStyle, exStyle|wsExLayered)
	SetTransparencyActive(true)

	// Color key: R=1, G=1, B=1 as a COLORREF (0x00BBGGRR).
	colorKey := uintptr(0x00010101)
	procSetLayeredWindowAttributes.Call(hwnd, colorKey, 0, lwaColorKey)
}

// MonitorRect describes the bounding rectangle of a display monitor.
type MonitorRect struct {
	Left, Top, Right, Bottom int
}

// monitorInfoW matches the Win32 MONITORINFO structure (cbSize + rcMonitor + rcWork + dwFlags).
type monitorInfoW struct {
	CbSize    uint32
	RcMonitor rect // full monitor area
	RcWork    rect // work area (excludes taskbar)
	DwFlags   uint32
}

// enumeratedMonitors collects monitor handles during EnumDisplayMonitors callback.
var enumeratedMonitors []uintptr

// monitorEnumProc is the callback for EnumDisplayMonitors.
// It appends each monitor handle to enumeratedMonitors.
func monitorEnumProc(hMonitor, hdc, lprcClip, dwData uintptr) uintptr {
	enumeratedMonitors = append(enumeratedMonitors, hMonitor)
	return 1 // TRUE — continue enumeration
}

// getMonitorCount returns the number of display monitors attached to the system.
func getMonitorCount() int {
	enumeratedMonitors = nil
	cb := windows.NewCallback(monitorEnumProc)
	procEnumDisplayMonitors.Call(0, 0, cb, 0)
	n := len(enumeratedMonitors)
	if n == 0 {
		return 1
	}
	return n
}

// getMonitorBounds returns the work-area rectangle for the monitor at the
// given 0-based index. If the index is out of range it falls back to the
// primary monitor (index 0). Returns (left, top, width, height).
func getMonitorBounds(index int) (int, int, int, int) {
	enumeratedMonitors = nil
	cb := windows.NewCallback(monitorEnumProc)
	procEnumDisplayMonitors.Call(0, 0, cb, 0)

	if len(enumeratedMonitors) == 0 {
		// Fallback to primary via GetSystemMetrics.
		w, h := getScreenSize()
		return 0, 0, w, h
	}

	if index < 0 || index >= len(enumeratedMonitors) {
		index = 0
	}

	hMon := enumeratedMonitors[index]
	var mi monitorInfoW
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	ret, _, _ := procGetMonitorInfoW.Call(hMon, uintptr(unsafe.Pointer(&mi)))
	if ret == 0 {
		w, h := getScreenSize()
		return 0, 0, w, h
	}

	work := mi.RcWork
	return int(work.Left), int(work.Top),
		int(work.Right - work.Left), int(work.Bottom - work.Top)
}
