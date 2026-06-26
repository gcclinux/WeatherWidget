//go:build windows

// Package power provides system power-state notifications.
// On Windows it listens for WM_POWERBROADCAST / PBT_APMRESUMESUSPEND
// so the application can react immediately when the PC wakes from
// sleep or hibernation.
package power

import (
	"log"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Win32 constants for power broadcast messages.
const (
	wmPowerBroadcast    = 0x0218
	pbtAPMResumeSuspend = 0x0007
	pbtAPMResumeAuto    = 0x0012
)

// Window class/procedure related Win32 calls.
var (
	modUser32           = windows.NewLazySystemDLL("user32.dll")
	procRegisterClassW  = modUser32.NewProc("RegisterClassExW")
	procCreateWindowExW = modUser32.NewProc("CreateWindowExW")
	procGetMessageW     = modUser32.NewProc("GetMessageW")
	procDefWindowProcW  = modUser32.NewProc("DefWindowProcW")
)

// WNDCLASSEXW mirrors the Win32 WNDCLASSEX structure.
type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm      windows.Handle
}

// msg mirrors the Win32 MSG structure.
type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// resumeCh is the package-level channel shared with the wndproc callback.
var resumeCh chan struct{}

// ResumeNotifier creates a hidden window that listens for power broadcast
// messages and returns a channel that receives a value each time the system
// resumes from sleep or hibernation.
//
// The returned channel is buffered (size 1) so a resume event is never lost
// even if the consumer is briefly busy. Call this once at application startup.
func ResumeNotifier() <-chan struct{} {
	resumeCh = make(chan struct{}, 1)
	go listenPowerEvents()
	return resumeCh
}

// listenPowerEvents registers a hidden message-only window and pumps messages.
func listenPowerEvents() {
	className, _ := syscall.UTF16PtrFromString("WeatherWidgetPowerWnd")

	wc := wndClassExW{
		LpfnWndProc:   syscall.NewCallback(powerWndProc),
		LpszClassName: className,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))

	ret, _, err := procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		log.Printf("power: RegisterClassExW failed: %v", err)
		return
	}

	// HWND_MESSAGE (-3) creates a message-only window (no visible UI).
	hwndMessage := uintptr(^uintptr(2)) // -3 as uintptr
	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0, 0, 0, 0, 0,
		hwndMessage, 0, 0, 0,
	)
	if hwnd == 0 {
		log.Printf("power: CreateWindowExW failed: %v", err)
		return
	}

	log.Printf("power: listening for resume events (hwnd=%#x)", hwnd)

	// Message pump — blocks forever (runs in its own goroutine).
	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&m)),
			hwnd, 0, 0,
		)
		if ret == 0 || ret == ^uintptr(0) {
			// WM_QUIT or error
			return
		}
	}
}

// powerWndProc handles window messages for the hidden power-listener window.
func powerWndProc(hwnd uintptr, umsg uint32, wparam, lparam uintptr) uintptr {
	if umsg == wmPowerBroadcast {
		switch wparam {
		case pbtAPMResumeSuspend, pbtAPMResumeAuto:
			log.Printf("power: system resumed from sleep/hibernate (event=%#x)", wparam)
			// Non-blocking send — drop if channel already has a pending signal.
			select {
			case resumeCh <- struct{}{}:
			default:
			}
		}
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(umsg), wparam, lparam)
	return ret
}
