//go:build windows

package uiwindows

import (
	"sync/atomic"
	"testing"
	"unsafe"
	"weatherwidget/assets"
)

func TestLoadTrayIconFromPNG(t *testing.T) {
	pngData, err := assets.Icons.ReadFile("icons/original/clear_tray.png")
	if err != nil {
		t.Fatalf("failed to read clear_tray.png: %v", err)
	}
	if len(pngData) == 0 {
		t.Fatal("clear_tray.png is empty")
	}

	hIcon, _, err := procCreateIconFromResource.Call(
		uintptr(unsafe.Pointer(&pngData[0])),
		uintptr(len(pngData)),
		1,          // fIcon = TRUE (icon, not cursor)
		0x00030000, // dwVersion = 3.0
		16,         // cxDesired
		16,         // cyDesired
		0,          // LR_DEFAULTCOLOR
	)

	if hIcon == 0 {
		t.Fatalf("CreateIconFromResourceEx failed: %v", err)
	}
	defer procDestroyIcon.Call(hIcon)

	t.Logf("Successfully created HICON: %#x from clear_tray.png", hIcon)
}

func TestTrayLifecycle(t *testing.T) {
	var showCalled int32
	var hideCalled int32
	var refreshCalled int32
	var settingsCalled int32
	var quitCalled int32

	callbacks := TrayCallbacks{
		OnShowWidget: func() { atomic.AddInt32(&showCalled, 1) },
		OnHideWidget: func() { atomic.AddInt32(&hideCalled, 1) },
		OnRefresh:    func() { atomic.AddInt32(&refreshCalled, 1) },
		OnSettings:   func() { atomic.AddInt32(&settingsCalled, 1) },
		OnQuit:       func() { atomic.AddInt32(&quitCalled, 1) },
		GetLabel: func(key string) string {
			return "Mock " + key
		},
	}

	tray, err := newTray(callbacks)
	if err != nil {
		t.Fatalf("newTray returned error: %v", err)
	}
	if tray == nil {
		t.Fatal("newTray returned nil tray")
	}
	if tray.hwnd == 0 {
		t.Fatal("tray hwnd is 0")
	}
	t.Logf("Created tray with hwnd: %#x, hIcon: %#x, nid.Size: %d", tray.hwnd, tray.hIcon, tray.nid.Size)

	// Verify left click handler dispatches ShowWidget
	tray.handleLeftClick()
	if atomic.LoadInt32(&showCalled) != 1 {
		t.Errorf("expected showCalled to be 1, got %d", showCalled)
	}

	// Verify destroying the tray cleans up properly
	tray.Destroy()
	if tray.hwnd != 0 {
		t.Errorf("expected hwnd to be cleared after Destroy, got %#x", tray.hwnd)
	}
}
