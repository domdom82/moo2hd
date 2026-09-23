package ui

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/game/galaxy"
	"github.com/domdom82/moo2hd/internal/lbx"
)

// starSpriteKey indexes the per-zoom sprite atlas.
type starSpriteKey struct {
	star string
	tier ZoomTier
}

// starFrames holds the decoded SDL textures for one (star, tier) combination.
// Multi-frame sprites have len > 1; single-frame sprites have exactly 1 entry.
type starFrames []*sdl.Texture

// starAnimState tracks the live animation state for one system.
type starAnimState struct {
	frame      int     // current frame index
	elapsed    float32 // seconds since last frame advance
	frameTime  float32 // seconds per frame (from FrameDelay)
	frameCount int     // total frames in the sprite
	playing    bool
}

// StarSpriteManager loads star sprite art from BUFFER0.LBX and drives the
// timed random-twinkle scheduler. One instance is shared across all draw calls.
type StarSpriteManager struct {
	sprites   map[starSpriteKey]starFrames // pre-decoded textures per (star, tier)
	animState map[galaxy.SystemID]*starAnimState

	g *galaxy.Galaxy // needed to look up star types for the scheduler

	// twinkle scheduler
	galaxySize  string
	timing      *config.StarAnimTiming
	systemIDs   []galaxy.SystemID // all system IDs, for random selection
	nextTrigger float32           // countdown in seconds until the next twinkle batch
	rng         *rand.Rand
}

// NewStarSpriteManager loads all configured star sprites from assetsDir.
// galaxySize should be a galaxy.GalaxySize string value. Missing or undecodable
// records are skipped silently so a partial asset set doesn't crash the game.
func NewStarSpriteManager(
	renderer *sdl.Renderer,
	assetsDir string,
	reg *config.Registry,
	g *galaxy.Galaxy,
	galaxySize string,
	seed uint64,
) *StarSpriteManager {
	sm := &StarSpriteManager{
		sprites:    make(map[starSpriteKey]starFrames),
		animState:  make(map[galaxy.SystemID]*starAnimState),
		g:          g,
		galaxySize: galaxySize,
		rng:        rand.New(rand.NewPCG(seed, seed>>32)),
	}

	if reg == nil {
		return sm
	}

	sm.timing = reg.StarAnimFor(galaxySize)

	// Collect system IDs for the twinkle scheduler.
	sm.systemIDs = make([]galaxy.SystemID, len(g.Systems))
	for i := range g.Systems {
		sm.systemIDs[i] = g.Systems[i].ID
	}

	// Open and parse BUFFER0.LBX once; we'll look up records by index.
	arcData, err := openLBX(assetsDir, "BUFFER0.LBX")
	if err != nil {
		return sm
	}
	arc, err := lbx.Parse(arcData)
	if err != nil {
		return sm
	}

	starTypes := []string{
		string(galaxy.StarBlue),
		string(galaxy.StarWhite),
		string(galaxy.StarYellow),
		string(galaxy.StarOrange),
		string(galaxy.StarRed),
		string(galaxy.StarBrown),
		string(galaxy.StarBlackHole),
	}

	zoomRefs := func(e *config.StarSpriteEntry) map[ZoomTier]config.LBXSpriteRef {
		return map[ZoomTier]config.LBXSpriteRef{
			ZoomSystem: e.System,
			ZoomClose:  e.Close,
			ZoomMid:    e.Mid,
			ZoomFar:    e.Far,
		}
	}

	for _, star := range starTypes {
		entry := reg.StarSprite(star)
		if entry == nil {
			continue
		}
		for tier, ref := range zoomRefs(entry) {
			if ref.LBX == "" || ref.Record < 0 || ref.Record >= len(arc.Records) {
				continue
			}
			palRefs := reg.LBXPaletteFor(ref.LBX, ref.Record)
			palette, err := lbx.BuildPalette(assetsDir, palRefs)
			if err != nil {
				continue
			}
			frames, err := lbx.DecodeFrames(arc.Records[ref.Record].Data, palette)
			if err != nil || len(frames) == 0 {
				continue
			}
			textures := make(starFrames, len(frames))
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
			sm.sprites[starSpriteKey{star, tier}] = textures
		}
	}

	// Schedule the first twinkle trigger.
	sm.resetTrigger()

	return sm
}

// openLBX tries the uppercase filename then the lowercase fallback.
func openLBX(assetsDir, name string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(assetsDir, name))
	if err != nil {
		data, err = os.ReadFile(filepath.Join(assetsDir, strings.ToLower(name)))
	}
	return data, err
}

// Update advances the twinkle scheduler and all active star animations by dt seconds.
func (sm *StarSpriteManager) Update(dt float32) {
	// Advance active animations.
	for id, st := range sm.animState {
		if !st.playing {
			continue
		}
		st.elapsed += dt
		if st.elapsed >= st.frameTime {
			st.elapsed -= st.frameTime
			st.frame++
		}
		// Stop when we've shown all frames once (one-shot twinkle).
		if st.frame >= st.frameCount {
			delete(sm.animState, id)
		}
	}

	if sm.timing == nil || len(sm.systemIDs) == 0 {
		return
	}

	// Twinkle scheduler.
	sm.nextTrigger -= dt
	if sm.nextTrigger > 0 {
		return
	}

	// Pick how many stars to twinkle this batch.
	count := sm.timing.CountMin
	if sm.timing.CountMax > sm.timing.CountMin {
		count += sm.rng.IntN(sm.timing.CountMax - sm.timing.CountMin + 1)
	}

	// Select random systems (skip any that are already animating).
	perm := sm.rng.Perm(len(sm.systemIDs))
	triggered := 0
	for _, idx := range perm {
		if triggered >= count {
			break
		}
		id := sm.systemIDs[idx]
		if _, active := sm.animState[id]; active {
			continue
		}
		sys := sm.g.System(id)
		// Use ZoomClose frames for the animation (representative frame count).
		frames := sm.sprites[starSpriteKey{string(sys.Star), ZoomClose}]
		if len(frames) <= 1 {
			continue // no multi-frame sprite; skip
		}
		sm.animState[id] = &starAnimState{
			frame:      0,
			elapsed:    0,
			frameTime:  1.0 / 24.0,
			frameCount: len(frames),
			playing:    true,
		}
		triggered++
	}

	sm.resetTrigger()
}

// resetTrigger picks a new random delay in [IntervalMinS, IntervalMaxS].
func (sm *StarSpriteManager) resetTrigger() {
	if sm.timing == nil {
		sm.nextTrigger = 60
		return
	}
	span := sm.timing.IntervalMaxS - sm.timing.IntervalMinS
	sm.nextTrigger = float32(sm.timing.IntervalMinS + sm.rng.Float64()*span)
}

// DrawStar renders the star for the given system at screen position (sx, sy).
// displaySize is the desired on-screen diameter in pixels. tier selects the
// sprite LOD. Returns false if no sprite is configured for this star/tier
// combination (caller should fall back to a circle).
func (sm *StarSpriteManager) DrawStar(
	r *sdl.Renderer,
	sys *galaxy.System,
	sx, sy, displaySize float32,
	tier ZoomTier,
) bool {
	if sm == nil {
		return false
	}
	key := starSpriteKey{string(sys.Star), tier}
	frames, ok := sm.sprites[key]
	if !ok || len(frames) == 0 {
		return false
	}

	frame := 0
	if st, active := sm.animState[sys.ID]; active && st.playing {
		frame = st.frame
		if frame >= len(frames) {
			frame = len(frames) - 1
		}
	}

	tex := frames[frame]
	if tex == nil {
		return false
	}

	half := displaySize / 2
	dst := sdl.FRect{
		X: sx - half,
		Y: sy - half,
		W: displaySize,
		H: displaySize,
	}
	r.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
	_ = r.RenderTexture(tex, nil, &dst)
	return true
}

// Close releases all SDL textures.
func (sm *StarSpriteManager) Close() {
	if sm == nil {
		return
	}
	for k, frames := range sm.sprites {
		for _, tex := range frames {
			if tex != nil {
				tex.Destroy()
			}
		}
		delete(sm.sprites, k)
	}
}
