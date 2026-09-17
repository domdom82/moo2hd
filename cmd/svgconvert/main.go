// Command svgconvert decodes MOO2 sprite records and writes upscaled SVG files.
//
// It is designed to chain with lbxextract:
//
//	lbxextract --extract assets/SHIPS.LBX
//	svgconvert --assets assets/ --out-dir out/ SHIPS_*.lbx
//
// Each input file is treated as a raw MOO2 sprite record (a payload extracted by
// lbxextract). For each animation frame a separate SVG is written:
//
//	<stem>_frame<N>.svg
//
// If the sprite record uses an external palette (no embedded palette), the full
// MOO2 palette is loaded automatically from BUFFER0.LBX in --assets. Alternatively,
// supply a single extracted palette record with --palette.
//
// Usage:
//
//	svgconvert [--assets <dir>] [--palette <file>] [--out-dir <dir>] <sprite.lbx> [...]
//
// Flags:
//
//	--assets    Directory containing BUFFER0.LBX for the full merged palette (default: assets/)
//	--palette   Path to an extracted palette record file (overrides --assets)
//	--out-dir   Directory to write SVG files (default: current directory)
package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"github.com/domdom82/moo2hd/internal/lbx"
	"github.com/domdom82/moo2hd/internal/svg"
)

func main() {
	assetsDir := flag.String("assets", "assets/", "directory containing BUFFER0.LBX for the full merged palette")
	palettePath := flag.String("palette", "", "path to extracted palette record file (overrides --assets)")
	outDir := flag.String("out-dir", ".", "directory to write SVG files")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: svgconvert [--assets <dir>] [--palette <file>] [--out-dir <dir>] <sprite.lbx> [...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	var extPalette color.Palette
	if *palettePath != "" {
		var err error
		extPalette, err = loadPalette(*palettePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: loading palette %s: %v\n", *palettePath, err)
			os.Exit(1)
		}
		fmt.Printf("loaded external palette from %s\n", *palettePath)
	} else {
		var err error
		extPalette, err = loadDefaultPalette(*assetsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load palette from %s: %v\n", *assetsDir, err)
		} else if extPalette != nil {
			fmt.Printf("loaded merged palette from %s\n", filepath.Join(*assetsDir, "BUFFER0.LBX"))
		}
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: creating output directory %s: %v\n", *outDir, err)
		os.Exit(1)
	}

	exitCode := 0
	upscaler := svg.XBRZUpscaler{}
	for _, path := range flag.Args() {
		if err := processSprite(path, *outDir, extPalette, upscaler); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func processSprite(path, outDir string, extPalette color.Palette, upscaler svg.XBRZUpscaler) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	hdr, err := lbx.ParseSpriteHeader(data)
	if err != nil {
		return fmt.Errorf("not a sprite record: %w", err)
	}

	frames, err := lbx.DecodeFrames(data, extPalette)
	if err != nil {
		return fmt.Errorf("decoding frames: %w", err)
	}

	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	fmt.Printf("%s: %dx%d, %d frame(s)\n", filepath.Base(path), hdr.Width, hdr.Height, hdr.FrameCount)

	for i, frame := range frames {
		doc, err := upscaler.Upscale(frame, nil)
		if err != nil {
			return fmt.Errorf("upscaling frame %d: %w", i, err)
		}

		outPath := filepath.Join(outDir, fmt.Sprintf("%s_frame%03d.svg", stem, i))
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("creating %s: %w", outPath, err)
		}
		writeErr := doc.Write(f)
		closeErr := f.Close()
		if writeErr != nil {
			return fmt.Errorf("writing %s: %w", outPath, writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("closing %s: %w", outPath, closeErr)
		}
		fmt.Printf("  wrote %s (%dx%d scaled to %dx%d)\n",
			outPath, hdr.Width, hdr.Height, hdr.Width*2, hdr.Height*2)
	}
	return nil
}

// loadDefaultPalette builds a full 256-entry palette from BUFFER0.LBX by merging
// the base palette (record 0, indices 0–191) with the first faction palette
// (record 92, FlagJunction → indices 192–213).
func loadDefaultPalette(assetsDir string) (color.Palette, error) {
	buf0 := filepath.Join(assetsDir, "BUFFER0.LBX")
	data, err := os.ReadFile(buf0)
	if err != nil {
		return nil, nil // assets not present — not fatal
	}
	arc, err := lbx.Parse(data)
	if err != nil || len(arc.Records) < 93 {
		return nil, err
	}
	base, err := lbx.ExtractInternalPalette(arc.Records[0].Data)
	if err != nil {
		return nil, err
	}
	faction, err := lbx.ExtractInternalPalette(arc.Records[92].Data)
	if err != nil {
		return base, nil
	}
	for i := lbx.PaletteJunctionOffset; i < 256; i++ {
		if faction[i] != nil {
			base[i] = faction[i]
		}
	}
	return base, nil
}

// loadPalette reads a palette from a file. Supported formats:
//   - 768 bytes: 256 × {R6, G6, B6}  (raw 6-bit VGA)
//   - 1024 bytes: 256 × {A, R6, G6, B6}  (with leading alpha byte)
//   - MOO2 sprite record carrying FlagInternalPalette: palette is extracted from it
func loadPalette(path string) (color.Palette, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Try to parse as a MOO2 sprite record with an internal palette.
	if pal, err := extractPaletteFromSpriteRecord(raw); err == nil {
		return pal, nil
	}

	// Fall back to raw palette formats.
	switch len(raw) {
	case 256 * 3:
		return decodeRawPalette(raw, 3), nil
	case 256 * 4:
		return decodeRawPalette(raw, 4), nil
	default:
		return nil, fmt.Errorf("unrecognised palette file: %d bytes (want 768 or 1024 for raw VGA, or a MOO2 sprite record with an embedded palette)", len(raw))
	}
}

// extractPaletteFromSpriteRecord parses a full MOO2 sprite record and returns its
// embedded palette when FlagInternalPalette is set. Returns an error if the record
// doesn't carry a palette (so the caller can fall back to other formats).
func extractPaletteFromSpriteRecord(data []byte) (color.Palette, error) {
	return lbx.ExtractInternalPalette(data)
}

// decodeRawPalette decodes a flat VGA palette where stride is 3 (R,G,B) or 4 (A,R,G,B).
func decodeRawPalette(raw []byte, stride int) color.Palette {
	pal := make(color.Palette, 256)
	pal[0] = color.Transparent
	for i := 1; i < 256; i++ {
		offset := i * stride
		rIdx, gIdx, bIdx := 0, 1, 2
		if stride == 4 {
			rIdx, gIdx, bIdx = 1, 2, 3
		}
		pal[i] = color.RGBA{
			R: raw[offset+rIdx] * 4,
			G: raw[offset+gIdx] * 4,
			B: raw[offset+bIdx] * 4,
			A: 255,
		}
	}
	return pal
}
