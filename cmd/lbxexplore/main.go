// Command lbxexplore is an interactive terminal explorer for LBX archive files.
//
// Usage:
//
//	lbxexplore [--assets <dir>] <file.lbx>
//
// Flags:
//
//	--assets   Directory containing BUFFER0.LBX for the external palette (default: assets/)
package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/domdom82/moo2hd/internal/lbx"
)

func main() {
	assetsDir := "assets/"
	args := os.Args[1:]

	// Simple manual flag parsing so we don't pull in flag package awkwardly alongside positional.
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--assets", "-assets":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --assets requires a value")
				os.Exit(1)
			}
			assetsDir = args[i+1]
			args = append(args[:i], args[i+2:]...)
			i--
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: lbxexplore [--assets <dir>] <file.lbx>")
			os.Exit(0)
		}
	}

	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: lbxexplore [--assets <dir>] <file.lbx>")
		os.Exit(1)
	}

	path := args[0]
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	arc, err := lbx.Parse(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parsing %s: %v\n", path, err)
		os.Exit(1)
	}

	// Try to load the default external palette from BUFFER0.LBX.
	palette, _ := loadDefaultPalette(assetsDir)

	model := NewAppModel(filepath.Base(path), arc, palette)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// loadDefaultPalette builds a full 256-entry palette from BUFFER0.LBX by merging:
//   - record 0: base palette (indices 0–191)
//   - record 92: first faction palette (indices 192–213, FlagJunction)
//
// Returns nil without error if the file is absent — callers degrade gracefully.
func loadDefaultPalette(assetsDir string) (color.Palette, error) {
	buf0 := filepath.Join(assetsDir, "BUFFER0.LBX")
	data, err := os.ReadFile(buf0)
	if err != nil {
		return nil, nil
	}
	arc, err := lbx.Parse(data)
	if err != nil || len(arc.Records) < 93 {
		return nil, err
	}

	base, err := lbx.ExtractInternalPalette(arc.Records[0].Data)
	if err != nil {
		return nil, err
	}

	// Overlay faction 0 palette (record 92) at PaletteJunctionOffset.
	faction, err := lbx.ExtractInternalPalette(arc.Records[92].Data)
	if err != nil {
		return base, nil // faction palette optional
	}
	for i := lbx.PaletteJunctionOffset; i < 256; i++ {
		if faction[i] != nil {
			base[i] = faction[i]
		}
	}
	return base, nil
}
