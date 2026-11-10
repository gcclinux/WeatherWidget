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
extern char *settingsTestConnectionCB(const char *provider, const char *apiKey);
extern char *settingsSearchCityCB(const char *provider, const char *apiKey, const char *name, const char *region);

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

@interface WWSettingsController : NSWindowController <NSWindowDelegate, NSTextFieldDelegate>
@property (nonatomic, strong) NSString  *cfgJSON;   // current config JSON
@property (nonatomic, strong) NSMutableDictionary *cfg; // mutable parsed config
@property (nonatomic, strong) NSDictionary *strings;    // localized UI strings (key → text)
@property (nonatomic, strong) NSArray *languages;       // [{code,native,english,flag}] for the Language tab
@property (nonatomic, strong) NSMutableArray *langCards; // WWLangCard views (for selection redraw)
@property (nonatomic, strong) NSString *selectedLangCode; // currently selected locale code
@property (nonatomic, strong) NSTabView *tabView;

// Provider tab
@property (nonatomic, strong) NSPopUpButton     *providerPop;
@property (nonatomic, strong) NSButton          *getApiBtn;
@property (nonatomic, strong) NSTextField       *apiKeyField;
@property (nonatomic, strong) NSSecureTextField *apiKeySecureField;
@property (nonatomic, strong) NSButton          *toggleApiKeyBtn;
@property (nonatomic, assign) BOOL              apiKeyVisible;
@property (nonatomic, strong) NSButton          *checkConnBtn;
@property (nonatomic, strong) NSTextField       *intervalField;
@property (nonatomic, strong) NSView            *activationCard;
@property (nonatomic, strong) NSTextField       *activationKeyLbl;

// Locations tab
@property (nonatomic, strong) NSView        *demoBanner;
@property (nonatomic, strong) NSMutableArray *cities;      // array of NSMutableDictionary
@property (nonatomic, strong) NSStackView    *cityListStack;
@property (nonatomic, strong) NSTextField    *addNameField;
@property (nonatomic, strong) NSTextField    *addRegionField;
@property (nonatomic, strong) NSTextField    *addLatField;
@property (nonatomic, strong) NSTextField    *addLonField;
@property (nonatomic, strong) NSTextField    *addTZField;
@property (nonatomic, strong) NSButton       *searchCityBtn;
@property (nonatomic, strong) NSButton       *addCityBtn;    // enabled only when Auto-Fill succeeds
@property (nonatomic, strong) NSTextField    *searchStatusLbl; // search feedback label
@property (nonatomic, assign) BOOL            cityAutoFilled;

// Widget tab — grid content & display fields
@property (nonatomic, strong) NSPopUpButton *gridModePop;
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
@property (nonatomic, strong) NSButton *dfSunrise;
@property (nonatomic, strong) NSButton *dfSunset;
@property (nonatomic, strong) NSButton *dfClouds;
@property (nonatomic, strong) NSButton *dfVisibility;
@property (nonatomic, strong) NSButton *dfPrecipRate;
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

// Ticker tab
@property (nonatomic, strong) NSView   *tickerProCard;
@property (nonatomic, strong) NSButton *tickerProBtn;
@property (nonatomic, strong) NSButton *pfTicker;
@property (nonatomic, strong) NSButton *profEveryday;
@property (nonatomic, strong) NSButton *profActive;
@property (nonatomic, strong) NSButton *profFamily;
@property (nonatomic, strong) NSButton *profSensitive;
@property (nonatomic, strong) NSButton *profVentilation;
@property (nonatomic, strong) NSButton *tfFeedAlert;
@property (nonatomic, strong) NSButton *tfFeedToday;
@property (nonatomic, strong) NSButton *tfFeedPrecip;
@property (nonatomic, strong) NSButton *tfFeedTomorrow;
@property (nonatomic, strong) NSButton *tfFeedTips;
@property (nonatomic, strong) NSButton *tfAQI;
@property (nonatomic, strong) NSButton *tfPM25;
@property (nonatomic, strong) NSButton *tfPM10;
@property (nonatomic, strong) NSButton *tfO3;
@property (nonatomic, strong) NSButton *tfNO2;
@property (nonatomic, strong) NSButton *tfSO2;
@property (nonatomic, strong) NSButton *tfCO;
@property (nonatomic, strong) NSButton *tfNO;
@property (nonatomic, strong) NSButton *tfNH3;
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
@property (nonatomic, strong) NSButton      *noBgCheck;
@property (nonatomic, strong) NSButton      *autostartCheck;
@property (nonatomic, strong) NSButton      *iconThemeNew;
@property (nonatomic, strong) NSButton      *iconThemeOriginal;
@property (nonatomic, strong) NSPopUpButton *cornerPosPop;
@property (nonatomic, assign) BOOL          placementChanged;
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

    [_tabView addTabViewItem:[self buildLocationsTab]];
    [_tabView addTabViewItem:[self buildAppearanceTab]];
    [_tabView addTabViewItem:[self buildWidgetTab]];
    [_tabView addTabViewItem:[self buildTickerTab]];
    [_tabView addTabViewItem:[self buildProviderTab]];
    [_tabView addTabViewItem:[self buildLanguageTab]];
    [_tabView addTabViewItem:[self buildAboutTab]];

    if ([self hasLicense]) {
        [_tabView selectTabViewItemWithIdentifier:@"locations"];
    } else {
        [_tabView selectTabViewItemWithIdentifier:@"provider"];
    }

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

static NSString *wwMaskApiKey(NSString *key) {
    if (key.length == 0) return @"";
    NSMutableString *masked = [NSMutableString stringWithCapacity:key.length];
    for (NSUInteger i = 0; i < key.length; i++) {
        unichar c = [key characterAtIndex:i];
        [masked appendString:(c == '-') ? @"-" : @"•"];
    }
    return masked;
}

// ── Provider tab ─────────────────────────────────────────────────────────────

- (NSTabViewItem *)buildProviderTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"provider"];
    item.label = [self L:@"settings.tab.provider" fallback:@"Provider"];

    CGFloat docH = 650;
    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:docH buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = docH - 34;

        [doc addSubview:[s label:[s L:@"settings.provider.label" fallback:@"Provider"] frame:NSMakeRect(16, y, 72, 20) bold:NO]];
        s.providerPop = [[NSPopUpButton alloc] initWithFrame:NSMakeRect(92, y - 2, 322, 26) pullsDown:NO];
        [s.providerPop addItemsWithTitles:@[@"EasyWeatherWidget (Pro)", @"OpenWeatherMap (Free)"]];
        s.providerPop.target = s;
        s.providerPop.action = @selector(onProviderChanged:);
        [doc addSubview:s.providerPop];

        s.getApiBtn = [NSButton buttonWithTitle:[s L:@"settings.provider.getProApi" fallback:@"Get PRO API"]
            target:s action:@selector(onGetApiClicked:)];
        s.getApiBtn.frame = NSMakeRect(422, y - 2, 138, 26);
        [doc addSubview:s.getApiBtn];
        y -= 40;

        [doc addSubview:[s label:[s L:@"settings.provider.apiKeyLabel" fallback:@"API Key"] frame:NSMakeRect(16, y, 72, 20) bold:NO]];

        // Hidden (masked with bullets) by default:
        s.apiKeySecureField = [[NSSecureTextField alloc] initWithFrame:NSMakeRect(92, y - 2, 286, 24)];
        s.apiKeySecureField.placeholderString = [s L:@"settings.provider.apiKeyPlaceholder" fallback:@"API Key"];
        s.apiKeySecureField.delegate = s;
        s.apiKeySecureField.hidden = NO;
        [doc addSubview:s.apiKeySecureField];

        // Plain text field (revealed when eye button is clicked):
        s.apiKeyField = [[NSTextField alloc] initWithFrame:NSMakeRect(92, y - 2, 286, 24)];
        s.apiKeyField.placeholderString = [s L:@"settings.provider.apiKeyPlaceholder" fallback:@"API Key"];
        s.apiKeyField.delegate = s;
        s.apiKeyField.hidden = YES;
        [doc addSubview:s.apiKeyField];

        // Eye toggle button:
        s.toggleApiKeyBtn = [NSButton buttonWithTitle:@"" target:s action:@selector(onToggleApiKeyVisibility:)];
        s.toggleApiKeyBtn.frame = NSMakeRect(382, y - 2, 32, 26);
        [s updateToggleApiKeyBtnIcon];
        [doc addSubview:s.toggleApiKeyBtn];

        s.checkConnBtn = [NSButton buttonWithTitle:[s L:@"settings.provider.checkConnection" fallback:@"Check Connection"]
            target:s action:@selector(onCheckConnectionClicked:)];
        s.checkConnBtn.frame = NSMakeRect(422, y - 2, 138, 26);
        [doc addSubview:s.checkConnBtn];
        y -= 40;

        [doc addSubview:[s label:[s L:@"settings.interval.title" fallback:@"Refresh Interval"] frame:NSMakeRect(16, y, 200, 20) bold:NO]];
        s.intervalField = [[NSTextField alloc] initWithFrame:NSMakeRect(220, y - 2, 100, 24)];
        s.intervalField.placeholderString = @"e.g. 30";
        [doc addSubview:s.intervalField];
        y -= 52;

        // Refresh-rate note box (Free vs Pro), matching the Fyne/GTK UI.
        NSTextField *note = [s sublabel:
            [s L:@"settings.provider.note"
              fallback:@"Note:\nFree = 120 minutes refresh rate (limited).\nPro = 10 minutes refresh rate (unlimited)."]
            frame:NSMakeRect(16, y - 44, 544, 56)];
        [doc addSubview:note];
        y -= 64;

        // ── "Your Pro API Key" activation card ────────────────────────────────────
        // Shown when the configured provider is EasyWeatherWidget (Pro) with a
        // full-length (UUID, 36-char) key — mirrors GTK's showActivationCard.
        NSDictionary *api = s.cfg[@"apiConfig"];
        NSString *provider = api[@"provider"] ?: @"";
        NSString *apiKey = api[@"apiKey"] ?: @"";
        BOOL isProKey = [provider isEqualToString:@"easyweatherwidget"] && apiKey.length == 36;

        NSView *card = [[NSView alloc] initWithFrame:NSMakeRect(16, y - 96, 544, 96)];
        card.wantsLayer = YES;
        card.layer.cornerRadius = 8;
        card.layer.borderWidth = 1.0;
        card.layer.borderColor = [NSColor colorWithWhite:1.0 alpha:0.16].CGColor;
        card.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.05].CGColor;

        NSTextField *cardTitle = [s label:
            [s L:@"settings.provider.apiKeyActivation.title" fallback:@"Your Pro API Key"]
            frame:NSMakeRect(12, 68, 520, 20) bold:YES];
        [card addSubview:cardTitle];

        NSString *initialKeyStr = s.apiKeyVisible ? apiKey : wwMaskApiKey(apiKey);
        NSTextField *keyLbl = [NSTextField labelWithString:initialKeyStr];
        keyLbl.font = [NSFont fontWithName:@"Menlo" size:12] ?: [NSFont systemFontOfSize:12];
        keyLbl.textColor = [NSColor whiteColor];
        keyLbl.selectable = YES;
        keyLbl.frame = NSMakeRect(12, 46, 520, 18);
        [card addSubview:keyLbl];
        s.activationKeyLbl = keyLbl;

        NSTextField *msg = [s sublabel:
            [s L:@"settings.provider.apiKeyActivation.message"
              fallback:@"Keep this key safe. It is, or will be, activated once your subscription is "
                        "confirmed and will be disabled if the subscription is cancelled."]
            frame:NSMakeRect(12, 6, 520, 36)];
        [card addSubview:msg];

        card.hidden = !isProKey;
        [doc addSubview:card];
        s.activationCard = card;
        y -= 108;

        // ── Pro features note (persistent blue benefit line) ──────────────────────
        NSTextField *proNote = [s sublabel:
            [s L:@"settings.provider.proFeaturesNote"
              fallback:@"🍃 Always-On Desktop Air Quality & UV Monitor (Pro): Track live PM2.5, Wildfire Smoke, Smog (NO₂, O₃) & AQI across up to 5 cities with 10-min updates — ideal for runners, families, and allergy/asthma care."]
            frame:NSMakeRect(16, y - 50, 544, 48)];
        proNote.textColor = [NSColor colorWithRed:0.22 green:0.74 blue:0.97 alpha:1.0]; // #38bdf8
        [doc addSubview:proNote];
        y -= 62;

        // ── Testing notice banner (Free cities / free key note) ───────────────────
        NSView *testBox = [[NSView alloc] initWithFrame:NSMakeRect(16, y - 72, 544, 72)];
        testBox.wantsLayer = YES;
        testBox.layer.cornerRadius = 8;
        testBox.layer.borderWidth = 1.0;
        testBox.layer.borderColor = [NSColor colorWithRed:0.22 green:0.74 blue:0.97 alpha:0.35].CGColor;
        testBox.layer.backgroundColor = [NSColor colorWithRed:0.05 green:0.65 blue:0.91 alpha:0.10].CGColor;

        NSTextField *testTitle = [s label:
            [NSString stringWithFormat:@"🧪 %@", [s L:@"settings.provider.testingNotice.title" fallback:@"Testing Access (Pro API Key)"]]
            frame:NSMakeRect(12, 46, 520, 20) bold:YES];
        testTitle.textColor = [NSColor colorWithRed:0.22 green:0.74 blue:0.97 alpha:1.0];
        [testBox addSubview:testTitle];

        NSTextField *testMsg = [s sublabel:
            [s L:@"settings.provider.testingNotice.message"
              fallback:@"For testing purposes we have added 3 selected cities (London (GB), New York (US) & Amsterdam (NL)) with full access to all features using Pro API key = free. The API key = free provides access to all available features and data in the app for all of these 3 selected cities."]
            frame:NSMakeRect(12, 6, 520, 38)];
        testMsg.textColor = [NSColor colorWithRed:0.89 green:0.91 blue:0.94 alpha:1.0];
        [testBox addSubview:testMsg];

        [doc addSubview:testBox];
        [testBox release];
    }];
    scroll.frame = NSMakeRect(0, 0, 572, 600);
    item.view = scroll;
    return item;
}

static NSImage *wwEyeImage(BOOL slash) {
    if (@available(macOS 11.0, *)) {
        return [NSImage imageWithSystemSymbolName:slash ? @"eye.slash" : @"eye" accessibilityDescription:nil];
    }
    return nil;
}

- (NSString *)currentApiKey {
    NSString *key = self.apiKeyVisible ? self.apiKeyField.stringValue : self.apiKeySecureField.stringValue;
    if (key.length == 0 && self.apiKeyField.stringValue.length > 0) {
        key = self.apiKeyField.stringValue;
    }
    return [key stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
}

- (void)setApiKeyValue:(NSString *)val {
    NSString *v = val ?: @"";
    self.apiKeyField.stringValue = v;
    self.apiKeySecureField.stringValue = v;
}

- (void)updateToggleApiKeyBtnIcon {
    if (!self.toggleApiKeyBtn) return;
    NSImage *img = wwEyeImage(self.apiKeyVisible);
    if (img) {
        self.toggleApiKeyBtn.image = img;
        self.toggleApiKeyBtn.imagePosition = NSImageOnly;
        self.toggleApiKeyBtn.title = @"";
    } else {
        self.toggleApiKeyBtn.image = nil;
        self.toggleApiKeyBtn.imagePosition = NSNoImage;
        self.toggleApiKeyBtn.title = self.apiKeyVisible ? @"\U0001F648" : @"\U0001F441";
    }
    self.toggleApiKeyBtn.toolTip = self.apiKeyVisible ? @"Hide API Key" : @"Show API Key";
}

- (void)onToggleApiKeyVisibility:(id)sender {
    self.apiKeyVisible = !self.apiKeyVisible;
    BOOL hadFocus = (self.window.firstResponder == self.apiKeySecureField.currentEditor ||
                     self.window.firstResponder == self.apiKeyField.currentEditor);

    if (self.apiKeyVisible) {
        self.apiKeyField.stringValue = self.apiKeySecureField.stringValue;
        self.apiKeySecureField.hidden = YES;
        self.apiKeyField.hidden = NO;
        if (hadFocus) [self.window makeFirstResponder:self.apiKeyField];
    } else {
        self.apiKeySecureField.stringValue = self.apiKeyField.stringValue;
        self.apiKeyField.hidden = YES;
        self.apiKeySecureField.hidden = NO;
        if (hadFocus) [self.window makeFirstResponder:self.apiKeySecureField];
    }
    [self updateToggleApiKeyBtnIcon];
    [self updateActivationCardState];
}

- (void)updateGetApiBtnTitle {
    if (!self.getApiBtn) return;
    if (self.providerPop.indexOfSelectedItem == 1) {
        self.getApiBtn.title = [self L:@"settings.provider.getFreeApi" fallback:@"Get FREE API"];
    } else {
        self.getApiBtn.title = [self L:@"settings.provider.getProApi" fallback:@"Get PRO API"];
    }
}

- (void)updateActivationCardState {
    if (!self.activationCard) return;
    NSString *liveKey = [self currentApiKey];
    BOOL isProKey = (self.providerPop.indexOfSelectedItem == 0) && (liveKey.length == 36);
    if (isProKey && self.activationKeyLbl) {
        self.activationKeyLbl.stringValue = self.apiKeyVisible ? liveKey : wwMaskApiKey(liveKey);
    }
    self.activationCard.hidden = !isProKey;
}

- (void)onProviderChanged:(id)sender {
    [self updateGetApiBtnTitle];
    [self updateActivationCardState];
    [self updateLocationsUnlockedState];
    [self updatePollutionUnlockedState];
}

- (void)onGetApiClicked:(id)sender {
    if (self.providerPop.indexOfSelectedItem == 1) {
        [[NSWorkspace sharedWorkspace] openURL:[NSURL URLWithString:@"https://openweathermap.org/"]];
    } else {
        [self triggerGetProAPI];
    }
}

- (NSString *)formatConnectionFailedMsgWithProvider:(NSString *)provider detail:(NSString *)detail {
    NSString *tmpl = [self L:@"settings.provider.connectionFailedMsg"
                    fallback:@"Failed to connect to %s: %v"];
    if ([tmpl containsString:@"%s"] || [tmpl containsString:@"%v"]) {
        NSString *out = [tmpl stringByReplacingOccurrencesOfString:@"%s" withString:(provider ?: @"weather provider")];
        return [out stringByReplacingOccurrencesOfString:@"%v" withString:(detail ?: @"")];
    }
    return [NSString stringWithFormat:@"%@: %@", tmpl, detail ?: @""];
}

- (NSString *)formatConnectionSuccessMsgWithProvider:(NSString *)provider {
    NSString *tmpl = [self L:@"settings.provider.connectionSuccessMsg"
                    fallback:@"Successfully connected to %s and verified weather data access. Click Save below to apply and unlock Locations."];
    return [tmpl stringByReplacingOccurrencesOfString:@"%s" withString:(provider ?: @"weather provider")];
}

- (void)onCheckConnectionClicked:(id)sender {
    NSString *apiKey = [self currentApiKey];
    if (apiKey.length == 0) {
        [self showAlert:[self L:@"settings.provider.connectionFailedTitle" fallback:@"Connection Failed"]
                   info:[self L:@"settings.provider.apiKeyEmpty" fallback:@"API key cannot be empty"]];
        return;
    }

    _checkConnBtn.enabled = NO;
    _checkConnBtn.title = [self L:@"settings.provider.checkingConnection" fallback:@"Checking..."];

    BOOL isOwm = (_providerPop.indexOfSelectedItem == 1);
    NSString *provider = isOwm ? @"openweathermap" : @"easyweatherwidget";
    NSString *providerName = [(_providerPop.titleOfSelectedItem ?: (isOwm ? @"OpenWeatherMap (Free)" : @"EasyWeatherWidget (Pro)")) copy];
    NSString *capturedKey = [apiKey copy];
    WWSettingsController *weakSelf = self;

    dispatch_async(dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0), ^{
        char *cErr = settingsTestConnectionCB([provider UTF8String], [capturedKey UTF8String]);
        NSString *errStr = cErr ? [[NSString alloc] initWithUTF8String:cErr] : nil;
        if (cErr) free(cErr);

        dispatch_async(dispatch_get_main_queue(), ^{
            WWSettingsController *s = weakSelf;
            if (s) {
                s.checkConnBtn.enabled = YES;
                s.checkConnBtn.title = [s L:@"settings.provider.checkConnection" fallback:@"Check Connection"];

                if (errStr) {
                    if (s.activationCard) s.activationCard.hidden = YES;
                    NSString *msg = [s formatConnectionFailedMsgWithProvider:providerName detail:errStr];
                    [s showAlert:[s L:@"settings.provider.connectionFailedTitle" fallback:@"Connection Failed"] info:msg];
                } else {
                    [s updateActivationCardState];
                    [s updateLocationsUnlockedState];
                    [s updatePollutionUnlockedState];
                    NSString *msg = [s formatConnectionSuccessMsgWithProvider:providerName];
                    [s showAlert:[s L:@"settings.provider.connectionSuccessTitle" fallback:@"Connection Successful"]
                            info:msg];
                }
            }
            [errStr release];
            [capturedKey release];
            [providerName release];
        });
    });
}

- (void)triggerGetProAPI {
    [self.providerPop selectItemAtIndex:0];
    [self updateGetApiBtnTitle];

    NSString *currentKey = [self currentApiKey];
    NSString *clientRef = currentKey;
    if (currentKey.length != 36) {
        clientRef = [[[NSUUID UUID] UUIDString] lowercaseString];
        [self setApiKeyValue:clientRef];
    }

    NSMutableDictionary *apiCfg = [self.cfg[@"apiConfig"] mutableCopy] ?: [NSMutableDictionary dictionary];
    apiCfg[@"provider"] = @"easyweatherwidget";
    apiCfg[@"apiKey"] = clientRef;
    self.cfg[@"apiConfig"] = apiCfg;
    [apiCfg release];

    if (self.activationKeyLbl) {
        self.activationKeyLbl.stringValue = self.apiKeyVisible ? clientRef : wwMaskApiKey(clientRef);
    }
    if (self.activationCard) self.activationCard.hidden = NO;

    NSString *updated = [self buildUpdatedJSON];
    settingsSaveCB([updated UTF8String]);
    self.cfgJSON = updated;
    [self updateLocationsUnlockedState];
    [self updatePollutionUnlockedState];

    NSString *stripeURL = [NSString stringWithFormat:
        @"https://buy.stripe.com/bJe3cvaJOa650fQ8cPdZ603?client_reference_id=%@", clientRef];
    [[NSWorkspace sharedWorkspace] openURL:[NSURL URLWithString:stripeURL]];
}

- (void)onDemoBannerPro:(id)sender {
    [self triggerGetProAPI];
    [self.tabView selectTabViewItemWithIdentifier:@"provider"];
}

- (void)onDemoBannerFree:(id)sender {
    [self.providerPop selectItemAtIndex:1];
    [self updateGetApiBtnTitle];
    [self.tabView selectTabViewItemWithIdentifier:@"provider"];
    [self.window makeFirstResponder:(self.apiKeyVisible ? self.apiKeyField : (NSTextField *)self.apiKeySecureField)];
}

// ── Locations tab ────────────────────────────────────────────────────────────

- (NSTabViewItem *)buildLocationsTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"locations"];
    item.label = [self L:@"settings.tab.locations" fallback:@"Locations"];

    BOOL hasLic = [self hasLicense];
    CGFloat docH = hasLic ? 560 : 690;

    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:docH buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = docH - 34;

        // ── Clickable Demo Mode Banner (when no API key is configured) ────────
        if (!hasLic) {
            NSView *banner = [[NSView alloc] initWithFrame:NSMakeRect(16, y - 98, 524, 116)];
            banner.wantsLayer = YES;
            banner.layer.cornerRadius = 10;
            banner.layer.borderWidth = 1.5;
            banner.layer.borderColor = [NSColor colorWithRed:0.96 green:0.62 blue:0.04 alpha:0.65].CGColor;
            banner.layer.backgroundColor = [NSColor colorWithRed:0.96 green:0.62 blue:0.04 alpha:0.14].CGColor;

            NSTextField *bTitle = [s label:
                [s L:@"settings.locations.demoBanner.title" fallback:@"🔒 Demo Mode Active (Showing 3 Preview Cities)"]
                frame:NSMakeRect(12, 88, 500, 20) bold:YES];
            bTitle.textColor = [NSColor colorWithRed:0.98 green:0.75 blue:0.14 alpha:1.0];
            [banner addSubview:bTitle];

            NSTextField *bSub = [s sublabel:
                [s L:@"settings.locations.demoBanner.subtitle" fallback:@"To unlock custom cities, choose an option below:"]
                frame:NSMakeRect(12, 68, 500, 18)];
            [banner addSubview:bSub];

            NSButton *proBtn = [NSButton buttonWithTitle:
                [s L:@"settings.locations.demoBanner.proBtn" fallback:@"⚡ Get an Instant Pro Key ($1.99/mo)"]
                target:s action:@selector(onDemoBannerPro:)];
            proBtn.frame = NSMakeRect(12, 38, 500, 26);
            proBtn.bezelStyle = NSBezelStyleRounded;
            [banner addSubview:proBtn];

            NSButton *freeBtn = [NSButton buttonWithTitle:
                [s L:@"settings.locations.demoBanner.freeBtn" fallback:@"Enter a Free OpenWeatherMap Key in Data Provider"]
                target:s action:@selector(onDemoBannerFree:)];
            freeBtn.frame = NSMakeRect(12, 8, 500, 26);
            freeBtn.bezelStyle = NSBezelStyleRounded;
            [banner addSubview:freeBtn];

            [doc addSubview:banner];
            s.demoBanner = banner;
            y -= 128;
        }

        [doc addSubview:[s label:[s L:@"settings.locations.savedTitle" fallback:@"Saved Cities"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 22;
        [doc addSubview:[s sublabel:[s L:@"settings.locations.savedSubtitle" fallback:@"Manage your tracked locations (1\u20135 cities)."]
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

        // Name row: entry + Auto-Fill button side by side.
        [doc addSubview:[s label:[s L:@"settings.locations.nameLabel" fallback:@"Name"] frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addNameField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 310, 24)];
        s.addNameField.placeholderString = [s L:@"settings.locations.namePlaceholder" fallback:@"City name"];
        s.addNameField.delegate = s;
        s.addNameField.target = s;
        s.addNameField.action = @selector(onNameFieldEnter:);
        if (!hasLic) s.addNameField.enabled = NO;
        [doc addSubview:s.addNameField];

        NSButton *searchBtn = [NSButton buttonWithTitle:[s L:@"settings.locations.searchBtn" fallback:@"Auto-Fill"]
            target:s action:@selector(onSearchCity:)];
        searchBtn.frame = NSMakeRect(426, y - 2, 114, 26);
        searchBtn.toolTip = [s L:@"settings.locations.searchBtn" fallback:@"Auto-Fill"];
        if (!hasLic) searchBtn.enabled = NO;
        s.searchCityBtn = searchBtn;
        [doc addSubview:searchBtn];
        y -= 32;

        // Region, Country row (manual entry — required).
        [doc addSubview:[s label:[s L:@"settings.locations.regionLabel" fallback:@"Region, Country"] frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addRegionField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 430, 24)];
        s.addRegionField.placeholderString = [s L:@"settings.locations.regionPlaceholder" fallback:@"Region, Country (e.g.  CA, US)"];
        s.addRegionField.delegate = s;
        if (!hasLic) s.addRegionField.enabled = NO;
        [doc addSubview:s.addRegionField];
        y -= 32;

        // Coordinates row (populated by Auto-Fill; read-only).
        [doc addSubview:[s label:[s L:@"settings.locations.latLabel" fallback:@"Latitude"] frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addLatField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 200, 24)];
        s.addLatField.placeholderString = [s L:@"settings.locations.latPlaceholder" fallback:@"Latitude (optional)"];
        s.addLatField.editable = NO;
        s.addLatField.selectable = YES;
        [doc addSubview:s.addLatField];
        [doc addSubview:[s label:[s L:@"settings.locations.lonLabel" fallback:@"Longitude"] frame:NSMakeRect(320, y, 80, 20) bold:NO]];
        s.addLonField = [[NSTextField alloc] initWithFrame:NSMakeRect(400, y - 2, 140, 24)];
        s.addLonField.placeholderString = [s L:@"settings.locations.lonPlaceholder" fallback:@"Longitude (optional)"];
        s.addLonField.editable = NO;
        s.addLonField.selectable = YES;
        [doc addSubview:s.addLonField];
        y -= 32;

        // Timezone row (populated by Auto-Fill; read-only).
        [doc addSubview:[s label:[s L:@"settings.locations.tzLabel" fallback:@"Timezone"] frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addTZField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 430, 24)];
        s.addTZField.placeholderString = [s L:@"settings.locations.tzPlaceholder" fallback:@"America/Los_Angeles"];
        s.addTZField.editable = NO;
        s.addTZField.selectable = YES;
        [doc addSubview:s.addTZField];
        y -= 36;

        // Search-feedback / status label.
        s.searchStatusLbl = [NSTextField labelWithString:@""];
        s.searchStatusLbl.frame = NSMakeRect(16, y, 420, 18);
        s.searchStatusLbl.font = [NSFont systemFontOfSize:11];
        s.searchStatusLbl.textColor = [NSColor secondaryLabelColor];
        [doc addSubview:s.searchStatusLbl];
        y -= 30;

        // Add City button — disabled until Auto-Fill populates all 5 fields.
        NSButton *addBtn = [NSButton buttonWithTitle:[NSString stringWithFormat:@"\uff0b %@",
            [s L:@"settings.locations.addBtn" fallback:@"Add City"]]
            target:s action:@selector(onAddCity:)];
        addBtn.frame = NSMakeRect(436, y, 104, 28);
        addBtn.enabled = NO; // enabled dynamically by -updateAddCityBtn
        s.addCityBtn = addBtn;
        [doc addSubview:addBtn];
    }];
    scroll.frame = NSMakeRect(0, 0, 572, 600);
    item.view = scroll;
    return item;
}

- (void)onNameFieldEnter:(id)sender {
    if (self.searchCityBtn && self.searchCityBtn.enabled) {
        [self onSearchCity:self.searchCityBtn];
    }
}

// Enables or disables the Add City button based on whether Auto-Fill has
// populated all five required fields (Name, Region/Country, Latitude, Longitude, Timezone).
- (void)updateAddCityBtn {
    if (!self.addCityBtn) return;
    if (![self hasLicense] || !self.cityAutoFilled) {
        self.addCityBtn.enabled = NO;
        return;
    }
    NSString *name   = [self.addNameField.stringValue   stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *region = [self.addRegionField.stringValue stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *lat    = [self.addLatField.stringValue    stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *lon    = [self.addLonField.stringValue    stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *tz     = [self.addTZField.stringValue     stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    self.addCityBtn.enabled = (name.length > 0 && region.length > 0 &&
                               lat.length > 0 && lon.length > 0 && tz.length > 0);
}

- (void)updateLocationsUnlockedState {
    BOOL hasLic = [self hasLicense];
    if (self.demoBanner) {
        self.demoBanner.hidden = hasLic;
    }
    if (self.addNameField)   self.addNameField.enabled   = hasLic;
    if (self.addRegionField) self.addRegionField.enabled = hasLic;
    if (self.searchCityBtn)  self.searchCityBtn.enabled  = hasLic;
    [self refreshCityList];
    [self updateAddCityBtn];
}

- (void)updateTickerUnlockedState {
    BOOL pro = [self isPro];
    if (self.tickerProCard) {
        self.tickerProCard.hidden = pro;
    }
    if (self.pfTicker) self.pfTicker.enabled = pro;
    NSButton *profs[] = {
        self.profEveryday, self.profActive, self.profFamily, self.profSensitive, self.profVentilation
    };
    for (size_t i = 0; i < sizeof(profs)/sizeof(profs[0]); i++) {
        if (profs[i]) profs[i].enabled = pro;
    }
    NSButton *feeds[] = {
        self.tfFeedAlert, self.tfFeedToday, self.tfFeedPrecip, self.tfFeedTomorrow, self.tfFeedTips
    };
    for (size_t i = 0; i < sizeof(feeds)/sizeof(feeds[0]); i++) {
        if (feeds[i]) feeds[i].enabled = pro;
    }
    NSButton *tPolls[] = {
        self.tfAQI, self.tfPM25, self.tfPM10, self.tfO3, self.tfNO2, self.tfSO2, self.tfCO, self.tfNO, self.tfNH3
    };
    for (size_t i = 0; i < sizeof(tPolls)/sizeof(tPolls[0]); i++) {
        if (tPolls[i]) tPolls[i].enabled = pro;
    }
}

- (void)updatePollutionUnlockedState {
    BOOL pro = [self isPro];
    NSButton *buttons[] = {
        self.pfAQI, self.pfCO, self.pfNO, self.pfNO2, self.pfO3,
        self.pfSO2, self.pfNH3, self.pfPM25, self.pfPM10
    };
    for (size_t i = 0; i < sizeof(buttons) / sizeof(buttons[0]); i++) {
        if (buttons[i]) {
            buttons[i].enabled = pro;
        }
    }
    [self updateTickerUnlockedState];
}

// NSControlTextDidChangeNotification delegate — fired whenever the user edits
// addNameField or addRegionField. Invalidates any previous Auto-Fill coordinates
// so the user must run Auto-Fill again before Add City becomes enabled.
- (void)controlTextDidChange:(NSNotification *)aNotification {
    id obj = [aNotification object];
    if (obj == self.apiKeyField || obj == self.apiKeySecureField) {
        if (obj == self.apiKeySecureField) {
            self.apiKeyField.stringValue = self.apiKeySecureField.stringValue;
        } else {
            self.apiKeySecureField.stringValue = self.apiKeyField.stringValue;
        }
        [self updateActivationCardState];
        [self updateLocationsUnlockedState];
        return;
    }
    if ((obj == self.addNameField || obj == self.addRegionField) && self.cityAutoFilled) {
        self.cityAutoFilled = NO;
        self.addLatField.stringValue = @"";
        self.addLonField.stringValue = @"";
        self.addTZField.stringValue = @"";
        self.searchStatusLbl.stringValue = @"";
    }
    [self updateAddCityBtn];
}

// ── Search API action ─────────────────────────────────────────────────────────
// Geocodes the city entered in addNameField (and optionally addRegionField)
// using the configured weather provider's geocoding API, then populates the
// Latitude, Longitude, and Timezone fields.  The request runs on a background
// thread; UI updates are dispatched back to the main thread.
- (void)onSearchCity:(id)sender {
    if (![self hasLicense]) {
        [self showAlert:[self L:@"settings.locations.addTitle" fallback:@"Add New City"]
                   info:[self L:@"error.settings.licenseRequired"
                            fallback:@"Adding cities requires a Pro API key. Configure one in the Provider tab."]];
        return;
    }

    NSString *name = [self.addNameField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    if (name.length == 0) {
        [self showAlert:[self L:@"settings.locations.searchBtn" fallback:@"Search API"]
                   info:[self L:@"error.settings.cityNameRequiredSearch"
                            fallback:@"City name is required to search."]];
        return;
    }

    NSDictionary *api = self.cfg[@"apiConfig"];
    NSString *apiKey = [self currentApiKey];
    if (apiKey.length == 0) {
        apiKey = api[@"apiKey"] ?: @"";
    }
    if (apiKey.length == 0) {
        [self showAlert:[self L:@"settings.locations.searchBtn" fallback:@"Auto-Fill"]
                   info:[self L:@"error.settings.apiKeyRequired"
                            fallback:@"API Key is required in the settings above to search."]];
        return;
    }

    NSString *provider = (self.providerPop && self.providerPop.indexOfSelectedItem == 1)
        ? @"openweathermap"
        : (api[@"provider"] ?: @"easyweatherwidget");
    if (self.providerPop && self.providerPop.indexOfSelectedItem == 0) {
        provider = @"easyweatherwidget";
    }
    NSString *region = [self.addRegionField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];

    // For the EasyWeatherWidget provider, Region is required to narrow the search.
    if ([provider isEqualToString:@"easyweatherwidget"] && region.length == 0) {
        [self showAlert:[self L:@"settings.locations.searchBtn" fallback:@"Search API"]
                   info:[self L:@"error.settings.regionRequiredEww"
                            fallback:@"Region, Country is required to search via EasyWeatherWidget."]];
        return;
    }

    // Disable the search button and show the searching label while in-flight.
    NSButton *btn = (NSButton *)sender;
    btn.enabled = NO;
    self.searchStatusLbl.stringValue = [self L:@"settings.locations.searching" fallback:@"Searching..."];
    self.searchStatusLbl.textColor = [NSColor secondaryLabelColor];

    // Capture everything needed for the background task.
    NSString *capturedProvider = [provider copy];
    NSString *capturedKey      = [apiKey copy];
    NSString *capturedName     = [name copy];
    NSString *capturedRegion   = [region copy];
    WWSettingsController *weakSelf = self;

    dispatch_async(dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0), ^{
        // NOTE (MRC): this file is compiled without ARC, so __block object
        // variables are NOT retained by the block. Anything assigned here is an
        // autoreleased string owned by the background thread's (or the
        // NSURLSession completion handler's) autorelease pool. That pool drains
        // before the main-thread block below runs, which would leave these
        // pointers dangling and crash when the fields are populated. So every
        // result string is retained (via -copy / -retain) at assignment and
        // released again after the main-thread block has consumed it.
        __block NSString *foundName   = nil;
        __block NSString *foundRegion = nil;
        __block NSString *foundLat    = nil;
        __block NSString *foundLon    = nil;
        __block NSString *foundTZ     = nil;
        __block NSString *errorMsg    = nil;

        char *cRes = settingsSearchCityCB(
            [capturedProvider UTF8String],
            [capturedKey UTF8String],
            [capturedName UTF8String],
            [capturedRegion UTF8String]);
        if (!cRes) {
            errorMsg = [@"No data" copy];
        } else {
            NSData *resData = [NSData dataWithBytes:cRes length:strlen(cRes)];
            free(cRes);
            NSDictionary *res = [NSJSONSerialization JSONObjectWithData:resData options:0 error:nil];
            NSString *errField = [res isKindOfClass:[NSDictionary class]] ? res[@"error"] : nil;
            BOOL notFound = [res isKindOfClass:[NSDictionary class]] && [res[@"notFound"] boolValue];
            if (notFound) {
                errorMsg = [[NSString stringWithFormat:@"no city found matching \u2018%@\u2019", capturedName] copy];
            } else if (errField.length > 0) {
                errorMsg = [errField copy];
            } else if ([res isKindOfClass:[NSDictionary class]]) {
                NSString *rName   = res[@"name"];
                NSString *rRegion = res[@"region"];
                NSNumber *rLat    = res[@"latitude"];
                NSNumber *rLon    = res[@"longitude"];
                NSString *rTZ     = res[@"timezone"];
                if (rName.length > 0 && rLat && rLon) {
                    foundName   = [rName copy];
                    foundRegion = [(rRegion ?: @"") copy];
                    foundLat    = [[NSString stringWithFormat:@"%f", [rLat doubleValue]] copy];
                    foundLon    = [[NSString stringWithFormat:@"%f", [rLon doubleValue]] copy];
                    foundTZ     = [(rTZ ?: @"") copy];
                } else {
                    errorMsg = [[NSString stringWithFormat:@"no city found matching \u2018%@\u2019", capturedName] copy];
                }
            } else {
                errorMsg = [@"Invalid search response" copy];
            }
        }
        [capturedProvider release];
        [capturedKey release];
        [capturedName release];
        [capturedRegion release];

        // ── Back on the main thread: populate fields or show error ────────────
        dispatch_async(dispatch_get_main_queue(), ^{
            WWSettingsController *s = weakSelf;
            if (s) {
                btn.enabled = [s hasLicense];
                if (errorMsg) {
                    s.cityAutoFilled = NO;
                    s.searchStatusLbl.textColor = [NSColor systemRedColor];
                    s.searchStatusLbl.stringValue = errorMsg;
                    [s updateAddCityBtn];
                } else if (foundTZ.length == 0) {
                    s.addNameField.stringValue   = foundName   ?: @"";
                    s.addRegionField.stringValue = foundRegion ?: @"";
                    s.addLatField.stringValue    = foundLat    ?: @"";
                    s.addLonField.stringValue    = foundLon    ?: @"";
                    s.addTZField.stringValue     = @"";
                    s.cityAutoFilled = NO;
                    NSString *warn = [NSString stringWithFormat:@"⚠ %@, %@: could not determine timezone, city cannot be added",
                        foundName ?: @"", foundRegion ?: @""];
                    s.searchStatusLbl.textColor = [NSColor systemOrangeColor];
                    s.searchStatusLbl.stringValue = warn;
                    [s updateAddCityBtn];
                } else {
                    s.addNameField.stringValue   = foundName   ?: @"";
                    s.addRegionField.stringValue = foundRegion ?: @"";
                    s.addLatField.stringValue    = foundLat    ?: @"";
                    s.addLonField.stringValue    = foundLon    ?: @"";
                    s.addTZField.stringValue     = foundTZ;
                    s.cityAutoFilled = YES;
                    NSString *summary = [NSString stringWithFormat:@"\u2713 %@, %@",
                        foundName ?: @"", foundRegion ?: @""];
                    s.searchStatusLbl.textColor = [NSColor systemGreenColor];
                    s.searchStatusLbl.stringValue = summary;
                    [s updateAddCityBtn];
                }
            }
            // Balance the +1 retain each result took at assignment (MRC).
            // -release on a nil pointer or a string literal is a safe no-op.
            [foundName release];
            [foundRegion release];
            [foundLat release];
            [foundLon release];
            [foundTZ release];
            [errorMsg release];
        });
    });
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
    if (self.apiKeyField || self.apiKeySecureField) {
        NSString *liveKey = [self currentApiKey];
        if (liveKey.length > 0) return YES;
    }
    NSDictionary *api = self.cfg[@"apiConfig"];
    NSString *key = api[@"apiKey"];
    if (key && key.length > 0) return YES;
    NSDictionary *db = self.cfg[@"databaseConfig"];
    NSString *host = db[@"host"];
    return (host && host.length > 0);
}

- (BOOL)isPro {
    if (self.providerPop && (self.apiKeyField || self.apiKeySecureField)) {
        NSString *liveKey = [self currentApiKey];
        if (self.providerPop.indexOfSelectedItem == 0 && liveKey.length > 0) return YES;
    }
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

    // All five fields are required before a city can be added.
    NSString *name = [self.addNameField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *region = [self.addRegionField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *latStr = [self.addLatField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *lonStr = [self.addLonField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *tzStr = [self.addTZField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];

    if (name.length == 0) {
        [self showAlert:[self L:@"settings.locations.addTitle" fallback:@"Add New City"]
                   info:[self L:@"error.settings.cityNameRequired" fallback:@"City name is required."]];
        return;
    }
    // Guard: button should already be disabled if fields are empty, but be defensive.
    if (region.length == 0 || latStr.length == 0 || lonStr.length == 0 || tzStr.length == 0) {
        [self showAlert:[self L:@"settings.locations.addTitle" fallback:@"Add New City"]
                   info:[self L:@"error.settings.cityNameRequiredSearch"
                            fallback:@"Use Search API to populate Latitude, Longitude, and Timezone before adding."]];
        return;
    }

    NSInteger maxCities = [self isPro] ? 5 : 3;
    if ((NSInteger)self.cities.count >= maxCities) {
        [self showAlert:@"City limit reached"
                   info:[NSString stringWithFormat:@"Your tier allows up to %ld cities.", (long)maxCities]];
        return;
    }

    NSMutableDictionary *city = [NSMutableDictionary dictionary];
    city[@"name"]     = name;
    city[@"region"]   = region;
    city[@"timezone"] = tzStr;
    if (latStr.length > 0) city[@"latitude"]  = @([latStr doubleValue]);
    if (lonStr.length > 0) city[@"longitude"] = @([lonStr doubleValue]);

    [self.cities addObject:city];

    // Clear the form and reset the Add City button.
    self.cityAutoFilled = NO;
    self.addNameField.stringValue   = @"";
    self.addRegionField.stringValue = @"";
    self.addLatField.stringValue    = @"";
    self.addLonField.stringValue    = @"";
    self.addTZField.stringValue     = @"";
    self.searchStatusLbl.stringValue = @"";
    [self updateAddCityBtn];

    [self refreshCityList];
}

- (void)showAlert:(NSString *)title info:(NSString *)info {
    NSAlert *a = [[NSAlert alloc] init];
    a.messageText = title;
    a.informativeText = info;
    [a addButtonWithTitle:@"OK"];
    [a beginSheetModalForWindow:self.window completionHandler:nil];
}

// ── Widget tab (Data Settings) ────────────────────────────────────────────────

- (NSTabViewItem *)buildWidgetTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"widget"];
    item.label = [self L:@"settings.tab.widget" fallback:@"Widget"];

    CGFloat docH = 760;
    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:docH buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = docH - 34;

        // Panel display fields.
        [doc addSubview:[s label:[s L:@"settings.display.title" fallback:@"Panel Display"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 20;
        [doc addSubview:[s sublabel:[s L:@"settings.display.subtitle" fallback:@"Choose which elements appear on each city panel."]
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 28;

        // ── Grid Content (Profile) Selector ──
        [doc addSubview:[s label:[s L:@"settings.gridMode.title" fallback:@"Grid Content"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 20;
        [doc addSubview:[s sublabel:[s L:@"settings.gridMode.subtitle" fallback:@"Choose what the 6-tile metrics grid displays."]
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 28;

        s.gridModePop = [[NSPopUpButton alloc] initWithFrame:NSMakeRect(16, y - 2, 320, 26) pullsDown:NO];
        [s.gridModePop addItemWithTitle:[s L:@"settings.gridMode.live" fallback:@"Live Weather (Real-Time)"]];
        [s.gridModePop addItemWithTitle:[s L:@"settings.gridMode.today" fallback:@"Today's Forecast"]];
        [s.gridModePop addItemWithTitle:[s L:@"settings.gridMode.tomorrow" fallback:@"Tomorrow's Forecast"]];
        s.gridModePop.target = s;
        s.gridModePop.action = @selector(onGridModeChanged:);
        [doc addSubview:s.gridModePop];
        y -= 36;

        // ── Weather Metrics ──
        [doc addSubview:[s label:[s L:@"settings.display.weatherMetrics" fallback:@"Weather Metrics"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 26;

        CGFloat colW = 130, rowH = 26;
        CGFloat cx0 = 16;
        s.dfCity       = [s checkbox:[s L:@"settings.display.city" fallback:@"City"]        frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.dfIcon       = [s checkbox:[s L:@"settings.display.icon" fallback:@"Icon"]        frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.dfTemp       = [s checkbox:[s L:@"settings.display.temp" fallback:@"Temperature"] frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.dfDesc       = [s checkbox:[s L:@"settings.display.desc" fallback:@"Description"] frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        y -= rowH;
        s.dfTime       = [s checkbox:[s L:@"settings.display.time" fallback:@"Time"]         frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.dfDate       = [s checkbox:[s L:@"settings.display.date" fallback:@"Date"]         frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.dfHumidity   = [s checkbox:[s L:@"settings.display.humidity" fallback:@"Humidity"] frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.dfWind       = [s checkbox:[s L:@"settings.display.wind" fallback:@"Wind"]         frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        y -= rowH;
        s.dfWindGust   = [s checkbox:[s L:@"settings.display.windGust" fallback:@"Wind Gust"] frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.dfDewPoint   = [s checkbox:[s L:@"settings.display.dewPoint" fallback:@"Dew Point"] frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.dfPressure   = [s checkbox:[s L:@"settings.display.pressure" fallback:@"Pressure"]  frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.dfUVIndex    = [s checkbox:[s L:@"settings.display.uvIndex" fallback:@"UV Index"]   frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        y -= rowH;
        s.dfSunrise    = [s checkbox:[s L:@"settings.display.sunrise" fallback:@"Sunrise"]     frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.dfSunset     = [s checkbox:[s L:@"settings.display.sunset" fallback:@"Sunset"]       frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.dfClouds     = [s checkbox:[s L:@"settings.display.clouds" fallback:@"Cloud Cover"] frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.dfVisibility = [s checkbox:[s L:@"settings.display.visibility" fallback:@"Visibility"] frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        y -= rowH;
        s.dfPrecipRate = [s checkbox:[s L:@"settings.display.precipRate" fallback:@"Precip. Rate"] frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];

        for (NSButton *b in @[s.dfCity, s.dfIcon, s.dfTemp, s.dfDesc,
                              s.dfTime, s.dfDate, s.dfHumidity, s.dfWind,
                              s.dfWindGust, s.dfDewPoint, s.dfPressure, s.dfUVIndex,
                              s.dfSunrise, s.dfSunset, s.dfClouds, s.dfVisibility, s.dfPrecipRate]) {
            [doc addSubview:b];
        }
        y -= 34;

        // Divider.
        NSBox *div1 = [[NSBox alloc] initWithFrame:NSMakeRect(16, y, 524, 1)];
        div1.boxType = NSBoxSeparator;
        [doc addSubview:div1];
        [div1 release];
        y -= 26;

        // Air Quality & Pollution section.
        [doc addSubview:[s label:[s L:@"settings.pollution.title" fallback:@"Air Quality & Pollution"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 20;
        [doc addSubview:[s sublabel:[s L:@"settings.pollution.subtitle" fallback:@"Display live air pollutant concentrations on each card."]
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 26;

        s.pfAQI  = [s checkbox:[s L:@"settings.pollution.aqi" fallback:@"Air Quality Index (AQI)"] frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.pfCO   = [s checkbox:[s L:@"settings.pollution.co" fallback:@"Carbon Monoxide (CO)"]     frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.pfNO   = [s checkbox:[s L:@"settings.pollution.no" fallback:@"Nitrogen Monoxide (NO)"]   frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        y -= rowH;
        s.pfNO2  = [s checkbox:[s L:@"settings.pollution.no2" fallback:@"Nitrogen Dioxide (NO₂)"]  frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.pfO3   = [s checkbox:[s L:@"settings.pollution.o3" fallback:@"Ozone (O₃)"]               frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.pfSO2  = [s checkbox:[s L:@"settings.pollution.so2" fallback:@"Sulfur Dioxide (SO₂)"]   frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        y -= rowH;
        s.pfNH3  = [s checkbox:[s L:@"settings.pollution.nh3" fallback:@"Ammonia (NH₃)"]           frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.pfPM25 = [s checkbox:[s L:@"settings.pollution.pm2_5" fallback:@"PM2.5 (Fine particles)"] frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.pfPM10 = [s checkbox:[s L:@"settings.pollution.pm10" fallback:@"PM10 (Coarse particles)"] frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];

        BOOL pro = [s isPro];
        for (NSButton *b in @[s.pfAQI, s.pfCO, s.pfNO, s.pfNO2, s.pfO3, s.pfSO2, s.pfNH3, s.pfPM25, s.pfPM10]) {
            b.enabled = pro;
            [doc addSubview:b];
        }
        y -= 30;

        NSTextField *pollutionProNote = [s sublabel:
            [s L:@"settings.pollution.proNote"
              fallback:@"🍃 Always-On Desktop Air Quality & UV Monitor (Pro): Track live PM2.5, Wildfire Smoke, Smog (NO₂, O₃) & AQI across up to 5 cities with 10-min updates — ideal for runners, families, and allergy/asthma care."]
            frame:NSMakeRect(16, y - 40, 524, 46)];
        pollutionProNote.textColor = [NSColor colorWithRed:0.22 green:0.74 blue:0.97 alpha:1.0];
        [doc addSubview:pollutionProNote];
        y -= 54;

        // Divider.
        NSBox *div2 = [[NSBox alloc] initWithFrame:NSMakeRect(16, y, 524, 1)];
        div2.boxType = NSBoxSeparator;
        [doc addSubview:div2];
        [div2 release];
        y -= 26;

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

- (void)onGridModeChanged:(id)sender {
    BOOL live = (self.gridModePop.indexOfSelectedItem == 0);
    NSButton *liveMetrics[] = {
        self.dfHumidity, self.dfWind, self.dfWindGust, self.dfDewPoint,
        self.dfPressure, self.dfUVIndex, self.dfSunrise, self.dfSunset,
        self.dfClouds, self.dfVisibility, self.dfPrecipRate
    };
    for (size_t i = 0; i < sizeof(liveMetrics)/sizeof(liveMetrics[0]); i++) {
        if (liveMetrics[i]) liveMetrics[i].enabled = live;
    }
}

// ── Ticker tab ────────────────────────────────────────────────────────────────

- (NSTabViewItem *)buildTickerTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"ticker"];
    item.label = [self L:@"settings.tab.ticker" fallback:@"Ticker Settings"];

    CGFloat docH = 680;
    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:docH buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = docH - 34;

        // Title and Subtitle.
        [doc addSubview:[s label:[s L:@"settings.ticker.title" fallback:@"Ticker Settings"] frame:NSMakeRect(16, y, 400, 20) bold:YES]];
        y -= 20;
        [doc addSubview:[s sublabel:[s L:@"settings.ticker.subtitle" fallback:@"Configure the scrolling text ticker displayed on each card."]
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 28;

        // Demo/Pro Banner if unlicensed.
        BOOL pro = [s isPro];
        NSView *proBanner = [[NSView alloc] initWithFrame:NSMakeRect(16, y - 68, 524, 68)];
        proBanner.wantsLayer = YES;
        proBanner.layer.cornerRadius = 8;
        proBanner.layer.borderWidth = 1.0;
        proBanner.layer.borderColor = [NSColor colorWithRed:0.96 green:0.62 blue:0.04 alpha:0.65].CGColor;
        proBanner.layer.backgroundColor = [NSColor colorWithRed:0.96 green:0.62 blue:0.04 alpha:0.14].CGColor;

        NSTextField *pbTitle = [s label:[s L:@"settings.pollution.proRequired" fallback:@"PRO Feature Required"]
                                  frame:NSMakeRect(12, 40, 500, 20) bold:YES];
        pbTitle.textColor = [NSColor colorWithRed:0.98 green:0.75 blue:0.14 alpha:1.0];
        [proBanner addSubview:pbTitle];

        NSButton *getProBtn = [NSButton buttonWithTitle:[s L:@"settings.locations.demoBanner.proBtn" fallback:@"⚡ Get an Instant Pro Key ($1.99/mo)"]
                                                 target:s action:@selector(onTickerGetPro:)];
        getProBtn.frame = NSMakeRect(12, 10, 500, 26);
        getProBtn.bezelStyle = NSBezelStyleRounded;
        [proBanner addSubview:getProBtn];

        proBanner.hidden = pro;
        [doc addSubview:proBanner];
        s.tickerProCard = proBanner;
        s.tickerProBtn = getProBtn;
        y -= 78;

        // Enable Ticker Checkbox.
        s.pfTicker = [s checkbox:[s L:@"settings.ticker.enable" fallback:@"Enable Info Ticker"]
                           frame:NSMakeRect(16, y, 320, 20)];
        s.pfTicker.enabled = pro;
        [doc addSubview:s.pfTicker];
        y -= 32;

        // ── Health Profiles ──
        NSBox *div1 = [[NSBox alloc] initWithFrame:NSMakeRect(16, y, 524, 1)];
        div1.boxType = NSBoxSeparator;
        [doc addSubview:div1];
        [div1 release];
        y -= 24;

        [doc addSubview:[s label:[s L:@"settings.ticker.profilesTitle" fallback:@"Health Profiles"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 26;

        CGFloat colW2 = 250, rowH = 24;
        s.profEveryday    = [s checkbox:[s L:@"settings.pollution.profile.everyday" fallback:@"Everyday Outdoors"] frame:NSMakeRect(16, y, colW2, 20)];
        s.profActive      = [s checkbox:[s L:@"settings.pollution.profile.active" fallback:@"Runner & Cyclist"]    frame:NSMakeRect(16 + colW2, y, colW2, 20)];
        y -= rowH;
        s.profFamily      = [s checkbox:[s L:@"settings.pollution.profile.family" fallback:@"Family & School Run"] frame:NSMakeRect(16, y, colW2, 20)];
        s.profSensitive   = [s checkbox:[s L:@"settings.pollution.profile.sensitive" fallback:@"Sensitive Airways"] frame:NSMakeRect(16 + colW2, y, colW2, 20)];
        y -= rowH;
        s.profVentilation = [s checkbox:[s L:@"settings.pollution.profile.ventilation" fallback:@"Open Windows"]    frame:NSMakeRect(16, y, colW2, 20)];
        y -= 30;

        // ── Alerts & News Feed ──
        NSBox *div2 = [[NSBox alloc] initWithFrame:NSMakeRect(16, y, 524, 1)];
        div2.boxType = NSBoxSeparator;
        [doc addSubview:div2];
        [div2 release];
        y -= 24;

        [doc addSubview:[s label:[s L:@"settings.ticker.feedTitle" fallback:@"Alerts & News Feed"] frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 26;

        s.tfFeedAlert    = [s checkbox:[s L:@"settings.ticker.alert" fallback:@"Severe Weather Alerts"] frame:NSMakeRect(16, y, colW2, 20)];
        s.tfFeedToday    = [s checkbox:[s L:@"settings.ticker.newsToday" fallback:@"Today's Outlook Digest"] frame:NSMakeRect(16 + colW2, y, colW2, 20)];
        y -= rowH;
        s.tfFeedPrecip   = [s checkbox:[s L:@"settings.ticker.newsPrecip" fallback:@"Precipitation Watch"] frame:NSMakeRect(16, y, colW2, 20)];
        s.tfFeedTomorrow = [s checkbox:[s L:@"settings.ticker.newsTomorrow" fallback:@"Tomorrow's Forecast Preview"] frame:NSMakeRect(16 + colW2, y, colW2, 20)];
        y -= rowH;
        s.tfFeedTips     = [s checkbox:[s L:@"settings.ticker.weatherTips" fallback:@"Weather Tips"] frame:NSMakeRect(16, y, colW2, 20)];
        y -= 30;

        // ── Air Quality & Pollution for Ticker ──
        NSBox *div3 = [[NSBox alloc] initWithFrame:NSMakeRect(16, y, 524, 1)];
        div3.boxType = NSBoxSeparator;
        [doc addSubview:div3];
        [div3 release];
        y -= 24;

        [doc addSubview:[s label:[s L:@"settings.ticker.pollutantsTitle" fallback:@"Air Quality & Pollution for Ticker"] frame:NSMakeRect(16, y, 400, 20) bold:YES]];
        y -= 26;

        CGFloat colW3 = 170;
        s.tfAQI  = [s checkbox:[s L:@"settings.pollution.aqi" fallback:@"Air Quality Index (AQI)"] frame:NSMakeRect(16 + 0*colW3, y, colW3, 20)];
        s.tfPM25 = [s checkbox:[s L:@"settings.pollution.pm2_5" fallback:@"PM2.5 (Fine particles)"] frame:NSMakeRect(16 + 1*colW3, y, colW3, 20)];
        s.tfPM10 = [s checkbox:[s L:@"settings.pollution.pm10" fallback:@"PM10 (Coarse particles)"] frame:NSMakeRect(16 + 2*colW3, y, colW3, 20)];
        y -= rowH;
        s.tfO3   = [s checkbox:[s L:@"settings.pollution.o3" fallback:@"Ozone (O₃)"] frame:NSMakeRect(16 + 0*colW3, y, colW3, 20)];
        s.tfNO2  = [s checkbox:[s L:@"settings.pollution.no2" fallback:@"Nitrogen Dioxide (NO₂)"] frame:NSMakeRect(16 + 1*colW3, y, colW3, 20)];
        s.tfSO2  = [s checkbox:[s L:@"settings.pollution.so2" fallback:@"Sulfur Dioxide (SO₂)"] frame:NSMakeRect(16 + 2*colW3, y, colW3, 20)];
        y -= rowH;
        s.tfCO   = [s checkbox:[s L:@"settings.pollution.co" fallback:@"Carbon Monoxide (CO)"] frame:NSMakeRect(16 + 0*colW3, y, colW3, 20)];
        s.tfNO   = [s checkbox:[s L:@"settings.pollution.no" fallback:@"Nitrogen Monoxide (NO)"] frame:NSMakeRect(16 + 1*colW3, y, colW3, 20)];
        s.tfNH3  = [s checkbox:[s L:@"settings.pollution.nh3" fallback:@"Ammonia (NH₃)"] frame:NSMakeRect(16 + 2*colW3, y, colW3, 20)];
        y -= 34;

        for (NSButton *b in @[s.profEveryday, s.profActive, s.profFamily, s.profSensitive, s.profVentilation,
                              s.tfFeedAlert, s.tfFeedToday, s.tfFeedPrecip, s.tfFeedTomorrow, s.tfFeedTips,
                              s.tfAQI, s.tfPM25, s.tfPM10, s.tfO3, s.tfNO2, s.tfSO2, s.tfCO, s.tfNO, s.tfNH3]) {
            b.enabled = pro;
            [doc addSubview:b];
        }

        // Pro note.
        NSTextField *tickerProNote = [s sublabel:
            [s L:@"settings.pollution.proNote"
              fallback:@"🍃 Always-On Desktop Air Quality & UV Monitor (Pro): Track live PM2.5, Wildfire Smoke, Smog (NO₂, O₃) & AQI across up to 5 cities with 10-min updates — ideal for runners, families, and allergy/asthma care."]
            frame:NSMakeRect(16, y - 40, 524, 46)];
        tickerProNote.textColor = [NSColor colorWithRed:0.22 green:0.74 blue:0.97 alpha:1.0];
        [doc addSubview:tickerProNote];
    }];
    scroll.frame = NSMakeRect(0, 0, 572, 600);
    item.view = scroll;
    return item;
}

- (void)onTickerGetPro:(id)sender {
    [self triggerGetProAPI];
    [self.tabView selectTabViewItemWithIdentifier:@"provider"];
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
    NSScrollView *scroll = [self scrollableWithDocumentHeight:660 buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = 626;

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
        y -= 30;

        // No-background toggle (content only).
        s.noBgCheck = [NSButton checkboxWithTitle:[s L:@"settings.noBackground.checkbox" fallback:@"Remove panel background (content only)"]
            target:s action:@selector(onNoBgToggled:)];
        s.noBgCheck.frame = NSMakeRect(16, y, 500, 20);
        [doc addSubview:s.noBgCheck];
        y -= 20;

        [doc addSubview:[s sublabel:[s L:@"settings.noBackground.note" fallback:@"Removes the dark card behind each city. Works best with a compositing WM."]
            frame:NSMakeRect(36, y, 500, 18)]];
        y -= 36;

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
        [doc addSubview:[s sublabel:[s L:@"settings.position.subtitle" fallback:@"Where should the widget appear?"]
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 28;

        // Corner dropdown (Top-Left, Top-Right, Bottom-Left, Bottom-Right).
        s.cornerPosPop = [[NSPopUpButton alloc] initWithFrame:NSMakeRect(16, y, 220, 26) pullsDown:NO];
        [s.cornerPosPop addItemsWithTitles:@[
            [s L:@"settings.position.topLeft" fallback:@"Top-Left"],
            [s L:@"settings.position.topRight" fallback:@"Top-Right"],
            [s L:@"settings.position.bottomLeft" fallback:@"Bottom-Left"],
            [s L:@"settings.position.bottomRight" fallback:@"Bottom-Right"]
        ]];
        s.cornerPosPop.target = s;
        s.cornerPosPop.action = @selector(onCornerChanged:);
        [doc addSubview:s.cornerPosPop];
        y -= 36;

        // Position: X/Y fields + nudge arrow buttons + Apply.
        // The arrow buttons (◀ ▲ ▼ ▶) move the widget 10 px per click
        // immediately — no Save required — matching the GTK/Fyne behaviour.
        [doc addSubview:[s label:@"X:" frame:NSMakeRect(16, y, 24, 22) bold:NO]];
        s.posXField = [[NSTextField alloc] initWithFrame:NSMakeRect(44, y, 80, 24)];
        [doc addSubview:s.posXField];
        [doc addSubview:[s label:@"Y:" frame:NSMakeRect(134, y, 24, 22) bold:NO]];
        s.posYField = [[NSTextField alloc] initWithFrame:NSMakeRect(158, y, 80, 24)];
        [doc addSubview:s.posYField];

        // Apply button — commits the X/Y fields to the live window.
        NSButton *applyPos = [NSButton buttonWithTitle:@"Apply"
            target:s action:@selector(onApplyPosition:)];
        applyPos.frame = NSMakeRect(248, y - 1, 68, 26);
        [doc addSubview:applyPos];

        // Nudge buttons (10 px per click, immediate, no Save needed).
        NSButton *nudgeLeft  = [NSButton buttonWithTitle:@"\u25c0" target:s action:@selector(onNudgePosition:)];
        NSButton *nudgeUp    = [NSButton buttonWithTitle:@"\u25b2" target:s action:@selector(onNudgePosition:)];
        NSButton *nudgeDown  = [NSButton buttonWithTitle:@"\u25bc" target:s action:@selector(onNudgePosition:)];
        NSButton *nudgeRight = [NSButton buttonWithTitle:@"\u25b6" target:s action:@selector(onNudgePosition:)];
        nudgeLeft.tag  = 0;  // x - 10
        nudgeUp.tag    = 1;  // y - 10
        nudgeDown.tag  = 2;  // y + 10
        nudgeRight.tag = 3;  // x + 10
        CGFloat nx = 324;
        for (NSButton *nb in @[nudgeLeft, nudgeUp, nudgeDown, nudgeRight]) {
            nb.frame = NSMakeRect(nx, y - 1, 34, 26);
            [doc addSubview:nb];
            nx += 38;
        }
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

    CGFloat docH = 560;
    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:docH buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = docH - 34;

        NSTextField *title = [NSTextField labelWithString:[s L:@"settings.about.appName" fallback:@"WeatherWidget"]];
        title.font = [NSFont boldSystemFontOfSize:22];
        title.frame = NSMakeRect(16, y, 544, 28);
        [doc addSubview:title];
        y -= 26;

        NSString *rawVer = [s L:@"settings.about.version" fallback:@"version 2.3.0"];
        NSString *cleanVer = [rawVer stringByReplacingOccurrencesOfString:@"*" withString:@""];
        NSTextField *verLbl = [NSTextField labelWithString:cleanVer];
        verLbl.font = [NSFont systemFontOfSize:12];
        verLbl.textColor = [NSColor secondaryLabelColor];
        verLbl.frame = NSMakeRect(16, y, 544, 18);
        [doc addSubview:verLbl];
        y -= 32;

        NSTextField *sub = [NSTextField wrappingLabelWithString:[s L:@"settings.about.description" fallback:@"A compact, transparent time, weather & environmental health monitor that lives on your desktop.\nTrack up to 5 cities at a glance — including live UV Index, AQI, PM2.5, wildfire smoke, and urban smog (NO₂, O₃) for outdoor runners, families, and asthma/allergy care."]];
        sub.frame = NSMakeRect(16, y - 64, 544, 70);
        sub.textColor = [NSColor secondaryLabelColor];
        [doc addSubview:sub];
        y -= 84;

        // Divider.
        NSBox *sep1 = [[NSBox alloc] initWithFrame:NSMakeRect(16, y, 524, 1)];
        sep1.boxType = NSBoxSeparator;
        [doc addSubview:sep1];
        [sep1 release];
        y -= 24;

        // Launch at login checkbox.
        s.autostartCheck = [NSButton checkboxWithTitle:[s L:@"settings.startup.autostart" fallback:@"Launch at login"]
            target:s action:@selector(onAutostartToggled:)];
        s.autostartCheck.frame = NSMakeRect(16, y, 460, 20);
        s.autostartCheck.state = settingsAutostartGetCB() ? NSControlStateValueOn : NSControlStateValueOff;
        [doc addSubview:s.autostartCheck];
        y -= 32;

        // Divider.
        NSBox *sep2 = [[NSBox alloc] initWithFrame:NSMakeRect(16, y, 524, 1)];
        sep2.boxType = NSBoxSeparator;
        [doc addSubview:sep2];
        [sep2 release];
        y -= 28;

        // Single horizontal row of clickable links matching Windows and GTK:
        // Website • Manual • Environmental • Support • GitHub
        NSArray *linkItems = @[
            @{@"label": [s L:@"settings.windows.website" fallback:@"Website"],
              @"url": @"https://easysmartapps.co.uk/weatherwidget"},
            @{@"label": [s L:@"settings.windows.manual" fallback:@"Manual"],
              @"url": @"https://easysmartapps.co.uk/weatherwidget-manual"},
            @{@"label": [s L:@"settings.windows.environmental" fallback:@"Environmental"],
              @"url": @"https://easysmartapps.co.uk/weatherwidget-environmental"},
            @{@"label": [s L:@"settings.windows.support" fallback:@"Support"],
              @"url": @"https://easysmartapps.co.uk/contact"},
            @{@"label": @"GitHub",
              @"url": @"https://github.com/gcclinux/WeatherWidget"}
        ];

        NSMutableAttributedString *attrRow = [[[NSMutableAttributedString alloc] init] autorelease];
        for (NSUInteger i = 0; i < linkItems.count; i++) {
            if (i > 0) {
                NSAttributedString *dot = [[[NSAttributedString alloc]
                    initWithString:@" • "
                    attributes:@{ NSFontAttributeName: [NSFont systemFontOfSize:13],
                                  NSForegroundColorAttributeName: [NSColor secondaryLabelColor] }] autorelease];
                [attrRow appendAttributedString:dot];
            }
            NSDictionary *itemDict = linkItems[i];
            NSString *itemLabel = itemDict[@"label"];
            NSString *itemURL = itemDict[@"url"];
            NSAttributedString *linkPart = [[[NSAttributedString alloc]
                initWithString:itemLabel
                attributes:@{
                    NSFontAttributeName: [NSFont systemFontOfSize:13],
                    NSForegroundColorAttributeName: [NSColor linkColor],
                    NSUnderlineStyleAttributeName: @(NSUnderlineStyleSingle),
                    NSLinkAttributeName: [NSURL URLWithString:itemURL],
                    NSCursorAttributeName: [NSCursor pointingHandCursor]
                }] autorelease];
            [attrRow appendAttributedString:linkPart];
        }

        NSTextField *linksTf = [NSTextField labelWithAttributedString:attrRow];
        linksTf.allowsEditingTextAttributes = YES;
        linksTf.selectable = YES;
        linksTf.editable = NO;
        linksTf.drawsBackground = NO;
        linksTf.frame = NSMakeRect(16, y, 544, 22);
        [doc addSubview:linksTf];
    }];
    scroll.frame = NSMakeRect(0, 0, 572, 600);
    item.view = scroll;
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
    [self updateGetApiBtnTitle];
    [self setApiKeyValue:api[@"apiKey"] ?: @""];
    self.apiKeyVisible = NO;
    _apiKeySecureField.hidden = NO;
    _apiKeyField.hidden = YES;
    [self updateToggleApiKeyBtnIcon];
    [self updateActivationCardState];
    NSNumber *interval = cfg[@"refreshInterval"];
    _intervalField.stringValue = interval ? [interval stringValue] : @"120";

    // ── Locations ──
    [_cities removeAllObjects];
    NSArray *cityArr = cfg[@"cities"];
    for (NSDictionary *c in cityArr) {
        [_cities addObject:[c mutableCopy]];
    }
    [self refreshCityList];

    // ── Widget: grid mode & display fields (default true) ──
    NSDictionary *df = cfg[@"displayFields"];
    NSString *gridMode = df[@"gridMode"] ?: @"live";
    if ([gridMode isEqualToString:@"today"]) {
        [_gridModePop selectItemAtIndex:1];
    } else if ([gridMode isEqualToString:@"tomorrow"]) {
        [_gridModePop selectItemAtIndex:2];
    } else {
        [_gridModePop selectItemAtIndex:0];
    }
    [self onGridModeChanged:nil];

    _dfCity.state       = [self boolFor:df key:@"showCity"       defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfIcon.state       = [self boolFor:df key:@"showIcon"       defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfTemp.state       = [self boolFor:df key:@"showTemp"       defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfDesc.state       = [self boolFor:df key:@"showDesc"       defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfHumidity.state   = [self boolFor:df key:@"showHumidity"   defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfWind.state       = [self boolFor:df key:@"showWind"       defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfTime.state       = [self boolFor:df key:@"showTime"       defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfDate.state       = [self boolFor:df key:@"showDate"       defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfWindGust.state   = [self boolFor:df key:@"showWindGust"   defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfDewPoint.state   = [self boolFor:df key:@"showDewPoint"   defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfPressure.state   = [self boolFor:df key:@"showPressure"   defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfUVIndex.state    = [self boolFor:df key:@"showUVIndex"    defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfSunrise.state    = [self boolFor:df key:@"showSunrise"    defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfSunset.state     = [self boolFor:df key:@"showSunset"     defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfClouds.state     = [self boolFor:df key:@"showClouds"     defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfVisibility.state = [self boolFor:df key:@"showVisibility" defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;
    _dfPrecipRate.state = [self boolFor:df key:@"showPrecipRate" defaultVal:YES] ? NSControlStateValueOn : NSControlStateValueOff;

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

    // ── Ticker fields ──
    _pfTicker.state = [self boolFor:pf key:@"showTicker" defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;

    BOOL hasExplicitProfiles = (pf[@"profileEveryday"] != nil || pf[@"profileActive"] != nil ||
                                pf[@"profileFamily"] != nil || pf[@"profileSensitive"] != nil ||
                                pf[@"profileVentilation"] != nil);
    if (hasExplicitProfiles) {
        _profEveryday.state    = [self boolFor:pf key:@"profileEveryday"    defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
        _profActive.state      = [self boolFor:pf key:@"profileActive"      defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
        _profFamily.state      = [self boolFor:pf key:@"profileFamily"      defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
        _profSensitive.state   = [self boolFor:pf key:@"profileSensitive"   defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
        _profVentilation.state = [self boolFor:pf key:@"profileVentilation" defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    } else {
        NSString *legacyProf = pf[@"tickerProfile"] ?: @"everyday";
        _profEveryday.state    = [legacyProf isEqualToString:@"everyday"]    ? NSControlStateValueOn : NSControlStateValueOff;
        _profActive.state      = [legacyProf isEqualToString:@"active"]      ? NSControlStateValueOn : NSControlStateValueOff;
        _profFamily.state      = [legacyProf isEqualToString:@"family"]      ? NSControlStateValueOn : NSControlStateValueOff;
        _profSensitive.state   = [legacyProf isEqualToString:@"sensitive"]   ? NSControlStateValueOn : NSControlStateValueOff;
        _profVentilation.state = [legacyProf isEqualToString:@"ventilation"] ? NSControlStateValueOn : NSControlStateValueOff;
    }

    _tfFeedAlert.state    = [self boolFor:pf key:@"tickerAlert"        defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfFeedToday.state    = [self boolFor:pf key:@"tickerNewsToday"    defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfFeedPrecip.state   = [self boolFor:pf key:@"tickerNewsPrecip"   defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfFeedTomorrow.state = [self boolFor:pf key:@"tickerNewsTomorrow" defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfFeedTips.state     = [self boolFor:pf key:@"tickerWeatherTips"  defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;

    _tfAQI.state  = [self boolFor:pf key:@"tickerAQI"  defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfPM25.state = [self boolFor:pf key:@"tickerPM25" defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfPM10.state = [self boolFor:pf key:@"tickerPM10" defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfO3.state   = [self boolFor:pf key:@"tickerO3"   defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfNO2.state  = [self boolFor:pf key:@"tickerNO2"  defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfSO2.state  = [self boolFor:pf key:@"tickerSO2"  defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfCO.state   = [self boolFor:pf key:@"tickerCO"   defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfNO.state   = [self boolFor:pf key:@"tickerNO"   defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;
    _tfNH3.state  = [self boolFor:pf key:@"tickerNH3"  defaultVal:NO] ? NSControlStateValueOn : NSControlStateValueOff;

    [self updateTickerUnlockedState];

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

    // ── Appearance: no-background ──
    BOOL noBg = [self boolFor:cfg key:@"noBackground" defaultVal:NO];
    _noBgCheck.state = noBg ? NSControlStateValueOn : NSControlStateValueOff;
    _opacitySlider.enabled = !noBg;

    // ── Appearance: icon theme ──
    NSString *iconTheme = cfg[@"iconTheme"] ?: @"new";
    if ([iconTheme isEqualToString:@"original"]) {
        _iconThemeOriginal.state = NSControlStateValueOn;
    } else {
        _iconThemeNew.state = NSControlStateValueOn;
    }

    // ── Appearance: position ──
    NSString *corner = cfg[@"cornerPosition"] ?: @"top-right";
    if ([corner isEqualToString:@"top-left"]) {
        [_cornerPosPop selectItemAtIndex:0];
    } else if ([corner isEqualToString:@"bottom-left"]) {
        [_cornerPosPop selectItemAtIndex:2];
    } else if ([corner isEqualToString:@"bottom-right"]) {
        [_cornerPosPop selectItemAtIndex:3];
    } else {
        [_cornerPosPop selectItemAtIndex:1];
    }

    if (cfg[@"customX"] && cfg[@"customY"] &&
        cfg[@"customX"] != [NSNull null] && cfg[@"customY"] != [NSNull null]) {
        _posXField.stringValue = [cfg[@"customX"] stringValue];
        _posYField.stringValue = [cfg[@"customY"] stringValue];
    } else {
        _posXField.stringValue = @"";
        _posYField.stringValue = @"";
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
    apiCfg[@"apiKey"] = [self currentApiKey];
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
    NSString *gridMode = @"live";
    if (_gridModePop.indexOfSelectedItem == 1) gridMode = @"today";
    else if (_gridModePop.indexOfSelectedItem == 2) gridMode = @"tomorrow";

    cfg[@"displayFields"] = @{
        @"gridMode":     gridMode,
        @"showCity":     jbool(_dfCity.state       == NSControlStateValueOn),
        @"showIcon":     jbool(_dfIcon.state       == NSControlStateValueOn),
        @"showTemp":     jbool(_dfTemp.state       == NSControlStateValueOn),
        @"showDesc":     jbool(_dfDesc.state       == NSControlStateValueOn),
        @"showHumidity": jbool(_dfHumidity.state   == NSControlStateValueOn),
        @"showWind":     jbool(_dfWind.state       == NSControlStateValueOn),
        @"showTime":     jbool(_dfTime.state       == NSControlStateValueOn),
        @"showDate":     jbool(_dfDate.state       == NSControlStateValueOn),
        @"showWindGust": jbool(_dfWindGust.state   == NSControlStateValueOn),
        @"showDewPoint": jbool(_dfDewPoint.state   == NSControlStateValueOn),
        @"showPressure": jbool(_dfPressure.state   == NSControlStateValueOn),
        @"showUVIndex":  jbool(_dfUVIndex.state    == NSControlStateValueOn),
        @"showSunrise":  jbool(_dfSunrise.state    == NSControlStateValueOn),
        @"showSunset":   jbool(_dfSunset.state     == NSControlStateValueOn),
        @"showClouds":   jbool(_dfClouds.state     == NSControlStateValueOn),
        @"showVisibility": jbool(_dfVisibility.state == NSControlStateValueOn),
        @"showPrecipRate": jbool(_dfPrecipRate.state == NSControlStateValueOn),
    };

    // ── Pollution fields ──
    BOOL anyProf = (_profEveryday.state == NSControlStateValueOn ||
                    _profActive.state == NSControlStateValueOn ||
                    _profFamily.state == NSControlStateValueOn ||
                    _profSensitive.state == NSControlStateValueOn ||
                    _profVentilation.state == NSControlStateValueOn);
    NSString *tickerProfile = anyProf ? @"custom" : @"none";

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
        @"showTicker": jbool(_pfTicker.state == NSControlStateValueOn),
        @"profileEveryday": jbool(_profEveryday.state == NSControlStateValueOn),
        @"profileActive": jbool(_profActive.state == NSControlStateValueOn),
        @"profileFamily": jbool(_profFamily.state == NSControlStateValueOn),
        @"profileSensitive": jbool(_profSensitive.state == NSControlStateValueOn),
        @"profileVentilation": jbool(_profVentilation.state == NSControlStateValueOn),
        @"tickerProfile": tickerProfile,
        @"tickerAlert": jbool(_tfFeedAlert.state == NSControlStateValueOn),
        @"tickerNewsToday": jbool(_tfFeedToday.state == NSControlStateValueOn),
        @"tickerNewsPrecip": jbool(_tfFeedPrecip.state == NSControlStateValueOn),
        @"tickerNewsTomorrow": jbool(_tfFeedTomorrow.state == NSControlStateValueOn),
        @"tickerWeatherTips": jbool(_tfFeedTips.state == NSControlStateValueOn),
        @"tickerAQI": jbool(_tfAQI.state == NSControlStateValueOn),
        @"tickerPM25": jbool(_tfPM25.state == NSControlStateValueOn),
        @"tickerPM10": jbool(_tfPM10.state == NSControlStateValueOn),
        @"tickerO3": jbool(_tfO3.state == NSControlStateValueOn),
        @"tickerNO2": jbool(_tfNO2.state == NSControlStateValueOn),
        @"tickerSO2": jbool(_tfSO2.state == NSControlStateValueOn),
        @"tickerCO": jbool(_tfCO.state == NSControlStateValueOn),
        @"tickerNO": jbool(_tfNO.state == NSControlStateValueOn),
        @"tickerNH3": jbool(_tfNH3.state == NSControlStateValueOn),
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

    // ── No-background ──
    cfg[@"noBackground"] = @(_noBgCheck.state == NSControlStateValueOn);

    // ── Icon theme ──
    cfg[@"iconTheme"] = _iconThemeOriginal.state == NSControlStateValueOn ? @"original" : @"new";

    // ── Position ──
    NSArray *corners = @[@"top-left", @"top-right", @"bottom-left", @"bottom-right"];
    NSInteger cIdx = _cornerPosPop.indexOfSelectedItem;
    if (cIdx >= 0 && cIdx < (NSInteger)corners.count) {
        cfg[@"cornerPosition"] = corners[cIdx];
    } else {
        cfg[@"cornerPosition"] = @"top-right";
    }

    NSString *xStr = [_posXField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    NSString *yStr = [_posYField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    if (self.placementChanged || xStr.length == 0 || yStr.length == 0) {
        cfg[@"customX"] = [NSNull null];
        cfg[@"customY"] = [NSNull null];
    } else {
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

- (void)onNoBgToggled:(NSButton *)sender {
    BOOL noBg = (sender.state == NSControlStateValueOn);
    _opacitySlider.enabled = !noBg;
}

- (void)onCornerChanged:(id)sender {
    self.placementChanged = YES;
    _posXField.stringValue = @"";
    _posYField.stringValue = @"";
    [self onApplyPosition:sender];
}

- (void)onApplyPosition:(id)sender {
    if (sender != _cornerPosPop) {
        self.placementChanged = NO;
    }
    NSString *updated = [self buildUpdatedJSON];
    settingsSaveCB([updated UTF8String]);
    // Update stored config/JSON so a subsequent full Save keeps the new position.
    self.cfgJSON = updated;
    NSData *data = [updated dataUsingEncoding:NSUTF8StringEncoding];
    NSMutableDictionary *parsed = [[NSJSONSerialization JSONObjectWithData:data
        options:NSJSONReadingMutableContainers error:nil] mutableCopy];
    if (parsed) {
        self.cfg = parsed;
        [parsed release];
    }
}

// Nudges the widget position by ±10 px in the indicated direction and
// immediately applies the move (no Save button required).
//   tag 0 = left  (x - 10)
//   tag 1 = up    (y - 10)
//   tag 2 = down  (y + 10)
//   tag 3 = right (x + 10)
- (void)onNudgePosition:(NSButton *)sender {
    self.placementChanged = NO;
    const NSInteger delta = 10;
    NSInteger x = _posXField.integerValue;
    NSInteger y = _posYField.integerValue;
    switch (sender.tag) {
        case 0: x -= delta; break;  // left
        case 1: y -= delta; break;  // up   (screen-coords: smaller y = higher)
        case 2: y += delta; break;  // down
        case 3: x += delta; break;  // right
        default: break;
    }
    _posXField.stringValue = [NSString stringWithFormat:@"%ld", (long)x];
    _posYField.stringValue = [NSString stringWithFormat:@"%ld", (long)y];
    [self onApplyPosition:sender];
}

- (void)onSave:(id)sender {
    NSString *updated = [self buildUpdatedJSON];
    settingsSaveCB([updated UTF8String]);
    self.cfgJSON = updated;
    NSData *data = [updated dataUsingEncoding:NSUTF8StringEncoding];
    NSMutableDictionary *parsed = [[NSJSONSerialization JSONObjectWithData:data
        options:NSJSONReadingMutableContainers error:nil] mutableCopy];
    if (parsed) {
        self.cfg = parsed;
        [parsed release];
    }
    self.placementChanged = NO;
    [self updateLocationsUnlockedState];
    [self updatePollutionUnlockedState];
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
    [_apiKeyField release];
    [_apiKeySecureField release];
    [_toggleApiKeyBtn release];
    [_noBgCheck release];
    [_cornerPosPop release];
    [_checkConnBtn release];
    [_demoBanner release];
    [_gridModePop release];
    [_dfCity release]; [_dfIcon release]; [_dfTemp release]; [_dfDesc release];
    [_dfHumidity release]; [_dfWind release]; [_dfTime release]; [_dfDate release];
    [_dfWindGust release]; [_dfDewPoint release]; [_dfPressure release]; [_dfUVIndex release];
    [_dfSunrise release]; [_dfSunset release]; [_dfClouds release]; [_dfVisibility release]; [_dfPrecipRate release];
    [_pfAQI release]; [_pfCO release]; [_pfNO release]; [_pfNO2 release]; [_pfO3 release];
    [_pfSO2 release]; [_pfNH3 release]; [_pfPM25 release]; [_pfPM10 release];
    [_tickerProCard release]; [_tickerProBtn release]; [_pfTicker release];
    [_profEveryday release]; [_profActive release]; [_profFamily release]; [_profSensitive release]; [_profVentilation release];
    [_tfFeedAlert release]; [_tfFeedToday release]; [_tfFeedPrecip release]; [_tfFeedTomorrow release]; [_tfFeedTips release];
    [_tfAQI release]; [_tfPM25 release]; [_tfPM10 release]; [_tfO3 release]; [_tfNO2 release]; [_tfSO2 release]; [_tfCO release]; [_tfNO release]; [_tfNH3 release];
    [_autostartCheck release];
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
