//go:build windows

package uiwindows

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
	"weatherwidget/assets"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func TestFontLoadingAndDrawing(t *testing.T) {
	fontBytes, err := assets.Fonts.ReadFile("fonts/segoeui.ttf")
	if err != nil {
		t.Fatalf("failed to read segoeui.ttf: %v", err)
	}

	otf, err := opentype.Parse(fontBytes)
	if err != nil {
		t.Fatalf("failed to parse font: %v", err)
	}

	face, err := opentype.NewFace(otf, &opentype.FaceOptions{
		Size:    16,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		t.Fatalf("failed to create font face: %v", err)
	}
	defer face.Close()

	img := image.NewRGBA(image.Rect(0, 0, 380, 250))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{18, 20, 24, 200}}, image.Point{}, draw.Src)

	d := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{255, 255, 255, 255}),
		Face: face,
		Dot:  fixed.P(20, 40),
	}
	d.DrawString("London 21°C")

	// Verify some pixels were drawn (non-zero alpha)
	pixel := img.RGBAAt(25, 35)
	t.Logf("Sample pixel at (25, 35): R=%d G=%d B=%d A=%d", pixel.R, pixel.G, pixel.B, pixel.A)
	if pixel.A == 0 {
		t.Error("expected non-zero alpha on text pixel")
	}
}
