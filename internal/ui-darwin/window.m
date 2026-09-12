// window.m — NSWindow setup for the WeatherWidget desktop overlay on macOS.
//
// Creates a transparent, borderless NSWindow that sits below all normal windows
// (NSDesktopWindowLevel + 1) and does not appear in the Dock or Mission Control.
//
// THREADING RULES
// ───────────────
// All Cocoa/AppKit object creation and mutation must happen on the main thread.
//
// Functions called during start-up (createWidgetWindow, createCardContainer)
// are invoked from manager.start(), which already runs on the OS main thread
// (runtime.LockOSThread + NSApplication is on thread 1).  Using dispatch_sync
// from the main thread to the main queue deadlocks immediately.
//
// The safe pattern: run_on_main() checks [NSThread isMainThread] and either
// executes the block inline or uses dispatch_sync if called from a worker.
// This makes every C function safe to call from any context without deadlock.

#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import "callbacks.h"

// run_on_main — execute block on the main thread without deadlocking.
// • Already on main → run inline.
// • On a worker thread → dispatch_sync (blocks until done, result available).
#define run_on_main(blk) \
    do { \
        if ([NSThread isMainThread]) { blk(); } \
        else { dispatch_sync(dispatch_get_main_queue(), blk); } \
    } while(0)

// ── DragView ─────────────────────────────────────────────────────────────────
// Transparent NSView covering the window; routes left-click drags to reposition
// the borderless window by moving its frame origin.

@interface WWDragView : NSView
@property (nonatomic, assign) NSPoint dragStart;
@property (nonatomic, assign) NSPoint windowOriginOnDrag;
@property (nonatomic, assign) BOOL dragging;
@property (nonatomic, copy)   void (^onDragEnd)(void);
@end

@implementation WWDragView

- (instancetype)initWithFrame:(NSRect)frame {
    self = [super initWithFrame:frame];
    if (self) {
        self.dragging = NO;
        self.wantsLayer = YES;
        self.layer.backgroundColor = [[NSColor clearColor] CGColor];
    }
    return self;
}

- (BOOL)mouseDownCanMoveWindow { return NO; }
- (BOOL)isOpaque { return NO; }

- (void)mouseDown:(NSEvent *)event {
    self.dragStart = [NSEvent mouseLocation];
    self.windowOriginOnDrag = self.window.frame.origin;
    self.dragging = YES;
}

- (void)mouseDragged:(NSEvent *)event {
    if (!self.dragging) return;
    NSPoint current = [NSEvent mouseLocation];
    CGFloat dx = current.x - self.dragStart.x;
    CGFloat dy = current.y - self.dragStart.y;
    [self.window setFrameOrigin:NSMakePoint(
        self.windowOriginOnDrag.x + dx,
        self.windowOriginOnDrag.y + dy
    )];
}

- (void)mouseUp:(NSEvent *)event {
    if (!self.dragging) return;
    self.dragging = NO;
    if (self.onDragEnd) self.onDragEnd();
}

@end

// ── WidgetWindowDelegate ──────────────────────────────────────────────────────
// Keeps the window at desktop level when it momentarily becomes key/main.

@interface WWWindowDelegate : NSObject <NSWindowDelegate>
@end

@implementation WWWindowDelegate

- (void)windowDidBecomeKey:(NSNotification *)notification {
    [(NSWindow *)notification.object setLevel:NSNormalWindowLevel - 1];
}

- (void)windowDidBecomeMain:(NSNotification *)notification {
    [(NSWindow *)notification.object setLevel:NSNormalWindowLevel - 1];
}

@end


// ── Public C API ─────────────────────────────────────────────────────────────

uintptr_t createWidgetWindow(void) {
    __block uintptr_t handle = 0;
    run_on_main(^{
        NSRect frame = NSMakeRect(0, 0, 400, 300);

        // Use a plain NSWindow (not NSPanel). NSPanel with hidesOnDeactivate
        // (the default) disappears the instant an accessory app loses focus,
        // which is exactly what was happening. NSWindow stays put.
        NSWindow *w = [[NSWindow alloc]
            initWithContentRect:frame
            styleMask:NSWindowStyleMaskBorderless
            backing:NSBackingStoreBuffered
            defer:NO];

        [w setOpaque:NO];
        [w setBackgroundColor:[NSColor clearColor]];
        [w setHasShadow:YES];
        [w setIgnoresMouseEvents:NO];
        // Never hide when the app deactivates (critical for a desktop widget).
        [w setHidesOnDeactivate:NO];
        // Keep it out of window cycling / release behaviour.
        [w setReleasedWhenClosed:NO];
        // NSNormalWindowLevel - 1: just below normal app windows, always
        // visible above the wallpaper regardless of whether the user is on
        // the desktop.
        [w setLevel:NSNormalWindowLevel - 1];
        [w setCollectionBehavior:
            NSWindowCollectionBehaviorCanJoinAllSpaces |
            NSWindowCollectionBehaviorStationary |
            NSWindowCollectionBehaviorIgnoresCycle];

        CFRetain((__bridge CFTypeRef)w);

        WWWindowDelegate *del = [[WWWindowDelegate alloc] init];
        CFRetain((__bridge CFTypeRef)del);
        [w setDelegate:del];

        handle = (uintptr_t)(__bridge void *)w;
    });
    return handle;
}

void destroyWidgetWindow(uintptr_t win) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *w = (__bridge NSWindow *)(void *)win;
        [w orderOut:nil];
        CFRelease((__bridge CFTypeRef)w);
    });
}

void showWidgetWindow(uintptr_t win) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindow *w = (__bridge NSWindow *)(void *)win;
        [w orderFrontRegardless];
        // Re-assert termination prevention AFTER the window is shown. AppKit
        // re-enables automatic termination based on its window count during
        // launch ("No windows open yet"), overriding the earlier disable in
        // initAndRun. Re-disabling here makes it stick.
        [[NSProcessInfo processInfo]
            disableAutomaticTermination:@"WeatherWidget desktop widget"];
    });
}

void moveWidgetWindow(uintptr_t win, int x, int y) {
    run_on_main(^{
        NSWindow *w = (__bridge NSWindow *)(void *)win;
        NSScreen *screen = [w screen] ?: [NSScreen mainScreen];
        CGFloat screenH  = screen.frame.size.height;
        CGFloat winH     = w.frame.size.height;
        [w setFrameOrigin:NSMakePoint((CGFloat)x,
                                      screenH - (CGFloat)y - winH)];
    });
}

void getWidgetWindowPos(uintptr_t win, int *outX, int *outY) {
    *outX = 0; *outY = 0;
    NSWindow *w = (__bridge NSWindow *)(void *)win;
    NSScreen *screen = [w screen] ?: [NSScreen mainScreen];
    CGFloat screenH = screen.frame.size.height;
    NSRect f = w.frame;
    *outX = (int)f.origin.x;
    *outY = (int)(screenH - f.origin.y - f.size.height);
}

void setWidgetWindowOpacity(uintptr_t win, double alpha) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [(__bridge NSWindow *)(void *)win setAlphaValue:(CGFloat)alpha];
    });
}

void setWidgetWindowLevel(uintptr_t win, int level) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSWindowLevel wl = (level == 1) ? NSFloatingWindowLevel
                                        : kCGDesktopWindowLevel + 1;
        [(__bridge NSWindow *)(void *)win setLevel:wl];
    });
}

int getScreenCount(void) {
    return (int)[[NSScreen screens] count];
}

void getScreenBounds(int index, int *outX, int *outY, int *outW, int *outH) {
    NSArray<NSScreen *> *screens = [NSScreen screens];
    if (index < 0 || index >= (int)screens.count) index = 0;
    NSScreen *screen = screens[index];
    NSRect visible = screen.visibleFrame;
    NSRect full    = screen.frame;
    *outX = (int)visible.origin.x;
    *outY = (int)(full.size.height - visible.origin.y - visible.size.height);
    *outW = (int)visible.size.width;
    *outH = (int)visible.size.height;
}

void getMainScreenSize(int *w, int *h) {
    NSRect f = [NSScreen mainScreen].frame;
    *w = (int)f.size.width;
    *h = (int)f.size.height;
}

// ── Card container ────────────────────────────────────────────────────────────

uintptr_t createCardContainer(uintptr_t win) {
    __block uintptr_t handle = 0;
    run_on_main(^{
        NSWindow *w = (__bridge NSWindow *)(void *)win;
        WWDragView *container = [[WWDragView alloc]
            initWithFrame:w.contentView.bounds];
        container.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
        container.onDragEnd = ^{
            // Position is polled by Go via getWidgetWindowPos every 500 ms.
        };
        [w setContentView:container];
        CFRetain((__bridge CFTypeRef)container);
        handle = (uintptr_t)(__bridge void *)container;
    });
    return handle;
}
