//go:build windows

package ui

import (
	"unsafe"

	"golang.org/x/sys/windows"

	"fyne.io/fyne/v2"
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW      = user32.NewProc("FindWindowW")
	procGetWindowLongW   = user32.NewProc("GetWindowLongW")
	procSetWindowLongW   = user32.NewProc("SetWindowLongW")
	procSetWindowPos     = user32.NewProc("SetWindowPos")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	procGetWindowRect    = user32.NewProc("GetWindowRect")
)

const (
	wsExToolWindow  = 0x00000080
	wsExAppWindow   = 0x00040000
	wsCaption       = 0x00C00000
	wsSysMenu       = 0x00080000
	wsThickFrame    = 0x00040000
	wsMinimizeBox   = 0x00020000
	wsMaximizeBox   = 0x00010000
	hwndTopMost     = ^uintptr(0) // (HWND)-1 == HWND_TOPMOST
	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
	swpShowWindow   = 0x0040
	smCxScreen      = 0
	smCyScreen      = 1
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

	// 3. Set HWND_TOPMOST and force frame refresh with SWP_FRAMECHANGED.
	procSetWindowPos.Call(hwnd, hwndTopMost, 0, 0, 0, 0,
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
	procSetWindowPos.Call(hwnd, hwndTopMost,
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
