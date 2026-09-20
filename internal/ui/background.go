package ui

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/lbx"
)

const (
	starbgLBX   = "STARBG.LBX"
	starbgCount = 6
)

// BackgroundManager loads the star background images from STARBG.LBX at
// startup, uploads them as SDL textures, and draws the one selected by the
// galaxy seed scaled to fill the current viewport on every frame.
type BackgroundManager struct {
	textures [starbgCount]*sdl.Texture
	active   int // index selected for this galaxy
}

// NewBackgroundManager decodes STARBG.LBX records 0–5 using palette deps from
// reg, uploads each to an SDL texture, and selects one based on galaxySeed.
// assetsDir is the directory containing LBX files (e.g. "assets/").
// Returns a non-nil *BackgroundManager even on partial failure; missing
// textures are left nil and the selection falls back to the first available.
func NewBackgroundManager(renderer *sdl.Renderer, assetsDir string, reg *config.Registry, galaxySeed uint64) (*BackgroundManager, error) {
	bm := &BackgroundManager{
		active: int(galaxySeed % starbgCount),
	}

	lbxPath := filepath.Join(assetsDir, starbgLBX)
	data, err := os.ReadFile(lbxPath)
	if err != nil {
		// Try lowercase fallback.
		data, err = os.ReadFile(filepath.Join(assetsDir, "starbg.lbx"))
		if err != nil {
			return bm, fmt.Errorf("background: opening %s: %w", lbxPath, err)
		}
	}

	arc, err := lbx.Parse(data)
	if err != nil {
		return bm, fmt.Errorf("background: parsing %s: %w", starbgLBX, err)
	}

	for i := range starbgCount {
		if i >= len(arc.Records) {
			break
		}

		var refs []config.LBXPaletteRef
		if reg != nil {
			refs = reg.LBXPaletteFor(starbgLBX, i)
		}

		palette, palErr := lbx.BuildPalette(assetsDir, refs)
		if palErr != nil {
			continue
		}

		frames, decErr := lbx.DecodeFrames(arc.Records[i].Data, palette)
		if decErr != nil || len(frames) == 0 {
			continue
		}

		tex, texErr := imageToTexture(renderer, frames[0])
		if texErr != nil {
			continue
		}
		bm.textures[i] = tex
	}

	// Ensure the selected index actually has a texture; scan forward if not.
	for range starbgCount {
		if bm.textures[bm.active] != nil {
			break
		}
		bm.active = (bm.active + 1) % starbgCount
	}

	return bm, nil
}

// Draw renders the active background texture stretched to fill the full
// viewport. It is a no-op if no textures were loaded.
func (bm *BackgroundManager) Draw(r *sdl.Renderer) {
	if bm == nil {
		return
	}
	tex := bm.textures[bm.active]
	if tex == nil {
		return
	}
	w, h, err := r.CurrentOutputSize()
	if err != nil {
		return
	}
	dst := sdl.FRect{X: 0, Y: 0, W: float32(w), H: float32(h)}
	//_ = tex.SetColorModFloat(1.8, 1.8, 1.8) // increase brightness a bit (currently disabled; looks too washed out)
	_ = r.RenderTexture(tex, nil, &dst)
}

// Close releases all SDL textures held by the manager.
func (bm *BackgroundManager) Close() {
	if bm == nil {
		return
	}
	for i, tex := range bm.textures {
		if tex != nil {
			tex.Destroy()
			bm.textures[i] = nil
		}
	}
}

// imageToTexture converts an image.Image to an SDL texture owned by renderer.
func imageToTexture(renderer *sdl.Renderer, img image.Image) (*sdl.Texture, error) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Ensure we have a contiguous RGBA pixel buffer.
	rgba, ok := img.(*image.RGBA)
	if !ok {
		rgba = image.NewRGBA(bounds)
		draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	}

	// SDL3: PIXELFORMAT_RGBA32 maps to memory-order R,G,B,A on little-endian,
	// which matches image.RGBA's Pix layout exactly.
	surface, err := sdl.CreateSurfaceFrom(w, h, sdl.PIXELFORMAT_RGBA32, rgba.Pix, rgba.Stride)
	if err != nil {
		return nil, fmt.Errorf("background: creating surface: %w", err)
	}
	defer surface.Destroy()

	tex, err := renderer.CreateTextureFromSurface(surface)
	if err != nil {
		return nil, fmt.Errorf("background: creating texture: %w", err)
	}
	return tex, nil
}
