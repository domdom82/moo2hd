package svg

import (
	"image"
	"image/color"
	"math"
)

// Scale2x scales src to twice its dimensions using the xBRZ 2x algorithm.
// Diagonal edges in pixel art are smoothed by blending neighbouring colours.
func Scale2x(src image.Image) image.Image {
	sb := src.Bounds()
	W, H := sb.Dx(), sb.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, W*2, H*2))

	// Fast-path: avoid interface dispatch in the inner loop when src is *image.RGBA.
	if rgba, ok := src.(*image.RGBA); ok {
		scale2xRGBA(rgba, dst)
		return dst
	}

	for sy := range H {
		for sx := range W {
			ax, ay := sb.Min.X+sx, sb.Min.Y+sy
			a := getPixel(src, ax-1, ay-1)
			b := getPixel(src, ax, ay-1)
			c := getPixel(src, ax+1, ay-1)
			d := getPixel(src, ax-1, ay)
			e := src.At(ax, ay)
			f := getPixel(src, ax+1, ay)
			g := getPixel(src, ax-1, ay+1)
			h := getPixel(src, ax, ay+1)
			ii := getPixel(src, ax+1, ay+1)

			e0, e1, e2, e3 := blendBlock(a, b, c, d, e, f, g, h, ii)
			dx, dy := sx*2, sy*2
			dst.Set(dx, dy, e0)
			dst.Set(dx+1, dy, e1)
			dst.Set(dx, dy+1, e2)
			dst.Set(dx+1, dy+1, e3)
		}
	}
	return dst
}

// scale2xRGBA is the fast path for *image.RGBA sources.
func scale2xRGBA(src *image.RGBA, dst *image.RGBA) {
	sb := src.Bounds()
	W, H := sb.Dx(), sb.Dy()
	for sy := range H {
		for sx := range W {
			ax, ay := sb.Min.X+sx, sb.Min.Y+sy
			a := getPixelRGBA(src, ax-1, ay-1)
			b := getPixelRGBA(src, ax, ay-1)
			c := getPixelRGBA(src, ax+1, ay-1)
			d := getPixelRGBA(src, ax-1, ay)
			e := src.RGBAAt(ax, ay)
			f := getPixelRGBA(src, ax+1, ay)
			g := getPixelRGBA(src, ax-1, ay+1)
			h := getPixelRGBA(src, ax, ay+1)
			ii := getPixelRGBA(src, ax+1, ay+1)

			e0, e1, e2, e3 := blendBlock(a, b, c, d, e, f, g, h, ii)
			dx, dy := sx*2, sy*2
			dst.SetRGBA(dx, dy, toRGBA8(e0))
			dst.SetRGBA(dx+1, dy, toRGBA8(e1))
			dst.SetRGBA(dx, dy+1, toRGBA8(e2))
			dst.SetRGBA(dx+1, dy+1, toRGBA8(e3))
		}
	}
}

// blendBlock computes the four output pixels for a single source pixel E
// given its 3×3 neighbourhood (A..I, E is centre).
//
//	A B C
//	D E F
//	G H I
func blendBlock(a, b, c, d, e, f, g, h, ii color.Color) (e0, e1, e2, e3 color.Color) {
	const threshold = 30.0

	eq := func(c1, c2 color.Color) bool {
		return colorDist(c1, c2) < threshold
	}

	e0 = e
	if eq(a, e) && !eq(b, e) && !eq(d, e) {
		e0 = blendColor(e, a, 0.5)
	}
	e1 = e
	if eq(c, e) && !eq(b, e) && !eq(f, e) {
		e1 = blendColor(e, c, 0.5)
	}
	e2 = e
	if eq(g, e) && !eq(h, e) && !eq(d, e) {
		e2 = blendColor(e, g, 0.5)
	}
	e3 = e
	if eq(ii, e) && !eq(h, e) && !eq(f, e) {
		e3 = blendColor(e, ii, 0.5)
	}
	return
}

// colorDist returns the weighted Euclidean distance between two colours in YCbCr space.
// Luminance is weighted ×2 to better match human perception.
func colorDist(c1, c2 color.Color) float64 {
	y1, cb1, cr1 := rgbToYCbCr(c1)
	y2, cb2, cr2 := rgbToYCbCr(c2)
	dy := float64(y1) - float64(y2)
	dcb := float64(cb1) - float64(cb2)
	dcr := float64(cr1) - float64(cr2)
	return math.Sqrt(2*dy*dy + dcb*dcb + dcr*dcr)
}

// rgbToYCbCr converts any color.Color to YCbCr components without allocation.
func rgbToYCbCr(c color.Color) (y, cb, cr uint8) {
	r, g, b, _ := c.RGBA() // 16-bit premultiplied
	r8 := uint8(r >> 8)
	g8 := uint8(g >> 8)
	b8 := uint8(b >> 8)
	yy, cbb, crr := color.RGBToYCbCr(r8, g8, b8)
	return yy, cbb, crr
}

// blendColor linearly interpolates between c1 and c2. t=0 returns c1, t=1 returns c2.
// Operates on premultiplied 16-bit values as returned by RGBA().
func blendColor(c1, c2 color.Color, t float64) color.Color {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()
	lerp := func(a, b uint32) uint8 {
		return uint8((float64(a)*(1-t) + float64(b)*t) / 256.0)
	}
	return color.RGBA{
		R: lerp(r1, r2),
		G: lerp(g1, g2),
		B: lerp(b1, b2),
		A: lerp(a1, a2),
	}
}

// getPixel returns a pixel clamped to the image bounds (edge-extension).
func getPixel(src image.Image, x, y int) color.Color {
	b := src.Bounds()
	if x < b.Min.X {
		x = b.Min.X
	} else if x >= b.Max.X {
		x = b.Max.X - 1
	}
	if y < b.Min.Y {
		y = b.Min.Y
	} else if y >= b.Max.Y {
		y = b.Max.Y - 1
	}
	return src.At(x, y)
}

// getPixelRGBA is the fast-path version of getPixel for *image.RGBA.
func getPixelRGBA(src *image.RGBA, x, y int) color.RGBA {
	b := src.Bounds()
	if x < b.Min.X {
		x = b.Min.X
	} else if x >= b.Max.X {
		x = b.Max.X - 1
	}
	if y < b.Min.Y {
		y = b.Min.Y
	} else if y >= b.Max.Y {
		y = b.Max.Y - 1
	}
	return src.RGBAAt(x, y)
}

// toRGBA8 converts any color.Color to color.RGBA. When the input is already
// color.RGBA no allocation occurs (type assertion).
func toRGBA8(c color.Color) color.RGBA {
	if r, ok := c.(color.RGBA); ok {
		return r
	}
	rv, gv, bv, av := c.RGBA()
	return color.RGBA{R: uint8(rv >> 8), G: uint8(gv >> 8), B: uint8(bv >> 8), A: uint8(av >> 8)}
}
