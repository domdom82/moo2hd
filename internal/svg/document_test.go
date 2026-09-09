package svg_test

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/domdom82/moo2hd/internal/svg"
)

// pixel1x1 returns a 1×1 image with the given colour.
func pixel1x1(c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, c)
	return img
}

// pixels1xN returns a 1-row image with the given colours left-to-right.
func pixels1xN(colours ...color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, len(colours), 1))
	for x, c := range colours {
		img.Set(x, 0, c)
	}
	return img
}

var _ = Describe("NewDocument", func() {
	It("sets Width and Height from the image bounds", func() {
		doc := svg.NewDocument(solidImage(5, 3, color.RGBA{R: 255, A: 255}))
		Expect(doc.Width).To(Equal(5))
		Expect(doc.Height).To(Equal(3))
	})

	It("produces one rect for a 1×1 opaque image", func() {
		doc := svg.NewDocument(pixel1x1(color.RGBA{R: 255, A: 255}))
		var buf strings.Builder
		Expect(doc.Write(&buf)).To(Succeed())
		Expect(strings.Count(buf.String(), "<rect")).To(Equal(1))
	})

	It("merges two same-colour adjacent pixels into one wider rect", func() {
		red := color.RGBA{R: 255, A: 255}
		doc := svg.NewDocument(pixels1xN(red, red))
		var buf strings.Builder
		Expect(doc.Write(&buf)).To(Succeed())
		Expect(strings.Count(buf.String(), "<rect")).To(Equal(1))
		Expect(buf.String()).To(ContainSubstring(`width="2"`))
	})

	It("emits two rects for two different-colour adjacent pixels", func() {
		doc := svg.NewDocument(pixels1xN(
			color.RGBA{R: 255, A: 255},
			color.RGBA{G: 255, A: 255},
		))
		var buf strings.Builder
		Expect(doc.Write(&buf)).To(Succeed())
		Expect(strings.Count(buf.String(), "<rect")).To(Equal(2))
	})

	It("skips fully transparent pixels", func() {
		doc := svg.NewDocument(pixel1x1(color.RGBA{R: 0, G: 0, B: 0, A: 0}))
		var buf strings.Builder
		Expect(doc.Write(&buf)).To(Succeed())
		Expect(buf.String()).NotTo(ContainSubstring("<rect"))
	})

	It("emits the correct fill colour in hex", func() {
		doc := svg.NewDocument(pixel1x1(color.RGBA{R: 0xAB, G: 0xCD, B: 0xEF, A: 255}))
		var buf strings.Builder
		Expect(doc.Write(&buf)).To(Succeed())
		Expect(buf.String()).To(ContainSubstring(`fill="#abcdef"`))
	})
})

var _ = Describe("Document.Write", func() {
	It("produces a valid SVG envelope", func() {
		doc := svg.NewDocument(pixel1x1(color.RGBA{R: 10, G: 20, B: 30, A: 255}))
		var buf strings.Builder
		Expect(doc.Write(&buf)).To(Succeed())
		out := buf.String()
		Expect(out).To(ContainSubstring("<svg"))
		Expect(out).To(ContainSubstring("</svg>"))
		Expect(out).To(ContainSubstring(`viewBox="0 0 1 1"`))
	})

	It("reports an error when the writer fails", func() {
		doc := svg.NewDocument(pixel1x1(color.RGBA{R: 1, A: 255}))
		err := doc.Write(&failWriter{})
		Expect(err).To(HaveOccurred())
	})
})

// failWriter always returns an error on Write.
type failWriter struct{}

func (f *failWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("write error")
}
