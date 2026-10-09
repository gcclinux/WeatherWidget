package panel

import (
	"image"
	"image/color"
	"math"
)

// dropletImage renders an anti-aliased blue water droplet (round bottom,
// pointed top) on a transparent size×size canvas.
func dropletImage(size int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	s := float64(size)
	cx, tipY := s/2, s*0.04
	r := s * 0.34
	cy := s - r - s*0.04
	d := cy - tipY
	tanA := r / math.Sqrt(d*d-r*r)
	tangentY := cy - r*r/d

	inside := func(x, y float64) bool {
		if y <= tangentY {
			return y >= tipY && math.Abs(x-cx) <= (y-tipY)*tanA
		}
		return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
	}

	const ss = 4
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			hits := 0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					if inside(float64(px)+(float64(sx)+0.5)/ss, float64(py)+(float64(sy)+0.5)/ss) {
						hits++
					}
				}
			}
			if hits == 0 {
				continue
			}
			// Vertical gradient: light cyan at the top to deeper blue at the bottom.
			t := float64(py) / s
			c := color.NRGBA{
				R: uint8(90 - 60*t),
				G: uint8(200 - 80*t),
				B: 255,
				A: uint8(255 * hits / (ss * ss)),
			}
			img.SetNRGBA(px, py, c)
		}
	}
	return img
}
