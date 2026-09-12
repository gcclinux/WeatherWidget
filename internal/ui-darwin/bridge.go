//go:build darwin

// Package uidarwin implements a fully native AppKit/Objective-C UI for
// WeatherWidget on macOS. It replaces the Fyne widget window on Darwin while
// sharing all business-logic packages (config, weather, scheduler, i18n, etc.)
// with the rest of the application.
//
// Architecture
//
//   bridge.go       — CGo Go↔ObjC bridge: window/panel/tray C API wrappers
//   window.m        — Objective-C: NSWindow, NSVisualEffectView, drag support
//   panel.m         — Objective-C: per-city card NSView, CALayer drawing
//   tray.m          — Objective-C: NSStatusItem menu
//   manager.go      — Go orchestrator: lifecycle, scheduler, settings
//   settings.go     — Go: settings window (Fyne dialog reused on Darwin)
//   tray.go         — Go: tray setup + menu callbacks
//   autostart.go    — Go: LaunchAgent plist
//   uidispatch.go   — Go: main-thread work queue
package uidarwin

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore

#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <stdlib.h>
#import "callbacks.h"

// ── Forward declarations ────────────────────────────────────────────────────

// createWidgetWindow creates the transparent, borderless desktop-overlay
// NSWindow and returns its handle as an opaque uintptr.
extern uintptr_t createWidgetWindow(void);

// destroyWidgetWindow hides and releases the widget window.
extern void destroyWidgetWindow(uintptr_t win);

// showWidgetWindow makes the widget window visible.
extern void showWidgetWindow(uintptr_t win);

// moveWidgetWindow repositions the window to (x, y) in top-left screen coords.
extern void moveWidgetWindow(uintptr_t win, int x, int y);

// getWidgetWindowPos returns the current top-left position of the window.
extern void getWidgetWindowPos(uintptr_t win, int *outX, int *outY);

// setWidgetWindowOpacity sets the overall window alpha (0.0–1.0).
extern void setWidgetWindowOpacity(uintptr_t win, double alpha);

// setWidgetWindowLevel sets the NSWindowLevel: 0=normal/desktop, 1=floating.
extern void setWidgetWindowLevel(uintptr_t win, int level);

// getScreenCount returns the number of NSScreens.
extern int getScreenCount(void);

// getScreenBounds fills outX/Y/W/H with the visible frame (excluding Dock &
// menu bar) of the screen at index, using top-left origin.
extern void getScreenBounds(int index, int *outX, int *outY, int *outW, int *outH);

// getMainScreenSize returns the dimensions of [NSScreen mainScreen].
extern void getMainScreenSize(int *w, int *h);

// ── Panel C API ─────────────────────────────────────────────────────────────

// createCardContainer creates a transparent NSView that will hold all city
// cards as subviews. Returns an opaque handle.
extern uintptr_t createCardContainer(uintptr_t win);

// addCityCard appends one new city-card NSView to the container for the given
// city name + region and returns its handle.  The card is not yet populated
// with data.
extern uintptr_t addCityCard(uintptr_t container, const char *city, const char *region);

// removeAllCards removes and releases all city-card subviews from container.
extern void removeAllCards(uintptr_t container);

// updateCardData updates the display labels in a card with fresh weather data.
//   iconPath    — absolute path to the icon PNG (may be "" to keep current)
//   city        — "City, Region"
//   timeStr     — "HH:MM:SS"
//   dateStr     — e.g. "Monday, Jan 02"
//   tempStr     — e.g. "18°C"
//   descStr     — e.g. "Partly Cloudy"
//   humidStr    — e.g. "74%"
//   windStr     — e.g. "14.0 km/h"
//   windGustStr — e.g. "20.0 km/h"
//   dewPtStr    — e.g. "11.2°C"
//   pressStr    — e.g. "1013 hPa"
//   uvStr       — e.g. "3.0"
//   isNight     — 1 for night background, 0 for day
//   opacity     — card background opacity 0.0–1.0
extern void updateCardData(
    uintptr_t card,
    const char *iconPath,
    const char *city,
    const char *timeStr,
    const char *dateStr,
    const char *tempStr,
    const char *descStr,
    const char *humidStr,
    const char *windStr,
    const char *windGustStr,
    const char *dewPtStr,
    const char *pressStr,
    const char *uvStr,
    int isNight,
    double opacity
);

// updateCardAQI sets the AQI label (pass "" to hide).
extern void updateCardAQI(uintptr_t card, const char *label);

// updateCardPollutant updates one of the eight individual pollutant labels.
// slot: 0=CO 1=NO 2=NO2 3=O3 4=SO2 5=NH3 6=PM2.5 7=PM10.  "" to hide.
extern void updateCardPollutant(uintptr_t card, int slot, const char *iconPath, const char *value);

// setCardFieldVisibility shows/hides individual card sub-elements.
// Field bits — use the DisplayField* constants below.
extern void setCardFieldVisibility(uintptr_t card, unsigned int fieldMask);

// setContainerLayout re-stacks cards vertically (enhanced) or side-by-side
// (simple).  mode: 0=enhanced/vertical, 1=simple/horizontal.
extern void setContainerLayout(uintptr_t container, int mode, int cardCount);

// resizeWindowToContainer resizes the window to fit the container's content.
extern void resizeWindowToContainer(uintptr_t win, uintptr_t container, int mode, int cardCount);

// showCardError shows/hides the error overlay on a card (stale=1 shows "stale data").
extern void showCardError(uintptr_t card, int visible, int stale);

// setCardFontSizes adjusts font sizes on a card (px).  Pass 0 to use defaults.
extern void setCardFontSizes(uintptr_t card, int cityTimeSize, int tempSize, int condSize);

// ── Tray C API ──────────────────────────────────────────────────────────────

// createStatusItem creates an NSStatusItem and returns its handle.
// settingsLabel / quitLabel are the menu item titles.
// onSettings / onQuit are C function pointers invoked on the main thread
// (defined in tray.m as traySettingsCB / trayQuitCB).
extern uintptr_t createStatusItem(
    const char *settingsLabel,
    const char *quitLabel,
    void (*onSettings)(void),
    void (*onQuit)(void)
);

// updateStatusItemMenu rebuilds the menu with fresh translated strings.
extern void updateStatusItemMenu(
    uintptr_t item,
    const char *settingsLabel,
    const char *quitLabel,
    void (*onSettings)(void),
    void (*onQuit)(void)
);

// ── RunOnMain ───────────────────────────────────────────────────────────────

// runOnMain dispatches a Go callback to the Cocoa main thread via
// dispatch_async(dispatch_get_main_queue(), …). Safe to call from any goroutine.
// The fn pointer must remain valid until after the block executes; the bridge
// arranges this by retaining a copy in the block capture.
static inline void runOnMain(void (*fn)(void)) {
    // Capture fn by value so the block owns a copy.
    dispatch_async(dispatch_get_main_queue(), ^{ fn(); });
}

// ── displayFieldMask constants (must match setCardFieldVisibility) ───────────
#define DFCity     (1u << 0)
#define DFIcon     (1u << 1)
#define DFTemp     (1u << 2)
#define DFDesc     (1u << 3)
#define DFHumidity (1u << 4)
#define DFWind     (1u << 5)
#define DFTime     (1u << 6)
#define DFDate     (1u << 7)
#define DFWindGust (1u << 8)
#define DFDewPoint (1u << 9)
#define DFPressure (1u << 10)
#define DFUVIndex  (1u << 11)
*/
import "C"
import (
	"unsafe"
)

// ── Window ──────────────────────────────────────────────────────────────────

func nativeCreateWidgetWindow() uintptr {
	return uintptr(C.createWidgetWindow())
}

func nativeDestroyWidgetWindow(win uintptr) {
	C.destroyWidgetWindow(C.uintptr_t(win))
}

func nativeShowWidgetWindow(win uintptr) {
	C.showWidgetWindow(C.uintptr_t(win))
}

func nativeMoveWidgetWindow(win uintptr, x, y int) {
	C.moveWidgetWindow(C.uintptr_t(win), C.int(x), C.int(y))
}

func nativeGetWidgetWindowPos(win uintptr) (int, int) {
	var x, y C.int
	C.getWidgetWindowPos(C.uintptr_t(win), &x, &y)
	return int(x), int(y)
}

func nativeSetWidgetWindowOpacity(win uintptr, alpha float64) {
	C.setWidgetWindowOpacity(C.uintptr_t(win), C.double(alpha))
}

func nativeSetWidgetWindowLevel(win uintptr, level int) {
	C.setWidgetWindowLevel(C.uintptr_t(win), C.int(level))
}

func nativeGetScreenCount() int {
	return int(C.getScreenCount())
}

// nativeGetScreenBounds returns visible screen bounds (excluding Dock/menubar)
// for screen at index, in top-left-origin coordinates.
func nativeGetScreenBounds(index int) (x, y, w, h int) {
	var cx, cy, cw, ch C.int
	C.getScreenBounds(C.int(index), &cx, &cy, &cw, &ch)
	return int(cx), int(cy), int(cw), int(ch)
}

func nativeGetMainScreenSize() (int, int) {
	var w, h C.int
	C.getMainScreenSize(&w, &h)
	return int(w), int(h)
}

// ── Container & cards ───────────────────────────────────────────────────────

func nativeCreateCardContainer(win uintptr) uintptr {
	return uintptr(C.createCardContainer(C.uintptr_t(win)))
}

func nativeAddCityCard(container uintptr, city, region string) uintptr {
	cs := C.CString(city)
	cr := C.CString(region)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(cr))
	return uintptr(C.addCityCard(C.uintptr_t(container), cs, cr))
}

func nativeRemoveAllCards(container uintptr) {
	C.removeAllCards(C.uintptr_t(container))
}

func nativeUpdateCardData(card uintptr,
	iconPath, city, timeStr, dateStr, tempStr, descStr,
	humidStr, windStr, windGustStr, dewPtStr, pressStr, uvStr string,
	isNight bool, opacity float64,
) {
	mkC := func(s string) *C.char { return C.CString(s) }
	freeC := func(s *C.char) { C.free(unsafe.Pointer(s)) }

	cIcon := mkC(iconPath); defer freeC(cIcon)
	cCity := mkC(city); defer freeC(cCity)
	cTime := mkC(timeStr); defer freeC(cTime)
	cDate := mkC(dateStr); defer freeC(cDate)
	cTemp := mkC(tempStr); defer freeC(cTemp)
	cDesc := mkC(descStr); defer freeC(cDesc)
	cHumid := mkC(humidStr); defer freeC(cHumid)
	cWind := mkC(windStr); defer freeC(cWind)
	cGust := mkC(windGustStr); defer freeC(cGust)
	cDew := mkC(dewPtStr); defer freeC(cDew)
	cPress := mkC(pressStr); defer freeC(cPress)
	cUV := mkC(uvStr); defer freeC(cUV)

	night := C.int(0)
	if isNight {
		night = 1
	}
	C.updateCardData(C.uintptr_t(card),
		cIcon, cCity, cTime, cDate, cTemp, cDesc,
		cHumid, cWind, cGust, cDew, cPress, cUV,
		night, C.double(opacity))
}

func nativeUpdateCardAQI(card uintptr, label string) {
	cs := C.CString(label)
	defer C.free(unsafe.Pointer(cs))
	C.updateCardAQI(C.uintptr_t(card), cs)
}

func nativeUpdateCardPollutant(card uintptr, slot int, iconPath, value string) {
	ci := C.CString(iconPath)
	cv := C.CString(value)
	defer C.free(unsafe.Pointer(ci))
	defer C.free(unsafe.Pointer(cv))
	C.updateCardPollutant(C.uintptr_t(card), C.int(slot), ci, cv)
}

// DisplayField bitmask constants — must match the #defines in the C preamble.
const (
	DFCity     = uint32(1 << 0)
	DFIcon     = uint32(1 << 1)
	DFTemp     = uint32(1 << 2)
	DFDesc     = uint32(1 << 3)
	DFHumidity = uint32(1 << 4)
	DFWind     = uint32(1 << 5)
	DFTime     = uint32(1 << 6)
	DFDate     = uint32(1 << 7)
	DFWindGust = uint32(1 << 8)
	DFDewPoint = uint32(1 << 9)
	DFPressure = uint32(1 << 10)
	DFUVIndex  = uint32(1 << 11)
)

func nativeSetCardFieldVisibility(card uintptr, mask uint32) {
	C.setCardFieldVisibility(C.uintptr_t(card), C.uint(mask))
}

func nativeSetContainerLayout(container uintptr, simpleMode bool, cardCount int) {
	mode := C.int(0)
	if simpleMode {
		mode = 1
	}
	C.setContainerLayout(C.uintptr_t(container), mode, C.int(cardCount))
}

func nativeResizeWindowToContainer(win, container uintptr, simpleMode bool, cardCount int) {
	mode := C.int(0)
	if simpleMode {
		mode = 1
	}
	C.resizeWindowToContainer(C.uintptr_t(win), C.uintptr_t(container), mode, C.int(cardCount))
}

func nativeShowCardError(card uintptr, visible, stale bool) {
	v := C.int(0)
	if visible {
		v = 1
	}
	s := C.int(0)
	if stale {
		s = 1
	}
	C.showCardError(C.uintptr_t(card), v, s)
}

func nativeSetCardFontSizes(card uintptr, cityTime, temp, cond int) {
	C.setCardFontSizes(C.uintptr_t(card), C.int(cityTime), C.int(temp), C.int(cond))
}

// ── Tray ─────────────────────────────────────────────────────────────────────

// nativeCreateStatusItem creates the NSStatusItem and wires Go callbacks.
// The two function pointers (traySettingsCB / trayQuitCB) are defined in
// tray.m; they look up the registered Go callbacks via the global table in
// tray.go (registerCallback / invokeCallback).
func nativeCreateStatusItem(settingsLabel, quitLabel string) uintptr {
	cs := C.CString(settingsLabel)
	cq := C.CString(quitLabel)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(cq))
	return uintptr(C.createStatusItem(
		cs, cq,
		(*[0]byte)(C.traySettingsCB),
		(*[0]byte)(C.trayQuitCB),
	))
}

// nativeUpdateStatusItemMenu rebuilds the tray menu with fresh labels.
func nativeUpdateStatusItemMenu(item uintptr, settingsLabel, quitLabel string) {
	cs := C.CString(settingsLabel)
	cq := C.CString(quitLabel)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(cq))
	C.updateStatusItemMenu(
		C.uintptr_t(item), cs, cq,
		(*[0]byte)(C.traySettingsCB),
		(*[0]byte)(C.trayQuitCB),
	)
}

// ── Main-thread dispatch ─────────────────────────────────────────────────────

// RunOnMain schedules fn to run on the Cocoa main thread asynchronously.
// Safe to call from any goroutine.
func RunOnMain(fn func()) {
	mainQueue <- fn
}
