package ui

import "math"

// ZoomTier classifies the current zoom level for level-of-detail rendering.
type ZoomTier int

const (
	ZoomFar   ZoomTier = iota // faction-colored territory blobs
	ZoomMid                   // travel lanes + star discs
	ZoomClose                 // names + planet count labels
)

// Exported thresholds so tests and other packages can reason about tier
// transitions without duplicating magic numbers.
const (
	ZoomMidThreshold   float32 = 0.8
	ZoomCloseThreshold float32 = 2.0
	ZoomMin            float32 = 0.2
	ZoomMax            float32 = 6.0
	zoomStep           float32 = 0.15

	// zoomFriction is the fraction of zoom velocity remaining after one second.
	zoomFriction float32 = 0.05

	// zoomSpring is the spring constant (s^-2) that snaps logZoom back to the
	// nearest hard limit when no input is active and the zoom is out of range.
	// Higher = snappier return. ~25 gives a quick but smooth snap (~0.3 s).
	zoomSpring float32 = 25.0

	// zoomSpringDamp damps the spring so it doesn't oscillate.
	// Critical damping = 2*sqrt(k) ≈ 10; use slightly less for a tiny overshoot.
	zoomSpringDamp float32 = 8.0

	// zoomOvershootMax caps how far (in log-zoom units) the user can push past
	// the hard limits. ~0.18 ≈ one extra zoomStep worth of overshoot.
	zoomOvershootMax float32 = 0.18
)

// Camera holds the viewport state for the star map.
type Camera struct {
	CenterX float32 // galaxy-space X coordinate shown at screen centre
	CenterY float32 // galaxy-space Y coordinate shown at screen centre
	Zoom    float32 // scale factor: galaxy units × Zoom = screen pixels
	ScreenW float32
	ScreenH float32

	// logZoom is the authoritative zoom in log-space, which may temporarily
	// exceed [logMin, logMax] during a bounce.
	logZoom float32
	// zoomVel is zoom velocity in log(zoom) units per second.
	zoomVel float32
	// zoomActive is true while the user is actively sending zoom input.
	// The spring only fires once this is false.
	zoomActive bool
}

// NewCamera returns a Camera centred on the galaxy and zoomed out to fit it
// entirely within the screen. galaxyW and galaxyH are the map extents in
// galaxy units.
func NewCamera(screenW, screenH, galaxyW, galaxyH float32) Camera {
	// Fit-to-screen zoom: the smaller of the two axis ratios ensures the whole
	// map is visible with a small margin.
	fitZoom := float32(math.Min(
		float64(screenW/galaxyW),
		float64(screenH/galaxyH),
	)) * 0.9
	if fitZoom < 0.001 {
		fitZoom = 0.001
	}
	c := Camera{
		CenterX: galaxyW / 2,
		CenterY: galaxyH / 2,
		Zoom:    fitZoom,
		logZoom: float32(math.Log(float64(fitZoom))),
		ScreenW: screenW,
		ScreenH: screenH,
	}
	// Recompute logMin so the player can always zoom out to fit-zoom.
	logMin = float32(math.Log(float64(fitZoom)))
	return c
}

// GalaxyToScreen converts galaxy-space coordinates to screen pixels.
func (c Camera) GalaxyToScreen(gx, gy float32) (sx, sy float32) {
	sx = (gx-c.CenterX)*c.Zoom + c.ScreenW/2
	sy = (gy-c.CenterY)*c.Zoom + c.ScreenH/2
	return
}

// ScreenToGalaxy is the inverse of GalaxyToScreen.
func (c Camera) ScreenToGalaxy(sx, sy float32) (gx, gy float32) {
	gx = (sx-c.ScreenW/2)/c.Zoom + c.CenterX
	gy = (sy-c.ScreenH/2)/c.Zoom + c.CenterY
	return
}

// Tier classifies the current zoom level.
func (c Camera) Tier() ZoomTier {
	switch {
	case c.Zoom < ZoomMidThreshold:
		return ZoomFar
	case c.Zoom < ZoomCloseThreshold:
		return ZoomMid
	default:
		return ZoomClose
	}
}

// logMin and logMax are the hard limits in log-space.
var (
	logMin = float32(math.Log(float64(ZoomMin)))
	logMax = float32(math.Log(float64(ZoomMax)))
)

// zoomImpulse is the log-space speed added per scroll tick, sized so a single
// tick always coasts for ~the same duration regardless of how many are queued.
var zoomImpulse = float32(math.Log(float64(1+zoomStep)) / (1 - math.Pow(float64(zoomFriction), 1.0/60)))

// ZoomIn adds a log-space impulse toward larger zoom values.
// The zoom may temporarily exceed ZoomMax; it springs back once input stops.
func (c *Camera) ZoomIn() {
	c.zoomVel += zoomImpulse
	c.zoomActive = true
}

// ZoomOut adds a log-space impulse toward smaller zoom values.
// The zoom may temporarily go below ZoomMin; it springs back once input stops.
func (c *Camera) ZoomOut() {
	c.zoomVel -= zoomImpulse
	c.zoomActive = true
}

// ZoomInputDone signals that the user has released the scroll wheel or zoom
// keys. The spring will now pull any overshoot back to the hard limit.
func (c *Camera) ZoomInputDone() {
	c.zoomActive = false
}

// UpdateZoom advances the smooth zoom animation for one frame.
// dt is elapsed seconds since the last call.
func (c *Camera) UpdateZoom(dt float32) {
	if c.zoomVel == 0 && !c.isOvershot() {
		return
	}

	if c.zoomActive {
		// While input is active: apply velocity with friction, allow overshoot
		// up to zoomOvershootMax beyond each hard limit.
		decay := float32(math.Pow(float64(zoomFriction), float64(dt)))
		c.logZoom += c.zoomVel * dt
		c.zoomVel *= decay

		// Hard-cap overshoot so the user can't fling infinitely past the limit.
		if c.logZoom < logMin-zoomOvershootMax {
			c.logZoom = logMin - zoomOvershootMax
			if c.zoomVel < 0 {
				c.zoomVel = 0
			}
		} else if c.logZoom > logMax+zoomOvershootMax {
			c.logZoom = logMax + zoomOvershootMax
			if c.zoomVel > 0 {
				c.zoomVel = 0
			}
		}
	} else {
		// Input released: spring pulls logZoom back to [logMin, logMax].
		target := c.logZoom
		if target < logMin {
			target = logMin
		} else if target > logMax {
			target = logMax
		}

		if target != c.logZoom {
			// Damped spring: a = -k*x - d*v  where x is displacement from target.
			disp := c.logZoom - target
			accel := -zoomSpring*disp - zoomSpringDamp*c.zoomVel
			c.zoomVel += accel * dt
			c.logZoom += c.zoomVel * dt

			// Snap to rest when both displacement and velocity are negligible.
			newDisp := c.logZoom - target
			if newDisp*newDisp < 0.00001 && c.zoomVel*c.zoomVel < 0.0001 {
				c.logZoom = target
				c.zoomVel = 0
			}
		} else {
			// Inside bounds: normal friction decay.
			decay := float32(math.Pow(float64(zoomFriction), float64(dt)))
			c.logZoom += c.zoomVel * dt
			c.zoomVel *= decay

			// Clamp to hard limits once friction has settled.
			if c.logZoom < logMin {
				c.logZoom = logMin
				c.zoomVel = 0
			} else if c.logZoom > logMax {
				c.logZoom = logMax
				c.zoomVel = 0
			}

			if c.zoomVel*c.zoomVel < 0.001 {
				c.zoomVel = 0
			}
		}
	}

	c.Zoom = float32(math.Exp(float64(c.logZoom)))
}

// isOvershot reports whether logZoom is currently outside the hard limits.
func (c *Camera) isOvershot() bool {
	return c.logZoom < logMin || c.logZoom > logMax
}

// Pan moves the camera centre by (dx, dy) in galaxy units.
func (c *Camera) Pan(dx, dy float32) {
	c.CenterX += dx
	c.CenterY += dy
}

// PanPx converts a screen-pixel delta to galaxy units and calls Pan.
func (c *Camera) PanPx(dpx, dpy float32) {
	c.Pan(dpx/c.Zoom, dpy/c.Zoom)
}

// panMargin expands the effective galaxy bounds for clamping, letting the
// player pan ~17% beyond the nominal edge. This ensures stars placed near
// the boundary remain reachable, and leaves room for future UI chrome.
const panMargin float32 = 0.17

// Clamp keeps CenterX/Y inside the galaxy bounds so the view never
// drifts into empty space. galaxyW and galaxyH are the map extents.
func (c *Camera) Clamp(galaxyW, galaxyH float32) {
	marginX := galaxyW * panMargin
	marginY := galaxyH * panMargin
	half := float32(0.5)
	minX := c.ScreenW*half/c.Zoom - marginX
	minY := c.ScreenH*half/c.Zoom - marginY
	maxX := galaxyW - c.ScreenW*half/c.Zoom + marginX
	maxY := galaxyH - c.ScreenH*half/c.Zoom + marginY

	if minX > maxX {
		c.CenterX = galaxyW / 2
	} else if c.CenterX < minX {
		c.CenterX = minX
	} else if c.CenterX > maxX {
		c.CenterX = maxX
	}

	if minY > maxY {
		c.CenterY = galaxyH / 2
	} else if c.CenterY < minY {
		c.CenterY = minY
	} else if c.CenterY > maxY {
		c.CenterY = maxY
	}
}

// Visible reports whether the galaxy point (gx, gy) projects to within the
// screen rectangle extended by extraPx on each side.
func (c Camera) Visible(gx, gy, extraPx float32) bool {
	sx, sy := c.GalaxyToScreen(gx, gy)
	return sx+extraPx >= 0 && sx-extraPx <= c.ScreenW &&
		sy+extraPx >= 0 && sy-extraPx <= c.ScreenH
}
