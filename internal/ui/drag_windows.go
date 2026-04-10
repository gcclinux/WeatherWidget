//go:build windows

package ui

import (
	"log"

	"golang.org/x/sys/windows"
)

var (
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
	procCallWindowProcW   = user32.NewProc("CallWindowProcW")
	procSendMessageW      = user32.NewProc("SendMessageW")
	procReleaseCapture    = user32.NewProc("ReleaseCapture")
)

const (
	gwlpWndProc     uintptr = ^uintptr(3) // -4, GWLP_WNDPROC
	wmLButtonDown           = 0x0201      // WM_LBUTTONDOWN
	wmNCLButtonDown         = 0x00A1      // WM_NCLBUTTONDOWN
	wmExitSizeMove          = 0x0232      // WM_EXITSIZEMOVE
	htCaption               = 2           // HTCAPTION
)

// dragState holds the subclass state for a single window.
var dragState struct {
	origProc  uintptr
	onDragEnd func()
}

// dragWndProc is the replacement window procedure that intercepts mouse
// and move events to implement drag-to-reposition.
func dragWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmLButtonDown:
		// Initiate a native title-bar drag so Windows handles the move.
		procReleaseCapture.Call()
		procSendMessageW.Call(hwnd, wmNCLButtonDown, htCaption, 0)
		return 0

	case wmExitSizeMove:
		// The user released the mouse after dragging. Save position.
		if dragState.onDragEnd != nil {
			dragState.onDragEnd()
		}
	}

	// Forward everything else to the original window procedure.
	ret, _, _ := procCallWindowProcW.Call(dragState.origProc, hwnd, msg, wParam, lParam)
	return ret
}

// enableWindowDrag subclasses the widget window so that left-click anywhere
// initiates a native window drag, and calls onDragEnd when the drag finishes.
func enableWindowDrag(onDragEnd func()) {
	hwnd := findHWND(widgetTitle)
	if hwnd == 0 {
		log.Println("enableWindowDrag: could not find HWND")
		return
	}

	dragState.onDragEnd = onDragEnd

	// Only subclass once — if origProc is already set, just update the callback.
	if dragState.origProc != 0 {
		return
	}

	cb := windows.NewCallback(dragWndProc)
	origProc, _, _ := procSetWindowLongPtrW.Call(hwnd, uintptr(gwlpWndProc), cb)
	if origProc == 0 {
		// On 32-bit Windows, SetWindowLongPtrW may not exist; try GetWindowLongW path.
		origProc, _, _ = procGetWindowLongPtrW.Call(hwnd, uintptr(gwlpWndProc))
		log.Println("enableWindowDrag: SetWindowLongPtrW returned 0, fallback origProc:", origProc)
	}
	dragState.origProc = origProc
	log.Println("enableWindowDrag: window subclassed for drag support")
}
