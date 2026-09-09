package galaxy

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// Options controls how a galaxy is generated.
type Options struct {
	Seed        uint64
	SystemCount int     // 5–1000; default 70
	Width       float32 // map extent in arbitrary units; default 1000
	Height      float32
	MinLanes    int     // minimum lanes per system; default 1
	MaxLanes    int     // maximum lanes per system; default 4
	MinDistance float32 // minimum separation between systems; default 40
}

func (o *Options) setDefaults() {
	if o.SystemCount == 0 {
		o.SystemCount = 70
	}
	if o.Width == 0 {
		o.Width = 1000
	}
	if o.Height == 0 {
		o.Height = 1000
	}
	if o.MinLanes == 0 {
		o.MinLanes = 1
	}
	if o.MaxLanes == 0 {
		o.MaxLanes = 4
	}
	if o.MinDistance == 0 {
		o.MinDistance = 40
	}
}

// Generator produces Galaxy values deterministically from a seed.
type Generator struct {
	opts Options
	rng  *rand.Rand
}

// NewGenerator creates a Generator from the given options.
func NewGenerator(opts Options) *Generator {
	opts.setDefaults()
	rng := rand.New(rand.NewPCG(opts.Seed, opts.Seed^0xdeadbeef))
	return &Generator{opts: opts, rng: rng}
}

// Generate creates a new Galaxy. Calling Generate twice with the same seed
// yields identical results.
func (g *Generator) Generate() (*Galaxy, error) {
	n := g.opts.SystemCount
	if n < 2 {
		return nil, fmt.Errorf("galaxy: SystemCount must be >= 2, got %d", n)
	}
	if n > 1000 {
		return nil, fmt.Errorf("galaxy: SystemCount must be <= 1000, got %d", n)
	}

	systems := g.placeSystems(n)
	planets := g.generatePlanets(systems)
	adj := g.buildLanes(systems)

	nameMap := make(map[string]SystemID, len(systems))
	for i, s := range systems {
		nameMap[s.Name] = SystemID(i)
	}

	return &Galaxy{
		Seed:         g.opts.Seed,
		Systems:      systems,
		Planets:      planets,
		Adjacency:    adj,
		systemByName: nameMap,
	}, nil
}

// placeSystems distributes n systems with minimum separation across the map.
func (g *Generator) placeSystems(n int) []System {
	systems := make([]System, 0, n)
	maxAttempts := n * 200

	for len(systems) < n && maxAttempts > 0 {
		maxAttempts--
		x := g.rng.Float32() * g.opts.Width
		y := g.rng.Float32() * g.opts.Height
		if !g.tooClose(x, y, systems) {
			id := SystemID(len(systems))
			systems = append(systems, System{
				ID:   id,
				Name: systemName(int(id)),
				X:    x,
				Y:    y,
				Star: g.randomStar(),
				Size: g.randomSize(),
			})
		}
	}
	return systems
}

func (g *Generator) tooClose(x, y float32, systems []System) bool {
	minD2 := g.opts.MinDistance * g.opts.MinDistance
	for _, s := range systems {
		dx := x - s.X
		dy := y - s.Y
		if dx*dx+dy*dy < minD2 {
			return true
		}
	}
	return false
}

func dist(a, b System) float32 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

// buildLanes connects systems via k-nearest-neighbours, then ensures full
// connectivity via union-find, then prunes excess lanes.
func (g *Generator) buildLanes(systems []System) [][]Lane {
	n := len(systems)
	adj := make([][]Lane, n)

	k := g.opts.MaxLanes
	// k-NN pass
	for i := 0; i < n; i++ {
		type nd struct {
			id   int
			dist float32
		}
		nbrs := make([]nd, 0, n-1)
		for j := 0; j < n; j++ {
			if j == i {
				continue
			}
			nbrs = append(nbrs, nd{j, dist(systems[i], systems[j])})
		}
		// partial sort: keep top k closest
		for pick := 0; pick < k && pick < len(nbrs); pick++ {
			best := pick
			for m := pick + 1; m < len(nbrs); m++ {
				if nbrs[m].dist < nbrs[best].dist {
					best = m
				}
			}
			nbrs[pick], nbrs[best] = nbrs[best], nbrs[pick]
			j := nbrs[pick].id
			d := nbrs[pick].dist
			adj[i] = appendLane(adj[i], SystemID(j), d)
			adj[j] = appendLane(adj[j], SystemID(i), d)
		}
	}

	// union-find connectivity guarantee
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) { parent[find(a)] = find(b) }

	for i, lanes := range adj {
		for _, l := range lanes {
			union(i, int(l.To))
		}
	}

	// connect isolated components by stitching to nearest connected node
	for pass := 0; pass < n; pass++ {
		allSame := true
		root := find(0)
		for i := 1; i < n; i++ {
			if find(i) != root {
				allSame = false
				// find nearest system in the main component
				bestDist := float32(math.MaxFloat32)
				bestJ := -1
				for j := 0; j < n; j++ {
					if find(j) == root {
						d := dist(systems[i], systems[j])
						if d < bestDist {
							bestDist = d
							bestJ = j
						}
					}
				}
				if bestJ >= 0 {
					adj[i] = appendLane(adj[i], SystemID(bestJ), bestDist)
					adj[bestJ] = appendLane(adj[bestJ], SystemID(i), bestDist)
					union(i, bestJ)
				}
			}
		}
		if allSame {
			break
		}
	}

	return adj
}

func appendLane(lanes []Lane, to SystemID, d float32) []Lane {
	for _, l := range lanes {
		if l.To == to {
			return lanes // deduplicate
		}
	}
	return append(lanes, Lane{To: to, Distance: d})
}

// generatePlanets creates planets for all systems and attaches their IDs.
func (g *Generator) generatePlanets(systems []System) []Planet {
	planets := make([]Planet, 0, len(systems)*3)
	for i := range systems {
		slots := g.slotsForSize(systems[i].Size)
		for slot := 1; slot <= slots; slot++ {
			pid := PlanetID(len(planets))
			p := Planet{
				ID:       pid,
				SystemID: SystemID(i),
				Slot:     slot,
				Class:    g.randomPlanetClass(),
				Richness: g.randomRichness(),
				Gravity:  g.randomGravity(),
				MaxPop:   g.rng.IntN(13) + 3,  // 3–15
				Size:     g.rng.IntN(5) + 1,    // 1–5
			}
			planets = append(planets, p)
			systems[i].Planets = append(systems[i].Planets, pid)
		}
	}
	return planets
}

func (g *Generator) slotsForSize(sz SystemSize) int {
	switch sz {
	case SizeSmall:
		return 1 + g.rng.IntN(2) // 1–2
	case SizeLarge:
		return 3 + g.rng.IntN(3) // 3–5
	default:
		return 2 + g.rng.IntN(2) // 2–3
	}
}

func (g *Generator) randomStar() StarType {
	types := []StarType{StarYellow, StarBlue, StarWhite, StarOrange, StarRed, StarBrown, StarNeutron, StarBlackHole}
	weights := []int{30, 10, 15, 20, 15, 5, 3, 2} // rough MOO2-like distribution
	return pickWeighted(g.rng, types, weights)
}

func (g *Generator) randomSize() SystemSize {
	switch g.rng.IntN(3) {
	case 0:
		return SizeSmall
	case 1:
		return SizeLarge
	default:
		return SizeMedium
	}
}

var planetClasses = []string{
	"terran", "ocean", "arid", "desert", "tundra", "swamp",
	"volcanic", "barren", "radiated", "toxic", "inferno", "none",
}

var richnesses = []string{"ultra-poor", "poor", "abundant", "rich", "ultra-rich"}
var gravities = []string{"low", "normal", "high"}

func (g *Generator) randomPlanetClass() string {
	return planetClasses[g.rng.IntN(len(planetClasses))]
}

func (g *Generator) randomRichness() string {
	return richnesses[g.rng.IntN(len(richnesses))]
}

func (g *Generator) randomGravity() string {
	return gravities[g.rng.IntN(len(gravities))]
}

func pickWeighted[T any](rng *rand.Rand, items []T, weights []int) T {
	total := 0
	for _, w := range weights {
		total += w
	}
	r := rng.IntN(total)
	for i, w := range weights {
		r -= w
		if r < 0 {
			return items[i]
		}
	}
	return items[len(items)-1]
}

// systemName returns a deterministic name for a system index.
var systemNames = []string{
	"Sol", "Vega", "Altair", "Deneb", "Rigel", "Sirius", "Capella", "Betelgeuse",
	"Arcturus", "Aldebaran", "Antares", "Spica", "Fomalhaut", "Pollux", "Castor",
	"Regulus", "Procyon", "Achernar", "Hadar", "Acrux", "Mimosa", "Gacrux",
	"Shaula", "Sargas", "Kaus", "Nunki", "Zuben", "Zavijava", "Zaniah", "Porrima",
	"Vindemiatrix", "Minelauva", "Syrma", "Izar", "Nekkar", "Seginus", "Alkalurops",
	"Muphrid", "Thuban", "Rastaban", "Eltanin", "Grumium", "Kuma", "Altais",
	"Gianfar", "Tyl", "Aldhibah", "Alwaid", "Nodus", "Edasich",
}

func systemName(i int) string {
	if i < len(systemNames) {
		return systemNames[i]
	}
	return fmt.Sprintf("System-%d", i)
}
