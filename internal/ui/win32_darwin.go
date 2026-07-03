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

// setupDarwinWindow configures the NSWindow for rounded corners.
//
// Rounded corners: the NSWindow must be non-opaque with a clear background
// so that the corner regions become transparent. The contentView layer clips
// all subviews (including Fyne's GL canvas) to the rounded rect.
//
// Transparency: handled by setDarwinBackgroundAlpha via NSWindow.alphaValue.
// Since Fyne renders all content into a single opaque OpenGL framebuffer,
// NSWindow.alphaValue is the only mechanism that achieves see-through effect.
//
// IMPORTANT: This function dispatches to the main queue because all
// NSView/NSWindow operations must happen on the main thread.
static void setupDarwinWindow(uintptr_t winHandle) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
		// Non-opaque + clear background lets the corners show through to desktop.
		[w setOpaque:NO];
		[w setBackgroundColor:[NSColor clearColor]];
		[w setHasShadow:NO];

		NSView *contentView = [w contentView];
		contentView.wantsLayer = YES;
		contentView.layer.cornerRadius = 12.0;
		contentView.layer.masksToBounds = YES;

		// Also clip all child subviews (the Fyne GL NSOpenGLView) to the
		// rounded corner mask by ensuring each subview's layer respects bounds.
		for (NSView *sub in [contentView subviews]) {
			sub.wantsLayer = YES;
			sub.layer.cornerRadius = 12.0;
			sub.layer.masksToBounds = YES;
		}
	});
}

// setDarwinBackgroundAlpha applies window-level transparency.
// opacityPercent is in [1, 100]. Uses NSWindow.alphaValue because Fyne's
// OpenGL framebuffer is fully opaque — there is no way to make just the
// background transparent while keeping text at full opacity with a single
// GL surface. The entire window (including content) fades together.
//
// To keep content readable, we remap the user-facing opacity values to a
// narrower alphaValue range:
//   25% → 0.55 (semi-transparent but text still legible)
//   50% → 0.70
//   75% → 0.85
//  100% → 1.00
static void setDarwinBackgroundAlpha(uintptr_t winHandle, int opacityPercent) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
		// Map [25, 100] → [0.55, 1.0] linearly.
		// For values below 25, clamp to 0.55.
		CGFloat alpha;
		if (opacityPercent >= 100) {
			alpha = 1.0;
		} else if (opacityPercent <= 25) {
			alpha = 0.55;
		} else {
			// Linear interpolation: 25→0.55, 100→1.0
			alpha = 0.55 + (CGFloat)(opacityPercent - 25) * (0.45 / 75.0);
		}
		[w setAlphaValue:alpha];
	});
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

// applyDarwinWindowSetup calls setupDarwinWindow on the widget window's
// NSWindow handle.  Because the handle may not be available immediately after
// Show(), it uses the same retry-after-delay pattern as moveWindow
// (150 ms, 400 ms, 900 ms).
func applyDarwinWindowSetup() {
	handle := getNSWindowHandle()
	if handle != 0 {
		C.setupDarwinWindow(handle)
		return
	}
	// Handle not yet available — retry in a background goroutine.
	go func() {
		for _, delay := range []int{150, 400, 900} {
			time.Sleep(time.Duration(delay) * time.Millisecond)
			h := getNSWindowHandle()
			if h != 0 {
				C.setupDarwinWindow(h)
				return
			}
		}
		log.Println("macOS: applyDarwinWindowSetup — could not get NSWindow handle after retries")
	}()
}

// setWindowOpacity applies transparency to the widget window on macOS.
// Transparency is applied only to the WWidgetBackgroundView CALayer so that
// text labels and weather icons remain at full opacity.
func setWindowOpacity(opacityPercent int) {
	handle := getNSWindowHandle()
	if handle == 0 {
		log.Println("macOS: setWindowOpacity — could not get NSWindow handle")
		return
	}
	C.setDarwinBackgroundAlpha(handle, C.int(opacityPercent))
	log.Printf("macOS: setWindowOpacity %d%%", opacityPercent)
}

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
