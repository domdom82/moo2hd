package ui

import (
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
)

const (
	inputPanStep = float32(20.0) // screen pixels per arrow-key press

	// friction is the fraction of pan velocity remaining after one second.
	friction = float32(0.05)

	// velocitySmooth blends each drag sample into the running velocity,
	// damping single-frame spikes without noticeable lag.
	velocitySmooth = float32(0.35)

	// zoomInputTimeout is how long (seconds) after the last zoom event before
	// the camera treats input as released and allows the spring to activate.
	zoomInputTimeout = float32(0.15)
)

// InputHandler translates SDL events into Camera mutations.
type InputHandler struct {
	cam              *Camera
	galaxyW, galaxyH float32
	dragging         bool
	lastX, lastY     float32
	velX, velY       float32 // pan momentum in screen-pixels per second
	zoomIdleFor      float32 // seconds since last zoom input
}

// NewInputHandler creates an InputHandler that mutates cam.
func NewInputHandler(cam *Camera, galaxyW, galaxyH float32) *InputHandler {
	return &InputHandler{
		cam:         cam,
		galaxyW:     galaxyW,
		galaxyH:     galaxyH,
		zoomIdleFor: zoomInputTimeout, // start with spring inactive
	}
}

// Update advances smooth pan and zoom for one frame. dt is elapsed seconds.
// Call once per frame regardless of event activity.
func (h *InputHandler) Update(dt float32) {
	// Advance the zoom-idle timer and signal the camera when the user has
	// stopped scrolling so the spring can engage.
	h.zoomIdleFor += dt
	if h.zoomIdleFor >= zoomInputTimeout {
		h.cam.ZoomInputDone()
	}

	h.cam.UpdateZoom(dt)
	h.updatePan(dt)
}

func (h *InputHandler) updatePan(dt float32) {
	if h.dragging || (h.velX == 0 && h.velY == 0) {
		return
	}
	decay := float32(math.Pow(float64(friction), float64(dt)))
	h.velX *= decay
	h.velY *= decay
	if h.velX*h.velX+h.velY*h.velY < 0.25 {
		h.velX, h.velY = 0, 0
		return
	}
	h.cam.PanPx(h.velX*dt, h.velY*dt)
	h.cam.Clamp(h.galaxyW, h.galaxyH)
}

// Handle processes one SDL event.
// Returns sdl.EndLoop when the application should quit, nil otherwise.
func (h *InputHandler) Handle(event *sdl.Event) error {
	switch event.Type {
	case sdl.EVENT_QUIT:
		return sdl.EndLoop

	case sdl.EVENT_KEY_DOWN:
		return h.handleKey(event.KeyboardEvent())

	case sdl.EVENT_KEY_UP:
		h.handleKeyUp(event.KeyboardEvent())

	case sdl.EVENT_MOUSE_BUTTON_DOWN:
		evt := event.MouseButtonEvent()
		if evt.Button == uint8(sdl.BUTTON_LEFT) {
			h.dragging = true
			h.lastX, h.lastY = evt.X, evt.Y
			h.velX, h.velY = 0, 0
		}

	case sdl.EVENT_MOUSE_BUTTON_UP:
		if event.MouseButtonEvent().Button == uint8(sdl.BUTTON_LEFT) {
			h.dragging = false
		}

	case sdl.EVENT_MOUSE_MOTION:
		if h.dragging {
			evt := event.MouseMotionEvent()
			h.cam.PanPx(-evt.Xrel, -evt.Yrel)
			h.cam.Clamp(h.galaxyW, h.galaxyH)
			const sampleHz = float32(60)
			instVelX := -evt.Xrel * sampleHz
			instVelY := -evt.Yrel * sampleHz
			h.velX = h.velX*(1-velocitySmooth) + instVelX*velocitySmooth
			h.velY = h.velY*(1-velocitySmooth) + instVelY*velocitySmooth
		}

	case sdl.EVENT_MOUSE_WHEEL:
		evt := event.MouseWheelEvent()
		if evt.Y > 0 {
			h.cam.ZoomIn()
		} else if evt.Y < 0 {
			h.cam.ZoomOut()
		}
		h.zoomIdleFor = 0 // reset timeout so spring stays dormant while scrolling
		h.cam.Clamp(h.galaxyW, h.galaxyH)
	}
	return nil
}

func (h *InputHandler) handleKey(evt *sdl.KeyboardEvent) error {
	step := inputPanStep / h.cam.Zoom
	switch evt.Scancode {
	case sdl.SCANCODE_ESCAPE, sdl.SCANCODE_Q:
		return sdl.EndLoop
	case sdl.SCANCODE_LEFT, sdl.SCANCODE_A:
		h.cam.Pan(-step, 0)
	case sdl.SCANCODE_RIGHT, sdl.SCANCODE_D:
		h.cam.Pan(step, 0)
	case sdl.SCANCODE_UP, sdl.SCANCODE_W:
		h.cam.Pan(0, -step)
	case sdl.SCANCODE_DOWN, sdl.SCANCODE_S:
		h.cam.Pan(0, step)
	case sdl.SCANCODE_EQUALS, sdl.SCANCODE_KP_PLUS:
		h.cam.ZoomIn()
		h.zoomIdleFor = 0
	case sdl.SCANCODE_MINUS, sdl.SCANCODE_KP_MINUS:
		h.cam.ZoomOut()
		h.zoomIdleFor = 0
	}
	h.cam.Clamp(h.galaxyW, h.galaxyH)
	return nil
}

func (h *InputHandler) handleKeyUp(evt *sdl.KeyboardEvent) {
	switch evt.Scancode {
	case sdl.SCANCODE_EQUALS, sdl.SCANCODE_KP_PLUS,
		sdl.SCANCODE_MINUS, sdl.SCANCODE_KP_MINUS:
		// Force the timeout to expire immediately on key release so the spring
		// engages without the 150 ms delay.
		h.zoomIdleFor = zoomInputTimeout
	}
}
