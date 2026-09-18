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
// Palette resolution:
//   - The base palette comes from --palette if given, otherwise from BUFFER0.LBX in --assets.
//   - If a config entry in lbx_palettes.yaml covers this sprite, its junction palette
//     is always overlaid on top (regardless of --palette).
//   - Source LBX and record index are inferred from the filename stem (SHIPS_000 →
//     SHIPS.LBX record 0) or supplied explicitly with --lbx and --record.
//
// Usage:
//
//	svgconvert [--assets <dir>] [--configs <dir>] [--palette <file>] [--lbx <NAME.LBX>] [--out-dir <dir>] <sprite.lbx> [...]
//
// Flags:
//
//	--assets    Directory containing LBX files for palette loading (default: assets/)
//	--configs   Directory containing YAML config files (default: configs/)
//	--palette   Base palette: FONTS.LBX, extracted palette record, or raw 768/1024-byte VGA file.
//	            Junction palettes from lbx_palettes.yaml are still applied on top.
//	--lbx       Source LBX filename (e.g. SHIPS.LBX) for config lookup when the input
//	            filenames do not follow the NAME_NNN.lbx convention.
//	--out-dir   Directory to write SVG files (default: current directory)
package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/lbx"
	"github.com/domdom82/moo2hd/internal/svg"
)

// stemPattern matches lbxextract output filenames: <NAME>_<NNN>.<ext>
// Capture group 1 = uppercase LBX base name, group 2 = zero-padded record index.
var stemPattern = regexp.MustCompile(`^([A-Z0-9_]+)_(\d+)$`)
func main() {
	assetsDir := flag.String("assets", "assets/", "directory containing LBX files for palette loading")
	configsDir := flag.String("configs", "configs/", "directory containing YAML config files (lbx_palettes.yaml etc.)")
	palettePath := flag.String("palette", "", "base palette: FONTS.LBX, extracted palette record, or raw 768/1024-byte VGA file")
	sourceLBX := flag.String("lbx", "", "source LBX filename (e.g. SHIPS.LBX) for config lookup when filenames lack NAME_NNN prefix")
	outDir := flag.String("out-dir", ".", "directory to write SVG files")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: svgconvert [--assets <dir>] [--configs <dir>] [--palette <file>] [--lbx <NAME.LBX>] [--out-dir <dir>] <sprite.lbx> [...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	// Load config for per-sprite palette deps. Missing configs dir is not fatal.
	var reg *config.Registry
	if r, err := config.Load(*configsDir, "mods/"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config from %s: %v\n", *configsDir, err)
	} else {
		reg = r
	}

	// Base palette: --palette if given, otherwise BUFFER0.LBX.
	var basePalette color.Palette
	if *palettePath != "" {
		var err error
		basePalette, err = loadPalette(*palettePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: loading palette %s: %v\n", *palettePath, err)
			os.Exit(1)
		}
		fmt.Printf("loaded base palette from %s\n", *palettePath)
	} else {
		var err error
		basePalette, err = loadDefaultPalette(*assetsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load palette from %s: %v\n", *assetsDir, err)
		} else if basePalette != nil {
			fmt.Printf("loaded default palette from %s\n", filepath.Join(*assetsDir, "BUFFER0.LBX"))
		}
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: creating output directory %s: %v\n", *outDir, err)
		os.Exit(1)
	}

	exitCode := 0
	upscaler := svg.XBRZUpscaler{}
	for _, path := range flag.Args() {
		pal := paletteForFile(path, *sourceLBX, reg, *assetsDir, basePalette)
		if err := processSprite(path, *outDir, pal, upscaler); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// paletteForFile resolves the final palette for a sprite file by starting with
// basePalette and overlaying any junction palette from the config on top.
//
// The source LBX and record index are determined from (in order):
//  1. sourceLBX flag + numeric stem (e.g. "030" → record 30)
//  2. filename stem matching NAME_NNN convention (e.g. "SHIPS_000" → SHIPS.LBX record 0)
func paletteForFile(path, sourceLBX string, reg *config.Registry, assetsDir string, basePalette color.Palette) color.Palette {
	if reg == nil {
		return basePalette
	}

	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	upper := strings.ToUpper(stem)

	var lbxName string
	var recordIdx int
	found := false

	// Case 1: --lbx given; stem is a bare index (e.g. "030").
	if sourceLBX != "" {
		if n, err := strconv.Atoi(upper); err == nil {
			lbxName = strings.ToUpper(sourceLBX)
			recordIdx = n
			found = true
		}
	}

	// Case 2: stem matches NAME_NNN (e.g. "SHIPS_000").
	if !found {
		if m := stemPattern.FindStringSubmatch(upper); m != nil {
			lbxName = m[1] + ".LBX"
			recordIdx, _ = strconv.Atoi(m[2])
			found = true
		}
	}

	if !found {
		return basePalette
	}

	refs := reg.LBXPaletteFor(lbxName, recordIdx)
	if refs == nil {
		return basePalette
	}

	pal, err := lbx.BuildPalette(assetsDir, refs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: building palette for %s: %v; using base palette\n", filepath.Base(path), err)
		return basePalette
	}
	fmt.Printf("  palette: %s record %d → merged from config\n", lbxName, recordIdx)
	return pal
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
//   - MOO2 sprite record carrying FlagInternalPalette
//   - LBX archive (e.g. FONTS.LBX): palette extracted from record[1]
//   - FONTS.LBX palette record (≥1024 bytes): first 256 entries as {flag, R6, G6, B6}
func loadPalette(path string) (color.Palette, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if pal, err := lbx.ExtractInternalPalette(raw); err == nil {
		return pal, nil
	}

	if pal, err := extractPaletteFromLBX(raw); err == nil {
		return pal, nil
	}

	switch len(raw) {
	case 256 * 3:
		return decodeRawPalette(raw, 3), nil
	case 256 * 4:
		return decodeRawPalette(raw, 4), nil
	default:
		if len(raw) >= 256*4 {
			return lbx.DecodeFontsRecord(raw), nil
		}
		return nil, fmt.Errorf("unrecognised palette file: %d bytes (want 768 or 1024 for raw VGA, a MOO2 sprite record with an embedded palette, or a FONTS.LBX palette record)", len(raw))
	}
}

// extractPaletteFromLBX parses data as an LBX archive and extracts a palette from
// record[1] (first palette record in FONTS.LBX; record[0] is a font).
func extractPaletteFromLBX(data []byte) (color.Palette, error) {
	arc, err := lbx.Parse(data)
	if err != nil || len(arc.Records) < 2 {
		return nil, fmt.Errorf("not an LBX archive with at least 2 records")
	}
	rec := arc.Records[1].Data
	if len(rec) < 256*4 {
		return nil, fmt.Errorf("LBX record[1] too small for a palette (%d bytes)", len(rec))
	}
	return lbx.DecodeFontsRecord(rec), nil
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
