package svg

import (
	"fmt"
	"image"
	"image/color"
	"io"
)

type svgRect struct {
	x, y, w, h int
	fill        color.RGBA
}

// Document is a resolution-independent representation of a sprite as SVG rectangles.
type Document struct {
	Width, Height int
	rects         []svgRect
}

// NewDocument builds a Document from an image by run-length-encoding each row into
// horizontal rect spans of identical colour. Fully transparent pixels are skipped.
func NewDocument(img image.Image) *Document {
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	doc := &Document{Width: W, Height: H}

	for y := range H {
		x := 0
		for x < W {
			c := img.At(b.Min.X+x, b.Min.Y+y)
			rv, gv, bv, av := c.RGBA()
			if av == 0 {
				x++
				continue
			}
			fill := color.RGBA{
				R: uint8(rv >> 8),
				G: uint8(gv >> 8),
				B: uint8(bv >> 8),
				A: uint8(av >> 8),
			}
			// Extend span rightward while the colour matches.
			xEnd := x + 1
			for xEnd < W {
				c2 := img.At(b.Min.X+xEnd, b.Min.Y+y)
				r2, g2, b2, a2 := c2.RGBA()
				fill2 := color.RGBA{
					R: uint8(r2 >> 8),
					G: uint8(g2 >> 8),
					B: uint8(b2 >> 8),
					A: uint8(a2 >> 8),
				}
				if fill2 != fill {
					break
				}
				xEnd++
			}
			doc.rects = append(doc.rects, svgRect{x: x, y: y, w: xEnd - x, h: 1, fill: fill})
			x = xEnd
		}
	}
	return doc
}

// Write writes the Document as an SVG document to w.
func (d *Document) Write(w io.Writer) error {
	if _, err := fmt.Fprintf(w, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`+"\n", d.Width, d.Height); err != nil {
		return err
	}
	for _, r := range d.rects {
		fill := fmt.Sprintf("#%02x%02x%02x", r.fill.R, r.fill.G, r.fill.B)
		if _, err := fmt.Fprintf(w, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`+"\n", r.x, r.y, r.w, r.h, fill); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, `</svg>`)
	return err
}
