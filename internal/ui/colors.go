package ui

import (
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/domdom82/moo2hd/internal/game/galaxy"
)

// StarColor maps a star type to its visual colour.
func StarColor(t galaxy.StarType) sdl.FColor {
	switch t {
	case galaxy.StarYellow:
		return sdl.FColor{R: 1.0, G: 0.95, B: 0.3, A: 1.0}
	case galaxy.StarBlue:
		return sdl.FColor{R: 0.3, G: 0.5, B: 1.0, A: 1.0}
	case galaxy.StarWhite:
		return sdl.FColor{R: 0.95, G: 0.95, B: 0.95, A: 1.0}
	case galaxy.StarOrange:
		return sdl.FColor{R: 1.0, G: 0.6, B: 0.1, A: 1.0}
	case galaxy.StarRed:
		return sdl.FColor{R: 1.0, G: 0.15, B: 0.1, A: 1.0}
	case galaxy.StarBrown:
		return sdl.FColor{R: 0.55, G: 0.3, B: 0.1, A: 1.0}
	case galaxy.StarNeutron:
		return sdl.FColor{R: 0.8, G: 0.9, B: 1.0, A: 1.0}
	case galaxy.StarBlackHole:
		return sdl.FColor{R: 0.15, G: 0.0, B: 0.25, A: 1.0}
	default:
		return sdl.FColor{R: 0.7, G: 0.7, B: 0.7, A: 1.0}
	}
}

var factionColors = [8]sdl.FColor{
	{R: 0.22, G: 0.48, B: 1.00, A: 0.55},
	{R: 1.00, G: 0.22, B: 0.22, A: 0.55},
	{R: 0.22, G: 0.80, B: 0.22, A: 0.55},
	{R: 0.90, G: 0.75, B: 0.00, A: 0.55},
	{R: 0.80, G: 0.30, B: 1.00, A: 0.55},
	{R: 1.00, G: 0.55, B: 0.00, A: 0.55},
	{R: 0.00, G: 0.85, B: 0.85, A: 0.55},
	{R: 1.00, G: 0.50, B: 0.75, A: 0.55},
}

// FactionColor returns the territory color for a faction ID (0-based).
// Wraps at 8.
func FactionColor(factionID int) sdl.FColor {
	if factionID < 0 {
		return sdl.FColor{R: 0.3, G: 0.3, B: 0.3, A: 0.3} // unclaimed
	}
	return factionColors[factionID%len(factionColors)]
}

// LaneColor returns the color for travel lanes at the given zoom tier.
func LaneColor(tier ZoomTier) sdl.FColor {
	if tier == ZoomClose {
		return sdl.FColor{R: 0.4, G: 0.45, B: 0.65, A: 0.9}
	}
	return sdl.FColor{R: 0.3, G: 0.35, B: 0.5, A: 0.7}
}

// BackgroundColor returns the deep-space background colour.
func BackgroundColor() sdl.FColor {
	return sdl.FColor{R: 0.02, G: 0.02, B: 0.08, A: 1.0}
}

// PlanetColor maps a planet class string to a visual color for the system view.
func PlanetColor(class string) sdl.FColor {
	switch class {
	case "terran":
		return sdl.FColor{R: 0.2, G: 0.6, B: 0.2, A: 1.0}
	case "ocean":
		return sdl.FColor{R: 0.1, G: 0.4, B: 0.9, A: 1.0}
	case "arid":
		return sdl.FColor{R: 0.8, G: 0.6, B: 0.2, A: 1.0}
	case "desert":
		return sdl.FColor{R: 0.9, G: 0.8, B: 0.3, A: 1.0}
	case "tundra":
		return sdl.FColor{R: 0.6, G: 0.7, B: 0.8, A: 1.0}
	case "swamp":
		return sdl.FColor{R: 0.3, G: 0.5, B: 0.2, A: 1.0}
	case "volcanic":
		return sdl.FColor{R: 0.8, G: 0.2, B: 0.0, A: 1.0}
	case "barren":
		return sdl.FColor{R: 0.5, G: 0.5, B: 0.5, A: 1.0}
	case "radiated":
		return sdl.FColor{R: 0.6, G: 0.9, B: 0.3, A: 1.0}
	case "toxic":
		return sdl.FColor{R: 0.5, G: 0.2, B: 0.6, A: 1.0}
	case "inferno":
		return sdl.FColor{R: 1.0, G: 0.3, B: 0.0, A: 1.0}
	default:
		return sdl.FColor{R: 0.4, G: 0.4, B: 0.4, A: 1.0}
	}
}
