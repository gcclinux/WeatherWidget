// panel.m — Per-city weather card view for the native macOS WeatherWidget UI.
//
// WWCityCardView is a plain NSView with a rounded CALayer, a day/night
// background image layer, and a dark overlay.  All child widgets are laid out
// using a single root NSStackView with Auto Layout so AppKit handles the
// measurement passes without conflicts.
//
// Layout (enhanced / default view):
//
//   ┌─────────────────────────────────────────────────────────┐
//   │  📍 City, Region                                        │
//   │  ┌──────────┬───────────────────────────────────────┐  │
//   │  │  [icon]  │  HH:MM:SS   💧Humid  💨Wind  🌬Gust  │  │
//   │  │  18°C    │  Mon Jan 2  💧Dew    🌡Press  ☀UV    │  │
//   │  │  Cloudy  │                                       │  │
//   │  └──────────┴───────────────────────────────────────┘  │
//   │  [AQI] ·········· [CO] [NO] [NO₂] [O₃] [SO₂] ...     │
//   │  ⚠ Fetch error  (hidden when ok)                       │
//   └─────────────────────────────────────────────────────────┘

#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>

// ── Helpers ───────────────────────────────────────────────────────────────────

static const CGFloat kCardRadius = 16.0;
static const int kPollSlots = 8;    // CO NO NO2 O3 SO2 NH3 PM2.5 PM10

// Card layout widths (shared by the card view and the container helpers).
static const CGFloat kEnhancedCardW = 600;
static const CGFloat kSimpleCardW   = 210;
static const CGFloat kCardGap       = 8;
static const CGFloat kCardPad       = 10;

// Field-visibility bitmask (must match bridge.go DFxxx constants).
typedef NS_OPTIONS(unsigned int, DFMask) {
    DFCity     = 1u << 0,  DFIcon     = 1u << 1,
    DFTemp     = 1u << 2,  DFDesc     = 1u << 3,
    DFHumidity = 1u << 4,  DFWind     = 1u << 5,
    DFTime     = 1u << 6,  DFDate     = 1u << 7,
    DFWindGust = 1u << 8,  DFDewPoint = 1u << 9,
    DFPressure = 1u << 10, DFUVIndex  = 1u << 11,
};

// run_on_main — execute block on main thread without deadlocking.
#define run_on_main(blk) \
    do { \
        if ([NSThread isMainThread]) { blk(); } \
        else { dispatch_sync(dispatch_get_main_queue(), blk); } \
    } while(0)

static NSTextField *lbl(NSString *text, CGFloat size, BOOL bold) {
    NSTextField *f = [NSTextField labelWithString:text];
    f.textColor = [NSColor whiteColor];
    f.font = bold ? [NSFont boldSystemFontOfSize:size] : [NSFont systemFontOfSize:size];
    f.translatesAutoresizingMaskIntoConstraints = NO;
    f.lineBreakMode = NSLineBreakByTruncatingTail;
    f.drawsBackground = NO;
    f.bordered = NO;
    f.selectable = NO;
    return f;
}

static NSTextField *subLbl(NSString *text, CGFloat size) {
    NSTextField *f = lbl(text, size, NO);
    f.textColor = [NSColor colorWithWhite:0.80 alpha:1.0];
    return f;
}

// ── Metric tile ───────────────────────────────────────────────────────────────

@interface WWMetricTile : NSView
@property (nonatomic, strong) NSTextField *emojiLbl;
@property (nonatomic, strong) NSTextField *nameLbl;
@property (nonatomic, strong) NSTextField *valueLbl;
@end

@implementation WWMetricTile

- (instancetype)initWithEmoji:(NSString *)emoji name:(NSString *)name {
    self = [super initWithFrame:NSZeroRect];
    if (!self) return nil;
    self.translatesAutoresizingMaskIntoConstraints = NO;
    self.wantsLayer = YES;
    self.layer.cornerRadius = 7;
    self.layer.borderWidth  = 0.5;
    self.layer.borderColor  = [NSColor colorWithWhite:1 alpha:0.18].CGColor;
    self.layer.backgroundColor = [NSColor colorWithWhite:1 alpha:0.06].CGColor;

    _emojiLbl = lbl(emoji, 11, NO);
    _nameLbl  = subLbl(name, 9);
    _valueLbl = lbl(@"--",   11, YES);

    // Header row: emoji + name
    NSStackView *header = [NSStackView stackViewWithViews:@[_emojiLbl, _nameLbl]];
    header.orientation = NSUserInterfaceLayoutOrientationHorizontal;
    header.spacing = 3;
    header.translatesAutoresizingMaskIntoConstraints = NO;

    NSStackView *stack = [NSStackView stackViewWithViews:@[header, _valueLbl]];
    stack.orientation = NSUserInterfaceLayoutOrientationVertical;
    stack.spacing = 2;
    stack.alignment = NSLayoutAttributeLeading;
    stack.translatesAutoresizingMaskIntoConstraints = NO;
    [self addSubview:stack];

    [NSLayoutConstraint activateConstraints:@[
        [stack.leadingAnchor  constraintEqualToAnchor:self.leadingAnchor  constant:6],
        [stack.trailingAnchor constraintEqualToAnchor:self.trailingAnchor constant:-4],
        [stack.topAnchor      constraintEqualToAnchor:self.topAnchor      constant:5],
        [stack.bottomAnchor   constraintEqualToAnchor:self.bottomAnchor   constant:-5],
    ]];
    return self;
}

@end

// ── Air-quality tile ──────────────────────────────────────────────────────────

@interface WWAirTile : NSView
@property (nonatomic, strong) NSImageView *iconView;
@property (nonatomic, strong) NSTextField *nameLbl;
@property (nonatomic, strong) NSTextField *valueLbl;
@end

@implementation WWAirTile

- (instancetype)initWithName:(NSString *)name {
    self = [super initWithFrame:NSZeroRect];
    if (!self) return nil;
    self.translatesAutoresizingMaskIntoConstraints = NO;
    self.wantsLayer = YES;

    _iconView = [[NSImageView alloc] init];
    _iconView.translatesAutoresizingMaskIntoConstraints = NO;
    _iconView.imageScaling = NSImageScaleProportionallyUpOrDown;
    [_iconView setContentHuggingPriority:NSLayoutPriorityRequired
                          forOrientation:NSLayoutConstraintOrientationHorizontal];
    [_iconView setContentHuggingPriority:NSLayoutPriorityRequired
                          forOrientation:NSLayoutConstraintOrientationVertical];

    _nameLbl  = subLbl(name, 9);
    _nameLbl.alignment = NSTextAlignmentCenter;
    _valueLbl = lbl(@"", 10, YES);
    _valueLbl.alignment = NSTextAlignmentCenter;

    NSStackView *stack = [NSStackView stackViewWithViews:@[_iconView, _nameLbl, _valueLbl]];
    stack.orientation = NSUserInterfaceLayoutOrientationVertical;
    stack.spacing = 2;
    stack.alignment = NSLayoutAttributeCenterX;
    stack.translatesAutoresizingMaskIntoConstraints = NO;
    [self addSubview:stack];

    [NSLayoutConstraint activateConstraints:@[
        [_iconView.widthAnchor  constraintEqualToConstant:26],
        [_iconView.heightAnchor constraintEqualToConstant:26],
        [stack.leadingAnchor  constraintEqualToAnchor:self.leadingAnchor  constant:4],
        [stack.trailingAnchor constraintEqualToAnchor:self.trailingAnchor constant:-4],
        [stack.topAnchor      constraintEqualToAnchor:self.topAnchor      constant:4],
        [stack.bottomAnchor   constraintEqualToAnchor:self.bottomAnchor   constant:-4],
    ]];
    return self;
}

@end

// ── WWCityCardView ────────────────────────────────────────────────────────────

@interface WWCityCardView : NSView

// Background layers (below content)
@property (nonatomic, strong) CALayer     *bgImageLayer;
@property (nonatomic, strong) CALayer     *bgOverlayLayer;

// Content stack (fills the card, above background layers)
@property (nonatomic, strong) NSStackView *rootStack;

// Top labels
@property (nonatomic, strong) NSTextField *cityLbl;
@property (nonatomic, strong) NSImageView *iconView;
@property (nonatomic, strong) NSTextField *timeLbl;
@property (nonatomic, strong) NSTextField *dateLbl;
@property (nonatomic, strong) NSTextField *tempLbl;
@property (nonatomic, strong) NSTextField *descLbl;

// Metrics grid (3×2)
@property (nonatomic, strong) NSGridView  *metricsGrid;
@property (nonatomic, strong) NSArray<WWMetricTile *> *metricTiles;
// order: 0=Humidity 1=Wind 2=WindGust 3=DewPoint 4=Pressure 5=UVIndex

// Air quality row
@property (nonatomic, strong) WWAirTile  *aqiTile;
@property (nonatomic, strong) NSArray<WWAirTile *> *pollTiles; // kPollSlots

// Error
@property (nonatomic, strong) NSTextField *errorLbl;

// Layout containers — rebuilt when the view mode changes.
@property (nonatomic, strong) NSStackView *leftBlock;  // icon + info column (enhanced)
@property (nonatomic, strong) NSStackView *infoCol;    // time/date/temp/desc column
@property (nonatomic, strong) NSStackView *middle;     // enhanced: leftBlock | metricsGrid
@property (nonatomic, strong) NSStackView *airRow;     // AQI + pollutants
@property (nonatomic, strong) NSArray<NSLayoutConstraint *> *rootConstraints;
@property (nonatomic) BOOL simpleMode;
@property (nonatomic) BOOL layoutApplied;

// Simple-mode plain-text lines (compact single-line labels, no boxes).
// Metric order matches _metricTiles: 0=Humidity 1=Wind 2=WindGust 3=DewPoint
// 4=Pressure 5=UVIndex. Pollutant order matches _pollTiles.
@property (nonatomic, strong) NSArray<NSTextField *> *simpleMetricLines;
@property (nonatomic, strong) NSTextField *simpleAQILine;
@property (nonatomic, strong) NSArray<NSTextField *> *simplePollLines;
@property (nonatomic, strong) NSStackView *simpleContent; // holds the simple column

// State
@property (nonatomic) BOOL isNight;

- (void)applyLayoutMode:(BOOL)simple;

@end

@implementation WWCityCardView

- (instancetype)initWithCity:(NSString *)city region:(NSString *)region {
    self = [super initWithFrame:NSMakeRect(0, 0, 600, 220)];
    if (!self) return nil;
    // YES (frame-based): the container positions each card via card.frame in
    // relayoutCards. The card's internal _rootStack still uses Auto Layout,
    // pinned to the card's edges, so children lay out correctly. If this were
    // NO, Auto Layout would ignore the container's frame calls and the cards
    // would collapse on top of each other.
    self.translatesAutoresizingMaskIntoConstraints = YES;
    self.wantsLayer = YES;
    self.layer.cornerRadius  = kCardRadius;
    self.layer.masksToBounds = YES;

    // ── Background image layer ────────────────────────────────────────────────
    _bgImageLayer = [CALayer layer];
    _bgImageLayer.contentsGravity = kCAGravityResizeAspectFill;
    _bgImageLayer.frame = self.bounds;
    _bgImageLayer.autoresizingMask = kCALayerWidthSizable | kCALayerHeightSizable;
    [self.layer addSublayer:_bgImageLayer];

    // ── Dark overlay layer ────────────────────────────────────────────────────
    _bgOverlayLayer = [CALayer layer];
    _bgOverlayLayer.backgroundColor = [NSColor colorWithRed:0 green:0 blue:0 alpha:0.50].CGColor;
    _bgOverlayLayer.frame = self.bounds;
    _bgOverlayLayer.autoresizingMask = kCALayerWidthSizable | kCALayerHeightSizable;
    [self.layer addSublayer:_bgOverlayLayer];

    // ── City label ────────────────────────────────────────────────────────────
    _cityLbl = lbl([NSString stringWithFormat:@"📍 %@, %@", city, region], 14, YES);

    // ── Weather icon ──────────────────────────────────────────────────────────
    _iconView = [[NSImageView alloc] init];
    _iconView.translatesAutoresizingMaskIntoConstraints = NO;
    _iconView.imageScaling = NSImageScaleProportionallyUpOrDown;
    [_iconView setContentHuggingPriority:NSLayoutPriorityRequired forOrientation:NSLayoutConstraintOrientationHorizontal];
    [_iconView setContentHuggingPriority:NSLayoutPriorityRequired forOrientation:NSLayoutConstraintOrientationVertical];
    [NSLayoutConstraint activateConstraints:@[
        [_iconView.widthAnchor  constraintEqualToConstant:90],
        [_iconView.heightAnchor constraintEqualToConstant:90],
    ]];

    // ── Info labels ───────────────────────────────────────────────────────────
    _timeLbl = lbl(@"--:--:--", 20, YES);
    _dateLbl = subLbl(@"", 11);
    _tempLbl = lbl(@"--°C", 34, YES);
    _descLbl = subLbl(@"", 12);

    _infoCol = [NSStackView stackViewWithViews:@[_timeLbl, _dateLbl, _tempLbl, _descLbl]];
    _infoCol.orientation = NSUserInterfaceLayoutOrientationVertical;
    _infoCol.alignment   = NSLayoutAttributeLeading;
    _infoCol.spacing = 2;
    _infoCol.translatesAutoresizingMaskIntoConstraints = NO;

    // Left block: icon beside info column
    _leftBlock = [NSStackView stackViewWithViews:@[_iconView, _infoCol]];
    _leftBlock.orientation = NSUserInterfaceLayoutOrientationHorizontal;
    _leftBlock.alignment   = NSLayoutAttributeCenterY;
    _leftBlock.spacing = 10;
    _leftBlock.translatesAutoresizingMaskIntoConstraints = NO;
    [_leftBlock setContentHuggingPriority:NSLayoutPriorityDefaultLow forOrientation:NSLayoutConstraintOrientationHorizontal];

    // ── Metrics grid (NSGridView, 3×2) ────────────────────────────────────────
    NSArray<NSString *> *emojis = @[@"💧",@"💨",@"🌬",@"💧",@"🌡",@"☀"];
    NSArray<NSString *> *names  = @[@"Humidity",@"Wind",@"Wind Gust",@"Dew Point",@"Pressure",@"UV Index"];
    NSMutableArray *tiles = [NSMutableArray array];
    for (int i = 0; i < 6; i++) {
        [tiles addObject:[[WWMetricTile alloc] initWithEmoji:emojis[i] name:names[i]]];
    }
    _metricTiles = [tiles copy];

    NSArray *row0 = @[_metricTiles[0], _metricTiles[1], _metricTiles[2]];
    NSArray *row1 = @[_metricTiles[3], _metricTiles[4], _metricTiles[5]];
    _metricsGrid = [NSGridView gridViewWithViews:@[row0, row1]];
    _metricsGrid.translatesAutoresizingMaskIntoConstraints = NO;
    _metricsGrid.rowSpacing    = 2;
    _metricsGrid.columnSpacing = 2;
    [_metricsGrid setContentHuggingPriority:NSLayoutPriorityDefaultLow forOrientation:NSLayoutConstraintOrientationHorizontal];

    // Middle row: left info block + metrics grid (enhanced arrangement).
    _middle = [NSStackView stackViewWithViews:@[_leftBlock, _metricsGrid]];
    _middle.orientation = NSUserInterfaceLayoutOrientationHorizontal;
    _middle.alignment   = NSLayoutAttributeCenterY;
    _middle.spacing = 12;
    _middle.translatesAutoresizingMaskIntoConstraints = NO;

    // ── Air quality row ───────────────────────────────────────────────────────
    NSArray<NSString *> *pollNames = @[@"CO",@"NO",@"NO₂",@"O₃",@"SO₂",@"NH₃",@"PM2.5",@"PM10"];
    NSMutableArray *ptiles = [NSMutableArray array];
    for (int i = 0; i < kPollSlots; i++) {
        WWAirTile *pt = [[WWAirTile alloc] initWithName:pollNames[i]];
        pt.hidden = YES;
        [ptiles addObject:pt];
    }
    _pollTiles = [ptiles copy];

    _aqiTile = [[WWAirTile alloc] initWithName:@"AQI"];
    _aqiTile.hidden = YES;

    // Air row: AQI on left, spacer, pollutants on right
    NSView *airSpacer = [[NSView alloc] init];
    airSpacer.translatesAutoresizingMaskIntoConstraints = NO;
    [airSpacer setContentHuggingPriority:NSLayoutPriorityDefaultLow forOrientation:NSLayoutConstraintOrientationHorizontal];

    NSMutableArray *airViews = [NSMutableArray arrayWithObject:_aqiTile];
    [airViews addObject:airSpacer];
    [airViews addObjectsFromArray:_pollTiles];

    _airRow = [NSStackView stackViewWithViews:airViews];
    _airRow.orientation = NSUserInterfaceLayoutOrientationHorizontal;
    _airRow.spacing = 6;
    _airRow.translatesAutoresizingMaskIntoConstraints = NO;

    // ── Simple-mode plain-text lines ──────────────────────────────────────────
    // These mirror the classic compact widget (image "simple"): one text line
    // per metric/pollutant, small emoji prefix, no boxes.
    // NOTE: this file is compiled under MANUAL reference counting (no ARC).
    // Views added into the hierarchy are retained by their superview, but these
    // simple-mode labels are NOT in the hierarchy while the card is in enhanced
    // mode, so we must retain them explicitly or they get freed after init and
    // crash when data updates touch them. The arrays are held by -copy (which
    // returns an owned +1 object); the standalone AQI label is retained here.
    NSMutableArray *sMetrics = [NSMutableArray array];
    for (int i = 0; i < 6; i++) {
        NSTextField *m = subLbl(@"", 10);
        m.alignment = NSTextAlignmentCenter;
        [sMetrics addObject:m];
    }
    _simpleMetricLines = [sMetrics copy];   // -copy returns an owned +1 object

    _simpleAQILine = [subLbl(@"", 10) retain]; // subLbl is autoreleased → retain

    _simpleAQILine.alignment = NSTextAlignmentCenter;
    _simpleAQILine.hidden = YES;

    NSMutableArray *sPoll = [NSMutableArray array];
    for (int i = 0; i < kPollSlots; i++) {
        NSTextField *p = subLbl(@"", 10);
        p.alignment = NSTextAlignmentCenter;
        p.hidden = YES;
        [sPoll addObject:p];
    }
    _simplePollLines = [sPoll copy];        // -copy returns an owned +1 object

    // ── Error label ───────────────────────────────────────────────────────────
    _errorLbl = lbl(@"⚠ Fetch error", 11, NO);
    _errorLbl.textColor = [NSColor colorWithRed:1.0 green:0.53 blue:0.53 alpha:1.0];
    _errorLbl.hidden = YES;

    // ── Root stack ────────────────────────────────────────────────────────────
    // Populated by applyLayoutMode: which arranges children for enhanced (wide,
    // horizontal) or simple (narrow, vertical) view. Default: enhanced.
    _rootStack = [[NSStackView alloc] initWithFrame:NSZeroRect];
    _rootStack.translatesAutoresizingMaskIntoConstraints = NO;
    [self addSubview:_rootStack];

    [NSLayoutConstraint activateConstraints:@[
        [_rootStack.leadingAnchor  constraintEqualToAnchor:self.leadingAnchor],
        [_rootStack.trailingAnchor constraintEqualToAnchor:self.trailingAnchor],
        [_rootStack.topAnchor      constraintEqualToAnchor:self.topAnchor],
        [_rootStack.bottomAnchor   constraintEqualToAnchor:self.bottomAnchor],
    ]];

    [self applyLayoutMode:NO];

    return self;
}

// applyLayoutMode rearranges the card's children for the requested view mode.
// Enhanced (simple==NO): wide horizontal — icon+info on the left, a 3×2 boxed
//   metrics grid on the right, boxed air-quality row below. Cards stack
//   vertically (one city under another) in the container.
// Simple (simple==YES): narrow vertical column — city, icon, big temp,
//   description, then compact plain-text metric lines and pollutant lines, then
//   time/date. Cards sit side by side in the container. ~210 px wide.
- (void)applyLayoutMode:(BOOL)simple {
    // Idempotent: skip the (destructive) rebuild if the mode is unchanged and
    // already applied once. relayoutCards calls this on every layout pass.
    if (_layoutApplied && _simpleMode == simple) return;
    _simpleMode = simple;
    _layoutApplied = YES;

    // Detach everything from the root stack first.
    for (NSView *v in [_rootStack.arrangedSubviews copy]) {
        [_rootStack removeArrangedSubview:v];
        [v removeFromSuperview];
    }
    // Detach the metrics grid / left block from any prior parent stacks so we
    // can re-home them into the mode-specific arrangement.
    for (NSView *v in [_middle.arrangedSubviews copy]) {
        [_middle removeArrangedSubview:v];
        [v removeFromSuperview];
    }

    if (!simple) {
        // Enhanced: metrics as a 3×2 grid beside the icon/info block.
        _leftBlock.orientation = NSUserInterfaceLayoutOrientationHorizontal;
        _infoCol.alignment = NSLayoutAttributeLeading;

        _cityLbl.alignment = NSTextAlignmentLeft;
        _timeLbl.alignment = NSTextAlignmentLeft;
        _dateLbl.alignment = NSTextAlignmentLeft;
        _tempLbl.alignment = NSTextAlignmentLeft;
        _descLbl.alignment = NSTextAlignmentLeft;

        // Restore the info column to its full enhanced set (time/date/temp/desc).
        [_infoCol setViews:@[_timeLbl, _dateLbl, _tempLbl, _descLbl]
                 inGravity:NSStackViewGravityLeading];
        [_leftBlock setViews:@[_iconView, _infoCol]
                   inGravity:NSStackViewGravityLeading];

        [_middle setViews:@[_leftBlock, _metricsGrid]
                inGravity:NSStackViewGravityLeading];
        _middle.orientation = NSUserInterfaceLayoutOrientationHorizontal;
        _middle.alignment = NSLayoutAttributeCenterY;
        _middle.spacing = 12;

        _airRow.alignment = NSLayoutAttributeCenterY;

        [_rootStack setViews:@[_cityLbl, _middle, _airRow, _errorLbl]
                   inGravity:NSStackViewGravityLeading];
        _rootStack.orientation = NSUserInterfaceLayoutOrientationVertical;
        _rootStack.alignment   = NSLayoutAttributeLeading;
        _rootStack.spacing = 8;
        [_rootStack setEdgeInsets:NSEdgeInsetsMake(12, 14, 12, 14)];
    } else {
        // Simple: single centered column (classic widget). City → icon → temp →
        // description → compact plain-text metric lines → AQI + pollutant lines
        // → time → date. Uses the dedicated simple-mode text labels (no boxes).
        NSMutableArray *views = [NSMutableArray array];
        [views addObject:_cityLbl];
        [views addObject:_iconView];
        [views addObject:_tempLbl];
        [views addObject:_descLbl];
        [views addObjectsFromArray:_simpleMetricLines];
        [views addObject:_simpleAQILine];
        [views addObjectsFromArray:_simplePollLines];
        [views addObject:_timeLbl];
        [views addObject:_dateLbl];
        [views addObject:_errorLbl];

        _cityLbl.alignment = NSTextAlignmentCenter;
        _tempLbl.alignment = NSTextAlignmentCenter;
        _descLbl.alignment = NSTextAlignmentCenter;
        _timeLbl.alignment = NSTextAlignmentCenter;
        _dateLbl.alignment = NSTextAlignmentCenter;

        [_rootStack setViews:views inGravity:NSStackViewGravityLeading];
        _rootStack.orientation = NSUserInterfaceLayoutOrientationVertical;
        _rootStack.alignment   = NSLayoutAttributeCenterX;
        _rootStack.spacing = 4;
        [_rootStack setEdgeInsets:NSEdgeInsetsMake(12, 10, 12, 10)];
    }

    [self setNeedsLayout:YES];
}

// ── Background image loading ──────────────────────────────────────────────────

- (void)loadBackground:(BOOL)night {
    if (night == _isNight && _bgImageLayer.contents != nil) return;
    _isNight = night;
    NSString *name = night ? @"night" : @"day";

    // Try app bundle first.
    NSString *path = [[NSBundle mainBundle] pathForResource:name ofType:@"jpg"
                                                inDirectory:@"backgrounds"];
    if (!path) {
        // Dev build: look alongside executable.
        NSString *exeDir = [NSBundle mainBundle].executablePath.stringByDeletingLastPathComponent;
        path = [[exeDir stringByAppendingPathComponent:
                 [NSString stringWithFormat:@"assets/backgrounds/%@.jpg", name]]
                stringByStandardizingPath];
    }
    if (path) {
        NSImage *img = [[NSImage alloc] initWithContentsOfFile:path];
        if (img) {
            // Convert NSImage to CGImageRef for the CALayer.
            NSSize sz = img.size;
            CGRect r = CGRectMake(0, 0, sz.width, sz.height);
            _bgImageLayer.contents = (__bridge id)[img CGImageForProposedRect:&r context:nil hints:nil];
        }
    }
}

// ── Data update ───────────────────────────────────────────────────────────────

- (void)applyIconNS:(NSString *)iconPath {
    if (iconPath.length == 0) return;
    NSImage *img = [[NSImage alloc] initWithContentsOfFile:iconPath];
    if (img) _iconView.image = img;
}

- (void)updateWithIconNS:(NSString *)iconPath
    city:(NSString *)city time:(NSString *)timeStr date:(NSString *)dateStr
    temp:(NSString *)tempStr desc:(NSString *)descStr
    humid:(NSString *)humidStr wind:(NSString *)windStr
    windGust:(NSString *)windGustStr dewPt:(NSString *)dewPtStr
    press:(NSString *)pressStr uv:(NSString *)uvStr
    isNight:(BOOL)night opacity:(double)opacity
{
    [self loadBackground:night];

    // Overlay alpha: darker when more opaque so cards pop against desktop.
    CGFloat overlayAlpha = 0.45 + (1.0 - (CGFloat)opacity) * 0.25;
    _bgOverlayLayer.backgroundColor =
        [NSColor colorWithRed:0 green:0 blue:0 alpha:overlayAlpha].CGColor;

    // Set label only when the string is non-nil/non-empty (nil means "keep
    // current", so the live clock isn't wiped by a weather-only update).
    void (^set)(NSTextField *, NSString *) = ^(NSTextField *f, NSString *s) {
        if (s.length > 0) f.stringValue = s;
    };

    set(_cityLbl,             city);
    set(_timeLbl,             timeStr);
    set(_dateLbl,             dateStr);
    set(_tempLbl,             tempStr);
    set(_descLbl,             descStr);

    // Enhanced boxed tiles.
    set(_metricTiles[0].valueLbl, humidStr);
    set(_metricTiles[1].valueLbl, windStr);
    set(_metricTiles[2].valueLbl, windGustStr);
    set(_metricTiles[3].valueLbl, dewPtStr);
    set(_metricTiles[4].valueLbl, pressStr);
    set(_metricTiles[5].valueLbl, uvStr);

    // Simple-mode plain-text lines: "emoji  value". Only update when a value is
    // provided (clock-only refreshes pass empty weather strings).
    void (^setLine)(NSTextField *, NSString *, NSString *) =
        ^(NSTextField *f, NSString *emoji, NSString *s) {
        if (s.length > 0) f.stringValue = [NSString stringWithFormat:@"%@ %@", emoji, s];
    };
    setLine(_simpleMetricLines[0], @"💧", humidStr);
    setLine(_simpleMetricLines[1], @"💨", windStr);
    setLine(_simpleMetricLines[2], @"🌬", windGustStr);
    setLine(_simpleMetricLines[3], @"💧", dewPtStr);
    setLine(_simpleMetricLines[4], @"🌡", pressStr);
    setLine(_simpleMetricLines[5], @"☀", uvStr);

    [self applyIconNS:iconPath];
}

- (void)setAQINS:(NSString *)label {
    NSString *s = label.length > 0 ? label : nil;
    if (s) {
        _aqiTile.valueLbl.stringValue = s;
        _aqiTile.hidden = NO;
        _simpleAQILine.stringValue = [NSString stringWithFormat:@"AQI: %@", s];
        // Visibility in simple mode is governed by applyFieldMask/data; show it
        // here since AQI data just arrived.
        if (_simpleMode) _simpleAQILine.hidden = NO;
    } else {
        _aqiTile.hidden = YES;
        _simpleAQILine.hidden = YES;
    }
}

// Pollutant slot names for the simple-mode text lines (index matches slot).
static NSString *pollSlotName(int slot) {
    switch (slot) {
        case 0: return @"CO";   case 1: return @"NO";
        case 2: return @"NO₂";  case 3: return @"O₃";
        case 4: return @"SO₂";  case 5: return @"NH₃";
        case 6: return @"PM2.5"; case 7: return @"PM10";
    }
    return @"";
}

- (void)setPollSlotNS:(int)slot iconPath:(NSString *)iconPath value:(NSString *)value {
    if (slot < 0 || slot >= kPollSlots) return;
    WWAirTile *tile = _pollTiles[slot];
    NSTextField *line = _simplePollLines[slot];
    if (value.length > 0) {
        tile.valueLbl.stringValue = value;
        if (iconPath.length > 0) {
            NSImage *img = [[NSImage alloc] initWithContentsOfFile:iconPath];
            if (img) tile.iconView.image = img;
        }
        tile.hidden = NO;
        line.stringValue = [NSString stringWithFormat:@"%@: %@", pollSlotName(slot), value];
        if (_simpleMode) line.hidden = NO;
    } else {
        tile.hidden = YES;
        line.hidden = YES;
    }
}

- (void)applyFieldMask:(DFMask)mask {
    _cityLbl.hidden            = !(mask & DFCity);
    _iconView.hidden           = !(mask & DFIcon);
    _tempLbl.hidden            = !(mask & DFTemp);
    _descLbl.hidden            = !(mask & DFDesc);
    _timeLbl.hidden            = !(mask & DFTime);
    _dateLbl.hidden            = !(mask & DFDate);

    // Enhanced boxed tiles.
    _metricTiles[0].hidden     = !(mask & DFHumidity);
    _metricTiles[1].hidden     = !(mask & DFWind);
    _metricTiles[2].hidden     = !(mask & DFWindGust);
    _metricTiles[3].hidden     = !(mask & DFDewPoint);
    _metricTiles[4].hidden     = !(mask & DFPressure);
    _metricTiles[5].hidden     = !(mask & DFUVIndex);

    // Simple-mode plain-text metric lines (same visibility rules).
    _simpleMetricLines[0].hidden = !(mask & DFHumidity);
    _simpleMetricLines[1].hidden = !(mask & DFWind);
    _simpleMetricLines[2].hidden = !(mask & DFWindGust);
    _simpleMetricLines[3].hidden = !(mask & DFDewPoint);
    _simpleMetricLines[4].hidden = !(mask & DFPressure);
    _simpleMetricLines[5].hidden = !(mask & DFUVIndex);
}

- (void)showError:(BOOL)visible stale:(BOOL)stale {
    _errorLbl.stringValue = stale ? @"⚠ Stale data" : @"⚠ Fetch error";
    _errorLbl.hidden = !visible;
}

- (void)applyFontSizes:(CGFloat)cityTime temp:(CGFloat)temp cond:(CGFloat)cond {
    _cityLbl.font = [NSFont boldSystemFontOfSize:cityTime > 0 ? cityTime : 14];
    _timeLbl.font = [NSFont boldSystemFontOfSize:cityTime > 0 ? cityTime * 1.14 : 16];
    _dateLbl.font = [NSFont systemFontOfSize:cond > 0 ? cond : 10];
    _tempLbl.font = [NSFont boldSystemFontOfSize:temp > 0 ? temp : 32];
    _descLbl.font = [NSFont systemFontOfSize:cond > 0 ? cond : 10];

    CGFloat lineSize = cond > 0 ? cond : 10;
    for (NSTextField *l in _simpleMetricLines) l.font = [NSFont systemFontOfSize:lineSize];
    _simpleAQILine.font = [NSFont systemFontOfSize:lineSize];
    for (NSTextField *l in _simplePollLines) l.font = [NSFont systemFontOfSize:lineSize];
}

@end


// ── Container helpers ─────────────────────────────────────────────────────────

static NSArray<WWCityCardView *> *allCards(NSView *container) {
    NSMutableArray *out = [NSMutableArray array];
    for (NSView *v in container.subviews)
        if ([v isKindOfClass:[WWCityCardView class]])
            [out addObject:(WWCityCardView *)v];
    return out;
}

// measuredHeight sets the card to its target width, forces a layout pass, then
// returns the height its Auto Layout content needs. Width must be fixed first
// or the wrapping metrics grid reports the wrong height.
static CGFloat measuredHeight(WWCityCardView *card, CGFloat width) {
    NSRect f = card.frame;
    f.size.width = width;
    card.frame = f;
    [card layoutSubtreeIfNeeded];
    CGFloat h = card.fittingSize.height;
    if (h < 40) h = 200; // sane fallback if Auto Layout hasn't resolved yet
    return h;
}

static void relayoutCards(NSView *container, BOOL simple, int count) {
    NSArray<WWCityCardView *> *cards = allCards(container);
    if (!cards.count) return;

    // Ensure each card's internal arrangement matches the requested mode.
    for (WWCityCardView *card in cards) {
        [card applyLayoutMode:simple];
    }

    CGFloat containerH = container.bounds.size.height;

    if (!simple) {
        // Enhanced: stack vertically. NSView is bottom-left origin, so place the
        // first card at the TOP (highest y) and work downward.
        CGFloat y = containerH - kCardPad;
        for (WWCityCardView *card in cards) {
            CGFloat h = measuredHeight(card, kEnhancedCardW);
            y -= h;
            card.frame = NSMakeRect(kCardPad, y, kEnhancedCardW, h);
            y -= kCardGap;
        }
    } else {
        // Simple: side by side. Pin to top.
        CGFloat x = kCardPad;
        for (WWCityCardView *card in cards) {
            CGFloat h = measuredHeight(card, kSimpleCardW);
            card.frame = NSMakeRect(x, containerH - kCardPad - h, kSimpleCardW, h);
            x += kSimpleCardW + kCardGap;
        }
    }
}

static NSSize containerFitSize(NSView *container, BOOL simple) {
    NSArray<WWCityCardView *> *cards = allCards(container);
    if (!cards.count) return NSMakeSize(kEnhancedCardW + kCardPad * 2, 300);

    if (!simple) {
        CGFloat h = kCardPad;
        for (WWCityCardView *card in cards) {
            h += measuredHeight(card, kEnhancedCardW) + kCardGap;
        }
        return NSMakeSize(kEnhancedCardW + kCardPad * 2, h - kCardGap + kCardPad);
    } else {
        CGFloat maxH = 0;
        NSInteger n = 0;
        for (WWCityCardView *card in cards) {
            maxH = MAX(maxH, measuredHeight(card, kSimpleCardW));
            n++;
        }
        CGFloat w = kCardPad + n * (kSimpleCardW + kCardGap) - kCardGap + kCardPad;
        return NSMakeSize(w, maxH + kCardPad * 2);
    }
}


// ── Public C API ──────────────────────────────────────────────────────────────

uintptr_t addCityCard(uintptr_t containerHandle, const char *city, const char *region) {
    __block uintptr_t handle = 0;
    run_on_main(^{
        NSView *container = (__bridge NSView *)(void *)containerHandle;
        NSString *c = [NSString stringWithUTF8String:city   ?: ""];
        NSString *r = [NSString stringWithUTF8String:region ?: ""];
        WWCityCardView *card = [[WWCityCardView alloc] initWithCity:c region:r];
        [container addSubview:card];
        CFRetain((__bridge CFTypeRef)card);
        handle = (uintptr_t)(__bridge void *)card;
    });
    return handle;
}

void removeAllCards(uintptr_t containerHandle) {
    // run_on_main (inline on main thread) so that when buildCards calls
    // removeAllCards then addCityCard, the ordering is preserved. Using
    // dispatch_async here would queue the removal to run AFTER the inline
    // addCityCard calls, wiping out freshly-added cards once the run loop starts.
    run_on_main(^{
        NSView *container = (__bridge NSView *)(void *)containerHandle;
        for (NSView *v in [container.subviews copy]) {
            if ([v isKindOfClass:[WWCityCardView class]]) {
                [v removeFromSuperview];
                CFRelease((__bridge CFTypeRef)v);
            }
        }
    });
}

// nsFromC converts a C string to an NSString, or nil if the C string is NULL
// or empty. The conversion COPIES the bytes, which is essential: these C
// functions are called from Go with strings that Go frees immediately after
// the call returns (defer C.free). We must copy synchronously — before the
// dispatch_async block runs — or the block reads freed/garbage memory.
static NSString *nsFromC(const char *s) {
    if (!s || !*s) return nil;
    return [NSString stringWithUTF8String:s];
}

void updateCardData(
    uintptr_t cardHandle,
    const char *iconPath, const char *city,
    const char *timeStr,  const char *dateStr,
    const char *tempStr,  const char *descStr,
    const char *humidStr, const char *windStr,
    const char *windGustStr, const char *dewPtStr,
    const char *pressStr, const char *uvStr,
    int isNight, double opacity
) {
    // Copy all C strings into NSStrings NOW, synchronously, before the async
    // block. Go frees the C strings as soon as this function returns.
    NSString *nIcon  = nsFromC(iconPath);
    NSString *nCity  = nsFromC(city);
    NSString *nTime  = nsFromC(timeStr);
    NSString *nDate  = nsFromC(dateStr);
    NSString *nTemp  = nsFromC(tempStr);
    NSString *nDesc  = nsFromC(descStr);
    NSString *nHumid = nsFromC(humidStr);
    NSString *nWind  = nsFromC(windStr);
    NSString *nGust  = nsFromC(windGustStr);
    NSString *nDew   = nsFromC(dewPtStr);
    NSString *nPress = nsFromC(pressStr);
    NSString *nUV    = nsFromC(uvStr);

    dispatch_async(dispatch_get_main_queue(), ^{
        WWCityCardView *card = (__bridge WWCityCardView *)(void *)cardHandle;
        [card updateWithIconNS:nIcon city:nCity time:nTime date:nDate
              temp:nTemp desc:nDesc
              humid:nHumid wind:nWind windGust:nGust
              dewPt:nDew press:nPress uv:nUV
              isNight:(BOOL)isNight opacity:opacity];
    });
}

void updateCardAQI(uintptr_t cardHandle, const char *label) {
    NSString *nLabel = nsFromC(label);
    dispatch_async(dispatch_get_main_queue(), ^{
        [(__bridge WWCityCardView *)(void *)cardHandle setAQINS:nLabel];
    });
}

void updateCardPollutant(uintptr_t cardHandle, int slot,
                          const char *iconPath, const char *value) {
    NSString *nIcon  = nsFromC(iconPath);
    NSString *nValue = nsFromC(value);
    dispatch_async(dispatch_get_main_queue(), ^{
        [(__bridge WWCityCardView *)(void *)cardHandle
            setPollSlotNS:slot iconPath:nIcon value:nValue];
    });
}

void setCardFieldVisibility(uintptr_t cardHandle, unsigned int fieldMask) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [(__bridge WWCityCardView *)(void *)cardHandle
            applyFieldMask:(DFMask)fieldMask];
    });
}

void setContainerLayout(uintptr_t containerHandle, int mode, int cardCount) {
    run_on_main(^{
        NSView *c = (__bridge NSView *)(void *)containerHandle;
        relayoutCards(c, (BOOL)mode, cardCount);
    });
}

void resizeWindowToContainer(uintptr_t winHandle, uintptr_t containerHandle,
                              int mode, int cardCount) {
    run_on_main(^{
        NSWindow *w = (__bridge NSWindow *)(void *)winHandle;
        NSView *c   = (__bridge NSView *)(void *)containerHandle;

        // 1. Compute the required size from card content.
        NSSize sz = containerFitSize(c, (BOOL)mode);

        // 2. Resize the window (keep top-left anchor), which resizes the
        //    content view / container to match.
        NSRect f = w.frame;
        CGFloat top = f.origin.y + f.size.height;
        f.size = sz;
        f.origin.y = top - sz.height;
        [w setFrame:f display:YES];
        c.frame = w.contentView.bounds;

        // 3. Now that the container has its final height, position the cards
        //    (relayoutCards reads container.bounds.height for top-anchoring).
        relayoutCards(c, (BOOL)mode, cardCount);
    });
}

void showCardError(uintptr_t cardHandle, int visible, int stale) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [(__bridge WWCityCardView *)(void *)cardHandle
            showError:(BOOL)visible stale:(BOOL)stale];
    });
}

void setCardFontSizes(uintptr_t cardHandle, int cityTime, int temp, int cond) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [(__bridge WWCityCardView *)(void *)cardHandle
            applyFontSizes:(CGFloat)cityTime temp:(CGFloat)temp cond:(CGFloat)cond];
    });
}
