package main

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"

	"github.com/domdom82/moo2hd/internal/lbx"
)

const textPrintableThreshold = 0.85
const textSampleSize = 512

// renderPreview returns a string representation of r's contents for display in the preview pane.
// previewCache is keyed by record index to avoid re-decoding on every render tick.
func renderPreview(r *lbx.Record, palette color.Palette, cols, rows int, cache map[int]string) string {
	if cached, ok := cache[r.Index]; ok {
		return cached
	}
	result := computePreview(r, palette, cols, rows)
	cache[r.Index] = result
	return result
}

func computePreview(r *lbx.Record, palette color.Palette, cols, rows int) string {
	switch r.Type {
	case lbx.RecordVOC, lbx.RecordWAV:
		return fmt.Sprintf("[Audio — %d bytes]\n\nPress space to play.", len(r.Data))
	case lbx.RecordXMI:
		return fmt.Sprintf("[MIDI — %d bytes]\n\nPress space to play.\n(requires timidity or fluidsynth)", len(r.Data))
	case lbx.RecordSMK:
		return fmt.Sprintf("[Smacker video — %d bytes]\n\nPlayback not supported.", len(r.Data))
	case lbx.RecordLBX:
		return renderLBX(r, palette, cols, rows)
	default:
		if isText(r.Data) {
			return renderText(r.Data)
		}
		bpl := 16
		if cols < 60 {
			bpl = 8
		}
		return hexDump(r.Data, bpl)
	}
}

func renderLBX(r *lbx.Record, palette color.Palette, cols, rows int) string {
	// Check for text content before attempting sprite decode — text records have
	// type RecordLBX but their first bytes will satisfy ParseSpriteHeader with
	// garbage dimensions, causing a misleading decode error.
	if isText(r.Data) {
		return renderText(r.Data)
	}

	hdr, err := lbx.ParseSpriteHeader(r.Data)
	if err != nil {
		// Not a sprite — fall back to hex or text.
		if isText(r.Data) {
			return renderText(r.Data)
		}
		bpl := 16
		if cols < 60 {
			bpl = 8
		}
		return hexDump(r.Data, bpl)
	}

	frames, err := lbx.DecodeFrames(r.Data, palette)
	if err != nil {
		return fmt.Sprintf("[Sprite %dx%d, %d frames]\n\nDecode error: %v\n\nIf palette is missing, pass --assets <dir>.",
			hdr.Width, hdr.Height, hdr.FrameCount, err)
	}

	header := fmt.Sprintf("[Sprite %dx%d, %d frame(s)]\n\n", hdr.Width, hdr.Height, hdr.FrameCount)
	// Reserve rows used by the header text (2 lines + blank line).
	imgRows := rows - 3
	if imgRows < 1 {
		imgRows = 1
	}
	return header + imageToANSI(frames[0], cols, imgRows)
}

func renderText(data []byte) string {
	// Replace null bytes with newlines for display.
	s := string(data)
	s = strings.ReplaceAll(s, "\x00", "\n")
	return s
}

func isText(data []byte) bool {
	sample := data
	if len(sample) > textSampleSize {
		sample = sample[:textSampleSize]
	}
	if len(sample) == 0 {
		return false
	}
	printable := 0
	for _, b := range sample {
		r := rune(b)
		if unicode.IsPrint(r) || r == '\n' || r == '\r' || r == '\t' {
			printable++
		}
	}
	return float64(printable)/float64(len(sample)) >= textPrintableThreshold
}
