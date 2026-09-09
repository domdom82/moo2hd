package ship

import (
	"fmt"

	"github.com/domdom82/moo2hd/internal/config"
)

// DesignBook is the collection of ship designs owned by a single player.
type DesignBook struct {
	nextID  int
	designs []*Design
	reg     *config.Registry
}

// NewDesignBook creates an empty DesignBook backed by the given config registry.
func NewDesignBook(reg *config.Registry) *DesignBook {
	return &DesignBook{nextID: 1, reg: reg}
}

// NewDesign creates an unsaved Design skeleton for the given hull. It allocates
// weapon and special slots according to the hull template.
func (b *DesignBook) NewDesign(playerID int, name, hullID string) (*Design, error) {
	hull, err := b.reg.Ship(hullID)
	if err != nil {
		return nil, fmt.Errorf("ship: hull %q not found", hullID)
	}
	slots := make([]Slot, 0, hull.WeaponSlots+hull.SpecialSlots)
	for i := 0; i < hull.WeaponSlots; i++ {
		slots = append(slots, Slot{Kind: SlotWeapon})
	}
	for i := 0; i < hull.SpecialSlots; i++ {
		slots = append(slots, Slot{Kind: SlotSpecial})
	}
	return &Design{
		PlayerID: playerID,
		Name:     name,
		HullID:   hullID,
		Slots:    slots,
	}, nil
}

// Install places a component into the slot at slotIdx.
// Returns an error if the slot index is out of range, the component is not
// found, or the component's SlotKind does not match the slot.
func (b *DesignBook) Install(d *Design, slotIdx int, compID string) error {
	if slotIdx < 0 || slotIdx >= len(d.Slots) {
		return fmt.Errorf("ship: slot index %d out of range", slotIdx)
	}
	comp, err := b.reg.Component(compID)
	if err != nil {
		return fmt.Errorf("ship: component %q not found", compID)
	}
	want := slotKindFor(comp.SlotKind)
	if d.Slots[slotIdx].Kind != want {
		return fmt.Errorf("ship: component %q requires %s slot, got %s",
			compID, comp.SlotKind, slotName(d.Slots[slotIdx].Kind))
	}
	d.Slots[slotIdx].ComponentID = compID
	return nil
}

// Remove clears the component in the slot at slotIdx.
func (b *DesignBook) Remove(d *Design, slotIdx int) error {
	if slotIdx < 0 || slotIdx >= len(d.Slots) {
		return fmt.Errorf("ship: slot index %d out of range", slotIdx)
	}
	d.Slots[slotIdx].ComponentID = ""
	return nil
}

// Validate checks a design's space budget, power budget, slot constraints,
// and tech prerequisites. researched is the set of tech IDs the player has
// completed. Returns nil if the design is valid.
func (b *DesignBook) Validate(d *Design, researched map[string]struct{}) *ValidationError {
	hull, err := b.reg.Ship(d.HullID)
	if err != nil {
		return &ValidationError{Errors: []string{fmt.Sprintf("hull %q not found", d.HullID)}}
	}
	var errs []string
	totalSpace := 0
	totalDraw := 0
	totalGen := 0

	for i, slot := range d.Slots {
		if slot.ComponentID == "" {
			continue
		}
		comp, err := b.reg.Component(slot.ComponentID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("slot %d: component %q not found", i, slot.ComponentID))
			continue
		}
		// slot kind match
		want := slotKindFor(comp.SlotKind)
		if slot.Kind != want {
			errs = append(errs, fmt.Sprintf("slot %d: component %q requires %s slot", i, comp.ID, comp.SlotKind))
		}
		// tech prereq
		if comp.TechPrereq != "" {
			if _, ok := researched[comp.TechPrereq]; !ok {
				errs = append(errs, fmt.Sprintf("component %q requires tech %q", comp.ID, comp.TechPrereq))
			}
		}
		totalSpace += comp.SpaceCost
		totalDraw += comp.PowerDraw
		totalGen += comp.PowerGen
	}

	if totalSpace > hull.Space {
		errs = append(errs, fmt.Sprintf("space budget exceeded: used %d / %d", totalSpace, hull.Space))
	}
	if totalGen < totalDraw {
		errs = append(errs, fmt.Sprintf("power deficit: generates %d, draws %d", totalGen, totalDraw))
	}

	d.SpaceUsed = totalSpace
	d.PowerNet = totalGen - totalDraw

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// Save validates and stores a design in the book, assigning a unique ID.
// Returns an error if validation fails.
func (b *DesignBook) Save(d *Design, researched map[string]struct{}) error {
	if verr := b.Validate(d, researched); verr != nil {
		return verr
	}
	d.ID = b.nextID
	b.nextID++
	b.designs = append(b.designs, d)
	return nil
}

// Delete removes a saved design by ID. Returns an error if not found.
func (b *DesignBook) Delete(id int) error {
	for i, d := range b.designs {
		if d.ID == id {
			b.designs = append(b.designs[:i], b.designs[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("ship: design %d not found", id)
}

// All returns all saved designs.
func (b *DesignBook) All() []*Design {
	out := make([]*Design, len(b.designs))
	copy(out, b.designs)
	return out
}

// ByHull returns all saved designs using the given hull ID.
func (b *DesignBook) ByHull(hullID string) []*Design {
	var out []*Design
	for _, d := range b.designs {
		if d.HullID == hullID {
			out = append(out, d)
		}
	}
	return out
}

// ── helpers ───────────────────────────────────────────────────────────────────

func slotKindFor(sk config.SlotKind) SlotKind {
	if sk == config.SlotWeapon {
		return SlotWeapon
	}
	return SlotSpecial
}

func slotName(sk SlotKind) string {
	if sk == SlotWeapon {
		return "weapon"
	}
	return "special"
}
