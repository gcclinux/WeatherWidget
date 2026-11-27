//go:build windows

package uiwindows

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"weatherwidget/assets"
	"weatherwidget/internal/config"
	"weatherwidget/internal/i18n"
	"weatherwidget/internal/weather"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// WidgetDrawer handles all 32-bit ARGB offscreen drawing for desktop cards.
type WidgetDrawer struct {
	mu sync.Mutex

	// Cached font families
	regularFont   *opentype.Font
	boldFont      *opentype.Font
	fallbackFonts []*opentype.Font

	// Cached font faces key: "bold/regular:size"
	faceCache map[string]font.Face

	// Cached decoded icons key: path
	iconCache       map[string]image.Image
	animations      map[string]*nativeAnimation
	backgroundCache map[string]*image.RGBA

	// animSeen collects, during one Render, whether a moving element (ticker
	// text or a multi-frame GIF icon) was drawn; Render publishes it to animating.
	animSeen, animating atomic.Bool
}

// Animating reports whether the last Render drew anything that moves with time
// (scrolling ticker or animated GIF icon). When false, only the clock needs a
// once-per-second redraw.
func (wd *WidgetDrawer) Animating() bool {
	return wd != nil && wd.animating.Load()
}

// NewWidgetDrawer creates and initializes the card drawer.
func NewWidgetDrawer() *WidgetDrawer {
	wd := &WidgetDrawer{
		faceCache:       make(map[string]font.Face),
		iconCache:       make(map[string]image.Image),
		animations:      make(map[string]*nativeAnimation),
		backgroundCache: make(map[string]*image.RGBA),
	}

	for _, entry := range []struct {
		path   string
		target **opentype.Font
	}{
		{"fonts/segoeui.ttf", &wd.regularFont},
		{"fonts/segoeuib.ttf", &wd.boldFont},
	} {
		data, err := assets.Fonts.ReadFile(entry.path)
		if err == nil {
			*entry.target, err = opentype.Parse(data)
		}
		if err != nil {
			log.Printf("uiwindows: failed to load font %s: %v", entry.path, err)
		}
	}
	for _, path := range []string{"fonts/droidsansfallback.ttf", "fonts/notosanstamil.ttf"} {
		data, err := assets.Fonts.ReadFile(path)
		if err == nil {
			var f *opentype.Font
			f, err = opentype.Parse(data)
			if err == nil {
				wd.fallbackFonts = append(wd.fallbackFonts, f)
			}
		}
		if err != nil {
			log.Printf("uiwindows: failed to load font %s: %v", path, err)
		}
	}
	// Segoe UI Symbol supplies glyphs such as ✓ and ⚠ used by ticker messages.
	if dir := os.Getenv("WINDIR"); dir != "" {
		if data, err := os.ReadFile(filepath.Join(dir, "Fonts", "seguisym.ttf")); err == nil {
			if f, err := opentype.Parse(data); err == nil {
				wd.fallbackFonts = append(wd.fallbackFonts, f)
			}
		}
	}

	return wd
}

// getFace returns a cached font face for the requested weight and point size.
func (wd *WidgetDrawer) getFace(bold bool, size float64) font.Face {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	key := fmt.Sprintf("%v:%.1f", bold, size)
	if f, ok := wd.faceCache[key]; ok {
		return f
	}

	f := wd.regularFont
	if bold && wd.boldFont != nil {
		f = wd.boldFont
	}
	if f == nil {
		f = wd.regularFont
	}
	if f == nil {
		return nil
	}

	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Printf("uiwindows: failed to create font face size %.1f: %v", size, err)
		return nil
	}

	composite := &fallbackFace{fonts: []*opentype.Font{f}, faces: []font.Face{face}}
	for _, fallback := range wd.fallbackFonts {
		fallbackFace, err := opentype.NewFace(fallback, &opentype.FaceOptions{
			Size: size, DPI: 72, Hinting: font.HintingFull,
		})
		if err != nil {
			log.Printf("uiwindows: failed to create fallback font face size %.1f: %v", size, err)
			continue
		}
		composite.fonts = append(composite.fonts, fallback)
		composite.faces = append(composite.faces, fallbackFace)
	}
	wd.faceCache[key] = composite
	return composite
}

// loadIcon loads and caches an image from an embed.FS.
func (wd *WidgetDrawer) loadIcon(path string) image.Image {
	wd.mu.Lock()
	defer wd.mu.Unlock()

	if img, ok := wd.iconCache[path]; ok {
		return img
	}

	data, err := assets.Icons.ReadFile(path)
	if err != nil {
		data, err = assets.AirIcons.ReadFile(path)
		if err != nil {
			log.Printf("uiwindows: failed to load icon %s: %v", path, err)
			wd.iconCache[path] = nil
			return nil
		}
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("uiwindows: failed to decode icon %s: %v", path, err)
		wd.iconCache[path] = nil
		return nil
	}

	// Store as *image.RGBA so scaling takes the fast RGBA path and the
	// pre-scaled icon cache can key on the pointer.
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	wd.iconCache[path] = rgba
	return rgba
}

// Render draws all city cards and returns a 32-bit ARGB image ready for UpdateLayeredWindow.
func (wd *WidgetDrawer) Render(
	cities []config.CityConfig,
	results []weather.WeatherResult,
	cfg *config.Config,
	lm *i18n.LocaleManager,
	now time.Time,
) *image.RGBA {
	wd.animSeen.Store(false)
	defer func() { wd.animating.Store(wd.animSeen.Load()) }()
	if len(cities) == 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}

	viewMode := config.NormalizeViewMode(cfg.ViewMode)
	if viewMode == config.ViewModeSimple {
		return wd.renderSimpleView(cities, results, cfg, lm, now)
	}
	return wd.renderEnhancedView(cities, results, cfg, lm, now)
}

// ── Low-Level Pure-Go Anti-Aliased Graphics Utilities ─────────────────────────

// drawText draws a UTF-8 string on dst at the given baseline position (x, y).
func drawText(dst *image.RGBA, text string, x, y int, face font.Face, c color.RGBA) {
	if face == nil || text == "" {
		return
	}
	d := font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(text)
}

// measureText measures the pixel width of a string with the given font face.
func measureText(text string, face font.Face) int {
	if face == nil || text == "" {
		return 0
	}
	d := font.Drawer{Face: face}
	return d.MeasureString(text).Round()
}

// drawImage draws and scales an image onto dst with bilinear interpolation.
func drawImage(dst *image.RGBA, src image.Image, x, y, w, h int) {
	if src == nil {
		return
	}
	rect := image.Rect(x, y, x+w, y+h)
	if rgba, ok := src.(*image.RGBA); ok && w > 0 && h > 0 {
		draw.Draw(dst, rect, scaledIcon(rgba, w, h), image.Point{}, draw.Over)
		return
	}
	xdraw.BiLinear.Scale(dst, rect, src, src.Bounds(), draw.Over, nil)
}

// scaledIconKey identifies a pre-scaled icon by source image and final pixel size.
type scaledIconKey struct {
	src  *image.RGBA
	w, h int
}

// maxScaledIcons bounds the pre-scaled icon cache. A normal session needs a
// few dozen entries (icons x sizes, plus GIF frames); the cache is cleared if
// it ever exceeds this, for example after many scale changes.
const maxScaledIcons = 512

var (
	scaledIconMu    sync.Mutex
	scaledIconCache = make(map[scaledIconKey]*image.RGBA)
)

// scaledIcon returns src bilinear-scaled once to w x h pixels (origin 0,0).
// The cache key includes the final size, so DPI and widget-scale changes get
// correctly sized entries.
func scaledIcon(src *image.RGBA, w, h int) *image.RGBA {
	key := scaledIconKey{src, w, h}
	scaledIconMu.Lock()
	cached := scaledIconCache[key]
	scaledIconMu.Unlock()
	if cached != nil {
		return cached
	}
	scaled := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.BiLinear.Scale(scaled, scaled.Bounds(), src, src.Bounds(), draw.Src, nil)
	scaledIconMu.Lock()
	if len(scaledIconCache) >= maxScaledIcons {
		clear(scaledIconCache)
	}
	scaledIconCache[key] = scaled
	scaledIconMu.Unlock()
	return scaled
}

// drawImageContained scales an image uniformly to fit within the given box.
func drawImageContained(dst *image.RGBA, src image.Image, rect image.Rectangle) {
	if src == nil || src.Bounds().Empty() || rect.Empty() {
		return
	}
	w, h := rect.Dx(), rect.Dy()
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if w*sh > h*sw {
		w = max(1, (h*sw+sh/2)/sh)
	} else {
		h = max(1, (w*sh+sw/2)/sw)
	}
	drawImage(dst, src, rect.Min.X+(rect.Dx()-w)/2, rect.Min.Y+(rect.Dy()-h)/2, w, h)
}

// drawHorizontalLine draws a 1-pixel horizontal line.
func drawHorizontalLine(dst *image.RGBA, x1, x2, y int, c color.RGBA) {
	for x := x1; x <= x2; x++ {
		blendPixel(dst, x, y, c)
	}
}

// roundedCornerDist reports whether (px, py) falls in one of the four corner
// quadrants of a rounded rectangle and, if so, its distance to the arc centre.
func roundedCornerDist(px, py, minX, maxX, minY, maxY, radius int) (dist float64, inCorner bool) {
	if px < minX+radius && py < minY+radius { // Top-left
		dx := float64(minX+radius) - float64(px) - 0.5
		dy := float64(minY+radius) - float64(py) - 0.5
		return math.Sqrt(dx*dx + dy*dy), true
	} else if px > maxX-radius && py < minY+radius { // Top-right
		dx := float64(px) + 0.5 - float64(maxX-radius)
		dy := float64(minY+radius) - float64(py) - 0.5
		return math.Sqrt(dx*dx + dy*dy), true
	} else if px < minX+radius && py > maxY-radius { // Bottom-left
		dx := float64(minX+radius) - float64(px) - 0.5
		dy := float64(py) + 0.5 - float64(maxY-radius)
		return math.Sqrt(dx*dx + dy*dy), true
	} else if px > maxX-radius && py > maxY-radius { // Bottom-right
		dx := float64(px) + 0.5 - float64(maxX-radius)
		dy := float64(py) + 0.5 - float64(maxY-radius)
		return math.Sqrt(dx*dx + dy*dy), true
	}
	return 0, false
}

// roundedCornerBand reports whether row py intersects the top or bottom corner
// squares of a rounded rectangle with the given radius.
func roundedCornerBand(py, minY, maxY, radius int) bool {
	return radius > 0 && (py < minY+radius || py > maxY-radius)
}

// fillRoundedRect fills a rounded rectangle with smooth anti-aliased corners.
// Only the corner squares are classified per pixel; the span between them is
// blended directly.
func fillRoundedRect(dst *image.RGBA, x, y, w, h, radius int, c color.RGBA) {
	if radius <= 0 {
		draw.Draw(dst, image.Rect(x, y, x+w, y+h), image.NewUniform(color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}), image.Point{}, draw.Over)
		return
	}
	rF := float64(radius)
	minX, maxX := x, x+w-1
	minY, maxY := y, y+h-1
	cornerPixel := func(px, py int) {
		dist, inCorner := roundedCornerDist(px, py, minX, maxX, minY, maxY, radius)
		if !inCorner {
			blendPixel(dst, px, py, c)
		} else if dist <= rF-0.5 {
			blendPixel(dst, px, py, c)
		} else if dist < rF+0.5 {
			// Anti-aliased outer edge
			alphaFrac := (rF + 0.5 - dist)
			subAlpha := uint8(float64(c.A) * alphaFrac)
			blendPixel(dst, px, py, color.RGBA{c.R, c.G, c.B, subAlpha})
		}
	}
	for py := y; py < y+h; py++ {
		if !roundedCornerBand(py, minY, maxY, radius) {
			for px := x; px < x+w; px++ {
				blendPixel(dst, px, py, c)
			}
			continue
		}
		leftEnd := min(x+radius, x+w)
		rightStart := max(maxX-radius+1, x)
		if leftEnd > rightStart { // corner squares overlap: classify the whole row
			for px := x; px < x+w; px++ {
				cornerPixel(px, py)
			}
			continue
		}
		for px := x; px < leftEnd; px++ {
			cornerPixel(px, py)
		}
		for px := leftEnd; px < rightStart; px++ {
			blendPixel(dst, px, py, c)
		}
		for px := rightStart; px < x+w; px++ {
			cornerPixel(px, py)
		}
	}
}

// strokeRoundedRect draws a 1px rounded rectangle outline. It visits only the
// border pixels and the four corner squares instead of the whole rectangle.
func strokeRoundedRect(dst *image.RGBA, x, y, w, h, radius int, c color.RGBA, lineWidth int) {
	rF := float64(radius)
	minX, maxX := x, x+w-1
	minY, maxY := y, y+h-1
	pixel := func(px, py int) {
		dist, inCorner := roundedCornerDist(px, py, minX, maxX, minY, maxY, radius)
		if inCorner {
			if math.Abs(dist-rF) <= 0.8 {
				blendPixel(dst, px, py, c)
			}
		} else if px == minX || px == maxX || py == minY || py == maxY {
			blendPixel(dst, px, py, c)
		}
	}
	for py := y; py < y+h; py++ {
		// Columns [x, leftEnd) and [rightStart, x+w) can hold corner or border
		// pixels on this row; the span between them is empty unless the row is
		// the top or bottom edge.
		leftEnd, rightStart := x+1, maxX
		if roundedCornerBand(py, minY, maxY, radius) {
			leftEnd, rightStart = x+radius, maxX-radius+1
		}
		leftEnd = min(leftEnd, x+w)
		rightStart = max(rightStart, x)
		if leftEnd > rightStart || py == minY || py == maxY { // one contiguous span
			for px := x; px < x+w; px++ {
				pixel(px, py)
			}
			continue
		}
		for px := x; px < leftEnd; px++ {
			pixel(px, py)
		}
		for px := rightStart; px < x+w; px++ {
			pixel(px, py)
		}
	}
}

// blendPixel blends a color onto dst at (x, y) with alpha compositing.
func blendPixel(dst *image.RGBA, x, y int, src color.RGBA) {
	if x < dst.Rect.Min.X || x >= dst.Rect.Max.X || y < dst.Rect.Min.Y || y >= dst.Rect.Max.Y {
		return
	}
	if src.A == 0 {
		return
	}

	offset := dst.PixOffset(x, y)
	a := uint32(src.A)
	inv := 255 - a
	dst.Pix[offset] = uint8((uint32(src.R)*a + uint32(dst.Pix[offset])*inv + 127) / 255)
	dst.Pix[offset+1] = uint8((uint32(src.G)*a + uint32(dst.Pix[offset+1])*inv + 127) / 255)
	dst.Pix[offset+2] = uint8((uint32(src.B)*a + uint32(dst.Pix[offset+2])*inv + 127) / 255)
	dst.Pix[offset+3] = uint8(a + (uint32(dst.Pix[offset+3])*inv+127)/255)
}
