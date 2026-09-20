package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/game/galaxy"
	"github.com/domdom82/moo2hd/internal/lbx"
)

// nebulaSizeIdx maps a NebulaSize to its 0-based offset within a 4-record
// art group in STARBG.LBX (Large=0, Medium=1, Small=2, Huge=3).
var nebulaSizeIdx = map[galaxy.NebulaSize]int{
	galaxy.NebulaSizeLarge:  0,
	galaxy.NebulaSizeMedium: 1,
	galaxy.NebulaSizeSmall:  2,
	galaxy.NebulaSizeHuge:   3,
}

// NebulaManager holds SDL textures for all 48 nebula art records in
// STARBG.LBX (records 6–53: 12 art sets × 4 size variants).
type NebulaManager struct {
	// textures[artIndex][sizeOffset] — nil entries are silently skipped at draw time.
	textures [galaxy.NebulaArtCount][4]*sdl.Texture
}

// NewNebulaManager loads all 48 nebula textures from STARBG.LBX.
// Missing or undecodable records are skipped silently so a partial asset set
// doesn't crash the game. Returns a non-nil *NebulaManager even on failure.
func NewNebulaManager(renderer *sdl.Renderer, assetsDir string, reg *config.Registry) (*NebulaManager, error) {
	nm := &NebulaManager{}

	lbxPath := filepath.Join(assetsDir, "STARBG.LBX")
	data, err := os.ReadFile(lbxPath)
	if err != nil {
		data, err = os.ReadFile(filepath.Join(assetsDir, "starbg.lbx"))
		if err != nil {
			return nm, fmt.Errorf("nebula: opening STARBG.LBX: %w", err)
		}
	}

	arc, err := lbx.Parse(data)
	if err != nil {
		return nm, fmt.Errorf("nebula: parsing STARBG.LBX: %w", err)
	}

	for artIndex := range galaxy.NebulaArtCount {
		for sizeOffset := range 4 {
			record := 6 + artIndex*4 + sizeOffset
			if record >= len(arc.Records) {
				continue
			}

			var refs []config.LBXPaletteRef
			if reg != nil {
				refs = reg.LBXPaletteFor("STARBG.LBX", record)
			}

			palette, err := lbx.BuildPalette(assetsDir, refs)
			if err != nil {
				continue
			}

			frames, err := lbx.DecodeFrames(arc.Records[record].Data, palette)
			if err != nil || len(frames) == 0 {
				continue
			}

			tex, err := imageToTexture(renderer, frames[0])
			if err != nil {
				continue
			}
			nm.textures[artIndex][sizeOffset] = tex
		}
	}

	return nm, nil
}

// TextureFor returns the SDL texture for the given nebula, or nil if the
// texture was not loaded.
func (nm *NebulaManager) TextureFor(n *galaxy.Nebula) *sdl.Texture {
	if nm == nil {
		return nil
	}
	if n.ArtIndex < 0 || n.ArtIndex >= galaxy.NebulaArtCount {
		return nil
	}
	so, ok := nebulaSizeIdx[n.Size]
	if !ok {
		return nil
	}
	return nm.textures[n.ArtIndex][so]
}

// Close releases all held SDL textures.
func (nm *NebulaManager) Close() {
	if nm == nil {
		return
	}
	for i := range nm.textures {
		for j, tex := range nm.textures[i] {
			if tex != nil {
				tex.Destroy()
				nm.textures[i][j] = nil
			}
		}
	}
}
