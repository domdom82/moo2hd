package colony

import (
	"errors"
	"fmt"
	"math"

	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/game/galaxy"
)

var (
	errNoSuchBuilding = errors.New("colony: building not found in config")
	errNoSuchShip     = errors.New("colony: ship hull not found in config")
)

// NewColony creates a colony with default worker allocation (equal thirds).
func NewColony(id int, sid galaxy.SystemID, pid galaxy.PlanetID, ownerRace string) *Colony {
	return &Colony{
		ID:        id,
		SystemID:  sid,
		PlanetID:  pid,
		OwnerRace: ownerRace,
		Workers:   WorkerAllocation{Farmers: 1.0 / 3, Workers: 1.0 / 3, Scientists: 1.0 / 3},
	}
}

// richnessMod returns the production multiplier for the given richness string.
func richnessMod(richness string) float64 {
	switch richness {
	case "ultra-poor":
		return 0.5
	case "poor":
		return 0.75
	case "rich":
		return 1.5
	case "ultra-rich":
		return 2.0
	default: // abundant
		return 1.0
	}
}

// buildingMod accumulates multipliers from all buildings in the colony.
func buildingMod(c *Colony, reg *config.Registry, field string) float64 {
	total := 1.0
	for _, bid := range c.Buildings {
		b, err := reg.Building(bid)
		if err != nil {
			continue
		}
		switch field {
		case "food":
			if b.Effects.Food != 0 {
				total += b.Effects.Food - 1
			}
		case "production":
			if b.Effects.Production != 0 {
				total += b.Effects.Production - 1
			}
		case "research":
			if b.Effects.Research != 0 {
				total += b.Effects.Research - 1
			}
		case "morale":
			if b.Effects.Morale != 0 {
				total += b.Effects.Morale - 1
			}
		}
	}
	return total
}

// maxPopForColony returns the max population cap, accounting for buildings.
func maxPopForColony(c *Colony, reg *config.Registry, planet *config.Planet) int {
	base := planet.MaxPop
	for _, bid := range c.Buildings {
		b, err := reg.Building(bid)
		if err != nil {
			continue
		}
		base += b.Effects.MaxPop
	}
	return base
}

// ComputeOutput calculates one turn's output values for a colony.
func ComputeOutput(c *Colony, reg *config.Registry, planet *config.Planet, race *config.Race) (Output, error) {
	pop := c.Population
	if pop <= 0 {
		return Output{}, nil
	}

	fMod := buildingMod(c, reg, "food")
	pMod := buildingMod(c, reg, "production")
	rMod := buildingMod(c, reg, "research")
	mMod := buildingMod(c, reg, "morale")

	raceFoodMod := 1.0 + race.Bonuses.Food
	raceProdMod := 1.0 + race.Bonuses.Production
	raceResMod := 1.0 + race.Bonuses.Research

	food := pop * c.Workers.Farmers * planet.FoodMod * fMod * raceFoodMod
	prod := pop * c.Workers.Workers * planet.ProdMod * richnessMod(planet.Richness) * pMod * raceProdMod
	research := pop * c.Workers.Scientists * rMod * raceResMod

	morale := c.MoraleBase * mMod * (1 + race.Bonuses.Morale)

	maxPop := float64(maxPopForColony(c, reg, planet))
	foodNeeded := pop
	foodSurplus := food - foodNeeded

	// Logistic growth: approaches maxPop, slows near capacity.
	growthRate := 0.1
	if foodSurplus < 0 {
		growthRate = foodSurplus / pop // negative when starving
	}
	popGrowth := pop * growthRate * (1 - pop/maxPop)
	if pop+popGrowth > maxPop {
		popGrowth = maxPop - pop
	}

	// Industrial waste increases with production at low ecology tech (simplified).
	waste := prod * 0.05

	return Output{
		Food:       math.Max(food, 0),
		Production: math.Max(prod, 0),
		Research:   math.Max(research, 0),
		Morale:     morale,
		PopGrowth:  popGrowth,
		Waste:      waste,
	}, nil
}

// AdvanceQueue credits production toward the front of the queue.
// If the item completes it is removed (or re-queued if Repeat is true) and the
// completed BuildTarget plus true is returned.
func AdvanceQueue(c *Colony, production float64, reg *config.Registry) (BuildTarget, bool, error) {
	if len(c.Queue) == 0 {
		return BuildTarget{}, false, nil
	}
	item := &c.Queue[0]
	item.Progress += int(production)
	if item.Progress < item.Cost {
		return BuildTarget{}, false, nil
	}
	completed := item.Target
	if completed.Kind == BuildBuilding {
		c.Buildings = append(c.Buildings, completed.ConfigID)
	}
	if item.Repeat {
		item.Progress = 0
		c.Queue = append(c.Queue[1:], *item)
	} else {
		c.Queue = c.Queue[1:]
	}
	return completed, true, nil
}

// HasBuilding reports whether the colony has the named building.
func HasBuilding(c *Colony, id string) bool {
	for _, b := range c.Buildings {
		if b == id {
			return true
		}
	}
	return false
}

// CanBuild reports whether the colony can add the given target to its queue.
// It checks that the config entity exists and, for buildings, that it is not
// already built.
func CanBuild(c *Colony, target BuildTarget, reg *config.Registry) bool {
	switch target.Kind {
	case BuildBuilding:
		_, err := reg.Building(target.ConfigID)
		if err != nil {
			return false
		}
		return !HasBuilding(c, target.ConfigID)
	case BuildShip:
		_, err := reg.Ship(target.ConfigID)
		return err == nil
	}
	return false
}

// AddToQueue appends an item to the colony's build queue after validating it.
func AddToQueue(c *Colony, item QueueItem, reg *config.Registry) error {
	if !CanBuild(c, item.Target, reg) {
		return fmt.Errorf("colony: cannot build %q", item.Target.ConfigID)
	}
	if item.Cost <= 0 {
		switch item.Target.Kind {
		case BuildBuilding:
			b, err := reg.Building(item.Target.ConfigID)
			if err != nil {
				return errNoSuchBuilding
			}
			item.Cost = b.Cost
		case BuildShip:
			s, err := reg.Ship(item.Target.ConfigID)
			if err != nil {
				return errNoSuchShip
			}
			item.Cost = s.Cost
		}
	}
	c.Queue = append(c.Queue, item)
	return nil
}

// RemoveFromQueue removes the queue item at index i.
func RemoveFromQueue(c *Colony, i int) error {
	if i < 0 || i >= len(c.Queue) {
		return fmt.Errorf("colony: queue index %d out of range", i)
	}
	c.Queue = append(c.Queue[:i], c.Queue[i+1:]...)
	return nil
}

// ApplyGroupTemplate copies the group's queue template and rally point onto
// the colony. Existing queue and rally point are overwritten.
func ApplyGroupTemplate(c *Colony, group *ColonyGroup) {
	if group.QueueTemplate != nil {
		c.Queue = make([]QueueItem, len(group.QueueTemplate))
		copy(c.Queue, group.QueueTemplate)
	}
	if group.RallyPoint != nil {
		sid := *group.RallyPoint
		c.RallyPoint = &sid
	}
}
