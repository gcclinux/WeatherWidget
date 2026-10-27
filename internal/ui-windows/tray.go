//go:build windows

package uiwindows

import (
	"encoding/binary"
	"log"
	"syscall"
	"unsafe"
	"weatherwidget/assets"

	"golang.org/x/sys/windows"
)

// Win32 DLLs and procedures for system tray and context menu.
var (
	user32                     = windows.NewLazySystemDLL("user32.dll")
	shell32                    = windows.NewLazySystemDLL("shell32.dll")
	kernel32                   = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procDestroyWindow          = user32.NewProc("DestroyWindow")
	procCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	procAppendMenuW            = user32.NewProc("AppendMenuW")
	procTrackPopupMenuEx       = user32.NewProc("TrackPopupMenuEx")
	procDestroyMenu            = user32.NewProc("DestroyMenu")
	procGetCursorPos           = user32.NewProc("GetCursorPos")
	procSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	procPostMessageW           = user32.NewProc("PostMessageW")
	procPostQuitMessage        = user32.NewProc("PostQuitMessage")
	procCreateIconFromResource = user32.NewProc("CreateIconFromResourceEx")
	procDestroyIcon            = user32.NewProc("DestroyIcon")
	procLoadImageW             = user32.NewProc("LoadImageW")

	procShellNotifyIconW       = shell32.NewProc("Shell_NotifyIconW")
	procShellExecuteW          = shell32.NewProc("ShellExecuteW")

	procGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")
)

// Win32 constants.
const (
	// Window messages
	wmNull        = 0x0000
	wmDestroy     = 0x0002
	wmUser        = 0x0400
	wmTrayMessage = wmUser + 100

	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205
	wmContextMenu = 0x007B

	// Shell_NotifyIcon actions & flags
	nimAdd        = 0x00000000
	nimModify     = 0x00000001
	nimDelete     = 0x00000002

	nifMessage    = 0x00000001
	nifIcon       = 0x00000002
	nifTip        = 0x00000004

	// Menu flags
	mfString      = 0x00000000
	mfSeparator   = 0x00000800

	// TrackPopupMenu flags
	tpmReturnCmd   = 0x0100
	tpmRightButton = 0x0002

	// Command IDs
	cmdShowWidget  = 1001
	cmdHideWidget  = 1002
	cmdRefresh     = 1003
	cmdSettings    = 1004
	cmdSupport     = 1005
	cmdQuit        = 1006

	imageIcon      = 1
	lrDefaultColor = 0x0000
)

// notifyIconDataW mirrors the Win32 NOTIFYICONDATAW structure.
type notifyIconDataW struct {
	Size                       uint32
	Wnd                        windows.Handle
	ID, Flags, CallbackMessage uint32
	Icon                       windows.Handle
	Tip                        [128]uint16
	State, StateMask           uint32
	Info                       [256]uint16
	TimeoutOrVersion           uint32 // union uTimeout / uVersion (4 bytes)
	InfoTitle                  [64]uint16
	InfoFlags                  uint32
	GuidItem                   windows.GUID
	BalloonIcon                windows.Handle
}

// point matches the Win32 POINT structure.
type point struct {
	X int32
	Y int32
}

// wndClassExW mirrors the Win32 WNDCLASSEXW structure.
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

// TrayCallbacks bundles all actions triggered by system tray interactions.
type TrayCallbacks struct {
	OnShowWidget func()
	OnHideWidget func()
	OnRefresh    func()
	OnSettings   func()
	OnQuit       func()
	GetLabel     func(key string) string
}

// Tray manages the Win32 system tray icon and context menu.
type Tray struct {
	hwnd      uintptr
	hIcon     uintptr
	callbacks TrayCallbacks
	nid       notifyIconDataW
}

// global active tray instance for the message procedure.
var activeTray *Tray

// newTray registers the hidden message window and installs the notify icon.
func newTray(callbacks TrayCallbacks) (*Tray, error) {
	const (
		swHide              = 0
		cwUseDefault        = 0x80000000
		wsOverlappedWindow  = 0x00CF0000
		csHRedraw           = 0x0002
		csVRedraw           = 0x0001
	)

	className, _ := syscall.UTF16PtrFromString("WeatherWidgetTrayWndClass")
	hInst, _, _ := procGetModuleHandleW.Call(0)

	wc := wndClassExW{
		Style:         csHRedraw | csVRedraw,
		LpfnWndProc:   syscall.NewCallback(trayWndProc),
		HInstance:     windows.Handle(hInst),
		LpszClassName: className,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))

	// Register window class (ignore error if already registered)
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		uintptr(wsOverlappedWindow),
		uintptr(cwUseDefault),
		uintptr(cwUseDefault),
		uintptr(cwUseDefault),
		uintptr(cwUseDefault),
		0,
		0,
		hInst,
		0,
	)
	if hwnd == 0 {
		return nil, err
	}

	user32.NewProc("ShowWindow").Call(hwnd, uintptr(swHide))
	user32.NewProc("UpdateWindow").Call(hwnd)

	hIcon := loadTrayIcon()

	t := &Tray{
		hwnd:      hwnd,
		hIcon:     hIcon,
		callbacks: callbacks,
	}
	activeTray = t

	// Setup NOTIFYICONDATAW
	t.nid = notifyIconDataW{
		Wnd:             windows.Handle(hwnd),
		ID:              100,
		Flags:           nifMessage | nifIcon | nifTip,
		CallbackMessage: wmTrayMessage,
		Icon:            windows.Handle(hIcon),
	}
	t.nid.Size = uint32(unsafe.Sizeof(t.nid))

	tip := "WeatherWidget"
	copy(t.nid.Tip[:], windows.StringToUTF16(tip))

	ret, _, err := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&t.nid)))
	if ret == 0 {
		log.Printf("uiwindows: Shell_NotifyIconW(NIM_ADD) failed: %v", err)
	} else {
		log.Printf("uiwindows: system tray icon registered successfully")
	}

	return t, nil
}

// showContextMenu displays the Win32 native context menu at the mouse cursor.
func (t *Tray) showContextMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	lbl := func(key, fallback string) *uint16 {
		text := fallback
		if t.callbacks.GetLabel != nil {
			if s := t.callbacks.GetLabel(key); s != "" && s != key {
				text = s
			}
		}
		return windows.StringToUTF16Ptr(text)
	}

	// Build menu items
	procAppendMenuW.Call(hMenu, mfString, cmdShowWidget, uintptr(unsafe.Pointer(lbl("tray.showWidget", "Show Widget"))))
	procAppendMenuW.Call(hMenu, mfString, cmdHideWidget, uintptr(unsafe.Pointer(lbl("tray.hideWidget", "Hide Widget"))))
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	procAppendMenuW.Call(hMenu, mfString, cmdRefresh, uintptr(unsafe.Pointer(lbl("tray.refresh", "Refresh"))))
	procAppendMenuW.Call(hMenu, mfString, cmdSettings, uintptr(unsafe.Pointer(lbl("tray.settings", "Settings"))))
	procAppendMenuW.Call(hMenu, mfString, cmdSupport, uintptr(unsafe.Pointer(lbl("tray.support", "Support"))))
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	procAppendMenuW.Call(hMenu, mfString, cmdQuit, uintptr(unsafe.Pointer(lbl("tray.quit", "Quit"))))

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// SetForegroundWindow is required by Win32 so clicking outside cancels the popup
	procSetForegroundWindow.Call(t.hwnd)

	cmd, _, _ := procTrackPopupMenuEx.Call(
		hMenu,
		tpmReturnCmd|tpmRightButton,
		uintptr(pt.X),
		uintptr(pt.Y),
		t.hwnd,
		0,
	)

	// Post WM_NULL to force focus cleanup
	procPostMessageW.Call(t.hwnd, wmNull, 0, 0)

	switch cmd {
	case cmdShowWidget:
		if t.callbacks.OnShowWidget != nil {
			t.callbacks.OnShowWidget()
		}
	case cmdHideWidget:
		if t.callbacks.OnHideWidget != nil {
			t.callbacks.OnHideWidget()
		}
	case cmdRefresh:
		if t.callbacks.OnRefresh != nil {
			t.callbacks.OnRefresh()
		}
	case cmdSettings:
		if t.callbacks.OnSettings != nil {
			t.callbacks.OnSettings()
		}
	case cmdSupport:
		openURL("https://easysmartapps.co.uk/contact")
	case cmdQuit:
		if t.callbacks.OnQuit != nil {
			t.callbacks.OnQuit()
		}
	}
}

// handleLeftClick toggles the widget visibility or brings it forward.
func (t *Tray) handleLeftClick() {
	if t.callbacks.OnShowWidget != nil {
		t.callbacks.OnShowWidget()
	}
}

// Destroy removes the tray icon and tears down the message window.
func (t *Tray) Destroy() {
	if t.nid.Wnd != 0 {
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&t.nid)))
		t.nid.Wnd = 0
	}
	if t.hIcon != 0 {
		procDestroyIcon.Call(t.hIcon)
		t.hIcon = 0
	}
	if t.hwnd != 0 {
		procDestroyWindow.Call(t.hwnd)
		t.hwnd = 0
	}
	if activeTray == t {
		activeTray = nil
	}
}

// trayWndProc handles messages sent to the hidden tray window.
func trayWndProc(hwnd uintptr, umsg uint32, wparam, lparam uintptr) uintptr {
	if umsg == wmTrayMessage {
		switch lparam {
		case wmRButtonUp, wmContextMenu:
			if activeTray != nil {
				activeTray.showContextMenu()
			}
			return 0
		case wmLButtonUp:
			if activeTray != nil {
				activeTray.handleLeftClick()
			}
			return 0
		}
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(umsg), wparam, lparam)
	return ret
}

// loadTrayIcon attempts to load the tray icon from PE resources or embedded AppICO assets.
func loadTrayIcon() uintptr {
	// 1. Try loading from binary PE resource (RT_GROUP_ICON #1 generated by winres)
	hInst, _, _ := procGetModuleHandleW.Call(0)
	hIcon, _, _ := procLoadImageW.Call(hInst, 1, imageIcon, 16, 16, lrDefaultColor)
	if hIcon != 0 {
		return hIcon
	}

	// 2. Fall back to embedded assets.AppICO
	if len(assets.AppICO) > 0 {
		if hIcon := loadIconFromICO(assets.AppICO, 16); hIcon != 0 {
			return hIcon
		}
	}

	return 0
}

// loadIconFromICO parses an .ico file byte buffer and extracts the icon resource closest to targetSize.
func loadIconFromICO(icoData []byte, targetSize int) uintptr {
	if len(icoData) < 6 {
		return 0
	}
	// Header: idReserved (2), idType (2), idCount (2)
	idCount := int(binary.LittleEndian.Uint16(icoData[4:6]))
	if idCount <= 0 || len(icoData) < 6+idCount*16 {
		return 0
	}

	bestIdx := -1
	bestDiff := 99999

	for i := 0; i < idCount; i++ {
		entryOffset := 6 + i*16
		w := int(icoData[entryOffset])
		if w == 0 {
			w = 256
		}
		diff := w - targetSize
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			bestIdx = i
		}
	}

	if bestIdx < 0 {
		return 0
	}

	entryOffset := 6 + bestIdx*16
	bytesInRes := int(binary.LittleEndian.Uint32(icoData[entryOffset+8 : entryOffset+12]))
	imageOffset := int(binary.LittleEndian.Uint32(icoData[entryOffset+12 : entryOffset+16]))

	if imageOffset+bytesInRes > len(icoData) {
		return 0
	}

	resBits := icoData[imageOffset : imageOffset+bytesInRes]
	hIcon, _, _ := procCreateIconFromResource.Call(
		uintptr(unsafe.Pointer(&resBits[0])),
		uintptr(len(resBits)),
		1,          // fIcon = TRUE
		0x00030000, // dwVersion 3.0
		uintptr(targetSize),
		uintptr(targetSize),
		0,
	)
	return hIcon
}

// openURL launches the system's default browser pointing to the target URL.
func openURL(urlStr string) {
	urlPtr, err := windows.UTF16PtrFromString(urlStr)
	if err != nil {
		return
	}
	openPtr, _ := windows.UTF16PtrFromString("open")
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(openPtr)), uintptr(unsafe.Pointer(urlPtr)), 0, 0, 1) // SW_SHOWNORMAL = 1
}
