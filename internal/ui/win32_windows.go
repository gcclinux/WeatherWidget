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
)

const (
	wsExToolWindow = 0x00000080
	wsExAppWindow  = 0x00040000
	hwndTopMost    = ^uintptr(0) // (HWND)-1 == HWND_TOPMOST
	swpNoSize      = 0x0001
	swpNoMove      = 0x0002
	swpNoActivate  = 0x0010
	swpShowWindow  = 0x0040
	smCxScreen     = 0
	smCyScreen     = 1
)

// findHWND locates the window handle by its title.
func findHWND(title string) uintptr {
	titlePtr, _ := windows.UTF16PtrFromString(title)
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	return hwnd
}

// applyToolWindowStyle sets WS_EX_TOOLWINDOW (removes from taskbar) and
// makes the window always-on-top via HWND_TOPMOST.
func applyToolWindowStyle(title string) {
	hwnd := findHWND(title)
	if hwnd == 0 {
		return
	}

	// GWL_EXSTYLE = -20; pass as two's complement to avoid constant overflow.
	const gwlExStyle = ^uintptr(19) // == uintptr(-20)

	// Get current extended style.
	exStyle, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyle)

	// Add WS_EX_TOOLWINDOW, remove WS_EX_APPWINDOW.
	newStyle := (exStyle | wsExToolWindow) &^ wsExAppWindow
	procSetWindowLongW.Call(hwnd, gwlExStyle, newStyle)

	// Set HWND_TOPMOST with SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE | SWP_SHOWWINDOW.
	procSetWindowPos.Call(hwnd, hwndTopMost, 0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoActivate|swpShowWindow)
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
