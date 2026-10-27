//go:build windows

package uiwindows

import (
	"image"
	"log"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	gdi32 = windows.NewLazySystemDLL("gdi32.dll")

	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procDeleteObject       = gdi32.NewProc("DeleteObject")

	procGetDC                   = user32.NewProc("GetDC")
	procReleaseDC               = user32.NewProc("ReleaseDC")
	procUpdateLayeredWindow     = user32.NewProc("UpdateLayeredWindow")
	procSetWindowPos            = user32.NewProc("SetWindowPos")
	procGetWindowRect           = user32.NewProc("GetWindowRect")
	procGetSystemMetrics        = user32.NewProc("GetSystemMetrics")
	procEnumDisplayMonitors     = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW         = user32.NewProc("GetMonitorInfoW")
	procSetCapture              = user32.NewProc("SetCapture")
	procReleaseCapture          = user32.NewProc("ReleaseCapture")
	procShowWindow              = user32.NewProc("ShowWindow")
)

// Win32 Layered Window Constants
const (
	csHRedraw = 0x0002
	csVRedraw = 0x0001

	wsExLayered    = 0x00080000
	wsExToolWindow = 0x00000080
	wsPopup        = 0x80000000

	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoZOrder     = 0x0004
	swpNoActivate   = 0x0010
	swpShowWindow   = 0x0040
	swpHideWindow   = 0x0080
	swpFrameChanged = 0x0020

	swHide   = 0
	swShowNA = 8

	ulwAlpha = 0x00000002

	acSrcOver  = 0x00
	acSrcAlpha = 0x01

	wmLButtonDown = 0x0201
	wmMouseMove   = 0x0200
)

// blendFunction mirrors the Win32 BLENDFUNCTION structure.
type blendFunction struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

// size mirrors the Win32 SIZE structure.
type winSize struct {
	CX int32
	CY int32
}

// rect mirrors the Win32 RECT structure.
type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

// bitmapInfoHeader mirrors the Win32 BITMAPINFOHEADER structure.
type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

// bitmapInfo mirrors the Win32 BITMAPINFO structure for BI_RGB.
type bitmapInfo struct {
	BmiHeader bitmapInfoHeader
	BmiColors [1]uint32
}

// monitorInfoW matches the Win32 MONITORINFO structure.
type monitorInfoW struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

// Widget manages the transparent, borderless, draggable desktop layered window.
type Widget struct {
	mu sync.Mutex

	hwnd      uintptr
	hdcScreen uintptr
	hdcMem    uintptr
	hBitmap   uintptr
	pBits     unsafe.Pointer

	width  int
	height int
	posX   int
	posY   int

	isDragging      bool
	dragStartCursor point
	dragStartWin    point

	onPositionChanged func(x, y int)
	onContextMenu     func()
}

// activeWidget points to the current active widget instance for wndProc dispatch.
var activeWidget *Widget

// NewWidget creates the native layered window and initializes its DCs.
func NewWidget(onPosChanged func(x, y int), onContextMenu func()) (*Widget, error) {
	className, _ := syscall.UTF16PtrFromString("WeatherWidgetLayeredWndClass")
	hInst, _, _ := procGetModuleHandleW.Call(0)

	wc := wndClassExW{
		Style:         csHRedraw | csVRedraw,
		LpfnWndProc:   syscall.NewCallback(widgetWndProc),
		HInstance:     windows.Handle(hInst),
		LpszClassName: className,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// WS_EX_LAYERED | WS_EX_TOOLWINDOW: true per-pixel alpha and hidden from Alt+Tab
	exStyle := wsExLayered | wsExToolWindow
	style := wsPopup

	hwnd, _, err := procCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(className)),
		0,
		uintptr(style),
		0, 0, 0, 0,
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return nil, err
	}

	hdcScreen, _, _ := procGetDC.Call(0)
	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)

	w := &Widget{
		hwnd:              hwnd,
		hdcScreen:         hdcScreen,
		hdcMem:            hdcMem,
		onPositionChanged: onPosChanged,
		onContextMenu:     onContextMenu,
	}
	activeWidget = w

	return w, nil
}

// UpdateSurface updates the layered window with the provided 32-bit ARGB image.
func (w *Widget) UpdateSurface(img *image.RGBA) {
	if img == nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.hwnd == 0 {
		return
	}

	imgW := img.Bounds().Dx()
	imgH := img.Bounds().Dy()
	if imgW <= 0 || imgH <= 0 {
		return
	}

	// Reallocate DIBSection if dimensions changed
	if imgW != w.width || imgH != w.height || w.hBitmap == 0 {
		if w.hBitmap != 0 {
			procDeleteObject.Call(w.hBitmap)
			w.hBitmap = 0
		}

		var bmi bitmapInfo
		bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
		bmi.BmiHeader.BiWidth = int32(imgW)
		bmi.BmiHeader.BiHeight = -int32(imgH) // negative height = top-down DIB
		bmi.BmiHeader.BiPlanes = 1
		bmi.BmiHeader.BiBitCount = 32
		bmi.BmiHeader.BiCompression = 0 // BI_RGB

		hBitmap, _, err := procCreateDIBSection.Call(
			w.hdcMem,
			uintptr(unsafe.Pointer(&bmi)),
			0,
			uintptr(unsafe.Pointer(&w.pBits)),
			0, 0,
		)
		if hBitmap == 0 {
			log.Printf("uiwindows: CreateDIBSection failed: %v", err)
			return
		}

		procSelectObject.Call(w.hdcMem, hBitmap)
		w.hBitmap = hBitmap
		w.width = imgW
		w.height = imgH
	}

	// Fast convert RGBA to premultiplied BGRA directly into the DIBSection buffer
	pixCount := imgW * imgH
	srcPix := img.Pix
	dstPix := (*[1 << 28]byte)(w.pBits)[: pixCount*4 : pixCount*4]

	for i := 0; i < pixCount; i++ {
		si := i * 4
		r := srcPix[si]
		g := srcPix[si+1]
		b := srcPix[si+2]
		a := srcPix[si+3]

		if a == 0 {
			dstPix[si] = 0
			dstPix[si+1] = 0
			dstPix[si+2] = 0
			dstPix[si+3] = 0
		} else if a == 255 {
			dstPix[si] = b
			dstPix[si+1] = g
			dstPix[si+2] = r
			dstPix[si+3] = 255
		} else {
			dstPix[si] = byte((uint32(b)*uint32(a) + 127) / 255)
			dstPix[si+1] = byte((uint32(g)*uint32(a) + 127) / 255)
			dstPix[si+2] = byte((uint32(r)*uint32(a) + 127) / 255)
			dstPix[si+3] = a
		}
	}

	ptDst := point{X: int32(w.posX), Y: int32(w.posY)}
	ptSrc := point{X: 0, Y: 0}
	sz := winSize{CX: int32(imgW), CY: int32(imgH)}
	blend := blendFunction{
		BlendOp:             acSrcOver,
		BlendFlags:          0,
		SourceConstantAlpha: 255,
		AlphaFormat:         acSrcAlpha,
	}

	ret, _, err := procUpdateLayeredWindow.Call(
		w.hwnd,
		w.hdcScreen,
		uintptr(unsafe.Pointer(&ptDst)),
		uintptr(unsafe.Pointer(&sz)),
		w.hdcMem,
		uintptr(unsafe.Pointer(&ptSrc)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		ulwAlpha,
	)
	if ret == 0 {
		log.Printf("uiwindows: UpdateLayeredWindow failed: %v (hwnd=%#x, w=%d, h=%d)", err, w.hwnd, imgW, imgH)
	} else {
		log.Printf("uiwindows: UpdateLayeredWindow succeeded (hwnd=%#x, pos=(%d,%d), size=(%d,%d))", w.hwnd, w.posX, w.posY, imgW, imgH)
	}
}

// SetPosition moves the widget window to screen coordinates (x, y) maintaining Z-order.
func (w *Widget) SetPosition(x, y int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.posX = x
	w.posY = y
	if w.hwnd != 0 {
		procSetWindowPos.Call(
			w.hwnd,
			0,
			uintptr(x),
			uintptr(y),
			0, 0,
			swpNoSize|swpNoActivate|swpNoZOrder,
		)
	}
}

// ApplyPlacement calculates screen placement based on custom coordinates or corner position.
func (w *Widget) ApplyPlacement(customX, customY *int, corner string, monitorIndex int) {
	monLeft, monTop, monWidth, monHeight := getMonitorBounds(monitorIndex)
	const margin = 20

	cardW := w.width
	if cardW <= 0 {
		cardW = 380
	}
	cardH := w.height
	if cardH <= 0 {
		cardH = 250
	}

	if customX != nil && customY != nil {
		x := *customX
		y := *customY

		// Check if coordinates fall outside the monitor work area
		maxX := monLeft + monWidth - 80
		maxY := monTop + monHeight - 80

		if x < monLeft || x > maxX || y < monTop || y > maxY {
			log.Printf("uiwindows: saved coordinates (%d, %d) outside work area (%d, %d, %dx%d); safely clamping",
				x, y, monLeft, monTop, monWidth, monHeight)
			if x > monLeft+monWidth-cardW {
				x = monLeft + monWidth - cardW - margin
			}
			if x < monLeft {
				x = monLeft + margin
			}
			if y > monTop+monHeight-cardH {
				y = monTop + monHeight - cardH - margin
			}
			if y < monTop {
				y = monTop + margin
			}
		}

		w.SetPosition(x, y)
		return
	}

	targetX := monLeft + margin
	targetY := monTop + margin

	switch corner {
	case "top-right":
		targetX = monLeft + monWidth - cardW - margin
		targetY = monTop + margin
	case "bottom-left":
		targetX = monLeft + margin
		targetY = monTop + monHeight - cardH - margin
	case "bottom-right":
		targetX = monLeft + monWidth - cardW - margin
		targetY = monTop + monHeight - cardH - margin
	default: // "top-left"
		targetX = monLeft + margin
		targetY = monTop + margin
	}

	if targetX < monLeft {
		targetX = monLeft + margin
	}
	if targetY < monTop {
		targetY = monTop + margin
	}

	w.SetPosition(targetX, targetY)
}

// Show makes the layered window visible on the desktop.
func (w *Widget) Show() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.hwnd != 0 {
		procShowWindow.Call(w.hwnd, uintptr(swShowNA))
		procSetWindowPos.Call(w.hwnd, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpShowWindow|swpNoZOrder)
	}
}

// Hide hides the layered window.
func (w *Widget) Hide() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.hwnd != 0 {
		procShowWindow.Call(w.hwnd, uintptr(swHide))
	}
}

// Destroy cleans up window handles, bitmaps, and DCs.
func (w *Widget) Destroy() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.hBitmap != 0 {
		procDeleteObject.Call(w.hBitmap)
		w.hBitmap = 0
	}
	if w.hdcMem != 0 {
		procDeleteDC.Call(w.hdcMem)
		w.hdcMem = 0
	}
	if w.hdcScreen != 0 {
		procReleaseDC.Call(0, w.hdcScreen)
		w.hdcScreen = 0
	}
	if w.hwnd != 0 {
		procDestroyWindow.Call(w.hwnd)
		w.hwnd = 0
	}
	if activeWidget == w {
		activeWidget = nil
	}
}

// widgetWndProc handles dragging and context menu mouse events for the layered window.
func widgetWndProc(hwnd uintptr, umsg uint32, wparam, lparam uintptr) uintptr {
	w := activeWidget
	if w == nil || w.hwnd != hwnd {
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(umsg), wparam, lparam)
		return ret
	}

	switch umsg {
	case wmLButtonDown:
		procSetCapture.Call(hwnd)
		w.isDragging = true
		procGetCursorPos.Call(uintptr(unsafe.Pointer(&w.dragStartCursor)))
		var r rect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		w.dragStartWin = point{X: r.Left, Y: r.Top}
		return 0

	case wmMouseMove:
		if w.isDragging {
			var cur point
			procGetCursorPos.Call(uintptr(unsafe.Pointer(&cur)))
			dx := cur.X - w.dragStartCursor.X
			dy := cur.Y - w.dragStartCursor.Y
			newX := int(w.dragStartWin.X + dx)
			newY := int(w.dragStartWin.Y + dy)
			w.posX = newX
			w.posY = newY
			procSetWindowPos.Call(hwnd, 0, uintptr(newX), uintptr(newY), 0, 0, swpNoSize|swpNoActivate|swpNoZOrder)
		}
		return 0

	case wmLButtonUp:
		if w.isDragging {
			w.isDragging = false
			procReleaseCapture.Call()
			if w.onPositionChanged != nil {
				w.onPositionChanged(w.posX, w.posY)
			}
		}
		return 0

	case wmRButtonUp, wmContextMenu:
		if w.onContextMenu != nil {
			w.onContextMenu()
		}
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(umsg), wparam, lparam)
	return ret
}

// ── Multi-Monitor Bounds Helper ──────────────────────────────────────────────

var monList []uintptr

func monEnumCallback(hMonitor, hdc, lprcClip, dwData uintptr) uintptr {
	monList = append(monList, hMonitor)
	return 1
}

// getMonitorBounds retrieves the work area rectangle for the specified monitor index.
func getMonitorBounds(index int) (left, top, width, height int) {
	monList = nil
	cb := syscall.NewCallback(monEnumCallback)
	procEnumDisplayMonitors.Call(0, 0, cb, 0)

	if len(monList) == 0 {
		cx, _, _ := procGetSystemMetrics.Call(0) // SM_CXSCREEN
		cy, _, _ := procGetSystemMetrics.Call(1) // SM_CYSCREEN
		if cx == 0 || cy == 0 {
			return 0, 0, 1920, 1080
		}
		return 0, 0, int(cx), int(cy)
	}

	if index < 0 || index >= len(monList) {
		index = 0
	}

	hMon := monList[index]
	var mi monitorInfoW
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	ret, _, _ := procGetMonitorInfoW.Call(hMon, uintptr(unsafe.Pointer(&mi)))
	if ret == 0 {
		return 0, 0, 1920, 1080
	}

	work := mi.RcWork
	return int(work.Left), int(work.Top), int(work.Right - work.Left), int(work.Bottom - work.Top)
}
