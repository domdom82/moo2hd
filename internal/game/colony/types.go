package colony

import "github.com/domdom82/moo2hd/internal/game/galaxy"

// BuildKind distinguishes what is being built in a colony queue slot.
type BuildKind int

const (
	BuildBuilding BuildKind = iota
	BuildShip
)

// BuildTarget identifies what to build.
type BuildTarget struct {
	Kind     BuildKind
	ConfigID string // config.Building.ID or config.Ship.ID
}

// QueueItem is one slot in the construction queue.
type QueueItem struct {
	Target   BuildTarget
	Progress int  // production points accumulated toward Cost
	Cost     int  // total production points required
	Repeat   bool // re-queue after completion
}

// WorkerAllocation distributes population across the three job roles.
// Farmers + Workers + Scientists must sum to 1.0.
type WorkerAllocation struct {
	Farmers    float64
	Workers    float64
	Scientists float64
}

// Colony is a single settled planet.
type Colony struct {
	ID         int
	SystemID   galaxy.SystemID
	PlanetID   galaxy.PlanetID
	OwnerRace  string
	Population float64
	Workers    WorkerAllocation
	Buildings  []string // config.Building IDs present
	Queue      []QueueItem
	MoraleBase float64
	RallyPoint *galaxy.SystemID // nil = no rally point set
	Tags       []string
}

// ColonyGroup is a named set of colonies sharing a build template or rally point.
type ColonyGroup struct {
	Name          string
	Tags          []string
	QueueTemplate []QueueItem
	RallyPoint    *galaxy.SystemID
}

// Output holds one turn's computed production values for a colony.
type Output struct {
	Food       float64
	Production float64
	Research   float64
	Morale     float64
	PopGrowth  float64
	Waste      float64
}
