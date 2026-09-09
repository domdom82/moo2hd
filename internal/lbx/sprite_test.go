package lbx_test

import (
	"encoding/binary"
	"image/color"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/domdom82/moo2hd/internal/lbx"
)

// buildSpriteRecord constructs a synthetic MOO2 sprite record in memory.
//
// palette: if non-nil, sets FlagInternalPalette and appends {baseIndex=0, count=len/4, entries}.
// frames: each entry is a pre-encoded frame body (indicator+yOffset+sequences) to embed verbatim.
func buildSpriteRecord(w, h uint16, frameCount uint16, flags uint16, palette []byte, frameBodies [][]byte) []byte {
	n := int(frameCount)

	// Fixed 12-byte header.
	hdr := make([]byte, 12)
	binary.LittleEndian.PutUint16(hdr[0:2], w)
	binary.LittleEndian.PutUint16(hdr[2:4], h)
	// hdr[4:6] reserved — leave zero
	binary.LittleEndian.PutUint16(hdr[6:8], frameCount)
	// hdr[8:10] frameDelay — leave zero
	binary.LittleEndian.PutUint16(hdr[10:12], flags)

	// Compute where each frame body will live.
	// Layout: header(12) + offsets((n+1)*4) + palette(if any) + frame bodies.
	offsetsSize := (n + 1) * 4
	paletteSection := []byte(nil)
	if palette != nil {
		count := len(palette) / 4
		ps := make([]byte, 4+len(palette))
		binary.LittleEndian.PutUint16(ps[0:2], 0) // baseIndex
		binary.LittleEndian.PutUint16(ps[2:4], uint16(count))
		copy(ps[4:], palette)
		paletteSection = ps
	}

	frameDataStart := 12 + offsetsSize + len(paletteSection)
	frameOffsets := make([]int, n+1)
	pos := frameDataStart
	for i, body := range frameBodies {
		frameOffsets[i] = pos
		pos += len(body)
	}
	frameOffsets[n] = pos // EOF marker

	// Encode offset table.
	offsets := make([]byte, offsetsSize)
	for i, off := range frameOffsets {
		binary.LittleEndian.PutUint32(offsets[i*4:i*4+4], uint32(off))
	}

	// Concatenate everything.
	out := hdr
	out = append(out, offsets...)
	out = append(out, paletteSection...)
	for _, body := range frameBodies {
		out = append(out, body...)
	}
	return out
}

// buildSparseFrame encodes a sparse-format frame body.
// pixels maps (x,y) → palette index. All other pixels are transparent.
// The frame is encoded as a series of single-pixel sequences (simple but valid).
func buildSparseFrame(w, h int, pixels map[[2]int]byte) []byte {
	var body []byte

	writeU16 := func(v uint16) {
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, v)
		body = append(body, b...)
	}

	writeU16(1) // indicator
	writeU16(0) // initial yOffset

	currentY := 0
	for y := range h {
		// Collect non-transparent pixels in this row, sorted by x.
		type px struct{ x int; idx byte }
		var row []px
		for x := range w {
			if idx, ok := pixels[[2]int{x, y}]; ok && idx != 0 {
				row = append(row, px{x, idx})
			}
		}
		if len(row) == 0 {
			continue
		}

		// Emit y-advance if needed.
		if y > currentY {
			writeU16(0)              // seqLen = 0 → y-advance
			writeU16(uint16(y - currentY))
			currentY = y
		}

		// Emit each pixel as its own sequence.
		xOff := 0
		for _, p := range row {
			relX := p.x - xOff
			writeU16(1)            // seqLen = 1
			writeU16(uint16(relX)) // relXOffset
			body = append(body, p.idx)
			body = append(body, 0) // padding byte (seqLen=1 is odd)
			xOff = p.x + 1
		}
	}

	// End-of-frame marker: seqLen=0, relXOffset=1000.
	writeU16(0)
	writeU16(1000)

	return body
}

// buildFlatFrame encodes a flat (FlagNoCompression) frame body: raw width×height palette indices.
func buildFlatFrame(w, h int, pixels []byte) []byte {
	if len(pixels) < w*h {
		panic("buildFlatFrame: not enough pixel data")
	}
	return pixels[:w*h]
}

// testPalette builds a 256-entry × 4-byte raw palette section (ARGB, 6-bit).
// Index 1 → red (63,0,0 in 6-bit → 252,0,0 after ×4).
// Index 2 → green.
// Index 3 → blue.
func testPalette() []byte {
	raw := make([]byte, 256*4)
	// index 1: red
	raw[1*4+0] = 0  // alpha (ignored)
	raw[1*4+1] = 63 // red6 → 252
	raw[1*4+2] = 0
	raw[1*4+3] = 0
	// index 2: green
	raw[2*4+0] = 0
	raw[2*4+1] = 0
	raw[2*4+2] = 63 // green6 → 252
	raw[2*4+3] = 0
	// index 3: blue
	raw[3*4+0] = 0
	raw[3*4+1] = 0
	raw[3*4+2] = 0
	raw[3*4+3] = 63 // blue6 → 252
	return raw
}

var _ = Describe("Sprite", func() {
	Describe("ParseSpriteHeader", func() {
		Context("error cases", func() {
			It("rejects data shorter than 12 bytes", func() {
				_, err := lbx.ParseSpriteHeader(make([]byte, 8))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("too short"))
			})

			It("rejects zero width", func() {
				data := make([]byte, 12)
				binary.LittleEndian.PutUint16(data[0:2], 0) // width = 0
				binary.LittleEndian.PutUint16(data[2:4], 4)
				binary.LittleEndian.PutUint16(data[6:8], 1)
				_, err := lbx.ParseSpriteHeader(data)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dimensions"))
			})

			It("rejects zero height", func() {
				data := make([]byte, 12)
				binary.LittleEndian.PutUint16(data[0:2], 4)
				binary.LittleEndian.PutUint16(data[2:4], 0) // height = 0
				binary.LittleEndian.PutUint16(data[6:8], 1)
				_, err := lbx.ParseSpriteHeader(data)
				Expect(err).To(HaveOccurred())
			})

			It("rejects zero frame count", func() {
				data := make([]byte, 12)
				binary.LittleEndian.PutUint16(data[0:2], 4)
				binary.LittleEndian.PutUint16(data[2:4], 4)
				binary.LittleEndian.PutUint16(data[6:8], 0) // frameCount = 0
				_, err := lbx.ParseSpriteHeader(data)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("zero frames"))
			})
		})

		Context("valid header", func() {
			It("parses all fields correctly", func() {
				data := make([]byte, 12)
				binary.LittleEndian.PutUint16(data[0:2], 32)  // width
				binary.LittleEndian.PutUint16(data[2:4], 64)  // height
				binary.LittleEndian.PutUint16(data[6:8], 3)   // frameCount
				binary.LittleEndian.PutUint16(data[8:10], 10) // frameDelay
				binary.LittleEndian.PutUint16(data[10:12], lbx.FlagInternalPalette)
				hdr, err := lbx.ParseSpriteHeader(data)
				Expect(err).NotTo(HaveOccurred())
				Expect(hdr.Width).To(Equal(uint16(32)))
				Expect(hdr.Height).To(Equal(uint16(64)))
				Expect(hdr.FrameCount).To(Equal(uint16(3)))
				Expect(hdr.FrameDelay).To(Equal(uint16(10)))
				Expect(hdr.Flags).To(Equal(lbx.FlagInternalPalette))
			})
		})
	})

	Describe("DecodeFrames", func() {
		Context("external palette", func() {
			It("returns an error when external palette is nil", func() {
				frame := buildSparseFrame(2, 2, map[[2]int]byte{{0, 0}: 1})
				rec := buildSpriteRecord(2, 2, 1, 0, nil, [][]byte{frame})
				_, err := lbx.DecodeFrames(rec, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("external palette"))
			})

			It("decodes pixel colours from the supplied palette", func() {
				extPal := make(color.Palette, 4)
				extPal[0] = color.Transparent
				extPal[1] = color.RGBA{R: 200, G: 0, B: 0, A: 255}
				extPal[2] = color.RGBA{R: 0, G: 200, B: 0, A: 255}
				extPal[3] = color.RGBA{R: 0, G: 0, B: 200, A: 255}

				// 2×2 image: top-left=1(red), top-right=2(green), bottom-left=3(blue), bottom-right=0(transparent)
				pixels := map[[2]int]byte{
					{0, 0}: 1,
					{1, 0}: 2,
					{0, 1}: 3,
				}
				frame := buildSparseFrame(2, 2, pixels)
				rec := buildSpriteRecord(2, 2, 1, 0, nil, [][]byte{frame})

				images, err := lbx.DecodeFrames(rec, extPal)
				Expect(err).NotTo(HaveOccurred())
				Expect(images).To(HaveLen(1))
				img := images[0]
				Expect(img.At(0, 0)).To(Equal(color.RGBA{R: 200, G: 0, B: 0, A: 255}))
				Expect(img.At(1, 0)).To(Equal(color.RGBA{R: 0, G: 200, B: 0, A: 255}))
				Expect(img.At(0, 1)).To(Equal(color.RGBA{R: 0, G: 0, B: 200, A: 255}))
			})
		})

		Context("internal palette", func() {
			It("decodes colours from the embedded palette", func() {
				rawPal := testPalette() // index 1=red(252,0,0), 2=green(0,252,0), 3=blue(0,0,252)

				pixels := map[[2]int]byte{{0, 0}: 1, {1, 0}: 2}
				frame := buildSparseFrame(2, 2, pixels)
				flags := lbx.FlagInternalPalette
				rec := buildSpriteRecord(2, 2, 1, flags, rawPal, [][]byte{frame})

				images, err := lbx.DecodeFrames(rec, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(images).To(HaveLen(1))
				img := images[0]
				Expect(img.At(0, 0)).To(Equal(color.RGBA{R: 252, G: 0, B: 0, A: 255}))
				Expect(img.At(1, 0)).To(Equal(color.RGBA{R: 0, G: 252, B: 0, A: 255}))
			})
		})

		Context("transparent pixels", func() {
			It("leaves index-0 pixels as fully transparent", func() {
				extPal := make(color.Palette, 2)
				extPal[0] = color.Transparent
				extPal[1] = color.RGBA{R: 255, A: 255}
				// Only pixel at (1,0) is set; (0,0) stays transparent.
				frame := buildSparseFrame(2, 1, map[[2]int]byte{{1, 0}: 1})
				rec := buildSpriteRecord(2, 1, 1, 0, nil, [][]byte{frame})

				images, err := lbx.DecodeFrames(rec, extPal)
				Expect(err).NotTo(HaveOccurred())
				r, g, b, a := images[0].At(0, 0).RGBA()
				Expect(a).To(Equal(uint32(0)), "pixel (0,0) should be transparent")
				_, _, _, _ = r, g, b, a
			})
		})

		Context("multi-frame animation", func() {
			It("returns the correct number of frames with independent pixels", func() {
				extPal := make(color.Palette, 4)
				extPal[0] = color.Transparent
				extPal[1] = color.RGBA{R: 100, A: 255}
				extPal[2] = color.RGBA{G: 100, A: 255}
				extPal[3] = color.RGBA{B: 100, A: 255}

				f0 := buildSparseFrame(2, 2, map[[2]int]byte{{0, 0}: 1})
				f1 := buildSparseFrame(2, 2, map[[2]int]byte{{0, 0}: 2})
				f2 := buildSparseFrame(2, 2, map[[2]int]byte{{0, 0}: 3})
				rec := buildSpriteRecord(2, 2, 3, 0, nil, [][]byte{f0, f1, f2})

				images, err := lbx.DecodeFrames(rec, extPal)
				Expect(err).NotTo(HaveOccurred())
				Expect(images).To(HaveLen(3))
				Expect(images[0].At(0, 0)).To(Equal(color.RGBA{R: 100, A: 255}))
				Expect(images[1].At(0, 0)).To(Equal(color.RGBA{G: 100, A: 255}))
				Expect(images[2].At(0, 0)).To(Equal(color.RGBA{B: 100, A: 255}))
			})
		})

		Context("FlagNoCompression", func() {
			It("decodes a flat pixel array correctly", func() {
				extPal := make(color.Palette, 3)
				extPal[0] = color.Transparent
				extPal[1] = color.RGBA{R: 50, A: 255}
				extPal[2] = color.RGBA{G: 50, A: 255}

				// 2×2 flat frame: pixels in row-major order.
				flatPixels := []byte{1, 2, 2, 1}
				frame := buildFlatFrame(2, 2, flatPixels)
				rec := buildSpriteRecord(2, 2, 1, lbx.FlagNoCompression, nil, [][]byte{frame})

				images, err := lbx.DecodeFrames(rec, extPal)
				Expect(err).NotTo(HaveOccurred())
				Expect(images).To(HaveLen(1))
				img := images[0]
				Expect(img.At(0, 0)).To(Equal(color.RGBA{R: 50, A: 255}))
				Expect(img.At(1, 0)).To(Equal(color.RGBA{G: 50, A: 255}))
				Expect(img.At(1, 1)).To(Equal(color.RGBA{R: 50, A: 255}))
			})
		})
	})
})
