package ui

// ZoomTier classifies the current zoom level for level-of-detail rendering.
type ZoomTier int

const (
	ZoomFar   ZoomTier = iota // faction-colored territory blobs
	ZoomMid                    // travel lanes + star discs
	ZoomClose                  // names + planet count labels
)

// Exported thresholds so tests and other packages can reason about tier
// transitions without duplicating magic numbers.
const (
	ZoomMidThreshold   float32 = 0.8
	ZoomCloseThreshold float32 = 2.0
	ZoomMin  float32 = 0.2
	ZoomMax  float32 = 6.0
	zoomStep float32 = 0.15
)

// Camera holds the viewport state for the star map.
type Camera struct {
	CenterX float32 // galaxy-space X coordinate shown at screen centre
	CenterY float32 // galaxy-space Y coordinate shown at screen centre
	Zoom    float32 // scale factor: galaxy units × Zoom = screen pixels
	ScreenW float32
	ScreenH float32
}

// NewCamera returns a Camera centred on the middle of a 1000×1000 galaxy
// at a zoom level that starts in the Far tier.
func NewCamera(screenW, screenH float32) Camera {
	return Camera{
		CenterX: 500,
		CenterY: 500,
		Zoom:    0.6,
		ScreenW: screenW,
		ScreenH: screenH,
	}
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

// ZoomIn multiplies the zoom factor by (1+zoomStep), clamped to ZoomMax.
func (c *Camera) ZoomIn() {
	c.Zoom *= 1 + zoomStep
	if c.Zoom > ZoomMax {
		c.Zoom = ZoomMax
	}
}

// ZoomOut divides the zoom factor by (1+zoomStep), clamped to ZoomMin.
func (c *Camera) ZoomOut() {
	c.Zoom /= 1 + zoomStep
	if c.Zoom < ZoomMin {
		c.Zoom = ZoomMin
	}
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

// Clamp keeps CenterX/Y inside the galaxy bounds so the view never
// drifts into empty space. galaxyW and galaxyH are the map extents.
func (c *Camera) Clamp(galaxyW, galaxyH float32) {
	half := float32(0.5)
	minX := c.ScreenW * half / c.Zoom
	minY := c.ScreenH * half / c.Zoom
	maxX := galaxyW - minX
	maxY := galaxyH - minY

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
