// settings.m — Native NSPanel settings window for WeatherWidget on macOS.
//
// All state is passed as JSON-encoded config.Config from Go, and the updated
// JSON is passed back via the settingsSaveCB exported Go function.
//
// Tabs implemented (feature-parity with the GTK/Fyne UI):
//   Provider     · provider, API key, refresh interval, notes
//   Locations    · saved-city list (reorder / remove) + add-city form
//   Widget       · display-field toggles, pollution toggles, temp + wind units
//   Language     · 13-language selector grid
//   Appearance   · view mode, opacity, autostart, icon theme, position, fonts
//   About        · app info + links
//
// The widget itself is rendered in pure Cocoa (panel.m); this file only builds
// the settings surface and marshals config in/out.

#import <Cocoa/Cocoa.h>

// ── Go callback declarations ──────────────────────────────────────────────────
// These are //export-ed from settings_cb.go.
extern void settingsSaveCB(const char *newCfgJSON);
extern void settingsClosedCB(void);
extern int  settingsAutostartGetCB(void);
extern int  settingsAutostartSetCB(int enabled);

// ── Supported languages ───────────────────────────────────────────────────────
// The language list (code / native name / English name / flag path) is now
// supplied from Go via openSettingsNative's langsJSON argument and rendered as
// flag cards in the Language tab — see WWLangCard and -buildLanguageTab.

// jbool returns a boolean NSNumber (__NSCFBoolean) that NSJSONSerialization
// encodes as JSON true/false. Use this for every boolean written into the
// config dictionary — @(BOOL expr) boxes as a char-backed NSNumber that
// serializes as 0/1, which Go's json.Unmarshal rejects for bool fields.
static inline NSNumber *jbool(BOOL v) { return v ? @YES : @NO; }

// ── WWLangCard ────────────────────────────────────────────────────────────────
// A clickable language card matching the GTK language grid: a flag image on the
// left, the native name in bold, and the English name beneath it. Highlights
// when selected. Reports clicks to a target/action.
@interface WWLangCard : NSView
@property (nonatomic, copy)   NSString   *code;
@property (nonatomic, assign) BOOL        selected;
@property (nonatomic, strong) NSTextField *checkLbl; // ✓ shown when selected
@property (nonatomic, assign) id          target;    // weak-ish (controller outlives cards)
@property (nonatomic, assign) SEL         action;
@end

@implementation WWLangCard

- (instancetype)initWithCode:(NSString *)code
                      native:(NSString *)native
                     english:(NSString *)english
                    flagPath:(NSString *)flagPath {
    self = [super initWithFrame:NSZeroRect];
    if (!self) return nil;
    _code = [code copy];
    self.wantsLayer = YES;
    self.layer.cornerRadius = 10;
    self.layer.borderWidth = 1.0;

    // Flag image.
    NSImageView *flag = [[NSImageView alloc] initWithFrame:NSMakeRect(12, 14, 44, 30)];
    flag.imageScaling = NSImageScaleProportionallyUpOrDown;
    if (flagPath.length > 0) {
        NSImage *img = [[[NSImage alloc] initWithContentsOfFile:flagPath] autorelease];
        if (img) flag.image = img;
    }
    [self addSubview:flag];

    // Native name (bold, white).
    NSTextField *nativeLbl = [NSTextField labelWithString:native ?: @""];
    nativeLbl.font = [NSFont boldSystemFontOfSize:14];
    nativeLbl.textColor = [NSColor whiteColor];
    nativeLbl.frame = NSMakeRect(68, 30, 170, 20);
    [self addSubview:nativeLbl];

    // English name (secondary).
    NSTextField *engLbl = [NSTextField labelWithString:english ?: @""];
    engLbl.font = [NSFont systemFontOfSize:11];
    engLbl.textColor = [NSColor colorWithWhite:0.72 alpha:1.0];
    engLbl.frame = NSMakeRect(68, 12, 170, 16);
    [self addSubview:engLbl];

    // Selection check mark (top-right).
    _checkLbl = [[NSTextField labelWithString:@"✓"] retain];
    _checkLbl.font = [NSFont boldSystemFontOfSize:14];
    _checkLbl.textColor = [NSColor colorWithRed:0.22 green:0.60 blue:1.0 alpha:1.0];
    _checkLbl.frame = NSMakeRect(238, 30, 20, 20);
    _checkLbl.hidden = YES;
    [self addSubview:_checkLbl];

    [self applySelectionStyle];
    return self;
}

- (void)dealloc {
    [_code release];
    [_checkLbl release];
    [super dealloc];
}

- (void)setSelected:(BOOL)selected {
    _selected = selected;
    [self applySelectionStyle];
}

- (void)applySelectionStyle {
    if (_selected) {
        self.layer.backgroundColor = [NSColor colorWithRed:0.15 green:0.32 blue:0.55 alpha:0.55].CGColor;
        self.layer.borderColor = [NSColor colorWithRed:0.22 green:0.60 blue:1.0 alpha:1.0].CGColor;
        self.checkLbl.hidden = NO;
    } else {
        self.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.06].CGColor;
        self.layer.borderColor = [NSColor colorWithWhite:1.0 alpha:0.14].CGColor;
        self.checkLbl.hidden = YES;
    }
}

- (void)mouseDown:(NSEvent *)event {
    if (self.target && self.action) {
        // -performSelector: with the card as argument; controller updates state.
        IMP imp = [self.target methodForSelector:self.action];
        void (*fn)(id, SEL, id) = (void (*)(id, SEL, id))imp;
        fn(self.target, self.action, self);
    }
}

@end

// ── WWSettingsController ──────────────────────────────────────────────────────

@interface WWSettingsController : NSWindowController <NSWindowDelegate>
@property (nonatomic, strong) NSString  *cfgJSON;   // current config JSON
@property (nonatomic, strong) NSMutableDictionary *cfg; // mutable parsed config
@property (nonatomic, strong) NSDictionary *strings;    // localized UI strings (key → text)
@property (nonatomic, strong) NSArray *languages;       // [{code,native,english,flag}] for the Language tab
@property (nonatomic, strong) NSMutableArray *langCards; // WWLangCard views (for selection redraw)
@property (nonatomic, strong) NSString *selectedLangCode; // currently selected locale code
@property (nonatomic, strong) NSTabView *tabView;

// Provider tab
@property (nonatomic, strong) NSPopUpButton *providerPop;
@property (nonatomic, strong) NSTextField   *apiKeyField;
@property (nonatomic, strong) NSTextField   *intervalField;

// Locations tab
@property (nonatomic, strong) NSMutableArray *cities;      // array of NSMutableDictionary
@property (nonatomic, strong) NSStackView    *cityListStack;
@property (nonatomic, strong) NSTextField    *addNameField;
@property (nonatomic, strong) NSTextField    *addRegionField;
@property (nonatomic, strong) NSTextField    *addLatField;
@property (nonatomic, strong) NSTextField    *addLonField;
@property (nonatomic, strong) NSTextField    *addTZField;

// Widget tab — display fields
@property (nonatomic, strong) NSButton *dfCity;
@property (nonatomic, strong) NSButton *dfIcon;
@property (nonatomic, strong) NSButton *dfTemp;
@property (nonatomic, strong) NSButton *dfDesc;
@property (nonatomic, strong) NSButton *dfHumidity;
@property (nonatomic, strong) NSButton *dfWind;
@property (nonatomic, strong) NSButton *dfTime;
@property (nonatomic, strong) NSButton *dfDate;
@property (nonatomic, strong) NSButton *dfWindGust;
@property (nonatomic, strong) NSButton *dfDewPoint;
@property (nonatomic, strong) NSButton *dfPressure;
@property (nonatomic, strong) NSButton *dfUVIndex;
// Widget tab — pollution fields
@property (nonatomic, strong) NSButton *pfAQI;
@property (nonatomic, strong) NSButton *pfCO;
@property (nonatomic, strong) NSButton *pfNO;
@property (nonatomic, strong) NSButton *pfNO2;
@property (nonatomic, strong) NSButton *pfO3;
@property (nonatomic, strong) NSButton *pfSO2;
@property (nonatomic, strong) NSButton *pfNH3;
@property (nonatomic, strong) NSButton *pfPM25;
@property (nonatomic, strong) NSButton *pfPM10;
// Widget tab — units
@property (nonatomic, strong) NSButton *tempCelsius;
@property (nonatomic, strong) NSButton *tempFahrenheit;
@property (nonatomic, strong) NSButton *tempKelvin;
@property (nonatomic, strong) NSButton *windKmh;
@property (nonatomic, strong) NSButton *windMph;
@property (nonatomic, strong) NSButton *windKnots;

// Language tab
@property (nonatomic, assign) NSInteger selectedLangIndex;

// Appearance tab
@property (nonatomic, strong) NSButton      *viewEnhanced;
@property (nonatomic, strong) NSButton      *viewSimple;
@property (nonatomic, strong) NSSlider      *opacitySlider;
@property (nonatomic, strong) NSTextField   *opacityLabel;
@property (nonatomic, strong) NSButton      *autostartCheck;
@property (nonatomic, strong) NSButton      *iconThemeNew;
@property (nonatomic, strong) NSButton      *iconThemeOriginal;
@property (nonatomic, strong) NSTextField   *posXField;
@property (nonatomic, strong) NSTextField   *posYField;
// Font sizes
@property (nonatomic, assign) NSInteger fontCityTime;
@property (nonatomic, assign) NSInteger fontTempIcon;
@property (nonatomic, assign) NSInteger fontConditions;
@property (nonatomic, strong) NSTextField *fontCityTimeLabel;
@property (nonatomic, strong) NSTextField *fontTempIconLabel;
@property (nonatomic, strong) NSTextField *fontConditionsLabel;

+ (instancetype)sharedController;
- (void)loadFromJSON:(NSString *)json;
- (NSString *)buildUpdatedJSON;
@end

// Singleton
static WWSettingsController *g_settingsController = nil;

@implementation WWSettingsController

+ (instancetype)sharedController {
    return g_settingsController;
}

- (instancetype)initWithJSON:(NSString *)json strings:(NSString *)stringsJSON languages:(NSString *)langsJSON {
    NSRect frame = NSMakeRect(0, 0, 620, 720);
    NSWindowStyleMask style =
        NSWindowStyleMaskTitled |
        NSWindowStyleMaskClosable |
        NSWindowStyleMaskMiniaturizable |
        NSWindowStyleMaskResizable;

    NSPanel *panel = [[NSPanel alloc]
        initWithContentRect:frame
        styleMask:style
        backing:NSBackingStoreBuffered
        defer:NO];
    panel.title = @"WeatherWidget Settings";
    panel.releasedWhenClosed = NO;
    panel.minSize = NSMakeSize(560, 560);

    self = [super initWithWindow:panel];
    if (!self) return nil;

    panel.delegate = self;
    // MANUAL reference counting (no ARC): assigning an autoreleased object
    // DIRECTLY to a `strong` ivar does NOT retain it — the synthesized setter
    // is what retains. So `_cities = [NSMutableArray array]` left a dangling
    // pointer once the autorelease pool drained, and later reads (buildUpdatedJSON)
    // saw reused heap memory — which is why the saved `cities` came out as a
    // garbage number array and the app crashed intermittently. Assign through
    // the setters (self.x = ...) so the retain runs. `parsed` is a +1 owned
    // mutableCopy, so use the ivar directly for it and balance in -dealloc.
    self.cfgJSON = json;
    self.cities = [NSMutableArray array];
    _selectedLangIndex = 0;
    _fontCityTime = 14;
    _fontTempIcon = 32;
    _fontConditions = 10;

    // Parse config up-front so tab builders can read initial values.
    NSData *data = [json dataUsingEncoding:NSUTF8StringEncoding];
    NSMutableDictionary *parsed = [[NSJSONSerialization JSONObjectWithData:data
        options:NSJSONReadingMutableContainers error:nil] mutableCopy];
    self.cfg = parsed ?: [NSMutableDictionary dictionary];
    [parsed release]; // setter retained it (or the fallback); drop our +1 from mutableCopy

    // Parse the localized-strings table (key → text). Used by -L: below so
    // every label/placeholder follows the user's chosen language.
    NSDictionary *parsedStrings = nil;
    if (stringsJSON.length > 0) {
        NSData *sdata = [stringsJSON dataUsingEncoding:NSUTF8StringEncoding];
        parsedStrings = [NSJSONSerialization JSONObjectWithData:sdata options:0 error:nil];
    }
    self.strings = parsedStrings ?: @{};

    // Parse the language list (code/native/english/flag) for the Language tab.
    NSArray *parsedLangs = nil;
    if (langsJSON.length > 0) {
        NSData *ldata = [langsJSON dataUsingEncoding:NSUTF8StringEncoding];
        parsedLangs = [NSJSONSerialization JSONObjectWithData:ldata options:0 error:nil];
    }
    self.languages = parsedLangs ?: @[];
    self.langCards = [NSMutableArray array];

    // Seed the selected locale from the config BEFORE buildUI so the Language
    // tab's cards render with the correct one already highlighted.
    NSString *loc0 = self.cfg[@"locale"];
    self.selectedLangCode = (loc0.length > 0) ? loc0 : @"en-GB";

    [self buildUI];
    [self loadFromJSON:json];
    [self.window center];

    return self;
}

// L returns the localized string for key, falling back to the provided English
// default when the key is missing or the translation resolved to the key
// itself (which is how the Go i18n layer signals "no translation"). This keeps
// the native settings window fully localized and in sync with the Fyne/GTK UIs.
- (NSString *)L:(NSString *)key fallback:(NSString *)fallback {
    NSString *v = self.strings[key];
    if (v.length > 0 && ![v isEqualToString:key]) return v;
    return fallback;
}

// ── Small UI helpers ────────────────────────────────────────────────────────

- (NSTextField *)label:(NSString *)text frame:(NSRect)f bold:(BOOL)bold {
    NSTextField *l = [NSTextField labelWithString:text];
    l.frame = f;
    if (bold) l.font = [NSFont boldSystemFontOfSize:13];
    return l;
}

- (NSTextField *)sublabel:(NSString *)text frame:(NSRect)f {
    NSTextField *l = [NSTextField wrappingLabelWithString:text];
    l.frame = f;
    l.textColor = [NSColor secondaryLabelColor];
    l.font = [NSFont systemFontOfSize:11];
    return l;
}

- (NSButton *)checkbox:(NSString *)title frame:(NSRect)f {
    NSButton *b = [NSButton checkboxWithTitle:title target:nil action:nil];
    b.frame = f;
    return b;
}

// A scroll view wrapping a document view, so tall tabs stay reachable.
- (NSScrollView *)scrollableWithDocumentHeight:(CGFloat)docHeight
                                    buildBlock:(void (^)(NSView *doc))build {
    NSScrollView *scroll = [[NSScrollView alloc] initWithFrame:NSZeroRect];
    scroll.hasVerticalScroller = YES;
    scroll.hasHorizontalScroller = NO;
    scroll.autohidesScrollers = YES;
    scroll.drawsBackground = NO;
    scroll.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;

    NSView *doc = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 560, docHeight)];
    build(doc);
    scroll.documentView = doc;
    // Scroll to top.
    [doc scrollPoint:NSMakePoint(0, docHeight)];
    return scroll;
}

// ── UI construction ───────────────────────────────────────────────────────────

- (void)buildUI {
    NSView *content = self.window.contentView;

    // Localized window title (strings are available now, unlike in init).
    self.window.title = [self L:@"settings.title" fallback:@"WeatherWidget Settings"];

    _tabView = [[NSTabView alloc] initWithFrame:NSMakeRect(12, 52, 596, 656)];
    _tabView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    [content addSubview:_tabView];

    [_tabView addTabViewItem:[self buildProviderTab]];
    [_tabView addTabViewItem:[self buildLocationsTab]];
    [_tabView addTabViewItem:[self buildWidgetTab]];
    [_tabView addTabViewItem:[self buildLanguageTab]];
    [_tabView addTabViewItem:[self buildAppearanceTab]];
    [_tabView addTabViewItem:[self buildAboutTab]];

    // Save / Cancel buttons at the bottom.
    NSButton *saveBtn = [NSButton buttonWithTitle:[self L:@"settings.save" fallback:@"Save"]
        target:self action:@selector(onSave:)];
    saveBtn.keyEquivalent = @"\r";
    saveBtn.frame = NSMakeRect(520, 14, 88, 28);
    saveBtn.autoresizingMask = NSViewMinXMargin;
    [content addSubview:saveBtn];

    NSButton *cancelBtn = [NSButton buttonWithTitle:[self L:@"settings.cancel" fallback:@"Cancel"]
        target:self action:@selector(onCancel:)];
    cancelBtn.keyEquivalent = @"\033";
    cancelBtn.frame = NSMakeRect(424, 14, 88, 28);
    cancelBtn.autoresizingMask = NSViewMinXMargin;
    [content addSubview:cancelBtn];
}

// ── Provider tab ─────────────────────────────────────────────────────────────

- (NSTabViewItem *)buildProviderTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"provider"];
    item.label = [self L:@"settings.tab.provider" fallback:@"Provider"];
    NSView *v = [[NSView alloc] initWithFrame:NSZeroRect];

    CGFloat y = 560;

    [v addSubview:[self label:[self L:@"settings.provider.label" fallback:@"Provider"] frame:NSMakeRect(16, y, 200, 20) bold:NO]];
    _providerPop = [[NSPopUpButton alloc] initWithFrame:NSMakeRect(220, y - 2, 320, 26) pullsDown:NO];
    [_providerPop addItemsWithTitles:@[@"EasyWeatherWidget (Pro)", @"OpenWeatherMap (Free)"]];
    [v addSubview:_providerPop];
    y -= 40;

    [v addSubview:[self label:[self L:@"settings.provider.apiKeyLabel" fallback:@"API Key"] frame:NSMakeRect(16, y, 200, 20) bold:NO]];
    _apiKeyField = [[NSTextField alloc] initWithFrame:NSMakeRect(220, y - 2, 320, 24)];
    _apiKeyField.placeholderString = [self L:@"settings.provider.apiKeyPlaceholder" fallback:@"API Key"];
    [v addSubview:_apiKeyField];
    y -= 40;

    [v addSubview:[self label:[self L:@"settings.interval.title" fallback:@"Refresh Interval"] frame:NSMakeRect(16, y, 200, 20) bold:NO]];
    _intervalField = [[NSTextField alloc] initWithFrame:NSMakeRect(220, y - 2, 100, 24)];
    _intervalField.placeholderString = @"e.g. 30";
    [v addSubview:_intervalField];
    y -= 52;

    // Refresh-rate note box (Free vs Pro), matching the Fyne/GTK UI.
    NSTextField *note = [self sublabel:
        [self L:@"settings.provider.note"
          fallback:@"Note:\nFree = 120 minutes refresh rate (limited).\nPro = 10 minutes refresh rate (unlimited)."]
        frame:NSMakeRect(16, y - 44, 524, 56)];
    [v addSubview:note];
    y -= 64;

    // ── "Your Pro API Key" activation card ────────────────────────────────────
    // Shown only when the configured provider is EasyWeatherWidget (Pro) with a
    // full-length (UUID, 36-char) key — mirrors GTK's showActivationCard.
    NSDictionary *api = self.cfg[@"apiConfig"];
    NSString *provider = api[@"provider"] ?: @"";
    NSString *apiKey = api[@"apiKey"] ?: @"";
    BOOL isProKey = [provider isEqualToString:@"easyweatherwidget"] && apiKey.length == 36;

    if (isProKey) {
        // Bordered card containing the title, the key, and the "keep safe" note.
        NSView *card = [[NSView alloc] initWithFrame:NSMakeRect(16, y - 96, 524, 96)];
        card.wantsLayer = YES;
        card.layer.cornerRadius = 8;
        card.layer.borderWidth = 1.0;
        card.layer.borderColor = [NSColor colorWithWhite:1.0 alpha:0.16].CGColor;
        card.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.05].CGColor;

        NSTextField *cardTitle = [self label:
            [self L:@"settings.provider.apiKeyActivation.title" fallback:@"Your Pro API Key"]
            frame:NSMakeRect(12, 68, 500, 20) bold:YES];
        [card addSubview:cardTitle];

        NSTextField *keyLbl = [NSTextField labelWithString:apiKey];
        keyLbl.font = [NSFont fontWithName:@"Menlo" size:12] ?: [NSFont systemFontOfSize:12];
        keyLbl.textColor = [NSColor whiteColor];
        keyLbl.selectable = YES;
        keyLbl.frame = NSMakeRect(12, 46, 500, 18);
        [card addSubview:keyLbl];

        NSTextField *msg = [self sublabel:
            [self L:@"settings.provider.apiKeyActivation.message"
              fallback:@"Keep this key safe. It is, or will be, activated once your subscription is "
                        "confirmed and will be disabled if the subscription is cancelled."]
            frame:NSMakeRect(12, 6, 500, 36)];
        [card addSubview:msg];

        [v addSubview:card];
        y -= 108;
    }

    // ── Pro features note (persistent blue benefit line) ──────────────────────
    NSTextField *proNote = [self sublabel:
        [self L:@"settings.provider.proFeaturesNote"
          fallback:@"⭐ EasyWeatherWidget Pro: Enjoy pollution data plus the ability to preset up to 5 cities at once"]
        frame:NSMakeRect(16, y - 36, 524, 34)];
    proNote.textColor = [NSColor colorWithRed:0.22 green:0.74 blue:0.97 alpha:1.0]; // #38bdf8
    [v addSubview:proNote];

    item.view = v;
    return item;
}

// ── Locations tab ────────────────────────────────────────────────────────────

- (NSTabViewItem *)buildLocationsTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"locations"];
    item.label = [self L:@"settings.tab.locations" fallback:@"Locations"];

    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:520 buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = 486;

        [doc addSubview:[s label:[s L:@"settings.locations.savedTitle" fallback:@"Saved Cities"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 22;
        [doc addSubview:[s sublabel:[s L:@"settings.locations.savedSubtitle" fallback:@"Reorder or remove cities. Free tier is limited to the default cities."]
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 8;

        // City list inside its own scroll view.
        NSScrollView *listScroll = [[NSScrollView alloc] initWithFrame:NSMakeRect(16, y - 190, 524, 188)];
        listScroll.hasVerticalScroller = YES;
        listScroll.borderType = NSBezelBorder;
        listScroll.drawsBackground = NO;

        s.cityListStack = [[NSStackView alloc] initWithFrame:NSMakeRect(0, 0, 506, 188)];
        s.cityListStack.orientation = NSUserInterfaceLayoutOrientationVertical;
        s.cityListStack.alignment = NSLayoutAttributeLeading;
        s.cityListStack.spacing = 4;
        s.cityListStack.edgeInsets = NSEdgeInsetsMake(6, 6, 6, 6);
        listScroll.documentView = s.cityListStack;
        [doc addSubview:listScroll];
        y -= 200;

        // Add-city form.
        [doc addSubview:[s label:[s L:@"settings.locations.addTitle" fallback:@"Add New City"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 30;

        [doc addSubview:[s label:[s L:@"settings.locations.nameLabel" fallback:@"Name"] frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addNameField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 430, 24)];
        s.addNameField.placeholderString = [s L:@"settings.locations.namePlaceholder" fallback:@"City name"];
        [doc addSubview:s.addNameField];
        y -= 32;

        [doc addSubview:[s label:[s L:@"settings.locations.regionLabel" fallback:@"Region"] frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addRegionField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 430, 24)];
        s.addRegionField.placeholderString = [s L:@"settings.locations.regionPlaceholder" fallback:@"Country / state code (e.g. GB)"];
        [doc addSubview:s.addRegionField];
        y -= 32;

        [doc addSubview:[s label:[s L:@"settings.locations.latLabel" fallback:@"Latitude"] frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addLatField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 200, 24)];
        s.addLatField.placeholderString = [s L:@"settings.locations.latPlaceholder" fallback:@"e.g. 55.9344"];
        [doc addSubview:s.addLatField];
        [doc addSubview:[s label:[s L:@"settings.locations.lonLabel" fallback:@"Longitude"] frame:NSMakeRect(320, y, 80, 20) bold:NO]];
        s.addLonField = [[NSTextField alloc] initWithFrame:NSMakeRect(400, y - 2, 140, 24)];
        s.addLonField.placeholderString = [s L:@"settings.locations.lonPlaceholder" fallback:@"e.g. -3.4693"];
        [doc addSubview:s.addLonField];
        y -= 32;

        [doc addSubview:[s label:[s L:@"settings.locations.tzLabel" fallback:@"Timezone"] frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addTZField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 430, 24)];
        s.addTZField.placeholderString = [s L:@"settings.locations.tzPlaceholder" fallback:@"IANA zone (e.g. Europe/London)"];
        [doc addSubview:s.addTZField];
        y -= 40;

        NSButton *addBtn = [NSButton buttonWithTitle:[NSString stringWithFormat:@"＋ %@", [s L:@"settings.locations.addBtn" fallback:@"Add City"]]
            target:s action:@selector(onAddCity:)];
        addBtn.frame = NSMakeRect(440, y, 100, 28);
        [doc addSubview:addBtn];
    }];
    scroll.frame = NSMakeRect(0, 0, 572, 600);
    item.view = scroll;
    return item;
}

// Rebuilds the city rows in the stack view from self.cities.
- (void)refreshCityList {
    if (!self.cityListStack) return;
    for (NSView *sub in [self.cityListStack.arrangedSubviews copy]) {
        [self.cityListStack removeArrangedSubview:sub];
        [sub removeFromSuperview];
    }

    BOOL hasLicense = [self hasLicense];
    NSInteger count = self.cities.count;

    for (NSInteger i = 0; i < count; i++) {
        NSDictionary *city = self.cities[i];
        NSString *name = city[@"name"] ?: @"";
        NSString *region = city[@"region"] ?: @"";

        NSView *row = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 494, 26)];
        row.translatesAutoresizingMaskIntoConstraints = NO;
        [row.widthAnchor constraintEqualToConstant:494].active = YES;
        [row.heightAnchor constraintEqualToConstant:26].active = YES;

        NSTextField *lbl = [NSTextField labelWithString:
            [NSString stringWithFormat:@"%@, %@", name, region]];
        lbl.frame = NSMakeRect(4, 3, 320, 20);
        lbl.font = [NSFont boldSystemFontOfSize:12];
        [row addSubview:lbl];

        NSButton *upBtn = [NSButton buttonWithTitle:@"▲" target:self action:@selector(onCityUp:)];
        upBtn.tag = i;
        upBtn.frame = NSMakeRect(330, 1, 34, 24);
        upBtn.enabled = hasLicense && (i > 0);
        [row addSubview:upBtn];

        NSButton *downBtn = [NSButton buttonWithTitle:@"▼" target:self action:@selector(onCityDown:)];
        downBtn.tag = i;
        downBtn.frame = NSMakeRect(368, 1, 34, 24);
        downBtn.enabled = hasLicense && (i < count - 1);
        [row addSubview:downBtn];

        NSButton *delBtn = [NSButton buttonWithTitle:@"🗑" target:self action:@selector(onCityDelete:)];
        delBtn.tag = i;
        delBtn.frame = NSMakeRect(406, 1, 40, 24);
        delBtn.enabled = hasLicense && (count > 1);
        [row addSubview:delBtn];

        [self.cityListStack addArrangedSubview:row];
    }
}

- (BOOL)hasLicense {
    NSDictionary *api = self.cfg[@"apiConfig"];
    NSString *key = api[@"apiKey"];
    if (key && key.length > 0) return YES;
    NSDictionary *db = self.cfg[@"databaseConfig"];
    NSString *host = db[@"host"];
    return (host && host.length > 0);
}

- (BOOL)isPro {
    NSDictionary *api = self.cfg[@"apiConfig"];
    NSString *provider = api[@"provider"];
    NSString *key = api[@"apiKey"];
    return [provider isEqualToString:@"easyweatherwidget"] && key && key.length > 0;
}

- (void)onCityUp:(NSButton *)sender {
    NSInteger i = sender.tag;
    if (i > 0 && i < (NSInteger)self.cities.count) {
        [self.cities exchangeObjectAtIndex:i withObjectAtIndex:i - 1];
        [self refreshCityList];
    }
}

- (void)onCityDown:(NSButton *)sender {
    NSInteger i = sender.tag;
    if (i >= 0 && i < (NSInteger)self.cities.count - 1) {
        [self.cities exchangeObjectAtIndex:i withObjectAtIndex:i + 1];
        [self refreshCityList];
    }
}

- (void)onCityDelete:(NSButton *)sender {
    NSInteger i = sender.tag;
    if (self.cities.count <= 1) return;
    if (i >= 0 && i < (NSInteger)self.cities.count) {
        [self.cities removeObjectAtIndex:i];
        [self refreshCityList];
    }
}

- (void)onAddCity:(id)sender {
    if (![self hasLicense]) {
        [self showAlert:[self L:@"settings.locations.addTitle" fallback:@"Add New City"]
                   info:[self L:@"error.settings.licenseRequired"
                            fallback:@"Adding cities requires a Pro API key. Configure one in the Provider tab."]];
        return;
    }
    NSString *name = [self.addNameField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    if (name.length == 0) {
        [self showAlert:[self L:@"settings.locations.addTitle" fallback:@"Add New City"]
                   info:[self L:@"error.settings.cityNameRequired" fallback:@"Enter a city name before adding."]];
        return;
    }
    NSInteger maxCities = [self isPro] ? 5 : 3;
    if ((NSInteger)self.cities.count >= maxCities) {
        [self showAlert:@"City limit reached"
                   info:[NSString stringWithFormat:@"Your tier allows up to %ld cities.", (long)maxCities]];
        return;
    }

    NSMutableDictionary *city = [NSMutableDictionary dictionary];
    city[@"name"] = name;
    city[@"region"] = [self.addRegionField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    city[@"timezone"] = [self.addTZField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *latStr = [self.addLatField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *lonStr = [self.addLonField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    if (latStr.length > 0) city[@"latitude"] = @([latStr doubleValue]);
    if (lonStr.length > 0) city[@"longitude"] = @([lonStr doubleValue]);

    [self.cities addObject:city];

    self.addNameField.stringValue = @"";
    self.addRegionField.stringValue = @"";
    self.addLatField.stringValue = @"";
    self.addLonField.stringValue = @"";
    self.addTZField.stringValue = @"";

    [self refreshCityList];
}

- (void)showAlert:(NSString *)title info:(NSString *)info {
    NSAlert *a = [[NSAlert alloc] init];
    a.messageText = title;
    a.informativeText = info;
    [a addButtonWithTitle:@"OK"];
    [a beginSheetModalForWindow:self.window completionHandler:nil];
}

// ── Widget tab ───────────────────────────────────────────────────────────────

- (NSTabViewItem *)buildWidgetTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"widget"];
    item.label = [self L:@"settings.tab.widget" fallback:@"Widget"];

    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:560 buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = 526;

        // Panel display fields.
        [doc addSubview:[s label:[s L:@"settings.display.title" fallback:@"Panel Display"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 20;
        [doc addSubview:[s sublabel:[s L:@"settings.display.subtitle" fallback:@"Choose which elements appear on each city panel."]
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 26;

        // 4-column grid of checkboxes.
        CGFloat colW = 130, rowH = 26;
        CGFloat cx0 = 16;
        s.dfCity     = [s checkbox:[s L:@"settings.display.city" fallback:@"City"]        frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.dfIcon     = [s checkbox:[s L:@"settings.display.icon" fallback:@"Icon"]        frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.dfTemp     = [s checkbox:[s L:@"settings.display.temp" fallback:@"Temperature"] frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.dfDesc     = [s checkbox:[s L:@"settings.display.desc" fallback:@"Description"] frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        y -= rowH;
        s.dfHumidity = [s checkbox:[s L:@"settings.display.humidity" fallback:@"Humidity"] frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.dfWind     = [s checkbox:[s L:@"settings.display.wind" fallback:@"Wind"]         frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.dfTime     = [s checkbox:[s L:@"settings.display.time" fallback:@"Time"]         frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.dfDate     = [s checkbox:[s L:@"settings.display.date" fallback:@"Date"]         frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        y -= rowH;
        s.dfWindGust = [s checkbox:[s L:@"settings.display.windGust" fallback:@"Wind Gust"] frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.dfDewPoint = [s checkbox:[s L:@"settings.display.dewPoint" fallback:@"Dew Point"] frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.dfPressure = [s checkbox:[s L:@"settings.display.pressure" fallback:@"Pressure"]  frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.dfUVIndex  = [s checkbox:[s L:@"settings.display.uvIndex" fallback:@"UV Index"]   frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        for (NSButton *b in @[s.dfCity, s.dfIcon, s.dfTemp, s.dfDesc, s.dfHumidity, s.dfWind,
                              s.dfTime, s.dfDate, s.dfWindGust, s.dfDewPoint, s.dfPressure, s.dfUVIndex]) {
            [doc addSubview:b];
        }
        y -= 36;

        // Pollution fields. Chemical symbols (CO, NO₂, …) are universal notation
        // and intentionally not translated; only the section header is localized.
        [doc addSubview:[s label:[s L:@"settings.pollution.title" fallback:@"Air Quality"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 20;
        [doc addSubview:[s sublabel:[s L:@"settings.pollution.subtitle" fallback:@"Air-quality metrics (Pro only)."]
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 26;

        s.pfAQI  = [s checkbox:@"AQI"  frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.pfCO   = [s checkbox:@"CO"   frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.pfNO   = [s checkbox:@"NO"   frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.pfNO2  = [s checkbox:@"NO₂"  frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        y -= rowH;
        s.pfO3   = [s checkbox:@"O₃"   frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.pfSO2  = [s checkbox:@"SO₂"  frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.pfNH3  = [s checkbox:@"NH₃"  frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.pfPM25 = [s checkbox:@"PM2.5" frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        y -= rowH;
        s.pfPM10 = [s checkbox:@"PM10" frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];

        BOOL pro = [s isPro];
        for (NSButton *b in @[s.pfAQI, s.pfCO, s.pfNO, s.pfNO2, s.pfO3, s.pfSO2, s.pfNH3, s.pfPM25, s.pfPM10]) {
            b.enabled = pro;
            [doc addSubview:b];
        }
        y -= 40;

        // Temperature unit.
        [doc addSubview:[s label:[s L:@"settings.temperature.title" fallback:@"Temperature Unit"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 26;
        s.tempCelsius = [NSButton radioButtonWithTitle:[s L:@"settings.temperature.celsius" fallback:@"°C (Celsius)"]
            target:s action:@selector(onTempUnitRadio:)];
        s.tempCelsius.frame = NSMakeRect(16, y, 160, 20);
        [doc addSubview:s.tempCelsius];
        s.tempFahrenheit = [NSButton radioButtonWithTitle:[s L:@"settings.temperature.fahrenheit" fallback:@"°F (Fahrenheit)"]
            target:s action:@selector(onTempUnitRadio:)];
        s.tempFahrenheit.frame = NSMakeRect(190, y, 170, 20);
        [doc addSubview:s.tempFahrenheit];
        s.tempKelvin = [NSButton radioButtonWithTitle:[s L:@"settings.temperature.kelvin" fallback:@"K (Kelvin)"]
            target:s action:@selector(onTempUnitRadio:)];
        s.tempKelvin.frame = NSMakeRect(374, y, 160, 20);
        [doc addSubview:s.tempKelvin];
        y -= 40;

        // Wind speed unit.
        [doc addSubview:[s label:[s L:@"settings.windspeed.title" fallback:@"Wind Speed Unit"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 26;
        s.windKmh = [NSButton radioButtonWithTitle:@"km/h"
            target:s action:@selector(onWindUnitRadio:)];
        s.windKmh.frame = NSMakeRect(16, y, 90, 20);
        [doc addSubview:s.windKmh];
        s.windMph = [NSButton radioButtonWithTitle:@"mph"
            target:s action:@selector(onWindUnitRadio:)];
        s.windMph.frame = NSMakeRect(116, y, 90, 20);
        [doc addSubview:s.windMph];
        s.windKnots = [NSButton radioButtonWithTitle:@"knots"
            target:s action:@selector(onWindUnitRadio:)];
        s.windKnots.frame = NSMakeRect(216, y, 90, 20);
        [doc addSubview:s.windKnots];
    }];
    scroll.frame = NSMakeRect(0, 0, 572, 600);
    item.view = scroll;
    return item;
}

// ── Language tab ─────────────────────────────────────────────────────────────

- (NSTabViewItem *)buildLanguageTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"language"];
    item.label = [self L:@"settings.tab.language" fallback:@"Language"];

    // Card grid geometry (2 columns), matching the GTK language grid.
    const CGFloat cardW = 268, cardH = 58, gapX = 16, gapY = 12;
    const CGFloat leftX = 16;
    NSInteger count = self.languages.count;
    NSInteger rows = (count + 1) / 2;
    CGFloat headerH = 60;
    CGFloat docH = headerH + rows * (cardH + gapY) + 24;
    if (docH < 400) docH = 400;

    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:docH buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat top = docH - 20;

        // Header: 🌐 title + a "N LANGUAGES" badge on the right (like GTK).
        [doc addSubview:[s label:[NSString stringWithFormat:@"🌐 %@", [s L:@"settings.language.title" fallback:@"Language"]]
                            frame:NSMakeRect(16, top, 300, 20) bold:YES]];
        NSTextField *badge = [NSTextField labelWithString:
            [NSString stringWithFormat:@"%ld LANGUAGES", (long)count]];
        badge.font = [NSFont boldSystemFontOfSize:10];
        badge.textColor = [NSColor colorWithRed:0.22 green:0.60 blue:1.0 alpha:1.0];
        badge.alignment = NSTextAlignmentRight;
        badge.frame = NSMakeRect(360, top, 176, 18);
        [doc addSubview:badge];
        CGFloat y = top - 22;
        [doc addSubview:[s sublabel:[s L:@"settings.language.subtitle" fallback:@"Choose your preferred language."]
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 24;

        // Flag cards.
        [s.langCards removeAllObjects];
        CGFloat gridTop = y;
        for (NSInteger i = 0; i < count; i++) {
            NSDictionary *L = s.languages[i];
            NSInteger col = i % 2;
            NSInteger row = i / 2;
            CGFloat cx = leftX + col * (cardW + gapX);
            CGFloat cy = gridTop - cardH - row * (cardH + gapY);

            WWLangCard *card = [[[WWLangCard alloc]
                initWithCode:L[@"code"] native:L[@"native"]
                     english:L[@"english"] flagPath:L[@"flag"]] autorelease];
            card.frame = NSMakeRect(cx, cy, cardW, cardH);
            card.target = s;
            card.action = @selector(onLangCardClicked:);
            card.selected = [L[@"code"] isEqualToString:s.selectedLangCode];
            [doc addSubview:card];
            [s.langCards addObject:card];
        }
    }];
    scroll.frame = NSMakeRect(0, 0, 572, 600);
    item.view = scroll;
    return item;
}

// onLangCardClicked selects the clicked language card and deselects the others.
- (void)onLangCardClicked:(WWLangCard *)card {
    self.selectedLangCode = card.code;
    // Keep selectedLangIndex in sync for buildUpdatedJSON.
    for (NSInteger i = 0; i < (NSInteger)self.languages.count; i++) {
        if ([self.languages[i][@"code"] isEqualToString:card.code]) {
            self.selectedLangIndex = i;
            break;
        }
    }
    for (WWLangCard *c in self.langCards) {
        c.selected = [c.code isEqualToString:card.code];
    }
}

// Radio buttons only auto-deselect their siblings when they share the same
// target AND the same action selector within the same superview. Because
// several groups live in the same tab view, each group needs its OWN selector
// so AppKit keeps them independent. These handlers are otherwise no-ops —
// buildUpdatedJSON reads each button's .state at save time.
- (void)onViewModeRadio:(id)sender {}
- (void)onTempUnitRadio:(id)sender {}
- (void)onWindUnitRadio:(id)sender {}
- (void)onIconThemeRadio:(id)sender {}

// ── Appearance tab ────────────────────────────────────────────────────────────

- (NSTabViewItem *)buildAppearanceTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"appearance"];
    item.label = [self L:@"settings.tab.appearance" fallback:@"Appearance"];

    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:600 buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = 566;

        // View mode.
        [doc addSubview:[s label:[s L:@"settings.viewMode.title" fallback:@"View Mode"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 26;
        s.viewEnhanced = [NSButton radioButtonWithTitle:[s L:@"settings.viewMode.enhanced" fallback:@"Enhanced (modern)"]
            target:s action:@selector(onViewModeRadio:)];
        s.viewEnhanced.frame = NSMakeRect(16, y, 200, 20);
        [doc addSubview:s.viewEnhanced];
        s.viewSimple = [NSButton radioButtonWithTitle:[s L:@"settings.viewMode.simple" fallback:@"Simple (classic)"]
            target:s action:@selector(onViewModeRadio:)];
        s.viewSimple.frame = NSMakeRect(230, y, 200, 20);
        [doc addSubview:s.viewSimple];
        y -= 44;

        // Opacity — title on its own row, then the slider + % value on the row
        // below it (the wider localized title "Background Transparency" would
        // otherwise overlap the slider).
        [doc addSubview:[s label:[s L:@"settings.transparency.title" fallback:@"Opacity"] frame:NSMakeRect(16, y, 400, 20) bold:NO]];
        y -= 26;
        s.opacitySlider = [[NSSlider alloc] initWithFrame:NSMakeRect(16, y, 460, 20)];
        s.opacitySlider.minValue = 25; s.opacitySlider.maxValue = 100;
        s.opacitySlider.numberOfTickMarks = 4;
        s.opacitySlider.allowsTickMarkValuesOnly = YES;
        s.opacitySlider.target = s; s.opacitySlider.action = @selector(onOpacityChanged:);
        [doc addSubview:s.opacitySlider];
        s.opacityLabel = [NSTextField labelWithString:@"100%"];
        s.opacityLabel.frame = NSMakeRect(486, y, 54, 20);
        [doc addSubview:s.opacityLabel];
        y -= 40;

        // Autostart.
        s.autostartCheck = [NSButton checkboxWithTitle:[s L:@"settings.startup.autostart" fallback:@"Launch at login"]
            target:s action:@selector(onAutostartToggled:)];
        s.autostartCheck.frame = NSMakeRect(16, y, 460, 20);
        [doc addSubview:s.autostartCheck];
        y -= 44;

        // Weather icons.
        [doc addSubview:[s label:[s L:@"settings.icons.title" fallback:@"Weather Icons"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 26;
        s.iconThemeNew = [NSButton radioButtonWithTitle:[s L:@"settings.icons.new" fallback:@"New (day/night)"]
            target:s action:@selector(onIconThemeRadio:)];
        s.iconThemeNew.frame = NSMakeRect(16, y, 220, 20);
        [doc addSubview:s.iconThemeNew];
        s.iconThemeOriginal = [NSButton radioButtonWithTitle:[s L:@"settings.icons.original" fallback:@"Original"]
            target:s action:@selector(onIconThemeRadio:)];
        s.iconThemeOriginal.frame = NSMakeRect(250, y, 200, 20);
        [doc addSubview:s.iconThemeOriginal];
        y -= 44;

        // Position.
        [doc addSubview:[s label:[s L:@"settings.position.title" fallback:@"Widget Position"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 22;
        [doc addSubview:[s sublabel:@"Set exact coordinates, or just drag the widget on screen."
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 32;

        [doc addSubview:[s label:@"X:" frame:NSMakeRect(16, y, 24, 22) bold:NO]];
        s.posXField = [[NSTextField alloc] initWithFrame:NSMakeRect(44, y, 90, 24)];
        [doc addSubview:s.posXField];
        [doc addSubview:[s label:@"Y:" frame:NSMakeRect(150, y, 24, 22) bold:NO]];
        s.posYField = [[NSTextField alloc] initWithFrame:NSMakeRect(178, y, 90, 24)];
        [doc addSubview:s.posYField];
        NSButton *applyPos = [NSButton buttonWithTitle:@"Apply"
            target:s action:@selector(onApplyPosition:)];
        applyPos.frame = NSMakeRect(280, y - 1, 80, 26);
        [doc addSubview:applyPos];
        y -= 48;

        // Font sizes.
        [doc addSubview:[s label:[s L:@"settings.fontSize.title" fallback:@"Font Sizes"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 30;

        y = [s addFontRow:doc y:y label:[s L:@"settings.fontSize.cityTime" fallback:@"City & Time"] tag:0 valueLabelOut:&s->_fontCityTimeLabel];
        y = [s addFontRow:doc y:y label:[s L:@"settings.fontSize.tempIcon" fallback:@"Temperature"] tag:1 valueLabelOut:&s->_fontTempIconLabel];
        y = [s addFontRow:doc y:y label:[s L:@"settings.fontSize.conditions" fallback:@"Conditions"] tag:2 valueLabelOut:&s->_fontConditionsLabel];
    }];
    scroll.frame = NSMakeRect(0, 0, 572, 600);
    item.view = scroll;
    return item;
}

// Adds one "Label  [-]  Npx  [+]" row and returns the new y.
- (CGFloat)addFontRow:(NSView *)doc y:(CGFloat)y label:(NSString *)label tag:(NSInteger)tag
        valueLabelOut:(NSTextField * __strong *)outLabel {
    // Wider label column so longer localized names (e.g. "Temperature & Icon")
    // aren't clipped; controls shifted right to match.
    [doc addSubview:[self label:label frame:NSMakeRect(16, y, 210, 22) bold:NO]];

    NSButton *dec = [NSButton buttonWithTitle:@"−" target:self action:@selector(onFontDec:)];
    dec.tag = tag;
    dec.frame = NSMakeRect(236, y - 1, 34, 24);
    [doc addSubview:dec];

    NSTextField *val = [NSTextField labelWithString:@"0px"];
    val.frame = NSMakeRect(276, y, 60, 20);
    val.alignment = NSTextAlignmentCenter;
    [doc addSubview:val];
    *outLabel = val;

    NSButton *inc = [NSButton buttonWithTitle:@"＋" target:self action:@selector(onFontInc:)];
    inc.tag = tag;
    inc.frame = NSMakeRect(338, y - 1, 34, 24);
    [doc addSubview:inc];

    return y - 32;
}

- (void)refreshFontLabels {
    self.fontCityTimeLabel.stringValue   = [NSString stringWithFormat:@"%ldpx", (long)self.fontCityTime];
    self.fontTempIconLabel.stringValue   = [NSString stringWithFormat:@"%ldpx", (long)self.fontTempIcon];
    self.fontConditionsLabel.stringValue = [NSString stringWithFormat:@"%ldpx", (long)self.fontConditions];
}

- (void)onFontDec:(NSButton *)sender {
    switch (sender.tag) {
        case 0: if (self.fontCityTime > 8)   self.fontCityTime--;   break;
        case 1: if (self.fontTempIcon > 10)  self.fontTempIcon--;   break;
        case 2: if (self.fontConditions > 6) self.fontConditions--; break;
    }
    [self refreshFontLabels];
}

- (void)onFontInc:(NSButton *)sender {
    switch (sender.tag) {
        case 0: if (self.fontCityTime < 48)   self.fontCityTime++;   break;
        case 1: if (self.fontTempIcon < 72)   self.fontTempIcon++;   break;
        case 2: if (self.fontConditions < 36) self.fontConditions++; break;
    }
    [self refreshFontLabels];
}

- (void)onAutostartToggled:(NSButton *)sender {
    int ok = settingsAutostartSetCB(sender.state == NSControlStateValueOn ? 1 : 0);
    if (!ok) {
        // Revert on failure.
        sender.state = settingsAutostartGetCB() ? NSControlStateValueOn : NSControlStateValueOff;
    }
}

// ── About tab ─────────────────────────────────────────────────────────────────

- (NSTabViewItem *)buildAboutTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"about"];
    item.label = [self L:@"settings.tab.about" fallback:@"About"];
    NSView *v = [[NSView alloc] initWithFrame:NSZeroRect];

    NSTextField *title = [NSTextField labelWithString:[self L:@"settings.about.appName" fallback:@"WeatherWidget"]];
    title.font = [NSFont boldSystemFontOfSize:20];
    title.frame = NSMakeRect(16, 560, 560, 28);
    [v addSubview:title];

    NSTextField *sub = [NSTextField wrappingLabelWithString:[self L:@"settings.about.description" fallback:@"Desktop weather overlay for macOS"]];
    sub.frame = NSMakeRect(16, 500, 560, 48);
    sub.textColor = [NSColor secondaryLabelColor];
    [v addSubview:sub];

    NSTextField *website = [NSTextField labelWithString:
        [NSString stringWithFormat:@"%@ easysmartapps.co.uk/weatherwidget",
         [self L:@"settings.about.websiteLabel" fallback:@"Website:"]]];
    website.frame = NSMakeRect(16, 470, 560, 20);
    website.textColor = [NSColor linkColor];
    [v addSubview:website];

    NSTextField *manual = [NSTextField labelWithString:
        [NSString stringWithFormat:@"%@ easysmartapps.co.uk/weatherwidget-manual",
         [self L:@"settings.about.manualLabel" fallback:@"Manual:"]]];
    manual.frame = NSMakeRect(16, 444, 560, 20);
    manual.textColor = [NSColor linkColor];
    [v addSubview:manual];

    NSTextField *air = [NSTextField labelWithString:
        [NSString stringWithFormat:@"%@ easysmartapps.co.uk/weatherwidget-environmental",
         [self L:@"settings.about.airIndexLabel" fallback:@"Air Index:"]]];
    air.frame = NSMakeRect(16, 418, 560, 20);
    air.textColor = [NSColor linkColor];
    [v addSubview:air];

    item.view = v;
    return item;
}

// ── Config load ───────────────────────────────────────────────────────────────

- (BOOL)boolFor:(NSDictionary *)dict key:(NSString *)key defaultVal:(BOOL)def {
    id val = dict[key];
    if (val == nil) return def;
    return [val boolValue];
}

- (void)loadFromJSON:(NSString *)json {
    NSDictionary *cfg = self.cfg;
    if (!cfg) return;

    // ── Provider ──
    NSDictionary *api = cfg[@"apiConfig"];
    NSString *provider = api[@"provider"] ?: @"easyweatherwidget";
    [_providerPop selectItemAtIndex:[provider isEqualToString:@"openweathermap"] ? 1 : 0];
    _apiKeyField.stringValue = api[@"apiKey"] ?: @"";
    NSNumber *interval = cfg[@"refreshInterval"];
    _intervalField.stringValue = interval ? [interval stringValue] : @"120";

    // ── Locations ──
    [_cities removeAllObjects];
    NSArray *cityArr = cfg[@"cities"];
    for (NSDictionary *c in cityArr) {
        [_cities addObject:[c mutableCopy]];
    }
    [self refreshCityList];

    // ── Widget: display fields (default true) ──
    NSDictionary *df = cfg[@"displayFields"];
    _dfCity.state     = [self boolFor:df key:@"showCity"     defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfIcon.state     = [self boolFor:df key:@"showIcon"     defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfTemp.state     = [self boolFor:df key:@"showTemp"     defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfDesc.state     = [self boolFor:df key:@"showDesc"     defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfHumidity.state = [self boolFor:df key:@"showHumidity" defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfWind.state     = [self boolFor:df key:@"showWind"     defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfTime.state     = [self boolFor:df key:@"showTime"     defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfDate.state     = [self boolFor:df key:@"showDate"     defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfWindGust.state = [self boolFor:df key:@"showWindGust" defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfDewPoint.state = [self boolFor:df key:@"showDewPoint" defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfPressure.state = [self boolFor:df key:@"showPressure" defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfUVIndex.state  = [self boolFor:df key:@"showUVIndex"  defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;

    // ── Widget: pollution fields (default false) ──
    NSDictionary *pf = cfg[@"pollutionFields"];
    _pfAQI.state  = [self boolFor:pf key:@"showAQI"  defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _pfCO.state   = [self boolFor:pf key:@"showCO"   defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _pfNO.state   = [self boolFor:pf key:@"showNO"   defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _pfNO2.state  = [self boolFor:pf key:@"showNO2"  defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _pfO3.state   = [self boolFor:pf key:@"showO3"   defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _pfSO2.state  = [self boolFor:pf key:@"showSO2"  defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _pfNH3.state  = [self boolFor:pf key:@"showNH3"  defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _pfPM25.state = [self boolFor:pf key:@"showPM25" defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _pfPM10.state = [self boolFor:pf key:@"showPM10" defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;

    // ── Widget: units ──
    NSString *tempUnit = cfg[@"temperatureUnit"] ?: @"celsius";
    if ([tempUnit isEqualToString:@"fahrenheit"]) {
        _tempFahrenheit.state = NSControlStateValueOn;
    } else if ([tempUnit isEqualToString:@"kelvin"]) {
        _tempKelvin.state = NSControlStateValueOn;
    } else {
        _tempCelsius.state = NSControlStateValueOn;
    }
    NSString *windUnit = cfg[@"windSpeedUnit"] ?: @"kmh";
    if ([windUnit isEqualToString:@"mph"]) {
        _windMph.state = NSControlStateValueOn;
    } else if ([windUnit isEqualToString:@"knots"]) {
        _windKnots.state = NSControlStateValueOn;
    } else {
        _windKmh.state = NSControlStateValueOn;
    }

    // ── Language ──
    NSString *locale = cfg[@"locale"] ?: @"en-GB";
    self.selectedLangCode = locale;
    _selectedLangIndex = 0;
    for (NSInteger i = 0; i < (NSInteger)self.languages.count; i++) {
        if ([self.languages[i][@"code"] isEqualToString:locale]) { _selectedLangIndex = i; break; }
    }
    for (WWLangCard *c in self.langCards) {
        c.selected = [c.code isEqualToString:locale];
    }

    // ── Appearance: view mode ──
    NSString *viewMode = cfg[@"viewMode"] ?: @"enhanced";
    if ([viewMode isEqualToString:@"simple"]) {
        _viewSimple.state = NSControlStateValueOn;
    } else {
        _viewEnhanced.state = NSControlStateValueOn;
    }

    // ── Appearance: opacity ──
    NSNumber *opacity = cfg[@"opacity"];
    double opVal = opacity ? opacity.doubleValue : 100;
    if (opVal <= 0) opVal = 100;
    _opacitySlider.doubleValue = opVal;
    _opacityLabel.stringValue = [NSString stringWithFormat:@"%.0f%%", opVal];

    // ── Appearance: icon theme ──
    NSString *iconTheme = cfg[@"iconTheme"] ?: @"new";
    if ([iconTheme isEqualToString:@"original"]) {
        _iconThemeOriginal.state = NSControlStateValueOn;
    } else {
        _iconThemeNew.state = NSControlStateValueOn;
    }

    // ── Appearance: position ──
    if (cfg[@"customX"] && cfg[@"customY"]) {
        _posXField.stringValue = [cfg[@"customX"] stringValue];
        _posYField.stringValue = [cfg[@"customY"] stringValue];
    }

    // ── Appearance: font sizes ──
    NSNumber *fCT = cfg[@"fontSizeCityTime"];
    NSNumber *fTI = cfg[@"fontSizeTempIcon"];
    NSNumber *fC  = cfg[@"fontSizeConditions"];
    _fontCityTime   = (fCT && fCT.integerValue > 0) ? fCT.integerValue : 14;
    _fontTempIcon   = (fTI && fTI.integerValue > 0) ? fTI.integerValue : 32;
    _fontConditions = (fC  && fC.integerValue  > 0) ? fC.integerValue  : 10;
    [self refreshFontLabels];

    // ── Autostart ──
    _autostartCheck.state = settingsAutostartGetCB() ? NSControlStateValueOn : NSControlStateValueOff;
}

// ── Config save ─────────────────────────────────────────────────────────────

- (NSString *)buildUpdatedJSON {
    NSMutableDictionary *cfg = [self.cfg mutableCopy];
    if (!cfg) return _cfgJSON;

    // ── Provider ──
    NSMutableDictionary *apiCfg = [cfg[@"apiConfig"] mutableCopy] ?: [NSMutableDictionary dictionary];
    apiCfg[@"provider"] = _providerPop.indexOfSelectedItem == 1 ?
        @"openweathermap" : @"easyweatherwidget";
    apiCfg[@"apiKey"] = [_apiKeyField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    cfg[@"apiConfig"] = apiCfg;

    NSInteger intervalVal = _intervalField.integerValue;
    if (intervalVal >= 10 && intervalVal <= 120)
        cfg[@"refreshInterval"] = @(intervalVal);

    // ── Locations ──
    cfg[@"cities"] = [_cities copy];

    // ── Display fields ──
    // NOTE: use jbool() (@YES/@NO), NOT @(expr). Boxing a BOOL expression with
    // @(...) produces a char-backed NSNumber that NSJSONSerialization emits as
    // a JSON number (0/1), which Go then refuses to unmarshal into a Go bool
    // ("cannot unmarshal number into ... of type bool") — silently discarding
    // the entire save. @YES/@NO box as __NSCFBoolean → JSON true/false.
    cfg[@"displayFields"] = @{
        @"showCity":     jbool(_dfCity.state     == NSControlStateValueOn),
        @"showIcon":     jbool(_dfIcon.state     == NSControlStateValueOn),
        @"showTemp":     jbool(_dfTemp.state     == NSControlStateValueOn),
        @"showDesc":     jbool(_dfDesc.state     == NSControlStateValueOn),
        @"showHumidity": jbool(_dfHumidity.state == NSControlStateValueOn),
        @"showWind":     jbool(_dfWind.state     == NSControlStateValueOn),
        @"showTime":     jbool(_dfTime.state     == NSControlStateValueOn),
        @"showDate":     jbool(_dfDate.state     == NSControlStateValueOn),
        @"showWindGust": jbool(_dfWindGust.state == NSControlStateValueOn),
        @"showDewPoint": jbool(_dfDewPoint.state == NSControlStateValueOn),
        @"showPressure": jbool(_dfPressure.state == NSControlStateValueOn),
        @"showUVIndex":  jbool(_dfUVIndex.state  == NSControlStateValueOn),
    };

    // ── Pollution fields ──
    cfg[@"pollutionFields"] = @{
        @"showAQI":  jbool(_pfAQI.state  == NSControlStateValueOn),
        @"showCO":   jbool(_pfCO.state   == NSControlStateValueOn),
        @"showNO":   jbool(_pfNO.state   == NSControlStateValueOn),
        @"showNO2":  jbool(_pfNO2.state  == NSControlStateValueOn),
        @"showO3":   jbool(_pfO3.state   == NSControlStateValueOn),
        @"showSO2":  jbool(_pfSO2.state  == NSControlStateValueOn),
        @"showNH3":  jbool(_pfNH3.state  == NSControlStateValueOn),
        @"showPM25": jbool(_pfPM25.state == NSControlStateValueOn),
        @"showPM10": jbool(_pfPM10.state == NSControlStateValueOn),
    };

    // ── Units ──
    if (_tempFahrenheit.state == NSControlStateValueOn) {
        cfg[@"temperatureUnit"] = @"fahrenheit";
    } else if (_tempKelvin.state == NSControlStateValueOn) {
        cfg[@"temperatureUnit"] = @"kelvin";
    } else {
        cfg[@"temperatureUnit"] = @"celsius";
    }
    if (_windMph.state == NSControlStateValueOn) {
        cfg[@"windSpeedUnit"] = @"mph";
    } else if (_windKnots.state == NSControlStateValueOn) {
        cfg[@"windSpeedUnit"] = @"knots";
    } else {
        cfg[@"windSpeedUnit"] = @"kmh";
    }

    // ── Language ──
    if (self.selectedLangCode.length > 0) {
        cfg[@"locale"] = self.selectedLangCode;
    } else if (_selectedLangIndex >= 0 && _selectedLangIndex < (NSInteger)self.languages.count) {
        cfg[@"locale"] = self.languages[_selectedLangIndex][@"code"];
    }

    // ── View mode ──
    cfg[@"viewMode"] = _viewSimple.state == NSControlStateValueOn ? @"simple" : @"enhanced";

    // ── Opacity ──
    cfg[@"opacity"] = @((int)round(_opacitySlider.doubleValue / 25.0) * 25);

    // ── Icon theme ──
    cfg[@"iconTheme"] = _iconThemeOriginal.state == NSControlStateValueOn ? @"original" : @"new";

    // ── Position ──
    NSString *xStr = [_posXField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *yStr = [_posYField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    if (xStr.length > 0 && yStr.length > 0) {
        cfg[@"customX"] = @(_posXField.integerValue);
        cfg[@"customY"] = @(_posYField.integerValue);
    }

    // ── Font sizes ──
    cfg[@"fontSizeCityTime"]   = @(_fontCityTime);
    cfg[@"fontSizeTempIcon"]   = @(_fontTempIcon);
    cfg[@"fontSizeConditions"] = @(_fontConditions);

    NSData *out = [NSJSONSerialization dataWithJSONObject:cfg options:0 error:nil];
    return out ? [[NSString alloc] initWithData:out encoding:NSUTF8StringEncoding]
               : _cfgJSON;
}

// ── Actions ───────────────────────────────────────────────────────────────────

- (void)onOpacityChanged:(id)sender {
    int snapped = (int)round(_opacitySlider.doubleValue / 25.0) * 25;
    _opacityLabel.stringValue = [NSString stringWithFormat:@"%d%%", snapped];
}

- (void)onApplyPosition:(id)sender {
    NSString *updated = [self buildUpdatedJSON];
    settingsSaveCB([updated UTF8String]);
    // Update stored config/JSON so a subsequent full Save keeps the new position.
    _cfgJSON = updated;
    NSData *data = [updated dataUsingEncoding:NSUTF8StringEncoding];
    self.cfg = [[NSJSONSerialization JSONObjectWithData:data
        options:NSJSONReadingMutableContainers error:nil] mutableCopy] ?: self.cfg;
}

- (void)onSave:(id)sender {
    NSString *updated = [self buildUpdatedJSON];
    settingsSaveCB([updated UTF8String]);
    _cfgJSON = updated;
    [self close];
}

- (void)onCancel:(id)sender {
    [self close];
}

// NSWindowDelegate — fired when window closes via X button or onCancel.
- (void)windowWillClose:(NSNotification *)notification {
    g_settingsController = nil;
    settingsClosedCB();
}

// MRC: balance the retains taken in -initWithJSON: (via the strong setters).
- (void)dealloc {
    [_cfgJSON release];
    [_cities release];
    [_cfg release];
    [_strings release];
    [_languages release];
    [_langCards release];
    [_selectedLangCode release];
    [super dealloc];
}

@end


// ── Public C API ──────────────────────────────────────────────────────────────

void openSettingsNative(
    const char *cfgJSON,
    const char *stringsJSON,
    const char *langsJSON
) {
    // Copy the JSON to NSStrings NOW, synchronously. Go frees the C strings via
    // defer C.free the moment openSettingsWindow returns — before the async
    // block runs. Reading them inside the block would hit freed memory.
    NSString *json = (cfgJSON && *cfgJSON) ?
        [NSString stringWithUTF8String:cfgJSON] : @"{}";
    NSString *strings = (stringsJSON && *stringsJSON) ?
        [NSString stringWithUTF8String:stringsJSON] : @"{}";
    NSString *langs = (langsJSON && *langsJSON) ?
        [NSString stringWithUTF8String:langsJSON] : @"[]";

    dispatch_async(dispatch_get_main_queue(), ^{
        if (g_settingsController) {
            [g_settingsController.window makeKeyAndOrderFront:nil];
            [NSApp activateIgnoringOtherApps:YES];
            return;
        }

        g_settingsController = [[WWSettingsController alloc] initWithJSON:json strings:strings languages:langs];
        [g_settingsController showWindow:nil];
        [NSApp activateIgnoringOtherApps:YES];
        [g_settingsController.window makeKeyAndOrderFront:nil];
    });
}

void bringSettingsToFront(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (g_settingsController) {
            [g_settingsController.window makeKeyAndOrderFront:nil];
            [NSApp activateIgnoringOtherApps:YES];
        }
    });
}
