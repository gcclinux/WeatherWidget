//go:build darwin

package uidarwin

// runloop.go — Cocoa main run loop for the native Darwin UI.
//
// The correct pattern for a Go+Cocoa hybrid:
//
//   1. Call [NSApp run] on the OS main thread — this runs the real CFRunLoop,
//      which correctly services NSStatusItem menus, dispatch_async blocks,
//      window events, and all AppKit machinery.
//
//   2. Install a dispatch_source_t timer (10 ms) on the main queue that drains
//      the Go mainQueue channel. This fires inside [NSApp run]'s CFRunLoop,
//      so Go closures execute on the correct Cocoa main thread.
//
// The previous approach (manually calling nextEventMatchingMask with
// distantPast) does NOT process dispatch_async blocks reliably — those require
// the CFRunLoop to be running properly, not just polled.

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#import <stdlib.h>
#import <unistd.h>

// goWorkCallback is a Go function called by the dispatch_source timer to drain
// the Go mainQueue. Declared here; defined in tray.go via //export.
extern void goWorkCallback(void);

// goAppReady is called from applicationDidFinishLaunching so Go can show the
// window and tray at the correct point in the Cocoa launch lifecycle.
extern void goAppReady(void);

// goCreateTray is called synchronously before [app run] to create the status
// item while still on the main thread during launch.
extern void goCreateTray(void);

// WWAppDelegate signals Go once AppKit has finished launching.
@interface WWAppDelegate : NSObject <NSApplicationDelegate>
@end
@implementation WWAppDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)note {
    goAppReady();
}
@end

// initAndRun mirrors the proven caseymrm/menuet startup sequence EXACTLY:
// autorelease pool → sharedApplication → delegate → Accessory policy →
// create the status item synchronously → install the Go-work timer → [app run].
//
// Key lessons that cost many iterations:
//   • Set Accessory ONCE — do not toggle Regular↔Accessory (confuses RunningBoard).
//   • Create the status item synchronously BEFORE [app run] — it's what keeps
//     an accessory app alive past RunningBoard's ~6s launch assertion.
//   • Do NOT call finishLaunching manually, disableAutomaticTermination, or
//     pump events manually — [app run] handles the whole lifecycle.
static void initAndRun(void) {
    [NSAutoreleasePool new];

    NSApplication *app = NSApplication.sharedApplication;

    static WWAppDelegate *delegate = nil;
    delegate = [[WWAppDelegate alloc] init];
    [app setDelegate:delegate];

    [app setActivationPolicy:NSApplicationActivationPolicyAccessory];

    // Create the status item synchronously (in Go via goCreateTray → setupTray)
    // before the run loop. This keeps the accessory app alive past RunningBoard's
    // launch assertion.
    goCreateTray();

    // Repeating main-queue timer to drain Go work (weather updates, clock ticks,
    // window show). Serviced by the CFRunLoop that [app run] drives.
    dispatch_source_t timer = dispatch_source_create(
        DISPATCH_SOURCE_TYPE_TIMER, 0, 0, dispatch_get_main_queue());
    dispatch_source_set_timer(timer,
        dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_MSEC),
        10 * NSEC_PER_MSEC, 1 * NSEC_PER_MSEC);
    dispatch_source_set_event_handler(timer, ^{ goWorkCallback(); });
    dispatch_resume(timer);

    [app run];
}
*/
import "C"

// RunMainLoop initialises NSApplication and enters [NSApp run].
// Must be called on the OS main thread (the goroutine that holds
// runtime.LockOSThread). Never returns.
func RunMainLoop() {
	C.initAndRun()
}
