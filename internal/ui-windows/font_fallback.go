//go:build windows

package uiwindows

import (
	"image"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type fallbackFace struct {
	fonts []*opentype.Font
	faces []font.Face
	buf   sfnt.Buffer
}

func (f *fallbackFace) faceFor(r rune) font.Face {
	for i, source := range f.fonts {
		if glyph, err := source.GlyphIndex(&f.buf, r); err == nil && glyph != 0 {
			return f.faces[i]
		}
	}
	return f.faces[0]
}

func (f *fallbackFace) Close() error {
	var first error
	for _, face := range f.faces {
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
