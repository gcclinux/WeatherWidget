// tray.m — NSStatusItem (system tray) for WeatherWidget on macOS.
//
// Creates a small weather-icon status item in the menu bar with two items:
//   • Settings  — opens the settings panel
//   • Quit      — terminates the application
//
// Go callbacks are stored in global function-pointer variables so that the
// ObjC action target can invoke them without capturing Go closures (which
// CGo does not support in blocks).  traySettingsCB / trayQuitCB are the
// C-callable thunks declared in callbacks.h.

#import <Cocoa/Cocoa.h>
#import "callbacks.h"

// invokeCallback is exported from Go (tray.go). index 0 = Settings, 1 = Quit.
// The menu actions call it directly, which pushes the corresponding Go closure
// onto the main-thread work queue.
extern void invokeCallback(int index);

// ── Thunks referenced from callbacks.h ───────────────────────────────────────
// Kept for API compatibility with callbacks.h / bridge.go, but they now route
// straight to the Go callback registry instead of a C function-pointer.

void traySettingsCB(void) { invokeCallback(0); }
void trayQuitCB(void)     { invokeCallback(1); }


// ── WWTrayTarget ─────────────────────────────────────────────────────────────
// ObjC target that forwards menu actions to the Go callback registry.

@interface WWTrayTarget : NSObject
- (void)settingsAction:(id)sender;
- (void)quitAction:(id)sender;
@end

@implementation WWTrayTarget
- (void)settingsAction:(id)sender { invokeCallback(0); }
- (void)quitAction:(id)sender     { invokeCallback(1); }
@end


// ── Helpers ──────────────────────────────────────────────────────────────────

// buildMenu creates the NSMenu from the given labels and wires targets.
static NSMenu *buildMenu(NSString *settingsLabel, NSString *quitLabel,
                          WWTrayTarget *target) {
    // Guard against nil titles — NSMenuItem throws on a nil title.
    if (!settingsLabel) settingsLabel = @"Settings";
    if (!quitLabel)     quitLabel     = @"Quit";

    NSMenu *menu = [[NSMenu alloc] init];
    [menu setAutoenablesItems:NO];

    NSMenuItem *settingsItem = [[NSMenuItem alloc]
        initWithTitle:settingsLabel
        action:@selector(settingsAction:)
        keyEquivalent:@","];
    settingsItem.target = target;
    [menu addItem:settingsItem];

    [menu addItem:[NSMenuItem separatorItem]];

    NSMenuItem *quitItem = [[NSMenuItem alloc]
        initWithTitle:quitLabel
        action:@selector(quitAction:)
        keyEquivalent:@"q"];
    quitItem.target = target;
    [menu addItem:quitItem];

    return menu;
}

// statusItemIcon returns a small NSImage for the tray: tries to load
// weather-clear.png from the app bundle Resources, falls back to a plain
// cloud SF Symbol, or a 16×16 blue square as a last resort.
static NSImage *statusItemIcon(void) {
    // Try bundle resource
    NSImage *img = [NSImage imageNamed:@"weather-clear"];
    if (img) {
        img.size = NSMakeSize(16, 16);
        img.template = YES;
        return img;
    }
    // SF Symbol (macOS 11+)
    if (@available(macOS 11.0, *)) {
        NSImage *sf = [NSImage imageWithSystemSymbolName:@"cloud.sun.fill"
                                 accessibilityDescription:@"Weather Widget"];
        if (sf) {
            sf.size = NSMakeSize(16, 16);
            return sf;
        }
    }
    // Solid coloured square fallback
    NSImage *fallback = [[NSImage alloc] initWithSize:NSMakeSize(16, 16)];
    [fallback lockFocus];
    [[NSColor colorWithRed:0.20 green:0.60 blue:1.0 alpha:1.0] set];
    NSRectFill(NSMakeRect(0, 0, 16, 16));
    [fallback unlockFocus];
    fallback.template = NO;
    return fallback;
}


// run_on_main — run block on main thread without deadlocking.
#define run_on_main(blk) \
    do { \
        if ([NSThread isMainThread]) { blk(); } \
        else { dispatch_sync(dispatch_get_main_queue(), blk); } \
    } while(0)

// ── Public C API ──────────────────────────────────────────────────────────────

uintptr_t createStatusItem(
    const char *settingsLabel,
    const char *quitLabel,
    void (*onSettings)(void),
    void (*onQuit)(void)
) {
    (void)onSettings; (void)onQuit; // routing now goes through invokeCallback
    __block uintptr_t handle = 0;
    run_on_main(^{
        NSStatusBar *bar = [NSStatusBar systemStatusBar];
        NSStatusItem *item = [bar statusItemWithLength:NSSquareStatusItemLength];

        // Tray button image
        item.button.image = statusItemIcon();
        item.button.toolTip = @"WeatherWidget";

        WWTrayTarget *target = [[WWTrayTarget alloc] init];
        CFRetain((__bridge CFTypeRef)target); // keep alive

        NSString *settLbl = [NSString stringWithUTF8String:settingsLabel ?: "Settings"];
        NSString *quitLbl = [NSString stringWithUTF8String:quitLabel ?: "Quit"];
        item.menu = buildMenu(settLbl, quitLbl, target);

        // Retain item so it survives ARC scope
        CFRetain((__bridge CFTypeRef)item);
        handle = (uintptr_t)(__bridge void *)item;
    });
    return handle;
}

void updateStatusItemMenu(
    uintptr_t itemHandle,
    const char *settingsLabel,
    const char *quitLabel,
    void (*onSettings)(void),
    void (*onQuit)(void)
) {
    (void)onSettings; (void)onQuit; // routing now goes through invokeCallback
    NSString *settLbl0 = [NSString stringWithUTF8String:settingsLabel ?: "Settings"];
    NSString *quitLbl0 = [NSString stringWithUTF8String:quitLabel ?: "Quit"];
    dispatch_async(dispatch_get_main_queue(), ^{
        NSStatusItem *item = (__bridge NSStatusItem *)(void *)itemHandle;
        NSString *settLbl  = settLbl0;
        NSString *quitLbl  = quitLbl0;

        // Retrieve the existing target from the first menu item
        WWTrayTarget *target = nil;
        if (item.menu.itemArray.count > 0) {
            target = (WWTrayTarget *)item.menu.itemArray[0].target;
        }
        if (!target) {
            target = [[WWTrayTarget alloc] init];
            CFRetain((__bridge CFTypeRef)target);
        }
        item.menu = buildMenu(settLbl, quitLbl, target);
    });
}
