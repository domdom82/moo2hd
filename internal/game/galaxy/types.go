package galaxy

// SystemID is a typed index into Galaxy.Systems.
type SystemID int

// PlanetID is a typed index into Galaxy.Planets.
type PlanetID int

// StarType describes the spectral class or special type of a star.
type StarType string

const (
	StarYellow   StarType = "yellow"
	StarBlue     StarType = "blue"
	StarWhite    StarType = "white"
	StarOrange   StarType = "orange"
	StarRed      StarType = "red"
	StarBrown    StarType = "brown"
	StarNeutron  StarType = "neutron"
	StarBlackHole StarType = "black_hole"
)

// SystemSize describes how many planets a system may host.
type SystemSize string

const (
	SizeSmall  SystemSize = "small"
	SizeMedium SystemSize = "medium"
	SizeLarge  SystemSize = "large"
)

// Planet is a single planet body stored in Galaxy.Planets.
type Planet struct {
	ID       PlanetID
	SystemID SystemID
	Slot     int    // 1–5 position within the system
	Class    string // config.PlanetClass value
	Richness string // ultra-poor … ultra-rich
	Gravity  string // low / normal / high
	MaxPop   int
	Size     int // 1–5
}

// Lane is one directed edge in the travel-lane graph.
type Lane struct {
	To       SystemID
	Distance float32
}

// System is a star system in the galaxy.
type System struct {
	ID      SystemID
	Name    string
	X, Y    float32
	Star    StarType
	Size    SystemSize
	Planets []PlanetID
}

// Galaxy is the complete procedurally-generated star map.
type Galaxy struct {
	Seed       uint64
	Systems    []System
	Planets    []Planet
	Adjacency  [][]Lane // indexed by SystemID
	systemByName map[string]SystemID
}

// System returns the system with the given ID.
func (g *Galaxy) System(id SystemID) *System {
	return &g.Systems[id]
}

// Planet returns the planet with the given ID.
func (g *Galaxy) Planet(id PlanetID) *Planet {
	return &g.Planets[id]
}

// PlanetsOf returns all Planet values belonging to the given system.
func (g *Galaxy) PlanetsOf(sid SystemID) []Planet {
	sys := g.Systems[sid]
	out := make([]Planet, len(sys.Planets))
	for i, pid := range sys.Planets {
		out[i] = g.Planets[pid]
	}
	return out
}

// Neighbors returns the outgoing lanes from the given system.
func (g *Galaxy) Neighbors(sid SystemID) []Lane {
	return g.Adjacency[sid]
}

// SystemByName looks up a system by name; returns -1 and false if not found.
func (g *Galaxy) SystemByName(name string) (SystemID, bool) {
	id, ok := g.systemByName[name]
	return id, ok
}
