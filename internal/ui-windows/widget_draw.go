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

	wd.iconCache[path] = img
	return img
}

// Render draws all city cards and returns a 32-bit ARGB image ready for UpdateLayeredWindow.
func (wd *WidgetDrawer) Render(
	cities []config.CityConfig,
	results []weather.WeatherResult,
	cfg *config.Config,
	lm *i18n.LocaleManager,
	now time.Time,
) *image.RGBA {
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
	xdraw.BiLinear.Scale(dst, rect, src, src.Bounds(), draw.Over, nil)
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

// fillRoundedRect fills a rounded rectangle with smooth anti-aliased corners.
func fillRoundedRect(dst *image.RGBA, x, y, w, h, radius int, c color.RGBA) {
	if radius <= 0 {
		draw.Draw(dst, image.Rect(x, y, x+w, y+h), image.NewUniform(color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}), image.Point{}, draw.Over)
		return
	}

	rF := float64(radius)
	minX, maxX := x, x+w-1
	minY, maxY := y, y+h-1

	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			// Check if pixel falls inside one of the four corner quadrants
			var dist float64
			inCorner := false

			if px < minX+radius && py < minY+radius { // Top-left
				dx := float64(minX+radius) - float64(px) - 0.5
				dy := float64(minY+radius) - float64(py) - 0.5
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px > maxX-radius && py < minY+radius { // Top-right
				dx := float64(px) + 0.5 - float64(maxX-radius)
				dy := float64(minY+radius) - float64(py) - 0.5
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px < minX+radius && py > maxY-radius { // Bottom-left
				dx := float64(minX+radius) - float64(px) - 0.5
				dy := float64(py) + 0.5 - float64(maxY-radius)
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px > maxX-radius && py > maxY-radius { // Bottom-right
				dx := float64(px) + 0.5 - float64(maxX-radius)
				dy := float64(py) + 0.5 - float64(maxY-radius)
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			}

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
	}
}

// strokeRoundedRect draws a 1px rounded rectangle outline.
func strokeRoundedRect(dst *image.RGBA, x, y, w, h, radius int, c color.RGBA, lineWidth int) {
	rF := float64(radius)
	minX, maxX := x, x+w-1
	minY, maxY := y, y+h-1

	for py := y; py < y+h; py++ {
		for px := x; px < x+w; px++ {
			var dist float64
			inCorner := false

			if px < minX+radius && py < minY+radius {
				dx := float64(minX+radius) - float64(px) - 0.5
				dy := float64(minY+radius) - float64(py) - 0.5
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px > maxX-radius && py < minY+radius {
				dx := float64(px) + 0.5 - float64(maxX-radius)
				dy := float64(minY+radius) - float64(py) - 0.5
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px < minX+radius && py > maxY-radius {
				dx := float64(minX+radius) - float64(px) - 0.5
				dy := float64(py) + 0.5 - float64(maxY-radius)
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			} else if px > maxX-radius && py > maxY-radius {
				dx := float64(px) + 0.5 - float64(maxX-radius)
				dy := float64(py) + 0.5 - float64(maxY-radius)
				dist = math.Sqrt(dx*dx + dy*dy)
				inCorner = true
			}

			if inCorner {
				if math.Abs(dist-rF) <= 0.8 {
					blendPixel(dst, px, py, c)
				}
			} else if px == minX || px == maxX || py == minY || py == maxY {
				blendPixel(dst, px, py, c)
			}
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
