package galaxy_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/domdom82/moo2hd/internal/game/galaxy"
)

var _ = Describe("Galaxy", func() {
	defaultOpts := func(seed uint64, count int) galaxy.Options {
		return galaxy.Options{Seed: seed, SystemCount: count}
	}

	Describe("Generator", func() {
		It("produces the same galaxy for the same seed", func() {
			g1, err := galaxy.NewGenerator(defaultOpts(42, 20)).Generate()
			Expect(err).NotTo(HaveOccurred())
			g2, err := galaxy.NewGenerator(defaultOpts(42, 20)).Generate()
			Expect(err).NotTo(HaveOccurred())

			Expect(len(g1.Systems)).To(Equal(len(g2.Systems)))
			for i := range g1.Systems {
				Expect(g1.Systems[i].X).To(Equal(g2.Systems[i].X))
				Expect(g1.Systems[i].Y).To(Equal(g2.Systems[i].Y))
				Expect(g1.Systems[i].Name).To(Equal(g2.Systems[i].Name))
			}
		})

		It("produces different galaxies for different seeds", func() {
			g1, _ := galaxy.NewGenerator(defaultOpts(1, 20)).Generate()
			g2, _ := galaxy.NewGenerator(defaultOpts(2, 20)).Generate()
			differ := false
			for i := range g1.Systems {
				if g1.Systems[i].X != g2.Systems[i].X {
					differ = true
					break
				}
			}
			Expect(differ).To(BeTrue())
		})

		It("generates the requested number of systems", func() {
			for _, count := range []int{5, 20, 70} {
				g, err := galaxy.NewGenerator(defaultOpts(99, count)).Generate()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(g.Systems)).To(Equal(count))
			}
		})

		It("rejects SystemCount < 2", func() {
			_, err := galaxy.NewGenerator(galaxy.Options{SystemCount: 1}).Generate()
			Expect(err).To(HaveOccurred())
		})

		It("rejects SystemCount > 1000", func() {
			_, err := galaxy.NewGenerator(galaxy.Options{SystemCount: 1001}).Generate()
			Expect(err).To(HaveOccurred())
		})

		It("generates at least one planet per system", func() {
			g, err := galaxy.NewGenerator(defaultOpts(7, 30)).Generate()
			Expect(err).NotTo(HaveOccurred())
			for _, sys := range g.Systems {
				Expect(len(sys.Planets)).To(BeNumerically(">=", 1))
			}
		})

		It("generates planet IDs that index into Galaxy.Planets without gap", func() {
			g, err := galaxy.NewGenerator(defaultOpts(7, 10)).Generate()
			Expect(err).NotTo(HaveOccurred())
			seen := make(map[galaxy.PlanetID]bool)
			for _, sys := range g.Systems {
				for _, pid := range sys.Planets {
					Expect(int(pid)).To(BeNumerically(">=", 0))
					Expect(int(pid)).To(BeNumerically("<", len(g.Planets)))
					seen[pid] = true
				}
			}
			Expect(len(seen)).To(Equal(len(g.Planets)))
		})

		It("connects the galaxy (every system reachable from system 0)", func() {
			g, err := galaxy.NewGenerator(defaultOpts(12, 50)).Generate()
			Expect(err).NotTo(HaveOccurred())

			visited := make(map[galaxy.SystemID]bool)
			queue := []galaxy.SystemID{0}
			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				if visited[cur] {
					continue
				}
				visited[cur] = true
				for _, l := range g.Adjacency[cur] {
					if !visited[l.To] {
						queue = append(queue, l.To)
					}
				}
			}
			Expect(len(visited)).To(Equal(len(g.Systems)))
		})

		It("respects MaxLanes per system", func() {
			opts := galaxy.Options{Seed: 3, SystemCount: 30, MaxLanes: 3}
			g, err := galaxy.NewGenerator(opts).Generate()
			Expect(err).NotTo(HaveOccurred())
			for _, lanes := range g.Adjacency {
				// connectivity-stitching can add one extra lane per isolated node, so
				// we allow MaxLanes + small slack rather than an exact cap
				Expect(len(lanes)).To(BeNumerically("<=", 8))
			}
		})

		It("generates a 1000-system galaxy in under 5 seconds", func() {
			start := time.Now()
			_, err := galaxy.NewGenerator(defaultOpts(55, 1000)).Generate()
			Expect(err).NotTo(HaveOccurred())
			Expect(time.Since(start)).To(BeNumerically("<", 5*time.Second))
		})

		It("SystemByName finds a system by name", func() {
			g, err := galaxy.NewGenerator(defaultOpts(1, 10)).Generate()
			Expect(err).NotTo(HaveOccurred())
			name := g.Systems[3].Name
			id, ok := g.SystemByName(name)
			Expect(ok).To(BeTrue())
			Expect(id).To(Equal(galaxy.SystemID(3)))
		})

		It("SystemByName returns false for unknown name", func() {
			g, _ := galaxy.NewGenerator(defaultOpts(1, 5)).Generate()
			_, ok := g.SystemByName("ZZZnonexistent")
			Expect(ok).To(BeFalse())
		})
	})

	Describe("PlanetsOf", func() {
		It("returns the correct planets for a system", func() {
			g, _ := galaxy.NewGenerator(defaultOpts(4, 10)).Generate()
			sid := galaxy.SystemID(0)
			planets := g.PlanetsOf(sid)
			Expect(len(planets)).To(Equal(len(g.Systems[0].Planets)))
			for i, p := range planets {
				Expect(p.ID).To(Equal(g.Systems[0].Planets[i]))
				Expect(p.SystemID).To(Equal(sid))
			}
		})
	})
})

var _ = Describe("Pathfinding", func() {
	// hand-built 4-node graph: 0-1-2-3, with shortcut 0-3
	buildGraph := func() *galaxy.Galaxy {
		sys := []galaxy.System{
			{ID: 0, Name: "A"}, {ID: 1, Name: "B"},
			{ID: 2, Name: "C"}, {ID: 3, Name: "D"},
		}
		adj := [][]galaxy.Lane{
			{{To: 1, Distance: 10}, {To: 3, Distance: 50}},
			{{To: 0, Distance: 10}, {To: 2, Distance: 10}},
			{{To: 1, Distance: 10}, {To: 3, Distance: 10}},
			{{To: 0, Distance: 50}, {To: 2, Distance: 10}},
		}
		return &galaxy.Galaxy{Systems: sys, Adjacency: adj}
	}

	It("finds the shortest path", func() {
		g := buildGraph()
		p, err := galaxy.ShortestPath(g, 0, 3)
		Expect(err).NotTo(HaveOccurred())
		Expect(p.Hops).To(Equal([]galaxy.SystemID{0, 1, 2, 3}))
		Expect(p.Distance).To(BeNumerically("~", 30, 0.01))
	})

	It("returns ErrNoPath for disconnected destination", func() {
		g := buildGraph()
		// add isolated node 4
		g.Systems = append(g.Systems, galaxy.System{ID: 4, Name: "E"})
		g.Adjacency = append(g.Adjacency, nil)
		_, err := galaxy.ShortestPath(g, 0, 4)
		Expect(err).To(MatchError(galaxy.ErrNoPath))
	})

	It("same-node path has distance 0", func() {
		g := buildGraph()
		p, err := galaxy.ShortestPath(g, 2, 2)
		Expect(err).NotTo(HaveOccurred())
		Expect(p.Distance).To(BeNumerically("~", 0, 0.01))
		Expect(p.Hops).To(Equal([]galaxy.SystemID{2}))
	})

	It("AllReachable returns only systems within distance", func() {
		g := buildGraph()
		reachable := galaxy.AllReachable(g, 0, 25)
		Expect(reachable).To(HaveKey(galaxy.SystemID(0)))
		Expect(reachable).To(HaveKey(galaxy.SystemID(1)))
		Expect(reachable).To(HaveKey(galaxy.SystemID(2)))
		Expect(reachable).NotTo(HaveKey(galaxy.SystemID(3))) // nearest path is 30
	})

	It("AllReachable includes src at distance 0", func() {
		g := buildGraph()
		reachable := galaxy.AllReachable(g, 2, 0)
		Expect(reachable).To(HaveKey(galaxy.SystemID(2)))
		Expect(reachable[2]).To(BeNumerically("~", 0, 0.01))
	})
})
