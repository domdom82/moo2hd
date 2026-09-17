package ui

import (
	"fmt"
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/domdom82/moo2hd/internal/game/colony"
	"github.com/domdom82/moo2hd/internal/game/galaxy"
)

// System view layout constants (screen pixels, 1080p reference).
var sysOrbitRadii = [5]float32{200, 280, 370, 470, 580}

const (
	sysStarRadius          float32 = 40.0
	sysOrbitYScale         float32 = 0.38 // ellipse minor/major ratio (inclined view)
	sysOrbitAlpha          float32 = 0.35
	sysPlanetBaseRadius    float32 = 12.0
	sysPlanetRadiusPerSize float32 = 2.0
	ellipseSegments                = 64
)

// Pan animation constants.
const (
	panSpring    float32 = 8.0
	zoomInSpring float32 = 2.0 // log-space lerp rate for navigate-in zoom
	panSnapDist  float32 = 0.5
	zoomSnapDist float32 = 0.005 // log-zoom distance to snap to logMax
)

// StarMap renders the galaxy star map at the appropriate level-of-detail
// for the current zoom level. It reads game state but never modifies it.
type StarMap struct {
	g    *galaxy.Galaxy
	cam  Camera
	font *FontManager

	// focusedSystem is the system shown in ZoomSystem view; -1 when none.
	focusedSystem galaxy.SystemID

	// Pan animation toward a target system before entering system view.
	panTargetX, panTargetY float32
	panActive              bool
	zoomToMax              bool // true when navigate-in should also zoom to ZoomMax
	panOnComplete          func()

	// planetScreenPos caches each planet disc's screen-space centre, populated
	// every drawSystem call so hit-testing can find the right planet on click.
	planetScreenPos map[galaxy.PlanetID][2]float32

	// Colony screen state.
	activeColony  *ColonyScreen
	colonyManager *colony.Manager
	localRace     string

	// systemFactionCounts maps each system to a faction → planet-count table,
	// computed from colony ownership. Used by drawFar for pie-arc blobs.
	systemFactionCounts map[galaxy.SystemID]map[int]int
}

// NewStarMap creates a StarMap for the given galaxy.
func NewStarMap(g *galaxy.Galaxy, cam Camera, font *FontManager) *StarMap {
	return &StarMap{
		g:               g,
		cam:             cam,
		font:            font,
		focusedSystem:   -1,
		planetScreenPos: make(map[galaxy.PlanetID][2]float32),
	}
}

// Camera returns a pointer to the embedded Camera so InputHandler can mutate
// it and StarMap.Draw can read the updated state.
func (sm *StarMap) Camera() *Camera {
	return &sm.cam
}

// Bind attaches InputHandler callbacks so StarMap can respond to star click,
// scroll-into-system, and planet click gestures. Call once after both sm and
// ih are created.
func (sm *StarMap) Bind(ih *InputHandler) {
	ih.FindSystemAtScreen = sm.findSystemAtScreen
	ih.OnSystemEnter = sm.beginEnterSystem
	ih.OnSystemExit = func() {
		sm.focusedSystem = -1
		sm.panActive = false
		sm.zoomToMax = false
	}
	ih.FindPlanetAtScreen = sm.findPlanetAtScreen
	ih.OnPlanetClick = sm.handlePlanetClick
}

// Update advances per-frame animations (pan toward system). Call once per frame.
func (sm *StarMap) Update(dt float32) {
	sm.updatePanToSystem(dt)
}

func (sm *StarMap) updatePanToSystem(dt float32) {
	if !sm.panActive {
		return
	}

	// Pan: exponential approach toward target.
	dx := sm.panTargetX - sm.cam.CenterX
	dy := sm.panTargetY - sm.cam.CenterY
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	panDone := dist < panSnapDist
	if panDone {
		sm.cam.CenterX, sm.cam.CenterY = sm.panTargetX, sm.panTargetY
	} else {
		t := 1.0 - float32(math.Exp(float64(-panSpring*dt)))
		sm.cam.CenterX += dx * t
		sm.cam.CenterY += dy * t
	}

	// Zoom: lerp logZoom toward logMax when requested.
	zoomDone := true
	if sm.zoomToMax {
		zoomDiff := logMax - sm.cam.logZoom
		if zoomDiff < 0 {
			zoomDiff = -zoomDiff
		}
		if zoomDiff < zoomSnapDist {
			sm.cam.logZoom = logMax
			sm.cam.Zoom = ZoomMax
			sm.cam.zoomVel = 0
			sm.cam.zoomActive = false
		} else {
			zt := 1.0 - float32(math.Exp(float64(-zoomInSpring*dt)))
			sm.cam.logZoom += (logMax - sm.cam.logZoom) * zt
			sm.cam.Zoom = float32(math.Exp(float64(sm.cam.logZoom)))
			// Keep the zoom physics dormant while we're driving it directly.
			sm.cam.zoomVel = 0
			sm.cam.zoomActive = false
			zoomDone = false
		}
	}

	if panDone && zoomDone {
		sm.panActive = false
		sm.zoomToMax = false
		if sm.panOnComplete != nil {
			sm.panOnComplete()
			sm.panOnComplete = nil
		}
	}
}

// findSystemAtScreen returns the nearest system whose screen position is within
// radius screen-px of (sx, sy). Used by InputHandler for both click and scroll
// triggers.
func (sm *StarMap) findSystemAtScreen(sx, sy, radius float32) (galaxy.SystemID, bool) {
	r2 := radius * radius
	best := galaxy.SystemID(-1)
	bestDist2 := float32(math.MaxFloat32)
	for i := range sm.g.Systems {
		sys := &sm.g.Systems[i]
		scx, scy := sm.cam.GalaxyToScreen(sys.X, sys.Y)
		dx := sx - scx
		dy := sy - scy
		d2 := dx*dx + dy*dy
		if d2 <= r2 && d2 < bestDist2 {
			bestDist2 = d2
			best = sys.ID
		}
	}
	return best, best >= 0
}

// findPlanetAtScreen returns the PlanetID whose disc centre is closest to
// (sx, sy) and within its hit radius. Uses the positions cached by the most
// recent drawSystem call.
func (sm *StarMap) findPlanetAtScreen(sx, sy float32) (galaxy.PlanetID, bool) {
	best := galaxy.PlanetID(-1)
	bestDist2 := float32(math.MaxFloat32)
	for pid, pos := range sm.planetScreenPos {
		p := sm.g.Planet(pid)
		hitR := sysPlanetBaseRadius + float32(p.Size-1)*sysPlanetRadiusPerSize + 4
		dx := sx - pos[0]
		dy := sy - pos[1]
		d2 := dx*dx + dy*dy
		if d2 <= hitR*hitR && d2 < bestDist2 {
			bestDist2 = d2
			best = pid
		}
	}
	return best, best >= 0
}

// handlePlanetClick opens the colony screen if the clicked planet has a colony
// owned by the local player; otherwise does nothing.
func (sm *StarMap) handlePlanetClick(pid galaxy.PlanetID) {
	if sm.colonyManager == nil {
		return
	}
	c := sm.colonyManager.ColonyForPlanet(int(pid))
	if c == nil || c.OwnerRace != sm.localRace {
		return
	}
	p := sm.g.Planet(pid)
	sys := sm.g.System(p.SystemID)
	cs := NewColonyScreen(c, p, sys, sm.font)
	cs.OnClose = func() { sm.activeColony = nil }
	sm.activeColony = cs
}

// SetColonyData provides the colony manager and local player race to StarMap
// so planet clicks can look up ownership and faction blobs can show mixed colours.
func (sm *StarMap) SetColonyData(mgr *colony.Manager, localRace string) {
	sm.colonyManager = mgr
	sm.localRace = localRace

	// Build per-system faction → planet count index.
	counts := make(map[galaxy.SystemID]map[int]int)
	for _, c := range mgr.Colonies {
		f := factionForRace(c.OwnerRace)
		if counts[c.SystemID] == nil {
			counts[c.SystemID] = make(map[int]int)
		}
		counts[c.SystemID][f]++
	}
	sm.systemFactionCounts = counts
}

// factionForRace converts a race name back to a faction index for display.
// "human" → 0; "faction-N" → N; anything else → 0.
func factionForRace(race string) int {
	if race == "human" {
		return 0
	}
	var n int
	fmt.Sscanf(race, "faction-%d", &n)
	return n
}

// HasActiveScreen returns true when a modal screen (e.g. colony management)
// is open and should consume all input.
func (sm *StarMap) HasActiveScreen() bool {
	return sm.activeColony != nil
}

// HandleScreen forwards an SDL event to the currently active modal screen.
// Returns an error only when the application should quit.
func (sm *StarMap) HandleScreen(event *sdl.Event) error {
	if sm.activeColony != nil {
		return sm.activeColony.Handle(event)
	}
	return nil
}

// beginEnterSystem is the OnSystemEnter callback. It pans to the star and
// zooms to ZoomMax, then opens the system view. Works from any zoom tier.
func (sm *StarMap) beginEnterSystem(id galaxy.SystemID) {
	if sm.cam.InSystem() {
		return
	}
	sys := sm.g.System(id)
	sm.focusedSystem = id
	sm.panTargetX = sys.X
	sm.panTargetY = sys.Y
	sm.panActive = true
	sm.zoomToMax = true
	sm.panOnComplete = func() {
		sm.cam.EnterSystem()
	}
}

// Draw clears the renderer, renders the star map at the current zoom tier,
// then presents the frame. If a modal screen is active it is rendered instead
// of the system view.
func (sm *StarMap) Draw(r *sdl.Renderer) error {
	bg := BackgroundColor()
	r.SetDrawColorFloat(bg.R, bg.G, bg.B, bg.A)
	r.Clear()

	if sm.activeColony != nil {
		if err := sm.activeColony.Draw(r); err != nil {
			return err
		}
		return r.Present()
	}

	switch sm.cam.Tier() {
	case ZoomFar:
		sm.drawFar(r)
	case ZoomMid:
		sm.drawMid(r)
	case ZoomClose:
		sm.drawClose(r)
	case ZoomSystem:
		sm.drawSystem(r)
	}

	return r.Present()
}

// drawFar renders each system as a large semi-transparent filled circle
// coloured by faction ownership. Systems with mixed ownership are drawn as
// proportional pie slices (one per faction present). Unclaimed stars are
// rendered as small star discs so they remain visible.
func (sm *StarMap) drawFar(r *sdl.Renderer) {
	const galaxyRadius = float32(60.0)
	r.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
	for i := range sm.g.Systems {
		sys := &sm.g.Systems[i]
		screenR := galaxyRadius * sm.cam.Zoom
		if !sm.cam.Visible(sys.X, sys.Y, screenR) {
			continue
		}
		sx, sy := sm.cam.GalaxyToScreen(sys.X, sys.Y)

		if sys.Faction < 0 {
			continue // unclaimed: no territory disc; star disc drawn by drawStars
		}
		fCounts := sm.systemFactionCounts[sys.ID]
		if len(fCounts) <= 1 {
			// Single owning faction: fast path, solid circle.
			col := FactionColor(sys.Faction)
			verts := buildCircleVertices(sx, sy, screenR, col)
			r.RenderGeometry(nil, verts, circleIndices[:])
			continue
		}

		// Multiple factions: build sorted pie slices.
		total := 0
		for _, cnt := range fCounts {
			total += cnt
		}
		// Sort by faction index for deterministic order.
		factions := make([]int, 0, len(fCounts))
		for f := range fCounts {
			factions = append(factions, f)
		}
		for i := 1; i < len(factions); i++ {
			for j := i; j > 0 && factions[j] < factions[j-1]; j-- {
				factions[j], factions[j-1] = factions[j-1], factions[j]
			}
		}
		slices := make([]pieSlice, len(factions))
		for i, f := range factions {
			slices[i] = pieSlice{
				fraction: float32(fCounts[f]) / float32(total),
				col:      FactionColor(f),
			}
		}
		verts, indices := buildPieVertices(sx, sy, screenR, slices)
		r.RenderGeometry(nil, verts, indices)
	}
	sm.drawStars(r, 5.0)
}

// drawRegions draws the faction territory blobs (same as drawFar).
func (sm *StarMap) drawRegions(r *sdl.Renderer) {
	sm.drawFar(r)
}

// drawMid renders faction regions, travel lanes, star discs, and system names.
func (sm *StarMap) drawMid(r *sdl.Renderer) {
	sm.drawRegions(r)
	sm.drawLanes(r, ZoomMid)
	sm.drawStars(r, 4.0)
	sm.drawLabels(r, 4.0)
}

// drawClose renders lanes, stars, and system name/planet-count labels.
func (sm *StarMap) drawClose(r *sdl.Renderer) {
	sm.drawLanes(r, ZoomClose)
	sm.drawStars(r, 5.0)
	sm.drawLabels(r, 5.0)
}

func (sm *StarMap) drawLanes(r *sdl.Renderer, tier ZoomTier) {
	lc := LaneColor(tier)
	r.SetDrawColorFloat(lc.R, lc.G, lc.B, lc.A)
	for i, lanes := range sm.g.Adjacency {
		for _, lane := range lanes {
			if i >= int(lane.To) {
				continue // draw each undirected lane once
			}
			srcSys := &sm.g.Systems[i]
			dstSys := &sm.g.Systems[lane.To]
			x1, y1 := sm.cam.GalaxyToScreen(srcSys.X, srcSys.Y)
			x2, y2 := sm.cam.GalaxyToScreen(dstSys.X, dstSys.Y)
			r.RenderLine(x1, y1, x2, y2)
		}
	}
}

func (sm *StarMap) drawStars(r *sdl.Renderer, baseRadius float32) {
	r.SetDrawBlendMode(sdl.BLENDMODE_NONE)
	for i := range sm.g.Systems {
		sys := &sm.g.Systems[i]
		radius := baseRadius * sm.cam.Zoom
		if !sm.cam.Visible(sys.X, sys.Y, radius+4) {
			continue
		}
		sx, sy := sm.cam.GalaxyToScreen(sys.X, sys.Y)
		if radius < 2 {
			radius = 2
		}
		col := StarColor(sys.Star)
		verts := buildCircleVertices(sx, sy, radius, col)
		r.RenderGeometry(nil, verts, circleIndices[:])
	}
}

func (sm *StarMap) drawLabels(r *sdl.Renderer, baseRadius float32) {
	white := sdl.Color{R: 220, G: 220, B: 220, A: 255}
	grey := sdl.Color{R: 140, G: 140, B: 160, A: 255}
	const gap = float32(4)
	const lineH = float32(16)
	for i := range sm.g.Systems {
		sys := &sm.g.Systems[i]
		if !sm.cam.Visible(sys.X, sys.Y, 60) {
			continue
		}
		sx, sy := sm.cam.GalaxyToScreen(sys.X, sys.Y)
		starR := baseRadius * sm.cam.Zoom
		if starR < 2 {
			starR = 2
		}
		labelY := sy + starR + gap
		sm.font.DrawText(sys.Name, sx-float32(len(sys.Name))*4, labelY, white)
		sm.font.DrawText(fmt.Sprintf("P:%d", len(sys.Planets)), sx-8, labelY+lineH, grey)
	}
}

// drawSystem renders the ZoomSystem view: central star, orbital ellipses, and
// planet discs at seeded fixed positions. Everything is in screen space with
// the star at screen centre.
func (sm *StarMap) drawSystem(r *sdl.Renderer) {
	if sm.focusedSystem < 0 {
		return
	}
	sys := sm.g.System(sm.focusedSystem)
	cx := sm.cam.ScreenW / 2
	cy := sm.cam.ScreenH / 2

	// Orbit ellipses (drawn first so planets render on top).
	orbitCol := sdl.FColor{R: 0.4, G: 0.5, B: 0.7, A: sysOrbitAlpha}
	r.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
	for _, pid := range sys.Planets {
		p := sm.g.Planet(pid)
		rx := sysOrbitRadii[p.Slot-1]
		ry := rx * sysOrbitYScale
		drawEllipse(r, cx, cy, rx, ry, orbitCol)
	}

	// Central star.
	r.SetDrawBlendMode(sdl.BLENDMODE_NONE)
	starCol := StarColor(sys.Star)
	verts := buildCircleVertices(cx, cy, sysStarRadius, starCol)
	r.RenderGeometry(nil, verts, circleIndices[:])

	// Planet discs at seeded angles.
	for _, pid := range sys.Planets {
		p := sm.g.Planet(pid)
		angle := planetAngle(pid)
		rx := sysOrbitRadii[p.Slot-1]
		ry := rx * sysOrbitYScale
		px := cx + rx*float32(math.Cos(float64(angle)))
		py := cy + ry*float32(math.Sin(float64(angle)))
		pRadius := sysPlanetBaseRadius + float32(p.Size-1)*sysPlanetRadiusPerSize
		col := PlanetColor(p.Class)
		verts := buildCircleVertices(px, py, pRadius, col)
		r.SetDrawBlendMode(sdl.BLENDMODE_NONE)
		r.RenderGeometry(nil, verts, circleIndices[:])
		sm.planetScreenPos[pid] = [2]float32{px, py}
	}

	// System name below the star.
	white := sdl.Color{R: 220, G: 220, B: 220, A: 255}
	sm.font.DrawText(sys.Name, cx-float32(len(sys.Name))*4, cy+sysStarRadius+4, white)
}

// drawEllipse draws an unfilled ellipse outline using a 64-segment polyline.
func drawEllipse(r *sdl.Renderer, cx, cy, rx, ry float32, col sdl.FColor) {
	r.SetDrawColorFloat(col.R, col.G, col.B, col.A)
	pts := make([]sdl.FPoint, ellipseSegments+1)
	for i := 0; i <= ellipseSegments; i++ {
		angle := float64(i) / float64(ellipseSegments) * 2 * math.Pi
		pts[i] = sdl.FPoint{
			X: cx + rx*float32(math.Cos(angle)),
			Y: cy + ry*float32(math.Sin(angle)),
		}
	}
	r.RenderLines(pts)
}

// planetAngle returns a stable seeded angle in [0, 2π) for a planet's position
// on its orbit. The LCG hash of the PlanetID ensures different planets get
// spread angles without any per-frame randomness.
func planetAngle(pid galaxy.PlanetID) float32 {
	h := uint32(pid)*2654435761 + 0x9e3779b9
	return float32(h%10000) / 10000.0 * 2 * math.Pi
}
