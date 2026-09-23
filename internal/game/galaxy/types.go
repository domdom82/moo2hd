package galaxy

// GalaxySize is the named size tier for a generated galaxy.
type GalaxySize string

const (
	GalaxySizeSmall   GalaxySize = "small"
	GalaxySizeMedium  GalaxySize = "medium"
	GalaxySizeLarge   GalaxySize = "large"
	GalaxySizeCluster GalaxySize = "cluster"
	GalaxySizeHuge    GalaxySize = "huge"
)

// galaxySizeParams holds the canonical parameters for each galaxy size.
type galaxySizeParams struct {
	StarCount int
	Width     float32
	Height    float32
}

const (
	GalaxyAspectRatio = 1.265 // width / height
	UnitsPerParsec    = 30    // 1 parsec = 30 units in the galaxy coordinate system
)

var sizeParams = map[GalaxySize]galaxySizeParams{
	GalaxySizeSmall:   {StarCount: 280, Width: 7084, Height: 5600},
	GalaxySizeMedium:  {StarCount: 504, Width: 10626, Height: 8400},
	GalaxySizeLarge:   {StarCount: 756, Width: 14168, Height: 11200},
	GalaxySizeCluster: {StarCount: 1000, Width: 14168, Height: 11200},
	GalaxySizeHuge:    {StarCount: 1000, Width: 21252, Height: 16800},
}

// SystemID is a typed index into Galaxy.Systems.
type SystemID int

// PlanetID is a typed index into Galaxy.Planets.
type PlanetID int

// StarType describes the spectral class or special type of a star.
type StarType string

const (
	StarYellow    StarType = "yellow"
	StarBlue      StarType = "blue"
	StarWhite     StarType = "white"
	StarOrange    StarType = "orange"
	StarRed       StarType = "red"
	StarBrown     StarType = "brown"
	StarBlackHole StarType = "black_hole"
)

// SystemSize describes how many planets a system may host.
type SystemSize string

const (
	SizeSmall  SystemSize = "small"
	SizeMedium SystemSize = "medium"
	SizeLarge  SystemSize = "large"
)

// Special describes the optional special property of a star system.
type Special string

const (
	SpecialNone        Special = ""
	SpecialPlanet      Special = "planet"
	SpecialWormhole    Special = "wormhole"
	SpecialShipDebris  Special = "ship_debris"
	SpecialPirateCache Special = "pirate_cache"
	SpecialLostHero    Special = "lost_hero"
)

// NebulaSize is the visual/gameplay size tier of a nebula.
type NebulaSize string

const (
	NebulaSizeSmall  NebulaSize = "small"
	NebulaSizeMedium NebulaSize = "medium"
	NebulaSizeLarge  NebulaSize = "large"
	NebulaSizeHuge   NebulaSize = "huge"
)

// nebulaSizeOrder maps NebulaSize to its 0-based index within a 4-record art
// group (records 6–53 in STARBG.LBX are arranged as groups of 4: large,
// medium, small, tiny/huge — offsets 0..3 within each group).
var nebulaSizeOffset = map[NebulaSize]int{
	NebulaSizeLarge:  0,
	NebulaSizeMedium: 1,
	NebulaSizeSmall:  2,
	NebulaSizeHuge:   3,
}

// NebulaArtCount is the number of distinct nebula art sets in STARBG.LBX.
// Records 6–53 are 12 groups of 4 size variants.
const NebulaArtCount = 12

// Nebula is a cloud region placed in the galaxy.
type Nebula struct {
	// X, Y is the centre of the nebula in galaxy coordinates.
	X, Y float32
	// RadiusX, RadiusY is the half-extents of the nebula ellipse in galaxy
	// coordinates. Systems whose distance falls within these radii are inside.
	RadiusX, RadiusY float32
	// Size is the display/gameplay size tier.
	Size NebulaSize
	// ArtIndex selects which of the 12 nebula art sets to use (0–11).
	ArtIndex int
	// Systems holds the IDs of star systems located inside this nebula.
	Systems []SystemID
}

// LBXRecord returns the STARBG.LBX record index for this nebula's art.
// Records 6–53: group = ArtIndex*4, offset within group = nebulaSizeOffset[Size].
func (n *Nebula) LBXRecord() int {
	return 6 + n.ArtIndex*4 + nebulaSizeOffset[n.Size]
}

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
	ID         SystemID
	Name       string
	X, Y       float32
	Star       StarType
	Size       SystemSize
	Planets    []PlanetID
	Faction    int      // 0-based faction index; -1 = unclaimed
	Special    Special  // optional special property
	WormholeTo SystemID // partner system ID for wormholes; -1 if none
}

// Galaxy is the complete procedurally-generated star map.
type Galaxy struct {
	Seed         uint64
	Systems      []System
	Planets      []Planet
	Nebulas      []Nebula
	Adjacency    [][]Lane // indexed by SystemID
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
