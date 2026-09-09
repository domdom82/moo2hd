package lbx

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

const spriteHeaderLen = 12

// Sprite flag bits embedded in SpriteHeader.Flags.
const (
	FlagNoCompression   uint16 = 0x0100 // frame pixels are a flat width×height array, no sequence encoding
	FlagFillBackground  uint16 = 0x0400
	FlagFunctionalColor uint16 = 0x0800
	FlagInternalPalette uint16 = 0x1000 // embedded palette follows the frame-offset table
	FlagJunction        uint16 = 0x2000
)

// SpriteHeader is the decoded fixed portion of a MOO2 sprite record.
type SpriteHeader struct {
	Width      uint16
	Height     uint16
	FrameCount uint16
	FrameDelay uint16
	Flags      uint16
}

// ParseSpriteHeader reads the 12-byte fixed header from a raw sprite record payload.
func ParseSpriteHeader(data []byte) (*SpriteHeader, error) {
	if len(data) < spriteHeaderLen {
		return nil, fmt.Errorf("lbx: sprite data too short: %d bytes (need at least %d)", len(data), spriteHeaderLen)
	}
	h := &SpriteHeader{
		Width:      binary.LittleEndian.Uint16(data[0:2]),
		Height:     binary.LittleEndian.Uint16(data[2:4]),
		// data[4:6] reserved / ignored
		FrameCount: binary.LittleEndian.Uint16(data[6:8]),
		FrameDelay: binary.LittleEndian.Uint16(data[8:10]),
		Flags:      binary.LittleEndian.Uint16(data[10:12]),
	}
	if h.Width == 0 || h.Height == 0 {
		return nil, fmt.Errorf("lbx: invalid sprite dimensions %dx%d", h.Width, h.Height)
	}
	if h.FrameCount == 0 {
		return nil, fmt.Errorf("lbx: sprite has zero frames")
	}
	return h, nil
}

// DecodeFrames decodes all animation frames from a raw MOO2 sprite record payload.
// extPalette must be supplied when the record does not carry an internal palette
// (FlagInternalPalette not set). Index 0 is always transparent.
func DecodeFrames(data []byte, extPalette color.Palette) ([]image.Image, error) {
	hdr, err := ParseSpriteHeader(data)
	if err != nil {
		return nil, err
	}
	n := int(hdr.FrameCount)

	// Frame-offset table: N+1 entries (N starts + 1 EOF marker).
	offsetsEnd := spriteHeaderLen + (n+1)*4
	if len(data) < offsetsEnd {
		return nil, fmt.Errorf("lbx: data too short for frame-offset table (need %d, have %d)", offsetsEnd, len(data))
	}
	frameOffsets := make([]int, n+1)
	for i := range n + 1 {
		frameOffsets[i] = int(binary.LittleEndian.Uint32(data[offsetsEnd-4*(n+1-i) : offsetsEnd-4*(n-i)]))
	}
	// Re-read in order: table starts at spriteHeaderLen.
	for i := range n + 1 {
		pos := spriteHeaderLen + i*4
		frameOffsets[i] = int(binary.LittleEndian.Uint32(data[pos : pos+4]))
	}

	// Resolve palette.
	var pal color.Palette
	if hdr.Flags&FlagInternalPalette != 0 {
		pal, err = readInternalPalette(data, offsetsEnd)
		if err != nil {
			return nil, err
		}
	} else {
		if extPalette == nil {
			return nil, fmt.Errorf("lbx: sprite requires an external palette but none was supplied")
		}
		pal = extPalette
	}

	images := make([]image.Image, n)
	for i := range n {
		img, err := decodeFrame(data, frameOffsets[i], hdr, pal)
		if err != nil {
			return nil, fmt.Errorf("lbx: frame %d: %w", i, err)
		}
		images[i] = img
	}
	return images, nil
}

// readInternalPalette reads the partial embedded palette that follows the frame-offset table.
// It returns a full 256-entry palette; entries outside the stored range are zero/transparent.
func readInternalPalette(data []byte, afterOffsets int) (color.Palette, error) {
	if len(data) < afterOffsets+4 {
		return nil, fmt.Errorf("lbx: data too short for internal palette header")
	}
	baseIdx := int(binary.LittleEndian.Uint16(data[afterOffsets : afterOffsets+2]))
	count := int(binary.LittleEndian.Uint16(data[afterOffsets+2 : afterOffsets+4]))
	rawStart := afterOffsets + 4
	rawEnd := rawStart + count*4
	if len(data) < rawEnd {
		return nil, fmt.Errorf("lbx: data too short for internal palette entries (need %d, have %d)", rawEnd, len(data))
	}
	pal := make(color.Palette, 256)
	pal[0] = color.Transparent
	for i := range count {
		if baseIdx+i >= 256 {
			break
		}
		// Layout: alpha (skip), red6, green6, blue6 — VGA 6-bit, multiply by 4 for 8-bit.
		r := data[rawStart+i*4+1] * 4
		g := data[rawStart+i*4+2] * 4
		b := data[rawStart+i*4+3] * 4
		pal[baseIdx+i] = color.RGBA{R: r, G: g, B: b, A: 255}
	}
	return pal, nil
}

// decodeFrame decodes a single frame starting at data[frameStart].
func decodeFrame(data []byte, frameStart int, hdr *SpriteHeader, pal color.Palette) (image.Image, error) {
	w, h := int(hdr.Width), int(hdr.Height)

	if hdr.Flags&FlagNoCompression != 0 {
		return decodeFrameFlat(data, frameStart, w, h, pal)
	}
	return decodeFrameSparse(data, frameStart, w, h, pal)
}

// decodeFrameFlat handles FlagNoCompression: raw width×height palette-index bytes.
func decodeFrameFlat(data []byte, frameStart, w, h int, pal color.Palette) (image.Image, error) {
	need := frameStart + w*h
	if len(data) < need {
		return nil, fmt.Errorf("lbx: flat frame data truncated (need %d, have %d)", need, len(data))
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	pixels := data[frameStart : frameStart+w*h]
	for i, idx := range pixels {
		if idx == 0 {
			continue // transparent — leave zero RGBA
		}
		x := i % w
		y := i / w
		img.SetRGBA(x, y, palRGBA(pal, idx))
	}
	return img, nil
}

// decodeFrameSparse handles the standard MOO2 sparse horizontal-sequence encoding.
func decodeFrameSparse(data []byte, frameStart, w, h int, pal color.Palette) (image.Image, error) {
	if len(data) < frameStart+4 {
		return nil, fmt.Errorf("lbx: frame data too short")
	}
	indicator := binary.LittleEndian.Uint16(data[frameStart : frameStart+2])
	if indicator != 1 {
		return nil, fmt.Errorf("lbx: unexpected frame indicator %d (want 1)", indicator)
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	yOff := int(binary.LittleEndian.Uint16(data[frameStart+2 : frameStart+4]))
	xOff := 0
	pos := frameStart + 4

	for {
		if len(data) < pos+4 {
			return nil, fmt.Errorf("lbx: sequence record truncated at offset %d", pos)
		}
		seqLen := int(binary.LittleEndian.Uint16(data[pos : pos+2]))
		relX := int(binary.LittleEndian.Uint16(data[pos+2 : pos+4]))
		pos += 4

		if seqLen == 0 {
			if relX == 1000 {
				break // end-of-frame marker
			}
			yOff += relX
			xOff = 0
			continue
		}

		xOff += relX
		if len(data) < pos+seqLen {
			return nil, fmt.Errorf("lbx: pixel sequence truncated at offset %d", pos)
		}
		for j := range seqLen {
			idx := data[pos+j]
			if idx != 0 {
				px := xOff + j
				py := yOff
				if px >= 0 && px < w && py >= 0 && py < h {
					img.SetRGBA(px, py, palRGBA(pal, idx))
				}
			}
		}
		pos += seqLen
		if seqLen%2 != 0 {
			pos++ // padding byte to word-align
		}
		xOff += seqLen
	}
	return img, nil
}

// palRGBA returns the RGBA value for a palette index, falling back to transparent for
// out-of-range indices. It avoids the RGBA() round-trip by type-asserting color.RGBA directly.
func palRGBA(pal color.Palette, idx uint8) color.RGBA {
	if int(idx) >= len(pal) || pal[idx] == nil {
		return color.RGBA{}
	}
	c := pal[idx]
	if r, ok := c.(color.RGBA); ok {
		return r
	}
	rv, gv, bv, av := c.RGBA()
	return color.RGBA{R: uint8(rv >> 8), G: uint8(gv >> 8), B: uint8(bv >> 8), A: uint8(av >> 8)}
}
