//go:build darwin

package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// moveNSWindowTo moves the given NSWindow to (x, y) using top-left origin.
// macOS uses bottom-left origin, so we flip Y relative to the screen that
// contains the window.
static void moveNSWindowTo(uintptr_t winHandle, int x, int y) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
		NSScreen *screen = [w screen];
		if (screen == nil) screen = [NSScreen mainScreen];
		CGFloat screenHeight = [screen frame].size.height;
		CGFloat windowHeight = [w frame].size.height;
		CGFloat flippedY = screenHeight - (CGFloat)y - windowHeight;
		[w setFrameOrigin:NSMakePoint((CGFloat)x, flippedY)];
	});
}

// getNSWindowPos returns the current top-left position of the NSWindow.
static void getNSWindowPos(uintptr_t winHandle, int* outX, int* outY) {
	*outX = 0;
	*outY = 0;
	NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
	NSScreen *screen = [w screen];
	if (screen == nil) screen = [NSScreen mainScreen];
	CGFloat screenHeight = [screen frame].size.height;
	NSRect frame = [w frame];
	*outX = (int)frame.origin.x;
	*outY = (int)(screenHeight - frame.origin.y - frame.size.height);
}

// getMainScreenSize returns the main screen dimensions.
static void getMainScreenSize(int* w, int* h) {
	NSScreen *screen = [NSScreen mainScreen];
	NSRect frame = [screen frame];
	*w = (int)frame.size.width;
	*h = (int)frame.size.height;
}

// getScreenCount returns the number of screens.
static int getScreenCount(void) {
	return (int)[[NSScreen screens] count];
}

// getScreenBounds returns the visible frame (excluding menu bar/dock) for the
// screen at the given index. Coordinates use top-left origin.
static void getScreenBounds(int index, int* outX, int* outY, int* outW, int* outH) {
	NSArray *screens = [NSScreen screens];
	if (index < 0 || index >= (int)[screens count]) {
		index = 0;
	}
	NSScreen *screen = [screens objectAtIndex:index];
	NSRect visible = [screen visibleFrame];
	NSRect full    = [screen frame];

	*outX = (int)visible.origin.x;
	// Convert Y from bottom-left to top-left origin.
	*outY = (int)(full.size.height - visible.origin.y - visible.size.height);
	*outW = (int)visible.size.width;
	*outH = (int)visible.size.height;
}
*/
import "C"

import (
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

// darwinWidgetWindow holds a reference to the widget window so we can
// retrieve its native NSWindow pointer without relying on title search.
var darwinWidgetWindow fyne.Window

// registerDarwinWindow stores the widget window reference for later use
// by moveWindow and getWindowPosition. Must be called once after the window
// is created.
func registerDarwinWindow(w fyne.Window) {
	darwinWidgetWindow = w
}

// getNSWindowHandle returns the native NSWindow handle (uintptr) for the
// stored widget window using Fyne's driver.NativeWindow interface.
func getNSWindowHandle() C.uintptr_t {
	if darwinWidgetWindow == nil {
		log.Println("macOS: getNSWindowHandle — darwinWidgetWindow is nil")
		return 0
	}
	nativeWin, ok := darwinWidgetWindow.(driver.NativeWindow)
	if !ok {
		log.Printf("macOS: window type %T does not implement driver.NativeWindow", darwinWidgetWindow)
		return 0
	}
	var handle uintptr
	nativeWin.RunNative(func(ctx any) {
		// Fyne passes MacWindowContext by value, not pointer.
		if macCtx, ok := ctx.(driver.MacWindowContext); ok {
			handle = macCtx.NSWindow
		}
	})
	return C.uintptr_t(handle)
}

// applyToolWindowStyle is a no-op on macOS.
// Window decorations are already removed via CreateSplashWindow.
func applyToolWindowStyle(_ string) {}

// getScreenSize returns the main screen dimensions on macOS.
func getScreenSize() (int, int) {
	var w, h C.int
	C.getMainScreenSize(&w, &h)
	return int(w), int(h)
}

// moveWindow repositions the widget window to the given screen coordinates.
// It moves immediately and retries after short delays to handle the case where
// Fyne/GLFW repositions the window after Show().
func moveWindow(_ fyne.Window, x, y int) {
	notifyDarwinMoveByUs()
	handle := getNSWindowHandle()
	if handle == 0 {
		log.Println("macOS: moveWindow — could not get NSWindow handle")
		return
	}
	log.Printf("macOS: moveWindow x=%d y=%d", x, y)
	C.moveNSWindowTo(handle, C.int(x), C.int(y))
	go func(h C.uintptr_t, px, py C.int) {
		for _, delay := range []int{150, 400, 900} {
			time.Sleep(time.Duration(delay) * time.Millisecond)
			notifyDarwinMoveByUs()
			C.moveNSWindowTo(h, px, py)
		}
	}(handle, C.int(x), C.int(y))
}

// getWindowPosition returns the current top-left position of the widget window.
func getWindowPosition() (int, int) {
	handle := getNSWindowHandle()
	if handle == 0 {
		return 0, 0
	}
	var x, y C.int
	C.getNSWindowPos(handle, &x, &y)
	return int(x), int(y)
}

// setWindowOpacity is a no-op on macOS for now.
func setWindowOpacity(_ int) {}

// getMonitorCount returns the number of display monitors on macOS.
func getMonitorCount() int {
	return int(C.getScreenCount())
}

// getMonitorBounds returns the visible work-area rectangle for the monitor
// at the given index. Coordinates use top-left origin.
func getMonitorBounds(index int) (int, int, int, int) {
	var x, y, w, h C.int
	C.getScreenBounds(C.int(index), &x, &y, &w, &h)
	return int(x), int(y), int(w), int(h)
}
