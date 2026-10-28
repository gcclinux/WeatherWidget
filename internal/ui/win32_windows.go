//go:build windows

package ui

import (
	"math"
	"unsafe"

	"golang.org/x/sys/windows"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

var (
	user32                         = windows.NewLazySystemDLL("user32.dll")
	shcore                         = windows.NewLazySystemDLL("shcore.dll")
	procFindWindowW                = user32.NewProc("FindWindowW")
	procGetWindowLongW             = user32.NewProc("GetWindowLongW")
	procSetWindowLongW             = user32.NewProc("SetWindowLongW")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procGetClientRect              = user32.NewProc("GetClientRect")
	procGetDpiForWindow            = user32.NewProc("GetDpiForWindow")
	procGetDpiForSystem            = user32.NewProc("GetDpiForSystem")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procEnumDisplayMonitors        = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW            = user32.NewProc("GetMonitorInfoW")
	procGetDpiForMonitor           = shcore.NewProc("GetDpiForMonitor")
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
	swpNoZOrder     = 0x0004
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
// removes the title bar (WS_CAPTION), makes the window always-on-top,
// and configures WS_EX_LAYERED with color-key transparency (#010101).
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

	// 2. Get current extended style and set tool-window + layered behavior.
	exStyle, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyle)
	newExStyle := (exStyle | wsExToolWindow | wsExLayered) &^ wsExAppWindow
	procSetWindowLongW.Call(hwnd, gwlExStyle, newExStyle)

	// Set initial layered window attributes: color key transparent, content fully opaque.
	colorKey := uintptr(0x00010101)
	procSetLayeredWindowAttributes.Call(hwnd, colorKey, 255, lwaColorKey|lwaAlpha)
	SetTransparencyActive(true)

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
// Transparency is always fixed to 100% (alpha 255) for fully opaque content
// with a completely transparent window background.
func setWindowOpacity(_ int) {
	hwnd := findHWND(widgetTitle)
	if hwnd == 0 {
		return
	}

	const gwlExStyle = ^uintptr(19) // GWL_EXSTYLE = -20

	exStyle, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyle)

	// Always ensure WS_EX_LAYERED is set so the background color key is transparent.
	procSetWindowLongW.Call(hwnd, gwlExStyle, exStyle|wsExLayered)
	SetTransparencyActive(true)

	// Color key: R=1, G=1, B=1 as a COLORREF (0x00BBGGRR). Alpha is always 255 (100%).
	colorKey := uintptr(0x00010101)
	procSetLayeredWindowAttributes.Call(hwnd, colorKey, 255, lwaColorKey|lwaAlpha)
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

// getMonitorDPIScale returns the DPI scale factor (e.g., 1.0, 1.5, 2.0) for
// the monitor at the given 0-based index, falling back to GetDpiForSystem
// and then fallbackScale if needed.
func getMonitorDPIScale(index int, fallbackScale float32) float32 {
	enumeratedMonitors = nil
	cb := windows.NewCallback(monitorEnumProc)
	procEnumDisplayMonitors.Call(0, 0, cb, 0)

	if len(enumeratedMonitors) > 0 {
		if index < 0 || index >= len(enumeratedMonitors) {
			index = 0
		}
		hMon := enumeratedMonitors[index]
		if procGetDpiForMonitor.Find() == nil {
			var dpiX, dpiY uint32
			// MDT_EFFECTIVE_DPI = 0
			ret, _, _ := procGetDpiForMonitor.Call(hMon, 0, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
			if ret == 0 && dpiX >= 96 {
				raw := float64(dpiX) / 96.0
				return float32(math.Round(raw*10.0)) / 10.0
			}
		}
	}

	if procGetDpiForSystem.Find() == nil {
		dpi, _, _ := procGetDpiForSystem.Call()
		if dpi >= 96 {
			raw := float64(dpi) / 96.0
			return float32(math.Round(raw*10.0)) / 10.0
		}
	}

	if fallbackScale >= 1.0 {
		return fallbackScale
	}
	return 1.0
}

// getLogicalWorkArea returns the monitor's usable work-area dimensions
// (excluding the Windows taskbar) in Fyne device-independent pixels (dp).
func getLogicalWorkArea(index int, fallbackScale float32) (int, int) {
	_, _, workW, workH := getMonitorBounds(index)
	scale := getMonitorDPIScale(index, fallbackScale)
	if scale <= 0 {
		scale = 1.0
	}
	return int(float32(workW) / scale), int(float32(workH) / scale)
}

// syncSettingsWindow ensures that after the Settings window is shown on Windows:
// 1. Its outer frame fits within and is centered inside the monitor's work area
//    (preventing the title bar or bottom Save bar from clipping off-screen or behind the taskbar).
// 2. A 1px SetWindowPos pulse forces GLFW's WM_SIZE handler to fire _glfwInputFramebufferSize
//    and _glfwInputWindowSize with the actual Win32 client area, fixing the Fyne/OpenGL
//    viewport desync where the top of the tab bar is clipped until manually resized.
func syncSettingsWindow(win fyne.Window, monitorIndex int) {
	if win == nil {
		return
	}

	var hwnd uintptr
	if nw, ok := win.(driver.NativeWindow); ok {
		nw.RunNative(func(ctx any) {
			if wctx, ok := ctx.(driver.WindowsWindowContext); ok {
				hwnd = wctx.HWND
			}
		})
	}
	if hwnd == 0 {
		hwnd = findHWND(win.Title())
	}
	if hwnd == 0 {
		return
	}

	workX, workY, workW, workH := getMonitorBounds(monitorIndex)
	var winRect rect
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&winRect)))
	outerW := int(winRect.Right - winRect.Left)
	outerH := int(winRect.Bottom - winRect.Top)
	if outerW <= 0 || outerH <= 0 {
		return
	}

	if outerW > workW {
		outerW = workW
	}
	if outerH > workH {
		outerH = workH
	}

	newX := workX + (workW-outerW)/2
	newY := workY + (workH-outerH)/2
	if newX < workX {
		newX = workX
	}
	if newY < workY {
		newY = workY
	}

	// Pulse height by -1px and back to outerH so GLFW's WM_SIZE check
	// (width != window->win32.width || height != window->win32.height)
	// unconditionally fires _glfwInputFramebufferSize and _glfwInputWindowSize
	// with the real Win32 client area dimensions.
	if outerH > 2 {
		procSetWindowPos.Call(hwnd, 0,
			uintptr(newX), uintptr(newY),
			uintptr(outerW), uintptr(outerH-1),
			swpNoZOrder|swpNoActivate)
	}
	procSetWindowPos.Call(hwnd, 0,
		uintptr(newX), uintptr(newY),
		uintptr(outerW), uintptr(outerH),
		swpNoZOrder|swpNoActivate|swpFrameChanged)

	var clientRect rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&clientRect)))
	clientW := int(clientRect.Right - clientRect.Left)
	clientH := int(clientRect.Bottom - clientRect.Top)
	if clientW > 0 && clientH > 0 && win.Canvas() != nil && win.Content() != nil {
		scale := win.Canvas().Scale()
		if scale <= 0 {
			scale = getMonitorDPIScale(monitorIndex, 1.0)
		}
		dpW := float32(clientW) / scale
		dpH := float32(clientH) / scale
		win.Content().Resize(fyne.NewSize(dpW, dpH))
		win.Content().Refresh()
	}
}
