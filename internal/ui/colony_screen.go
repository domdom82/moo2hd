package ui

import (
	"fmt"

	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/domdom82/moo2hd/internal/game/colony"
	"github.com/domdom82/moo2hd/internal/game/galaxy"
)

// Colony screen layout constants (1080p reference).
const (
	csMargin    float32 = 40.0
	csLineH     float32 = 22.0
	csSectionGap float32 = 12.0
	csPanelAlpha uint8   = 220
)

// ColonyScreen is a full-screen overlay showing colony details and management.
type ColonyScreen struct {
	col    *colony.Colony
	planet *galaxy.Planet
	sys    *galaxy.System
	font   *FontManager

	// OnClose is called when the player dismisses the screen (Escape key).
	OnClose func()
}

// NewColonyScreen creates a colony screen for the given colony.
func NewColonyScreen(c *colony.Colony, p *galaxy.Planet, sys *galaxy.System, font *FontManager) *ColonyScreen {
	return &ColonyScreen{col: c, planet: p, sys: sys, font: font}
}

// Handle processes one SDL event. Returns sdl.EndLoop only when the
// application should quit; Escape closes the screen via OnClose.
func (cs *ColonyScreen) Handle(event *sdl.Event) error {
	if event.Type == sdl.EVENT_QUIT {
		return sdl.EndLoop
	}
	if event.Type == sdl.EVENT_KEY_DOWN {
		evt := event.KeyboardEvent()
		if evt.Scancode == sdl.SCANCODE_ESCAPE {
			if cs.OnClose != nil {
				cs.OnClose()
			}
		}
	}
	return nil
}

// Draw renders the colony management panel. The caller is responsible for
// calling r.Present() after this returns.
func (cs *ColonyScreen) Draw(r *sdl.Renderer) error {
	w, h, err := r.RenderOutputSize()
	if err != nil {
		return err
	}
	fw := float32(w)
	fh := float32(h)

	// Semi-transparent dark panel covering the full screen.
	r.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
	r.SetDrawColor(10, 12, 20, csPanelAlpha)
	r.RenderFillRect(&sdl.FRect{X: 0, Y: 0, W: fw, H: fh})
	r.SetDrawBlendMode(sdl.BLENDMODE_NONE)

	white := sdl.Color{R: 220, G: 220, B: 220, A: 255}
	gold := sdl.Color{R: 255, G: 210, B: 100, A: 255}
	grey := sdl.Color{R: 140, G: 140, B: 160, A: 255}
	green := sdl.Color{R: 100, G: 220, B: 100, A: 255}
	cyan := sdl.Color{R: 80, G: 200, B: 220, A: 255}

	x := csMargin
	y := csMargin

	// ── Header ───────────────────────────────────────────────────────────────
	title := fmt.Sprintf("%s  —  Slot %d", cs.sys.Name, cs.planet.Slot)
	cs.font.DrawText(title, x, y, gold)
	y += csLineH + csSectionGap

	planetInfo := fmt.Sprintf(
		"Class: %-10s  Richness: %-10s  Gravity: %-6s  Max Pop: %d",
		cs.planet.Class, cs.planet.Richness, cs.planet.Gravity, cs.planet.MaxPop,
	)
	cs.font.DrawText(planetInfo, x, y, white)
	y += csLineH
	cs.font.DrawText(fmt.Sprintf("Owner: %s", cs.col.OwnerRace), x, y, grey)
	y += csLineH + csSectionGap

	// ── Population & workers ─────────────────────────────────────────────────
	cs.font.DrawText(
		fmt.Sprintf("Population: %.1f   Morale: %.0f%%", cs.col.Population, cs.col.MoraleBase*100),
		x, y, white,
	)
	y += csLineH
	w60 := cs.col.Workers
	cs.font.DrawText(
		fmt.Sprintf(
			"Farmers: %.0f%%   Workers: %.0f%%   Scientists: %.0f%%",
			w60.Farmers*100, w60.Workers*100, w60.Scientists*100,
		),
		x, y, grey,
	)
	y += csLineH + csSectionGap

	// ── Estimated output (no config registry available here — show raw factors) ─
	cs.font.DrawText("Estimated Output (base rates, no building bonuses):", x, y, gold)
	y += csLineH
	pop := cs.col.Population
	foodEst := pop * cs.col.Workers.Farmers
	prodEst := pop * cs.col.Workers.Workers
	resEst := pop * cs.col.Workers.Scientists
	cs.font.DrawText(fmt.Sprintf("  Food: %.1f   Production: %.1f   Research: %.1f", foodEst, prodEst, resEst), x, y, green)
	y += csLineH + csSectionGap

	// ── Buildings ────────────────────────────────────────────────────────────
	cs.font.DrawText("Buildings:", x, y, gold)
	y += csLineH
	if len(cs.col.Buildings) == 0 {
		cs.font.DrawText("  (none)", x, y, grey)
		y += csLineH
	} else {
		for _, bid := range cs.col.Buildings {
			cs.font.DrawText("  "+bid, x, y, white)
			y += csLineH
		}
	}
	y += csSectionGap

	// ── Build queue ──────────────────────────────────────────────────────────
	cs.font.DrawText("Build Queue:", x, y, gold)
	y += csLineH
	if len(cs.col.Queue) == 0 {
		cs.font.DrawText("  (empty)", x, y, grey)
		y += csLineH
	} else {
		for i, item := range cs.col.Queue {
			pct := 0.0
			if item.Cost > 0 {
				pct = float64(item.Progress) / float64(item.Cost) * 100
			}
			label := item.Target.ConfigID
			if item.Target.Kind == colony.BuildShip {
				label = "[Ship] " + label
			}
			if item.Repeat {
				label += " (repeat)"
			}
			cs.font.DrawText(fmt.Sprintf("  %d. %-28s %3.0f%%", i+1, label, pct), x, y, cyan)
			y += csLineH

			// Progress bar.
			barX := x + 16
			barW := float32(300)
			barH := float32(6)
			r.SetDrawColor(40, 40, 60, 255)
			r.RenderFillRect(&sdl.FRect{X: barX, Y: y, W: barW, H: barH})
			if item.Cost > 0 {
				filled := barW * float32(item.Progress) / float32(item.Cost)
				r.SetDrawColor(80, 180, 100, 255)
				r.RenderFillRect(&sdl.FRect{X: barX, Y: y, W: filled, H: barH})
			}
			y += barH + 4
		}
	}

	// ── Footer ───────────────────────────────────────────────────────────────
	cs.font.DrawText("[Esc] Close", csMargin, fh-csMargin, grey)

	return nil
}
