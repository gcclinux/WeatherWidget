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
// Mirrors gtkLocaleData in ui-gtk/settings.go (code + native name).
static NSString *const kLangCodes[] = {
    @"en-GB", @"es-ES", @"fr-FR", @"de-DE", @"it-IT", @"pt-BR", @"nl-NL",
    @"pl-PL", @"tr-TR", @"ta-IN", @"ja-JP", @"zh-CN"
};
static NSString *const kLangNames[] = {
    @"English", @"Español", @"Français", @"Deutsch", @"Italiano", @"Português (BR)",
    @"Nederlands", @"Polski", @"Türkçe", @"தமிழ்", @"日本語", @"中文"
};
static const int kLangCount = 12;

// ── WWSettingsController ──────────────────────────────────────────────────────

@interface WWSettingsController : NSWindowController <NSWindowDelegate>
@property (nonatomic, strong) NSString  *cfgJSON;   // current config JSON
@property (nonatomic, strong) NSMutableDictionary *cfg; // mutable parsed config
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
@property (nonatomic, strong) NSButton *windKmh;
@property (nonatomic, strong) NSButton *windMph;
@property (nonatomic, strong) NSButton *windKnots;

// Language tab
@property (nonatomic, assign) NSInteger selectedLangIndex;
@property (nonatomic, strong) NSMutableArray<NSButton *> *langButtons;

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

- (instancetype)initWithJSON:(NSString *)json {
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
    _cfgJSON = json;
    _cities = [NSMutableArray array];
    _langButtons = [NSMutableArray array];
    _selectedLangIndex = 0;
    _fontCityTime = 14;
    _fontTempIcon = 32;
    _fontConditions = 10;

    // Parse config up-front so tab builders can read initial values.
    NSData *data = [json dataUsingEncoding:NSUTF8StringEncoding];
    NSMutableDictionary *parsed = [[NSJSONSerialization JSONObjectWithData:data
        options:NSJSONReadingMutableContainers error:nil] mutableCopy];
    _cfg = parsed ?: [NSMutableDictionary dictionary];

    [self buildUI];
    [self loadFromJSON:json];
    [self.window center];

    return self;
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
    NSButton *saveBtn = [NSButton buttonWithTitle:@"Save"
        target:self action:@selector(onSave:)];
    saveBtn.keyEquivalent = @"\r";
    saveBtn.frame = NSMakeRect(520, 14, 88, 28);
    saveBtn.autoresizingMask = NSViewMinXMargin;
    [content addSubview:saveBtn];

    NSButton *cancelBtn = [NSButton buttonWithTitle:@"Cancel"
        target:self action:@selector(onCancel:)];
    cancelBtn.keyEquivalent = @"\033";
    cancelBtn.frame = NSMakeRect(424, 14, 88, 28);
    cancelBtn.autoresizingMask = NSViewMinXMargin;
    [content addSubview:cancelBtn];
}

// ── Provider tab ─────────────────────────────────────────────────────────────

- (NSTabViewItem *)buildProviderTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"provider"];
    item.label = @"Provider";
    NSView *v = [[NSView alloc] initWithFrame:NSZeroRect];

    CGFloat y = 560;

    [v addSubview:[self label:@"Provider:" frame:NSMakeRect(16, y, 200, 20) bold:NO]];
    _providerPop = [[NSPopUpButton alloc] initWithFrame:NSMakeRect(220, y - 2, 320, 26) pullsDown:NO];
    [_providerPop addItemsWithTitles:@[@"EasyWeatherWidget (Pro)", @"OpenWeatherMap (Free)"]];
    [v addSubview:_providerPop];
    y -= 40;

    [v addSubview:[self label:@"API Key:" frame:NSMakeRect(16, y, 200, 20) bold:NO]];
    _apiKeyField = [[NSTextField alloc] initWithFrame:NSMakeRect(220, y - 2, 320, 24)];
    _apiKeyField.placeholderString = @"Paste your API key here";
    [v addSubview:_apiKeyField];
    y -= 40;

    [v addSubview:[self label:@"Refresh Interval (min):" frame:NSMakeRect(16, y, 200, 20) bold:NO]];
    _intervalField = [[NSTextField alloc] initWithFrame:NSMakeRect(220, y - 2, 100, 24)];
    _intervalField.placeholderString = @"e.g. 30";
    [v addSubview:_intervalField];
    y -= 60;

    NSTextField *note = [self sublabel:
        @"EasyWeatherWidget Pro provides air-quality data, more cities, faster updates, "
         "and priority support.\nOpenWeatherMap Free works with a free API key from "
         "openweathermap.org and is limited to the built-in default cities."
        frame:NSMakeRect(16, y - 20, 520, 56)];
    [v addSubview:note];

    item.view = v;
    return item;
}

// ── Locations tab ────────────────────────────────────────────────────────────

- (NSTabViewItem *)buildLocationsTab {
    NSTabViewItem *item = [[NSTabViewItem alloc] initWithIdentifier:@"locations"];
    item.label = @"Locations";

    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:520 buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = 486;

        [doc addSubview:[s label:@"Saved Cities" frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 22;
        [doc addSubview:[s sublabel:@"Reorder or remove cities. Free tier is limited to the default cities."
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
        [doc addSubview:[s label:@"Add New City" frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 30;

        [doc addSubview:[s label:@"Name:" frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addNameField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 430, 24)];
        s.addNameField.placeholderString = @"City name";
        [doc addSubview:s.addNameField];
        y -= 32;

        [doc addSubview:[s label:@"Region:" frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addRegionField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 430, 24)];
        s.addRegionField.placeholderString = @"Country / state code (e.g. GB)";
        [doc addSubview:s.addRegionField];
        y -= 32;

        [doc addSubview:[s label:@"Latitude:" frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addLatField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 200, 24)];
        s.addLatField.placeholderString = @"e.g. 55.9344";
        [doc addSubview:s.addLatField];
        [doc addSubview:[s label:@"Longitude:" frame:NSMakeRect(320, y, 80, 20) bold:NO]];
        s.addLonField = [[NSTextField alloc] initWithFrame:NSMakeRect(400, y - 2, 140, 24)];
        s.addLonField.placeholderString = @"e.g. -3.4693";
        [doc addSubview:s.addLonField];
        y -= 32;

        [doc addSubview:[s label:@"Timezone:" frame:NSMakeRect(16, y, 90, 20) bold:NO]];
        s.addTZField = [[NSTextField alloc] initWithFrame:NSMakeRect(110, y - 2, 430, 24)];
        s.addTZField.placeholderString = @"IANA zone (e.g. Europe/London)";
        [doc addSubview:s.addTZField];
        y -= 40;

        NSButton *addBtn = [NSButton buttonWithTitle:@"＋ Add City"
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
        [self showAlert:@"License required"
                   info:@"Adding cities requires a Pro API key. Configure one in the Provider tab."];
        return;
    }
    NSString *name = [self.addNameField.stringValue
        stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    if (name.length == 0) {
        [self showAlert:@"City name required" info:@"Enter a city name before adding."];
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
    item.label = @"Widget";

    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:560 buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = 526;

        // Panel display fields.
        [doc addSubview:[s label:@"Panel Display" frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 20;
        [doc addSubview:[s sublabel:@"Choose which elements appear on each city panel."
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 26;

        // 4-column grid of checkboxes.
        CGFloat colW = 130, rowH = 26;
        CGFloat cx0 = 16;
        s.dfCity     = [s checkbox:@"City"       frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.dfIcon     = [s checkbox:@"Icon"       frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.dfTemp     = [s checkbox:@"Temperature" frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.dfDesc     = [s checkbox:@"Description" frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        y -= rowH;
        s.dfHumidity = [s checkbox:@"Humidity"   frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.dfWind     = [s checkbox:@"Wind"       frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.dfTime     = [s checkbox:@"Time"       frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.dfDate     = [s checkbox:@"Date"       frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        y -= rowH;
        s.dfWindGust = [s checkbox:@"Wind Gust"  frame:NSMakeRect(cx0 + 0*colW, y, colW, 20)];
        s.dfDewPoint = [s checkbox:@"Dew Point"  frame:NSMakeRect(cx0 + 1*colW, y, colW, 20)];
        s.dfPressure = [s checkbox:@"Pressure"   frame:NSMakeRect(cx0 + 2*colW, y, colW, 20)];
        s.dfUVIndex  = [s checkbox:@"UV Index"   frame:NSMakeRect(cx0 + 3*colW, y, colW, 20)];
        for (NSButton *b in @[s.dfCity, s.dfIcon, s.dfTemp, s.dfDesc, s.dfHumidity, s.dfWind,
                              s.dfTime, s.dfDate, s.dfWindGust, s.dfDewPoint, s.dfPressure, s.dfUVIndex]) {
            [doc addSubview:b];
        }
        y -= 36;

        // Pollution fields.
        [doc addSubview:[s label:@"Air Quality" frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 20;
        [doc addSubview:[s sublabel:@"Air-quality metrics (Pro only)."
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
        [doc addSubview:[s label:@"Temperature Unit" frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 26;
        s.tempCelsius = [NSButton radioButtonWithTitle:@"°C (Celsius)"
            target:s action:@selector(onTempUnitRadio:)];
        s.tempCelsius.frame = NSMakeRect(16, y, 160, 20);
        [doc addSubview:s.tempCelsius];
        s.tempFahrenheit = [NSButton radioButtonWithTitle:@"°F (Fahrenheit)"
            target:s action:@selector(onTempUnitRadio:)];
        s.tempFahrenheit.frame = NSMakeRect(190, y, 180, 20);
        [doc addSubview:s.tempFahrenheit];
        y -= 40;

        // Wind speed unit.
        [doc addSubview:[s label:@"Wind Speed Unit" frame:NSMakeRect(16, y, 300, 20) bold:YES]];
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
    item.label = @"Language";

    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:400 buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = 366;

        [doc addSubview:[s label:@"🌐 Display Language" frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 22;
        [doc addSubview:[s sublabel:@"Select the interface and weather-description language."
            frame:NSMakeRect(16, y, 520, 18)]];
        y -= 30;

        // 2-column grid of radio buttons.
        CGFloat colW = 262, rowH = 30;
        for (int i = 0; i < kLangCount; i++) {
            int col = i % 2;
            int row = i / 2;
            NSButton *b = [NSButton radioButtonWithTitle:kLangNames[i]
                target:s action:@selector(onLangSelected:)];
            b.tag = i;
            b.frame = NSMakeRect(16 + col * colW, y - row * rowH, colW - 10, 24);
            [doc addSubview:b];
            [s.langButtons addObject:b];
        }
    }];
    scroll.frame = NSMakeRect(0, 0, 572, 600);
    item.view = scroll;
    return item;
}

- (void)onLangSelected:(NSButton *)sender {
    self.selectedLangIndex = sender.tag;
    for (NSButton *b in self.langButtons) {
        b.state = (b.tag == sender.tag) ? NSControlStateValueOn : NSControlStateValueOff;
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
    item.label = @"Appearance";

    WWSettingsController *weakSelf = self;
    NSScrollView *scroll = [self scrollableWithDocumentHeight:600 buildBlock:^(NSView *doc) {
        WWSettingsController *s = weakSelf;
        CGFloat y = 566;

        // View mode.
        [doc addSubview:[s label:@"View Mode" frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 26;
        s.viewEnhanced = [NSButton radioButtonWithTitle:@"Enhanced (modern)"
            target:s action:@selector(onViewModeRadio:)];
        s.viewEnhanced.frame = NSMakeRect(16, y, 200, 20);
        [doc addSubview:s.viewEnhanced];
        s.viewSimple = [NSButton radioButtonWithTitle:@"Simple (classic)"
            target:s action:@selector(onViewModeRadio:)];
        s.viewSimple.frame = NSMakeRect(230, y, 200, 20);
        [doc addSubview:s.viewSimple];
        y -= 44;

        // Opacity.
        [doc addSubview:[s label:@"Opacity:" frame:NSMakeRect(16, y, 120, 20) bold:NO]];
        s.opacityLabel = [NSTextField labelWithString:@"100%"];
        s.opacityLabel.frame = NSMakeRect(490, y, 50, 20);
        [doc addSubview:s.opacityLabel];
        s.opacitySlider = [[NSSlider alloc] initWithFrame:NSMakeRect(120, y, 360, 20)];
        s.opacitySlider.minValue = 25; s.opacitySlider.maxValue = 100;
        s.opacitySlider.numberOfTickMarks = 4;
        s.opacitySlider.allowsTickMarkValuesOnly = YES;
        s.opacitySlider.target = s; s.opacitySlider.action = @selector(onOpacityChanged:);
        [doc addSubview:s.opacitySlider];
        y -= 40;

        // Autostart.
        s.autostartCheck = [NSButton checkboxWithTitle:@"Launch at login"
            target:s action:@selector(onAutostartToggled:)];
        s.autostartCheck.frame = NSMakeRect(16, y, 300, 20);
        [doc addSubview:s.autostartCheck];
        y -= 44;

        // Weather icons.
        [doc addSubview:[s label:@"Weather Icons" frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 26;
        s.iconThemeNew = [NSButton radioButtonWithTitle:@"New (day/night)"
            target:s action:@selector(onIconThemeRadio:)];
        s.iconThemeNew.frame = NSMakeRect(16, y, 200, 20);
        [doc addSubview:s.iconThemeNew];
        s.iconThemeOriginal = [NSButton radioButtonWithTitle:@"Original"
            target:s action:@selector(onIconThemeRadio:)];
        s.iconThemeOriginal.frame = NSMakeRect(230, y, 200, 20);
        [doc addSubview:s.iconThemeOriginal];
        y -= 44;

        // Position.
        [doc addSubview:[s label:@"Widget Position" frame:NSMakeRect(16, y, 300, 20) bold:YES]];
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
        [doc addSubview:[s label:@"Font Sizes" frame:NSMakeRect(16, y, 300, 20) bold:YES]];
        y -= 30;

        y = [s addFontRow:doc y:y label:@"City & Time" tag:0 valueLabelOut:&s->_fontCityTimeLabel];
        y = [s addFontRow:doc y:y label:@"Temperature" tag:1 valueLabelOut:&s->_fontTempIconLabel];
        y = [s addFontRow:doc y:y label:@"Conditions" tag:2 valueLabelOut:&s->_fontConditionsLabel];
    }];
    scroll.frame = NSMakeRect(0, 0, 572, 600);
    item.view = scroll;
    return item;
}

// Adds one "Label  [-]  Npx  [+]" row and returns the new y.
- (CGFloat)addFontRow:(NSView *)doc y:(CGFloat)y label:(NSString *)label tag:(NSInteger)tag
        valueLabelOut:(NSTextField * __strong *)outLabel {
    [doc addSubview:[self label:label frame:NSMakeRect(16, y, 140, 22) bold:NO]];

    NSButton *dec = [NSButton buttonWithTitle:@"−" target:self action:@selector(onFontDec:)];
    dec.tag = tag;
    dec.frame = NSMakeRect(170, y - 1, 34, 24);
    [doc addSubview:dec];

    NSTextField *val = [NSTextField labelWithString:@"0px"];
    val.frame = NSMakeRect(210, y, 60, 20);
    val.alignment = NSTextAlignmentCenter;
    [doc addSubview:val];
    *outLabel = val;

    NSButton *inc = [NSButton buttonWithTitle:@"＋" target:self action:@selector(onFontInc:)];
    inc.tag = tag;
    inc.frame = NSMakeRect(272, y - 1, 34, 24);
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
    item.label = @"About";
    NSView *v = [[NSView alloc] initWithFrame:NSZeroRect];

    NSTextField *title = [NSTextField labelWithString:@"WeatherWidget"];
    title.font = [NSFont boldSystemFontOfSize:20];
    title.frame = NSMakeRect(16, 560, 500, 28);
    [v addSubview:title];

    NSTextField *sub = [NSTextField labelWithString:@"Desktop weather overlay for macOS"];
    sub.frame = NSMakeRect(16, 532, 500, 20);
    sub.textColor = [NSColor secondaryLabelColor];
    [v addSubview:sub];

    NSTextField *website = [NSTextField labelWithString:@"easysmartapps.co.uk/weatherwidget"];
    website.frame = NSMakeRect(16, 496, 500, 20);
    website.textColor = [NSColor linkColor];
    [v addSubview:website];

    NSTextField *manual = [NSTextField labelWithString:@"easysmartapps.co.uk/weatherwidget-manual"];
    manual.frame = NSMakeRect(16, 470, 500, 20);
    manual.textColor = [NSColor linkColor];
    [v addSubview:manual];

    NSTextField *air = [NSTextField labelWithString:@"easysmartapps.co.uk/weatherwidget-environmental"];
    air.frame = NSMakeRect(16, 444, 500, 20);
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
    _selectedLangIndex = 0;
    for (int i = 0; i < kLangCount; i++) {
        if ([kLangCodes[i] isEqualToString:locale]) { _selectedLangIndex = i; break; }
    }
    for (NSButton *b in _langButtons) {
        b.state = (b.tag == _selectedLangIndex) ? NSControlStateValueOn : NSControlStateValueOff;
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
    cfg[@"displayFields"] = @{
        @"showCity":     @(_dfCity.state == NSControlStateValueOn),
        @"showIcon":     @(_dfIcon.state == NSControlStateValueOn),
        @"showTemp":     @(_dfTemp.state == NSControlStateValueOn),
        @"showDesc":     @(_dfDesc.state == NSControlStateValueOn),
        @"showHumidity": @(_dfHumidity.state == NSControlStateValueOn),
        @"showWind":     @(_dfWind.state == NSControlStateValueOn),
        @"showTime":     @(_dfTime.state == NSControlStateValueOn),
        @"showDate":     @(_dfDate.state == NSControlStateValueOn),
        @"showWindGust": @(_dfWindGust.state == NSControlStateValueOn),
        @"showDewPoint": @(_dfDewPoint.state == NSControlStateValueOn),
        @"showPressure": @(_dfPressure.state == NSControlStateValueOn),
        @"showUVIndex":  @(_dfUVIndex.state == NSControlStateValueOn),
    };

    // ── Pollution fields ──
    cfg[@"pollutionFields"] = @{
        @"showAQI":  @(_pfAQI.state == NSControlStateValueOn),
        @"showCO":   @(_pfCO.state == NSControlStateValueOn),
        @"showNO":   @(_pfNO.state == NSControlStateValueOn),
        @"showNO2":  @(_pfNO2.state == NSControlStateValueOn),
        @"showO3":   @(_pfO3.state == NSControlStateValueOn),
        @"showSO2":  @(_pfSO2.state == NSControlStateValueOn),
        @"showNH3":  @(_pfNH3.state == NSControlStateValueOn),
        @"showPM25": @(_pfPM25.state == NSControlStateValueOn),
        @"showPM10": @(_pfPM10.state == NSControlStateValueOn),
    };

    // ── Units ──
    cfg[@"temperatureUnit"] = _tempFahrenheit.state == NSControlStateValueOn ?
        @"fahrenheit" : @"celsius";
    if (_windMph.state == NSControlStateValueOn) {
        cfg[@"windSpeedUnit"] = @"mph";
    } else if (_windKnots.state == NSControlStateValueOn) {
        cfg[@"windSpeedUnit"] = @"knots";
    } else {
        cfg[@"windSpeedUnit"] = @"kmh";
    }

    // ── Language ──
    if (_selectedLangIndex >= 0 && _selectedLangIndex < kLangCount)
        cfg[@"locale"] = kLangCodes[_selectedLangIndex];

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

@end


// ── Public C API ──────────────────────────────────────────────────────────────

void openSettingsNative(
    const char *cfgJSON,
    void *unusedOnSave
) {
    (void)unusedOnSave;

    // Copy the JSON to an NSString NOW, synchronously. Go frees cfgJSON via
    // defer C.free the moment openSettingsWindow returns — before the async
    // block runs. Reading cfgJSON inside the block would hit freed memory.
    NSString *json = (cfgJSON && *cfgJSON) ?
        [NSString stringWithUTF8String:cfgJSON] : @"{}";

    dispatch_async(dispatch_get_main_queue(), ^{
        if (g_settingsController) {
            [g_settingsController.window makeKeyAndOrderFront:nil];
            [NSApp activateIgnoringOtherApps:YES];
            return;
        }

        g_settingsController = [[WWSettingsController alloc] initWithJSON:json];
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
