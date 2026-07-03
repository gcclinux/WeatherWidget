//go:build darwin

package ui

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

// createOffscreenNSWindow creates a minimal borderless NSWindow positioned
// off-screen at (-9999, -9999). No visible display is required; the window is
// used only to exercise NSWindow/NSView layer properties in unit tests.
static uintptr_t createOffscreenNSWindow(void) {
	NSRect frame = NSMakeRect(-9999, -9999, 1, 1);
	NSWindow *w = [[NSWindow alloc]
		initWithContentRect:frame
				  styleMask:NSWindowStyleMaskBorderless
					backing:NSBackingStoreBuffered
					  defer:NO];
	[w setReleasedWhenClosed:NO];
	[[w contentView] setWantsLayer:YES];
	return (uintptr_t)CFBridgingRetain(w);
}

// releaseOffscreenNSWindow releases a window created by createOffscreenNSWindow.
static void releaseOffscreenNSWindow(uintptr_t winHandle) {
	CFBridgingRelease((void*)winHandle);
}

// testGetNSWindowAlphaValue returns [w alphaValue].
static double testGetNSWindowAlphaValue(uintptr_t winHandle) {
	NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
	return (double)[w alphaValue];
}

// testGetContentViewCornerRadius returns contentView.layer.cornerRadius.
static double testGetContentViewCornerRadius(uintptr_t winHandle) {
	NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
	NSView *cv = [w contentView];
	[cv setWantsLayer:YES];
	return (double)cv.layer.cornerRadius;
}

// testGetContentViewMasksToBounds returns 1 if contentView.layer.masksToBounds,
// 0 otherwise.
static int testGetContentViewMasksToBounds(uintptr_t winHandle) {
	NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
	NSView *cv = [w contentView];
	[cv setWantsLayer:YES];
	return cv.layer.masksToBounds ? 1 : 0;
}

// testCallSetNSWindowAlphaDirect calls [w setAlphaValue: opacityPercent/100.0]
// directly without dispatch.
static void testCallSetNSWindowAlphaDirect(uintptr_t winHandle, int opacityPercent) {
	NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
	CGFloat alpha = (CGFloat)opacityPercent / 100.0;
	[w setAlphaValue:alpha];
}

// testMoveNSWindowToSync moves the window to (x, y) synchronously.
static void testMoveNSWindowToSync(uintptr_t winHandle, int x, int y) {
	NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
	NSScreen *screen = [w screen];
	if (screen == nil) screen = [NSScreen mainScreen];
	CGFloat screenHeight = [screen frame].size.height;
	CGFloat windowHeight = [w frame].size.height;
	CGFloat flippedY = screenHeight - (CGFloat)y - windowHeight;
	[w setFrameOrigin:NSMakePoint((CGFloat)x, flippedY)];
}

// testGetNSWindowPosition returns the current top-left position of the window.
static void testGetNSWindowPosition(uintptr_t winHandle, int* outX, int* outY) {
	NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
	NSScreen *screen = [w screen];
	if (screen == nil) screen = [NSScreen mainScreen];
	CGFloat screenHeight = [screen frame].size.height;
	NSRect frame = [w frame];
	*outX = (int)frame.origin.x;
	*outY = (int)(screenHeight - frame.origin.y - frame.size.height);
}

// testSetupDarwinWindow replicates setupDarwinWindow inline for test use.
static void testSetupDarwinWindow(uintptr_t winHandle) {
	NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
	[w setOpaque:NO];
	[w setBackgroundColor:[NSColor clearColor]];
	[w setHasShadow:NO];
	NSView *contentView = [w contentView];
	contentView.wantsLayer = YES;
	contentView.layer.cornerRadius = 12.0;
	contentView.layer.masksToBounds = YES;
	for (NSView *sub in [contentView subviews]) {
		sub.wantsLayer = YES;
		sub.layer.cornerRadius = 12.0;
		sub.layer.masksToBounds = YES;
	}
}

// testSetDarwinBackgroundAlpha replicates setDarwinBackgroundAlpha inline.
// Uses setAlphaValue directly (synchronous — no dispatch_async in tests).
static void testSetDarwinBackgroundAlpha(uintptr_t winHandle, int opacityPercent) {
	NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
	CGFloat alpha = (CGFloat)opacityPercent / 100.0;
	[w setAlphaValue:alpha];
}

// testGetWindowBackgroundAlpha returns the NSWindow.alphaValue (since we now
// use alphaValue for transparency, not backgroundColor).
static double testGetWindowBackgroundAlpha(uintptr_t winHandle) {
	NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
	return (double)[w alphaValue];
}

// testIsWindowOpaque returns 1 if [w isOpaque], 0 otherwise.
static int testIsWindowOpaque(uintptr_t winHandle) {
	NSWindow *w = (__bridge NSWindow*)(void*)winHandle;
	return [w isOpaque] ? 1 : 0;
}
*/
import "C"

// testCreateOffscreenNSWindow creates a minimal borderless NSWindow for testing.
func testCreateOffscreenNSWindow() C.uintptr_t {
	return C.createOffscreenNSWindow()
}

// testReleaseOffscreenNSWindow releases a window created by testCreateOffscreenNSWindow.
func testReleaseOffscreenNSWindow(handle C.uintptr_t) {
	C.releaseOffscreenNSWindow(handle)
}

// testGetNSWindowAlphaValue returns the alphaValue of the NSWindow.
func testGetNSWindowAlphaValue(handle C.uintptr_t) float64 {
	return float64(C.testGetNSWindowAlphaValue(handle))
}

// testGetContentViewCornerRadius returns contentView.layer.cornerRadius.
func testGetContentViewCornerRadius(handle C.uintptr_t) float64 {
	return float64(C.testGetContentViewCornerRadius(handle))
}

// testGetContentViewMasksToBounds returns true if contentView.layer.masksToBounds is set.
func testGetContentViewMasksToBounds(handle C.uintptr_t) bool {
	return C.testGetContentViewMasksToBounds(handle) != 0
}

// testApplySetNSWindowAlpha calls [w setAlphaValue:] directly.
func testApplySetNSWindowAlpha(handle C.uintptr_t, opacityPercent int) {
	C.testCallSetNSWindowAlphaDirect(handle, C.int(opacityPercent))
}

// testMoveNSWindowToSync moves the window synchronously to the given top-left coords.
func testMoveNSWindowToSync(handle C.uintptr_t, x, y int) {
	C.testMoveNSWindowToSync(handle, C.int(x), C.int(y))
}

// testGetNSWindowPosition returns the current top-left position of the window.
func testGetNSWindowPosition(handle C.uintptr_t) (int, int) {
	var x, y C.int
	C.testGetNSWindowPosition(handle, &x, &y)
	return int(x), int(y)
}

// testSetupDarwinWindow calls setupDarwinWindow on the given handle.
func testSetupDarwinWindow(handle C.uintptr_t) {
	C.testSetupDarwinWindow(handle)
}

// testSetDarwinBackgroundAlpha calls setDarwinBackgroundAlpha on the given handle.
func testSetDarwinBackgroundAlpha(handle C.uintptr_t, opacityPercent int) {
	C.testSetDarwinBackgroundAlpha(handle, C.int(opacityPercent))
}

// testGetWindowBackgroundAlpha returns the alpha component of NSWindow.backgroundColor.
func testGetWindowBackgroundAlpha(handle C.uintptr_t) float64 {
	return float64(C.testGetWindowBackgroundAlpha(handle))
}

// testIsWindowOpaque returns true if the NSWindow reports isOpaque = YES.
func testIsWindowOpaque(handle C.uintptr_t) bool {
	return C.testIsWindowOpaque(handle) == 1
}
