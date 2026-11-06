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

	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procSetCapture          = user32.NewProc("SetCapture")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procShowWindow          = user32.NewProc("ShowWindow")
	procGetDpiForWindow     = user32.NewProc("GetDpiForWindow")
	procGetDpiForSystem     = user32.NewProc("GetDpiForSystem")
)

// GetWindowDPI returns the monitor DPI where hwnd is displayed, or system DPI,
// or 96 (standard 100% scale) as fallback.
func GetWindowDPI(hwnd uintptr) int {
	if procGetDpiForWindow.Find() == nil && hwnd != 0 {
		dpi, _, _ := procGetDpiForWindow.Call(hwnd)
		if dpi > 0 {
			return int(dpi)
		}
	}
	if procGetDpiForSystem.Find() == nil {
		dpi, _, _ := procGetDpiForSystem.Call()
		if dpi > 0 {
			return int(dpi)
		}
	}
	return 96
}

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
	oldBitmap uintptr
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
	onDPIChanged      func()
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

// SetOnDPIChanged sets the callback invoked when the monitor DPI changes.
func (w *Widget) SetOnDPIChanged(fn func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onDPIChanged = fn
}

// UpdateSurface updates the layered window with the provided 32-bit ARGB image at 1:1 scale.
func (w *Widget) UpdateSurface(img *image.RGBA) {
	w.UpdateSurfaceScaled(img, 1.0)
}

// UpdateSurfaceScaled updates the layered window with the provided 32-bit ARGB image
// scaled by the specified factor (e.g. 1.0 for 100%, 2.0 for 200%).
func (w *Widget) UpdateSurfaceScaled(img *image.RGBA, scale float64) {
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

	if scale <= 0 {
		scale = 1.0
	}

	targetW := int(float64(imgW)*scale + 0.5)
	targetH := int(float64(imgH)*scale + 0.5)
	if targetW <= 0 || targetH <= 0 {
		targetW = imgW
		targetH = imgH
	}

	// Reallocate DIBSection if dimensions changed
	if targetW != w.width || targetH != w.height || w.hBitmap == 0 {
		if w.hBitmap != 0 {
			procSelectObject.Call(w.hdcMem, w.oldBitmap)
			procDeleteObject.Call(w.hBitmap)
			w.hBitmap = 0
		}

		var bmi bitmapInfo
		bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
		bmi.BmiHeader.BiWidth = int32(targetW)
		bmi.BmiHeader.BiHeight = -int32(targetH) // negative height = top-down DIB
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

		w.oldBitmap, _, _ = procSelectObject.Call(w.hdcMem, hBitmap)
		w.hBitmap = hBitmap
		w.width = targetW
		w.height = targetH
	}

	dstPix := (*[1 << 28]byte)(w.pBits)[: targetW*targetH*4 : targetW*targetH*4]

	if targetW == imgW && targetH == imgH {
		// Fast 1:1 convert RGBA to premultiplied BGRA directly into the DIBSection buffer
		pixCount := imgW * imgH
		srcPix := img.Pix
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
			} else {
				dstPix[si] = b
				dstPix[si+1] = g
				dstPix[si+2] = r
				dstPix[si+3] = a
			}
		}
	} else {
		// High-performance bilinear scale and premultiply directly into DIBSection buffer
		fastScaleAndPremultiply(img, dstPix, targetW, targetH)
	}

	ptDst := point{X: int32(w.posX), Y: int32(w.posY)}
	ptSrc := point{X: 0, Y: 0}
	sz := winSize{CX: int32(targetW), CY: int32(targetH)}
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
		log.Printf("uiwindows: UpdateLayeredWindow failed: %v (hwnd=%#x, w=%d, h=%d)", err, w.hwnd, targetW, targetH)
	}
}

// fastScaleAndPremultiply scales src into dst with bilinear interpolation and premultiplies alpha for AC_SRC_ALPHA.
func fastScaleAndPremultiply(src *image.RGBA, dst []byte, dstW, dstH int) {
	srcW := src.Bounds().Dx()
	srcH := src.Bounds().Dy()
	if srcW <= 0 || srcH <= 0 || dstW <= 0 || dstH <= 0 {
		return
	}
	srcPix := src.Pix
	srcStride := src.Stride

	xMap := make([]int, dstW)
	xWeight := make([]int, dstW)
	for dx := 0; dx < dstW; dx++ {
		srcXFixed := (dx * (srcW - 1) << 16) / max(1, dstW-1)
		xMap[dx] = srcXFixed >> 16
		xWeight[dx] = (srcXFixed >> 8) & 0xFF
	}

	for dy := 0; dy < dstH; dy++ {
		srcYFixed := (dy * (srcH - 1) << 16) / max(1, dstH-1)
		sy0 := srcYFixed >> 16
		sy1 := sy0 + 1
		if sy1 >= srcH {
			sy1 = sy0
		}
		wy1 := (srcYFixed >> 8) & 0xFF
		wy0 := 256 - wy1

		row0Off := sy0 * srcStride
		row1Off := sy1 * srcStride
		dstOff := dy * dstW * 4

		for dx := 0; dx < dstW; dx++ {
			sx0 := xMap[dx]
			sx1 := sx0 + 1
			if sx1 >= srcW {
				sx1 = sx0
			}
			wx1 := xWeight[dx]
			wx0 := 256 - wx1

			w00 := wx0 * wy0
			w10 := wx1 * wy0
			w01 := wx0 * wy1
			w11 := wx1 * wy1

			i00 := row0Off + sx0*4
			i10 := row0Off + sx1*4
			i01 := row1Off + sx0*4
			i11 := row1Off + sx1*4

			r := (int(srcPix[i00])*w00 + int(srcPix[i10])*w10 + int(srcPix[i01])*w01 + int(srcPix[i11])*w11) >> 16
			g := (int(srcPix[i00+1])*w00 + int(srcPix[i10+1])*w10 + int(srcPix[i01+1])*w01 + int(srcPix[i11+1])*w11) >> 16
			b := (int(srcPix[i00+2])*w00 + int(srcPix[i10+2])*w10 + int(srcPix[i01+2])*w01 + int(srcPix[i11+2])*w11) >> 16
			a := (int(srcPix[i00+3])*w00 + int(srcPix[i10+3])*w10 + int(srcPix[i01+3])*w01 + int(srcPix[i11+3])*w11) >> 16

			di := dstOff + dx*4
			if a == 0 {
				dst[di] = 0
				dst[di+1] = 0
				dst[di+2] = 0
				dst[di+3] = 0
			} else {
				dst[di] = byte(b)
				dst[di+1] = byte(g)
				dst[di+2] = byte(r)
				dst[di+3] = byte(a)
			}
		}
	}
}

// GetPosition returns the widget's current screen coordinates.
func (w *Widget) GetPosition() (int, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.hwnd != 0 {
		var r rect
		if ret, _, _ := procGetWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&r))); ret != 0 {
			return int(r.Left), int(r.Top)
		}
	}
	return w.posX, w.posY
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
		if x < monLeft || x+cardW > monLeft+monWidth || y < monTop || y+cardH > monTop+monHeight {
			log.Printf("uiwindows: saved coordinates (%d, %d, %dx%d) outside work area (%d, %d, %dx%d); safely clamping",
				x, y, cardW, cardH, monLeft, monTop, monWidth, monHeight)
			if x+cardW > monLeft+monWidth {
				x = monLeft + monWidth - cardW - margin
			}
			if x < monLeft {
				x = monLeft + margin
			}
			if y+cardH > monTop+monHeight {
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
		procSelectObject.Call(w.hdcMem, w.oldBitmap)
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

	case 0x02E0: // WM_DPICHANGED
		if w.onDPIChanged != nil {
			w.onDPIChanged()
		}
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(umsg), wparam, lparam)
	return ret
}

// ── Multi-Monitor Bounds Helper ──────────────────────────────────────────────

var monList []uintptr
var monMu sync.Mutex

func monEnumCallback(hMonitor, hdc, lprcClip, dwData uintptr) uintptr {
	monList = append(monList, hMonitor)
	return 1
}

// getMonitorBounds retrieves the work area rectangle for the specified monitor index.
func getMonitorBounds(index int) (left, top, width, height int) {
	monMu.Lock()
	defer monMu.Unlock()
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

func monitorCount() int {
	monMu.Lock()
	defer monMu.Unlock()
	monList = nil
	procEnumDisplayMonitors.Call(0, 0, syscall.NewCallback(monEnumCallback), 0)
	if len(monList) == 0 {
		return 1
	}
	return len(monList)
}
