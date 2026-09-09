package svg

import (
	"image"
	"image/color"
)

// Upscaler converts a decoded sprite image to a scalable SVG Document.
type Upscaler interface {
	Upscale(src image.Image, palette color.Palette) (*Document, error)
}

// XBRZUpscaler implements Upscaler using the xBRZ 2x algorithm.
type XBRZUpscaler struct{}

// Upscale scales src to 2× using xBRZ and converts the result to an SVG Document.
// The palette parameter is accepted for interface compatibility but is not used here
// because xBRZ operates on already-decoded image.Image values.
func (XBRZUpscaler) Upscale(src image.Image, _ color.Palette) (*Document, error) {
	return NewDocument(Scale2x(src)), nil
}
