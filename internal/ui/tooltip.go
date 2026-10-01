package ui

import (
	"github.com/Zyko0/go-sdl3/sdl"
)

const (
	tooltipPadX    = float32(10)
	tooltipPadY    = float32(8)
	tooltipLineH   = float32(16)
	tooltipOffsetX = float32(14) // cursor offset so the tip doesn't obscure the thing
	tooltipOffsetY = float32(14)
)

// Tooltip is a transient overlay with a rectangular border and translucent
// background. Lines are drawn in order; empty strings produce a blank spacer.
type Tooltip struct {
	Lines []string
}

// Draw renders the tooltip with its top-left corner near (mouseX, mouseY),
// nudged to stay fully on-screen. The tooltip floats at a small offset from
// the cursor so the hovered object stays visible.
func (t *Tooltip) Draw(r *sdl.Renderer, font *FontManager, mouseX, mouseY, screenW, screenH float32) {
	if len(t.Lines) == 0 {
		return
	}

	// Measure the widest line (approximate: 7 px per character with debug font,
	// or close enough for a small tooltip; TTF renders at ~8 px wide per char).
	maxChars := 0
	for _, l := range t.Lines {
		if len(l) > maxChars {
			maxChars = len(l)
		}
	}
	charW := float32(8)
	w := float32(maxChars)*charW + tooltipPadX*2
	h := float32(len(t.Lines))*tooltipLineH + tooltipPadY*2

	// Position with cursor offset, clamped to screen bounds.
	x := mouseX + tooltipOffsetX
	y := mouseY + tooltipOffsetY
	if x+w > screenW {
		x = mouseX - w - tooltipOffsetX
	}
	if y+h > screenH {
		y = mouseY - h - tooltipOffsetY
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	// Fill: dark, 50% alpha.
	r.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
	r.SetDrawColor(10, 10, 20, 128)
	r.RenderFillRect(&sdl.FRect{X: x, Y: y, W: w, H: h})

	// Border: light blue-grey, fully opaque.
	r.SetDrawBlendMode(sdl.BLENDMODE_NONE)
	r.SetDrawColor(160, 180, 220, 255)
	r.RenderRect(&sdl.FRect{X: x, Y: y, W: w, H: h})

	// Text lines.
	white := sdl.Color{R: 220, G: 220, B: 220, A: 255}
	ty := y + tooltipPadY
	for _, line := range t.Lines {
		if line != "" {
			font.DrawText(line, x+tooltipPadX, ty, white)
		}
		ty += tooltipLineH
	}
}
