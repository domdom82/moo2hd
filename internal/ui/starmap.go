package ui

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/domdom82/moo2hd/internal/game/galaxy"
)

// StarMap renders the galaxy star map at the appropriate level-of-detail
// for the current zoom level. It reads game state but never modifies it.
type StarMap struct {
	g    *galaxy.Galaxy
	cam  Camera
	font *FontManager
}

// NewStarMap creates a StarMap for the given galaxy.
func NewStarMap(g *galaxy.Galaxy, cam Camera, font *FontManager) *StarMap {
	return &StarMap{g: g, cam: cam, font: font}
}

// Camera returns a pointer to the embedded Camera so InputHandler can mutate
// it and StarMap.Draw can read the updated state.
func (sm *StarMap) Camera() *Camera {
	return &sm.cam
}

// Draw clears the renderer, renders the star map at the current zoom tier,
// then presents the frame.
func (sm *StarMap) Draw(r *sdl.Renderer) error {
	bg := BackgroundColor()
	r.SetDrawColorFloat(bg.R, bg.G, bg.B, bg.A)
	r.Clear()

	switch sm.cam.Tier() {
	case ZoomFar:
		sm.drawFar(r)
	case ZoomMid:
		sm.drawMid(r)
	case ZoomClose:
		sm.drawClose(r)
	}

	return r.Present()
}

// drawFar renders each system as a large semi-transparent filled circle
// coloured by faction. Overlapping alpha circles give an organic territory
// feel that approximates Voronoi regions.
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
		col := FactionColor(sys.Faction)
		verts := buildCircleVertices(sx, sy, screenR, col)
		r.RenderGeometry(nil, verts, circleIndices[:])
	}
}

// drawRegions draws the faction territory blobs (same as drawFar).
func (sm *StarMap) drawRegions(r *sdl.Renderer) {
	sm.drawFar(r)
}

// drawMid renders faction regions, travel lanes, and small star discs.
func (sm *StarMap) drawMid(r *sdl.Renderer) {
	sm.drawRegions(r)
	sm.drawLanes(r, ZoomMid)
	sm.drawStars(r, 4.0)
}

// drawClose renders lanes, stars, and system name/planet-count labels.
func (sm *StarMap) drawClose(r *sdl.Renderer) {
	sm.drawLanes(r, ZoomClose)
	sm.drawStars(r, 5.0)
	sm.drawLabels(r)
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
		if !sm.cam.Visible(sys.X, sys.Y, baseRadius*sm.cam.Zoom+4) {
			continue
		}
		sx, sy := sm.cam.GalaxyToScreen(sys.X, sys.Y)
		radius := baseRadius * sm.cam.Zoom
		if radius < 2 {
			radius = 2
		}
		col := StarColor(sys.Star)
		verts := buildCircleVertices(sx, sy, radius, col)
		r.RenderGeometry(nil, verts, circleIndices[:])
	}
}

func (sm *StarMap) drawLabels(r *sdl.Renderer) {
	white := sdl.Color{R: 220, G: 220, B: 220, A: 255}
	grey := sdl.Color{R: 140, G: 140, B: 160, A: 255}
	for i := range sm.g.Systems {
		sys := &sm.g.Systems[i]
		if !sm.cam.Visible(sys.X, sys.Y, 60) {
			continue
		}
		sx, sy := sm.cam.GalaxyToScreen(sys.X, sys.Y)
		sm.font.DrawText(sys.Name, sx-float32(len(sys.Name))*4, sy+10, white)
		sm.font.DrawText(fmt.Sprintf("P:%d", len(sys.Planets)), sx-8, sy+26, grey)
	}
}
