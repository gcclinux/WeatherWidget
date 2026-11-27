//go:build windows

package uiwindows

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// refFillRoundedRect is the original full-area implementation, kept as a pixel reference.
// fillRoundedRect fills a rounded rectangle with smooth anti-aliased corners.
func refFillRoundedRect(dst *image.RGBA, x, y, w, h, radius int, c color.RGBA) {
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

// refStrokeRoundedRect is the original full-area implementation, kept as a pixel reference.
// strokeRoundedRect draws a 1px rounded rectangle outline.
func refStrokeRoundedRect(dst *image.RGBA, x, y, w, h, radius int, c color.RGBA, lineWidth int) {
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
