// Command lbxexplore is an interactive terminal explorer for LBX archive files.
//
// Usage:
//
//	lbxexplore [--assets <dir>] [--configs <dir>] <file.lbx>
//
// Flags:
//
//	--assets   Directory containing LBX files for palette loading (default: assets/)
//	--configs  Directory containing YAML config files (default: configs/)
package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/lbx"
)

func main() {
	assetsDir := "assets/"
	configsDir := "configs/"
	args := os.Args[1:]

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
		case "--configs", "-configs":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --configs requires a value")
				os.Exit(1)
			}
			configsDir = args[i+1]
			args = append(args[:i], args[i+2:]...)
			i--
		case "--help", "-h":
			fmt.Fprintln(os.Stderr, "Usage: lbxexplore [--assets <dir>] [--configs <dir>] <file.lbx>")
			os.Exit(0)
		}
	}

	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: lbxexplore [--assets <dir>] [--configs <dir>] <file.lbx>")
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

	// Load config for per-record palette deps. Missing configs dir is not fatal.
	var reg *config.Registry
	if r, err := config.Load(configsDir, "mods/"); err == nil {
		reg = r
	}

	// Load fallback palette from BUFFER0.LBX.
	fallback, _ := loadDefaultPalette(assetsDir)

	lbxName := strings.ToUpper(filepath.Base(path))
	model := NewAppModel(filepath.Base(path), arc, lbxName, assetsDir, reg, fallback)
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
