package ship

import "strings"

// SlotKind mirrors config.SlotKind but is kept local to avoid coupling
// game logic directly to the config parse layer.
type SlotKind int

const (
	SlotWeapon  SlotKind = iota // weapon hardpoint
	SlotSpecial                 // special equipment bay
)

// Slot is one equipment position on a ship design.
type Slot struct {
	Kind        SlotKind
	ComponentID string // empty = unfilled
}

// Design is a player-defined ship configuration.
type Design struct {
	ID       int
	PlayerID int
	Name     string
	HullID   string
	Slots    []Slot
	// Cached validation totals — recomputed by Validate.
	SpaceUsed int
	PowerNet  int // PowerGen - PowerDraw across all components
}

// ValidationError holds all problems found when validating a design.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return strings.Join(e.Errors, "; ")
}
