package ui

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/game/galaxy"
	"github.com/domdom82/moo2hd/internal/lbx"
)

// planetSpriteKey indexes the sprite atlas by planet class and size (1–5).
type planetSpriteKey struct {
	class string
	size  int // 1=tiny 2=small 3=medium 4=large 5=huge
}

// planetAnimState tracks the looping animation state for one planet.
type planetAnimState struct {
	frame        int
	elapsedMs    float32
	frameDelayMs float32
	frameCount   int
}

// PlanetSpriteManager loads planet sprite art from LBX archives and drives
// per-planet looping animations. One instance is shared across all draw calls.
type PlanetSpriteManager struct {
	sprites   map[planetSpriteKey]starSprite // re-uses the starSprite struct
	animState map[galaxy.PlanetID]*planetAnimState
}

// sizeRef selects the LBXSpriteRef for the given size (1–5) from a PlanetSpriteVariants.
func sizeRef(v config.PlanetSpriteVariants, size int) config.LBXSpriteRef {
	switch size {
	case 1:
		return v.Tiny
	case 2:
		return v.Small
	case 3:
		return v.Medium
	case 4:
		return v.Large
	default:
		return v.Huge
	}
}

// NewPlanetSpriteManager loads all configured planet sprites from assetsDir.
// Missing or undecodable records are skipped silently.
func NewPlanetSpriteManager(
	renderer *sdl.Renderer,
	assetsDir string,
	reg *config.Registry,
) *PlanetSpriteManager {
	pm := &PlanetSpriteManager{
		sprites:   make(map[planetSpriteKey]starSprite),
		animState: make(map[galaxy.PlanetID]*planetAnimState),
	}
	if reg == nil {
		return pm
	}

	planetClasses := []string{
		"terran", "ocean", "arid", "desert", "tundra", "swamp",
		"barren", "radiated", "toxic", "gaia",
		"none", // gas giant
	}

	// Cache open LBX archives to avoid re-reading the same file for every record.
	archives := make(map[string]*lbx.Archive)
	openArc := func(name string) *lbx.Archive {
		if arc, ok := archives[name]; ok {
			return arc
		}
		data, err := openLBX(assetsDir, name)
		if err != nil {
			return nil
		}
		arc, err := lbx.Parse(data)
		if err != nil {
			return nil
		}
		archives[name] = arc
		return arc
	}

	sizes := []int{1, 2, 3, 4, 5}
	for _, class := range planetClasses {
		entry := reg.PlanetSprite(class)
		if entry == nil {
			continue
		}
		iterSizes := sizes
		if class == "none" {
			iterSizes = []int{5} // gas giant: huge only
		}
		for _, sz := range iterSizes {
			ref := sizeRef(entry.Sizes, sz)
			if ref.LBX == "" {
				continue
			}
			arc := openArc(ref.LBX)
			if arc == nil || ref.Record < 0 || ref.Record >= len(arc.Records) {
				continue
			}
			palRefs := reg.LBXPaletteFor(ref.LBX, ref.Record)
			palette, err := lbx.BuildPalette(assetsDir, palRefs)
			if err != nil {
				continue
			}
			recordData := arc.Records[ref.Record].Data
			hdr, err := lbx.ParseSpriteHeader(recordData)
			if err != nil {
				continue
			}
			frames, err := lbx.DecodeFrames(recordData, palette)
			if err != nil || len(frames) == 0 {
				continue
			}
			textures := make([]*sdl.Texture, len(frames))
			ok := true
			for i, f := range frames {
				tex, err := imageToTexture(renderer, f)
				if err != nil {
					ok = false
					break
				}
				textures[i] = tex
			}
			if !ok {
				for _, t := range textures {
					if t != nil {
						t.Destroy()
					}
				}
				continue
			}
			pm.sprites[planetSpriteKey{class, sz}] = starSprite{
				textures:     textures,
				frameDelayMs: float32(max(hdr.FrameDelay, minDelayMs)),
				scale:        ref.EffectiveScale(),
			}
		}
	}

	return pm
}

// Update advances all active planet animations by dt seconds.
func (pm *PlanetSpriteManager) Update(dt float32) {
	if pm == nil {
		return
	}
	dtMs := dt * 1000.0
	for _, st := range pm.animState {
		st.elapsedMs += dtMs
		if st.elapsedMs >= st.frameDelayMs {
			st.elapsedMs -= st.frameDelayMs
			st.frame = (st.frame + 1) % st.frameCount
		}
	}
}

// DrawPlanet renders the planet sprite centered at (px, py) scaled to displaySize.
// size is the planet size value (1–5). Returns false if no sprite is configured,
// in which case the caller should fall back to a coloured circle.
func (pm *PlanetSpriteManager) DrawPlanet(
	r *sdl.Renderer,
	planet *galaxy.Planet,
	px, py, displaySize float32,
) bool {
	if pm == nil {
		return false
	}
	key := planetSpriteKey{planet.Class, planet.Size}
	sp, ok := pm.sprites[key]
	if !ok || len(sp.textures) == 0 {
		return false
	}

	st, active := pm.animState[planet.ID]
	if !active {
		st = &planetAnimState{
			frameDelayMs: sp.frameDelayMs,
			frameCount:   len(sp.textures),
		}
		pm.animState[planet.ID] = st
	}

	frame := st.frame
	if frame >= len(sp.textures) {
		frame = 0
	}
	tex := sp.textures[frame]
	if tex == nil {
		return false
	}

	half := displaySize * sp.scale / 2
	dst := sdl.FRect{
		X: px - half,
		Y: py - half,
		W: displaySize * sp.scale,
		H: displaySize * sp.scale,
	}
	r.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
	_ = r.RenderTexture(tex, nil, &dst)
	return true
}

// Close releases all SDL textures.
func (pm *PlanetSpriteManager) Close() {
	if pm == nil {
		return
	}
	for k, sp := range pm.sprites {
		for _, tex := range sp.textures {
			if tex != nil {
				tex.Destroy()
			}
		}
		delete(pm.sprites, k)
	}
}

// ColonyBgManager loads and renders the per-planet-class colony screen background
// images. Each class may have up to 3 variants; the variant index is fixed per
// planet at generation time and passed to DrawBackground.
type ColonyBgManager struct {
	textures map[string][]*sdl.Texture // keyed by planet class; indexed by variant
}

// NewColonyBgManager loads all configured colony background images from assetsDir.
// Missing or undecodable records are skipped silently.
func NewColonyBgManager(
	renderer *sdl.Renderer,
	assetsDir string,
	reg *config.Registry,
) *ColonyBgManager {
	cm := &ColonyBgManager{
		textures: make(map[string][]*sdl.Texture),
	}
	if reg == nil {
		return cm
	}

	archives := make(map[string]*lbx.Archive)
	openArc := func(name string) *lbx.Archive {
		if arc, ok := archives[name]; ok {
			return arc
		}
		data, err := openLBX(assetsDir, name)
		if err != nil {
			return nil
		}
		arc, err := lbx.Parse(data)
		if err != nil {
			return nil
		}
		archives[name] = arc
		return arc
	}

	planetClasses := []string{
		"terran", "ocean", "arid", "desert", "tundra", "swamp",
		"barren", "radiated", "toxic", "gaia", "none",
	}
	for _, class := range planetClasses {
		entry := reg.PlanetSprite(class)
		if entry == nil || len(entry.Background) == 0 {
			continue
		}
		variants := make([]*sdl.Texture, 0, len(entry.Background))
		for _, ref := range entry.Background {
			if ref.LBX == "" {
				variants = append(variants, nil)
				continue
			}
			arc := openArc(ref.LBX)
			if arc == nil || ref.Record < 0 || ref.Record >= len(arc.Records) {
				variants = append(variants, nil)
				continue
			}
			palRefs := reg.LBXPaletteFor(ref.LBX, ref.Record)
			palette, err := lbx.BuildPalette(assetsDir, palRefs)
			if err != nil {
				variants = append(variants, nil)
				continue
			}
			frames, err := lbx.DecodeFrames(arc.Records[ref.Record].Data, palette)
			if err != nil || len(frames) == 0 {
				variants = append(variants, nil)
				continue
			}
			tex, err := imageToTexture(renderer, frames[0])
			if err != nil {
				variants = append(variants, nil)
				continue
			}
			variants = append(variants, tex)
		}
		cm.textures[class] = variants
	}

	return cm
}

// DrawBackground renders the colony background for planetClass stretched to fill
// the full renderer output. variant selects which background image to use (0-based,
// clamped to the available range). Returns false when no background is configured
// for the class, so the caller can fall back to a solid colour fill.
func (cm *ColonyBgManager) DrawBackground(r *sdl.Renderer, planetClass string, variant int) bool {
	if cm == nil {
		return false
	}
	variants, ok := cm.textures[planetClass]
	if !ok || len(variants) == 0 {
		return false
	}
	if variant < 0 || variant >= len(variants) {
		variant = 0
	}
	tex := variants[variant]
	if tex == nil {
		return false
	}
	w, h, err := r.RenderOutputSize()
	if err != nil {
		return false
	}
	dst := sdl.FRect{X: 0, Y: 0, W: float32(w), H: float32(h)}
	r.SetDrawBlendMode(sdl.BLENDMODE_NONE)
	_ = r.RenderTexture(tex, nil, &dst)
	return true
}

// Close releases all SDL textures held by the manager.
func (cm *ColonyBgManager) Close() {
	if cm == nil {
		return
	}
	for k, variants := range cm.textures {
		for _, tex := range variants {
			if tex != nil {
				tex.Destroy()
			}
		}
		delete(cm.textures, k)
	}
}

