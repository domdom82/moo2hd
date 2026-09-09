package ui

import "github.com/Zyko0/go-sdl3/sdl"

const inputPanStep = float32(20.0) // screen pixels per arrow-key press

// InputHandler translates SDL events into Camera mutations.
type InputHandler struct {
	cam            *Camera
	galaxyW, galaxyH float32
	dragging       bool
	lastX, lastY   float32
}

// NewInputHandler creates an InputHandler that mutates cam.
// galaxyW and galaxyH are the map extents used for Clamp.
func NewInputHandler(cam *Camera, galaxyW, galaxyH float32) *InputHandler {
	return &InputHandler{cam: cam, galaxyW: galaxyW, galaxyH: galaxyH}
}

// Handle processes one SDL event.
// Returns sdl.EndLoop when the application should quit, nil otherwise.
func (h *InputHandler) Handle(event *sdl.Event) error {
	switch event.Type {
	case sdl.EVENT_QUIT:
		return sdl.EndLoop

	case sdl.EVENT_KEY_DOWN:
		return h.handleKey(event.KeyboardEvent())

	case sdl.EVENT_MOUSE_BUTTON_DOWN:
		evt := event.MouseButtonEvent()
		if evt.Button == uint8(sdl.BUTTON_LEFT) {
			h.dragging = true
			h.lastX, h.lastY = evt.X, evt.Y
		}

	case sdl.EVENT_MOUSE_BUTTON_UP:
		if event.MouseButtonEvent().Button == uint8(sdl.BUTTON_LEFT) {
			h.dragging = false
		}

	case sdl.EVENT_MOUSE_MOTION:
		if h.dragging {
			evt := event.MouseMotionEvent()
			// Negate: dragging right pans camera left (world moves right).
			h.cam.PanPx(-evt.Xrel, -evt.Yrel)
			h.cam.Clamp(h.galaxyW, h.galaxyH)
		}

	case sdl.EVENT_MOUSE_WHEEL:
		evt := event.MouseWheelEvent()
		if evt.Y > 0 {
			h.cam.ZoomIn()
		} else if evt.Y < 0 {
			h.cam.ZoomOut()
		}
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
	case sdl.SCANCODE_MINUS, sdl.SCANCODE_KP_MINUS:
		h.cam.ZoomOut()
	}
	h.cam.Clamp(h.galaxyW, h.galaxyH)
	return nil
}
