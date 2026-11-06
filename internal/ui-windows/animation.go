//go:build windows

package uiwindows

import (
	"bytes"
	"image"
	"image/draw"
	"image/gif"
	"log"
	"time"

	"weatherwidget/assets"
)

type nativeAnimation struct {
	frames []image.Image
	delays []time.Duration
	total  time.Duration
}

func (wd *WidgetDrawer) weatherIcon(path string, now time.Time) image.Image {
	wd.mu.Lock()
	anim, cached := wd.animations[path]
	if !cached {
		anim = loadNativeAnimation(path + ".gif")
		wd.animations[path] = anim
	}
	wd.mu.Unlock()
	if anim == nil {
		return wd.loadIcon(path + ".png")
	}
	elapsed := time.Duration(now.UnixNano() % int64(anim.total))
	if elapsed < 0 {
		elapsed += anim.total
	}
	for i, delay := range anim.delays {
		if elapsed < delay {
			return anim.frames[i]
		}
		elapsed -= delay
	}
	return anim.frames[0]
}

func loadNativeAnimation(path string) *nativeAnimation {
	data, err := assets.Icons.ReadFile(path)
	if err != nil {
		return nil // Most weather icons have only a PNG.
	}
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		log.Printf("uiwindows: failed to decode animation %s: %v", path, err)
		return nil
	}
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	anim := &nativeAnimation{}
	for i, frame := range g.Image {
		var backup *image.RGBA
		disposal := byte(gif.DisposalNone)
		if i < len(g.Disposal) {
			disposal = g.Disposal[i]
		}
		if disposal == gif.DisposalPrevious {
			backup = image.NewRGBA(canvas.Bounds())
			copy(backup.Pix, canvas.Pix)
		}
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		copyFrame := image.NewRGBA(canvas.Bounds())
		copy(copyFrame.Pix, canvas.Pix)
		anim.frames = append(anim.frames, copyFrame)
		delay := 100 * time.Millisecond
		if i < len(g.Delay) && g.Delay[i] >= 2 {
			delay = time.Duration(g.Delay[i]) * 10 * time.Millisecond
		}
		anim.delays = append(anim.delays, delay)
		anim.total += delay
		switch disposal {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			canvas = backup
		}
	}
	if len(anim.frames) == 0 {
		log.Printf("uiwindows: animation %s has no frames", path)
		return nil
	}
	return anim
}
