package panel

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	_ "image/png"
	"time"

	"weatherwidget/assets"
)

type animatedFrames struct {
	frames []image.Image
	delays []time.Duration
}

// decodeGIF decodes an animated GIF and composites each frame properly
// onto an RGBA canvas taking disposal methods into account.
func decodeGIF(data []byte) (*animatedFrames, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(g.Image) == 0 {
		return nil, fmt.Errorf("empty gif")
	}

	width := g.Config.Width
	height := g.Config.Height
	if width <= 0 || height <= 0 {
		bounds := g.Image[0].Bounds()
		width = bounds.Dx()
		height = bounds.Dy()
	}

	rect := image.Rect(0, 0, width, height)
	canvas := image.NewRGBA(rect)

	frames := make([]image.Image, 0, len(g.Image))
	delays := make([]time.Duration, 0, len(g.Image))
	var backup *image.RGBA

	for i, srcFrame := range g.Image {
		disposal := byte(gif.DisposalNone)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}

		if disposal == gif.DisposalPrevious {
			backup = cloneRGBA(canvas)
		}

		// Draw current frame onto canvas
		draw.Draw(canvas, srcFrame.Bounds(), srcFrame, srcFrame.Bounds().Min, draw.Over)

		// Save a copy of the composited frame
		frames = append(frames, cloneRGBA(canvas))

		// Frame delay in GIF is in 100ths of a second (10ms units)
		delayMs := 100
		if i < len(g.Delay) && g.Delay[i] > 0 {
			delayMs = g.Delay[i] * 10
		}
		if delayMs < 20 {
			delayMs = 100
		}
		delays = append(delays, time.Duration(delayMs)*time.Millisecond)

		// Handle disposal for next frame
		switch disposal {
		case gif.DisposalBackground:
			draw.Draw(canvas, srcFrame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			if backup != nil {
				canvas = cloneRGBA(backup)
			}
		}
	}

	return &animatedFrames{
		frames: frames,
		delays: delays,
	}, nil
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	copy(dst.Pix, src.Pix)
	return dst
}

// loadIconAsset searches for the given icon in priority order (.gif -> .webp -> .png).
// If an animated GIF is found with > 1 frame, it returns animatedFrames.
// Otherwise, it returns the raw static bytes and file path.
func loadIconAsset(iconCode string) (anim *animatedFrames, staticData []byte, staticPath string, err error) {
	for _, ext := range []string{".gif", ".webp", ".png"} {
		path := fmt.Sprintf("icons/%s%s", iconCode, ext)
		data, readErr := assets.Icons.ReadFile(path)
		if readErr != nil {
			continue
		}

		if ext == ".gif" {
			if a, decErr := decodeGIF(data); decErr == nil && len(a.frames) > 1 {
				return a, nil, path, nil
			}
		}

		return nil, data, path, nil
	}

	return nil, nil, "", fmt.Errorf("icon not found for %s", iconCode)
}
