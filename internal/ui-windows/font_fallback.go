//go:build windows

package uiwindows

import (
	"image"
	"io"
	"log"
	"sync"
	"weatherwidget/assets"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// lazyFont is a fallback font that is read and parsed the first time a glyph
// is missing from the primary face. The result (including failure) is kept.
type lazyFont struct {
	name  string
	quiet bool // do not log load failures (optional system font)
	load  func() (*opentype.Font, error)

	once sync.Once
	font *opentype.Font
}

// embeddedFont returns a loader that parses an embedded font through its
// io.ReaderAt, so the table data stays in the executable image instead of
// being copied to the heap (droidsansfallback.ttf is about 4 MB).
func embeddedFont(path string) func() (*opentype.Font, error) {
	return func() (*opentype.Font, error) {
		file, err := assets.Fonts.Open(path)
		if err != nil {
			return nil, err
		}
		if ra, ok := file.(io.ReaderAt); ok {
			if f, err := opentype.ParseReaderAt(ra); err == nil {
				return f, nil
			}
			// Some fonts (Noto Sans Tamil) hit EOF in the ReaderAt path; copy those.
		}
		data, err := assets.Fonts.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return opentype.Parse(data)
	}
}

func (l *lazyFont) get() *opentype.Font {
	l.once.Do(func() {
		var err error
		l.font, err = l.load()
		if err != nil && !l.quiet {
			log.Printf("uiwindows: failed to load font %s: %v", l.name, err)
		}
	})
	return l.font
}

// fallbackFace draws with the primary face and, for runes the primary font has
// no glyph for, the first lazy fallback font that has one. Fallback fonts and
// their per-size faces are created on first need.
type fallbackFace struct {
	fonts []*opentype.Font // [0] is the primary font
	faces []font.Face      // [0] is the primary face

	lazy []*lazyFont
	size float64

	mu         sync.Mutex // guards buf and lazyFaces
	buf        sfnt.Buffer
	lazyFaces  []font.Face // parallel to lazy; nil until needed
	lazyFailed []bool      // face creation failed or font unavailable
}

func (f *fallbackFace) faceFor(r rune) font.Face {
	f.mu.Lock()
	defer f.mu.Unlock()
	if glyph, err := f.fonts[0].GlyphIndex(&f.buf, r); err == nil && glyph != 0 {
		return f.faces[0]
	}
	if f.lazyFaces == nil {
		f.lazyFaces = make([]font.Face, len(f.lazy))
		f.lazyFailed = make([]bool, len(f.lazy))
	}
	for i, l := range f.lazy {
		if f.lazyFailed[i] {
			continue
		}
		source := l.get()
		if source == nil {
			f.lazyFailed[i] = true
			continue
		}
		glyph, err := source.GlyphIndex(&f.buf, r)
		if err != nil || glyph == 0 {
			continue
		}
		if f.lazyFaces[i] == nil {
			face, err := opentype.NewFace(source, &opentype.FaceOptions{
				Size: f.size, DPI: 72, Hinting: font.HintingFull,
			})
			if err != nil {
				log.Printf("uiwindows: failed to create fallback font face size %.1f: %v", f.size, err)
				f.lazyFailed[i] = true
				continue
			}
			f.lazyFaces[i] = face
		}
		return f.lazyFaces[i]
	}
	return f.faces[0]
}

func (f *fallbackFace) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	var first error
	for _, face := range append(append([]font.Face(nil), f.faces...), f.lazyFaces...) {
		if face == nil {
			continue
		}
		if err := face.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (f *fallbackFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	return f.faceFor(r).Glyph(dot, r)
}

func (f *fallbackFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.faceFor(r).GlyphBounds(r)
}

func (f *fallbackFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	return f.faceFor(r).GlyphAdvance(r)
}

func (f *fallbackFace) Kern(r0, r1 rune) fixed.Int26_6 {
	first, second := f.faceFor(r0), f.faceFor(r1)
	if first == second {
		return first.Kern(r0, r1)
	}
	return 0
}

func (f *fallbackFace) Metrics() font.Metrics { return f.faces[0].Metrics() }
