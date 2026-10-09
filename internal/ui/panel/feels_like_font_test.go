package panel

import (
	"testing"
)

// TestLoadBoldFontFace_ScriptCoverage verifies that the rotated feels-like
// label uses a font face with glyph coverage matching the locale's script.
// Tamil and CJK locales previously fell back to Segoe UI Bold (Latin only),
// which rendered their "Feels Like" translations as blank/tofu.
func TestLoadBoldFontFace_ScriptCoverage(t *testing.T) {
	cases := []struct {
		locale string
		text   string // the (space-stripped) feels-like translation
	}{
		{"en-GB", "FeelsLike"},
		{"ta-IN", "உணரல்"},
		{"zh-CN", "体感"},
		{"ja-JP", "体感温度"},
	}

	for _, tc := range cases {
		t.Run(tc.locale, func(t *testing.T) {
			face := loadBoldFontFace(11, tc.locale)
			if face == nil {
				t.Fatalf("loadBoldFontFace returned nil for %s", tc.locale)
			}

			for _, r := range tc.text {
				if _, ok := face.GlyphAdvance(r); !ok {
					t.Errorf("font for %s is missing glyph for rune %q (U+%04X)", tc.locale, r, r)
				}
			}
		})
	}
}
