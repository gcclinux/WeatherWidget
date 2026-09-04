package ui

import (
	"image/color"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"weatherwidget/assets"
)

// transparencyKey is the color Windows will make invisible via LWA_COLORKEY.
// Chosen to be near-black but distinct from pure black so it doesn't clash
// with any real UI element color.
var transparencyKey = color.NRGBA{R: 1, G: 1, B: 1, A: 255}

// transparencyActive is 1 when a color-key background should be used.
var transparencyActive atomic.Int32

// linuxBgShade stores the background RGB value for Linux (0=black, 255=white).
// Controlled by the opacity setting: 100% = fully dark (30), 25% = lighter (90).
var linuxBgShade atomic.Int32

var (
	fontMu      sync.RWMutex
	fontRegular fyne.Resource
	fontBold    fyne.Resource
	fontItalic  fyne.Resource
	currentFontLocale string
)

func init() {
	linuxBgShade.Store(30) // default: dark background
	SetLocaleFont("en-GB")
}

// loadFirstAvailableFont tries loading from embedded assets first, falling back to system paths.
func loadFirstAvailableFont(assetPath string, systemPaths ...string) []byte {
	if assetPath != "" {
		if data, err := assets.Fonts.ReadFile(assetPath); err == nil && len(data) > 0 {
			return data
		}
	}
	for _, p := range systemPaths {
		if p == "" {
			continue
		}
		if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
			return data
		}
	}
	return nil
}

// SetLocaleFont updates the active font resources based on the active locale script:
// - Tamil ("ta-IN"): Loads Noto Sans Tamil / Nirmala UI / Tamil Sangam font
// - Chinese ("zh-CN"): Loads CJK font (Droid Sans Fallback / Noto Sans CJK / Microsoft YaHei / PingFang)
// - Japanese ("ja-JP"): Loads CJK/Japanese font (Droid Sans Fallback / Noto Sans CJK / Yu Gothic / Meiryo)
// - Others (Latin/Western): Loads Segoe UI (or Segoe UI Bold / Italic)
func SetLocaleFont(locale string) {
	fontMu.Lock()
	defer fontMu.Unlock()

	currentFontLocale = locale

	switch {
	case strings.HasPrefix(locale, "ta"):
		// Tamil script
		regData := loadFirstAvailableFont(
			"fonts/notosanstamil.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansTamil-Regular.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansTamilUI-Regular.ttf",
			`C:\Windows\Fonts\Nirmala.ttf`,
			"/System/Library/Fonts/Supplemental/Tamil Sangam MN.ttc",
		)
		boldData := loadFirstAvailableFont(
			"fonts/notosanstamilb.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansTamil-Bold.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansTamilUI-Bold.ttf",
			`C:\Windows\Fonts\NirmalaB.ttf`,
		)
		if regData != nil {
			fontRegular = fyne.NewStaticResource("notosanstamil.ttf", regData)
		}
		if boldData != nil {
			fontBold = fyne.NewStaticResource("notosanstamilb.ttf", boldData)
		} else if regData != nil {
			fontBold = fontRegular
		}
		fontItalic = nil

	case strings.HasPrefix(locale, "zh"):
		// Simplified Chinese script
		regData := loadFirstAvailableFont(
			"fonts/droidsansfallback.ttf",
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
			"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
			`C:\Windows\Fonts\msyh.ttc`,
			`C:\Windows\Fonts\msyh.ttf`,
			`C:\Windows\Fonts\simsun.ttc`,
			"/System/Library/Fonts/PingFang.ttc",
			"/System/Library/Fonts/STHeiti Light.ttc",
		)
		boldData := loadFirstAvailableFont(
			"fonts/droidsansfallback.ttf",
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc",
			`C:\Windows\Fonts\msyhbd.ttc`,
			`C:\Windows\Fonts\msyhbd.ttf`,
		)
		if regData != nil {
			fontRegular = fyne.NewStaticResource("cjk_regular.ttf", regData)
		}
		if boldData != nil {
			fontBold = fyne.NewStaticResource("cjk_bold.ttf", boldData)
		} else if regData != nil {
			fontBold = fontRegular
		}
		fontItalic = nil

	case strings.HasPrefix(locale, "ja"):
		// Japanese script
		regData := loadFirstAvailableFont(
			"fonts/droidsansfallback.ttf",
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
			`C:\Windows\Fonts\YuGothM.ttc`,
			`C:\Windows\Fonts\meiryo.ttc`,
			`C:\Windows\Fonts\msgothic.ttc`,
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
		)
		boldData := loadFirstAvailableFont(
			"fonts/droidsansfallback.ttf",
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc",
			`C:\Windows\Fonts\YuGothB.ttc`,
			`C:\Windows\Fonts\meiryob.ttc`,
		)
		if regData != nil {
			fontRegular = fyne.NewStaticResource("cjk_jp_regular.ttf", regData)
		}
		if boldData != nil {
			fontBold = fyne.NewStaticResource("cjk_jp_bold.ttf", boldData)
		} else if regData != nil {
			fontBold = fontRegular
		}
		fontItalic = nil

	default:
		// Latin / Western languages: use Segoe UI
		if data, err := assets.Fonts.ReadFile("fonts/segoeui.ttf"); err == nil {
			fontRegular = fyne.NewStaticResource("segoeui.ttf", data)
		}
		if data, err := assets.Fonts.ReadFile("fonts/segoeuib.ttf"); err == nil {
			fontBold = fyne.NewStaticResource("segoeuib.ttf", data)
		}
		if data, err := assets.Fonts.ReadFile("fonts/segoeuii.ttf"); err == nil {
			fontItalic = fyne.NewStaticResource("segoeuii.ttf", data)
		}
	}
}

var (
	tamilCardFont     fyne.Resource
	tamilCardFontOnce sync.Once
	cjkCardFont       fyne.Resource
	cjkCardFontOnce   sync.Once
)

// LanguageCardFont returns a dedicated font resource for language cards
// whose native names require non-Latin script coverage (e.g. Tamil or CJK),
// ensuring their native script renders correctly regardless of which theme font is active.
// For Latin-based languages, nil is returned so the active theme font is used.
func LanguageCardFont(locale string) fyne.Resource {
	switch {
	case strings.HasPrefix(locale, "ta"):
		tamilCardFontOnce.Do(func() {
			data := loadFirstAvailableFont(
				"fonts/notosanstamilb.ttf",
				"fonts/notosanstamil.ttf",
				"/usr/share/fonts/truetype/noto/NotoSansTamil-Bold.ttf",
				"/usr/share/fonts/truetype/noto/NotoSansTamilUI-Bold.ttf",
				`C:\Windows\Fonts\NirmalaB.ttf`,
				`C:\Windows\Fonts\Nirmala.ttf`,
				"/System/Library/Fonts/Supplemental/Tamil Sangam MN.ttc",
			)
			if data != nil {
				tamilCardFont = fyne.NewStaticResource("notosanstamilb_card.ttf", data)
			}
		})
		return tamilCardFont

	case strings.HasPrefix(locale, "zh"), strings.HasPrefix(locale, "ja"):
		cjkCardFontOnce.Do(func() {
			data := loadFirstAvailableFont(
				"fonts/droidsansfallback.ttf",
				"/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc",
				`C:\Windows\Fonts\msyhbd.ttc`,
				`C:\Windows\Fonts\msyh.ttc`,
			)
			if data != nil {
				cjkCardFont = fyne.NewStaticResource("cjk_card.ttf", data)
			}
		})
		return cjkCardFont

	default:
		return nil
	}
}


// RefreshFontCache forces Fyne to drop its internal font-face cache and
// re-resolve faces from the current theme.
//
// Fyne caches rendered font faces keyed only by text style (not by the font
// resource), and it never re-resolves them on its own after startup. When the
// locale font changes via SetLocaleFont, previously-cached faces (e.g. the
// Latin Segoe UI face, which has no Tamil glyphs) would otherwise keep being
// used for new text — producing tofu (□/◇) boxes for complex scripts such as
// Tamil, most visibly on Windows.
//
// Re-applying the theme triggers Fyne's settings listener, which calls
// painter.ClearFontCache() internally and re-applies faces across all windows.
// This is the only public mechanism to invalidate that cache.
//
// Call this after SetLocaleFont on the Fyne main goroutine.
func RefreshFontCache(app fyne.App) {
	if app == nil {
		return
	}
	app.Settings().SetTheme(NewWidgetTheme(theme.DefaultTheme()))
}

// SetTransparencyActive switches the background color key on or off.
func SetTransparencyActive(active bool) {
	if active {
		transparencyActive.Store(1)
	} else {
		transparencyActive.Store(0)
	}
}

// SetLinuxBackgroundShade sets the Linux background darkness level.
// opacityPercent maps to background shade:
//
//	100% → RGB(30,30,30)  — fully dark, content most visible
//	 75% → RGB(50,50,50)  — slightly lighter
//	 50% → RGB(70,70,70)  — medium grey
//	 25% → RGB(90,90,90)  — lighter grey
func SetLinuxBackgroundShade(opacityPercent int) {
	var shade int
	switch {
	case opacityPercent >= 100:
		shade = 30
	case opacityPercent >= 75:
		shade = 50
	case opacityPercent >= 50:
		shade = 70
	default:
		shade = 90
	}
	linuxBgShade.Store(int32(shade))
}

// widgetTheme is a Fyne theme that:
//   - On Windows: replaces the background with a color-key when transparency is active
//   - On Linux: always uses a dark background (ignoring system light/dark preference)
//     to ensure the widget looks consistent and native
type widgetTheme struct {
	base fyne.Theme
}

// NewWidgetTheme wraps the given base theme.
func NewWidgetTheme(base fyne.Theme) fyne.Theme {
	return &widgetTheme{base: base}
}

func (t *widgetTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// Windows: color-key transparency when active.
	if transparencyActive.Load() == 1 {
		switch name {
		case theme.ColorNameBackground, theme.ColorNameOverlayBackground:
			return transparencyKey
		}
	}

	// Linux: always use dark variant colors for a consistent widget appearance,
	// regardless of the system theme (light/dark). The background shade is
	// controlled by the opacity setting.
	if runtime.GOOS == "linux" {
		switch name {
		case theme.ColorNameBackground, theme.ColorNameOverlayBackground:
			shade := uint8(linuxBgShade.Load())
			return color.NRGBA{R: shade, G: shade, B: shade, A: 255}
		case theme.ColorNameForeground:
			return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		case theme.ColorNameDisabled:
			return color.NRGBA{R: 180, G: 180, B: 180, A: 255}
		case theme.ColorNameSeparator:
			return color.NRGBA{R: 80, G: 80, B: 80, A: 255}
		}
		// For all other colors, force dark variant.
		return t.base.Color(name, theme.VariantDark)
	}

	// macOS: use a dark opaque background so the Fyne GL renderer clears to
	// a visible color. Transparency is achieved by setting NSWindow.backgroundColor
	// with the desired alpha via setDarwinBackgroundAlpha — the window is
	// non-opaque so the desktop shows through the background while Fyne-rendered
	// content (text, icons) remains fully opaque.
	if runtime.GOOS == "darwin" {
		switch name {
		case theme.ColorNameBackground, theme.ColorNameOverlayBackground:
			return color.NRGBA{R: 30, G: 30, B: 30, A: 255}
		case theme.ColorNameForeground:
			return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		case theme.ColorNameDisabled:
			return color.NRGBA{R: 180, G: 180, B: 180, A: 255}
		case theme.ColorNameSeparator:
			return color.NRGBA{R: 80, G: 80, B: 80, A: 255}
		}
		return t.base.Color(name, theme.VariantDark)
	}

	return t.base.Color(name, variant)
}

func (t *widgetTheme) Font(style fyne.TextStyle) fyne.Resource {
	fontMu.RLock()
	defer fontMu.RUnlock()

	if style.Bold && fontBold != nil {
		return fontBold
	}
	if style.Italic && fontItalic != nil {
		return fontItalic
	}
	if fontRegular != nil {
		return fontRegular
	}
	if t.base != nil {
		return t.base.Font(style)
	}
	return theme.DefaultTheme().Font(style)
}

func (t *widgetTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *widgetTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}
