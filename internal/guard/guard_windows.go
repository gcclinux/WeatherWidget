//go:build windows

package guard

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modUser32               = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW         = modUser32.NewProc("FindWindowW")
	procSetForegroundWindow = modUser32.NewProc("SetForegroundWindow")
)

// SingleInstanceGuard prevents multiple application instances using a Windows named mutex.
type SingleInstanceGuard struct {
	mutexHandle windows.Handle
}

// NewSingleInstanceGuard creates a named mutex. If the mutex already exists
// (another instance is running), it attempts to bring the existing window to
// the foreground and returns an error.
func NewSingleInstanceGuard(name string) (*SingleInstanceGuard, error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("invalid mutex name: %w", err)
	}

	handle, err := windows.CreateMutex(nil, false, namePtr)
	if err == windows.ERROR_ALREADY_EXISTS {
		// Another instance is already running. Close the duplicate handle
		// and try to bring the existing window to the foreground.
		if handle != 0 {
			windows.CloseHandle(handle)
		}
		bringExistingToFront(name)
		return nil, fmt.Errorf("another instance is already running")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create mutex: %w", err)
	}

	return &SingleInstanceGuard{mutexHandle: handle}, nil
}

// Release closes the mutex handle, allowing another instance to start.
func (g *SingleInstanceGuard) Release() error {
	if g.mutexHandle != 0 {
		if err := windows.CloseHandle(g.mutexHandle); err != nil {
			return fmt.Errorf("failed to release mutex: %w", err)
		}
		g.mutexHandle = 0
	}
	return nil
}

// bringExistingToFront attempts to find the existing application window and
// bring it to the foreground using FindWindowW and SetForegroundWindow.
func bringExistingToFront(windowName string) {
	namePtr, err := syscall.UTF16PtrFromString(windowName)
	if err != nil {
		return
	}

	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(namePtr)))
	if hwnd != 0 {
		procSetForegroundWindow.Call(hwnd)
	}
}
