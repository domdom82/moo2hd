package lbx

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"github.com/domdom82/moo2hd/internal/config"
)

// BuildPalette builds a merged 256-entry palette by loading and overlaying the
// palette records described by refs, resolving each LBX file via assetsDir.
// Refs are applied in order: each record's encoded baseIdx and count determine
// which palette indices it fills (ExtractInternalPalette handles both base records
// at index 0 and junction records at index 192). Index 0 is always transparent.
func BuildPalette(assetsDir string, refs []config.LBXPaletteRef) (color.Palette, error) {
	merged := make(color.Palette, 256)
	merged[0] = color.Transparent

	for _, ref := range refs {
		pal, err := loadPaletteRecord(assetsDir, ref.LBX, ref.Record)
		if err != nil {
			return nil, fmt.Errorf("lbx: palette %s[%d]: %w", ref.LBX, ref.Record, err)
		}
		for i := 1; i < 256; i++ {
			if pal[i] != nil {
				merged[i] = pal[i]
			}
		}
	}
	return merged, nil
}

// loadPaletteRecord reads a single palette record from an LBX archive and returns
// a 256-entry palette. It handles both:
//   - Standard sprite records with FlagInternalPalette (ExtractInternalPalette)
//   - FONTS.LBX shade tables: N×256 entries of {flag, R6, G6, B6} with stride 4
func loadPaletteRecord(assetsDir, lbxFile string, record int) (color.Palette, error) {
	path := filepath.Join(assetsDir, lbxFile)
	// Try case-insensitive match on case-sensitive filesystems.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		path = filepath.Join(assetsDir, strings.ToLower(lbxFile))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	arc, err := Parse(data)
	if err != nil {
		return nil, err
	}
	if record < 0 || record >= len(arc.Records) {
		return nil, fmt.Errorf("record %d out of range (archive has %d records)", record, len(arc.Records))
	}
	rec := arc.Records[record].Data

	// Try as a sprite record with embedded palette first.
	if pal, err := ExtractInternalPalette(rec); err == nil {
		return pal, nil
	}

	// Fall back to FONTS.LBX shade-table format: N×256 entries of {flag, R6, G6, B6}.
	// Only the first 256 entries (bytes 0–1023) are the actual palette.
	if len(rec) >= 256*4 {
		return DecodeFontsRecord(rec), nil
	}

	return nil, fmt.Errorf("record %d (%d bytes) is not a recognised palette format", record, len(rec))
}

// DecodeFontsRecord decodes the first 256 entries of a FONTS.LBX shade/remap table.
// The table has stride-4 entries: {flag, R6, G6, B6}. Values are VGA 6-bit (0–63),
// multiplied by 4 to produce 8-bit. Index 0 is transparent.
func DecodeFontsRecord(raw []byte) color.Palette {
	pal := make(color.Palette, 256)
	pal[0] = color.Transparent
	for i := 1; i < 256; i++ {
		pal[i] = color.RGBA{
			R: raw[i*4+1] * 4,
			G: raw[i*4+2] * 4,
			B: raw[i*4+3] * 4,
			A: 255,
		}
	}
	return pal
}
