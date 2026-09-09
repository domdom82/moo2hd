package colony_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/game/colony"
	"github.com/domdom82/moo2hd/internal/game/galaxy"
)

func writeFile(dir, name, content string) {
	GinkgoHelper()
	Expect(os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)).To(Succeed())
}

const testYAML = `
races:
  - id: human
    name: Human
    home_world: terran
    government: democracy
    bonuses:
      food: 0.0
      production: 0.0
      research: 0.0
  - id: klackon
    name: Klackon
    home_world: terran
    government: unification
    bonuses:
      production: 0.5

planets:
  - id: terran
    class: terran
    max_pop: 12
    food_mod: 2.0
    prod_mod: 1.0
    research_mod: 1.0

buildings:
  - id: automated_factory
    name: Automated Factory
    cost: 120
    maintenance: 2
    effects:
      production: 1.5
  - id: hydroponic_farm
    name: Hydroponic Farm
    cost: 60
    maintenance: 1
    effects:
      food: 1.5
  - id: research_lab
    name: Research Lab
    cost: 75
    maintenance: 2
    effects:
      research: 1.5
  - id: housing
    name: Housing
    cost: 100
    maintenance: 1
    effects:
      max_pop: 2

ships:
  - id: colony_ship
    name: Colony Ship
    hull: frigate
    space: 100
    hp: 10
    structure: 10
    armor_class: 0
    shield_class: 0
    speed: 2
    cost: 200
    weapon_slots: 0
    special_slots: 2
`

func buildRegistry() *config.Registry {
	GinkgoHelper()
	dir := GinkgoT().TempDir()
	writeFile(dir, "data.yaml", testYAML)
	reg, err := config.Load(dir, "")
	Expect(err).NotTo(HaveOccurred())
	return reg
}

func terranPlanet(reg *config.Registry) *config.Planet {
	p, err := reg.Planet("terran")
	Expect(err).NotTo(HaveOccurred())
	return p
}

func humanRace(reg *config.Registry) *config.Race {
	r, err := reg.Race("human")
	Expect(err).NotTo(HaveOccurred())
	return r
}

var _ = Describe("Colony", func() {
	var (
		reg    *config.Registry
		planet *config.Planet
		race   *config.Race
		c      *colony.Colony
	)

	BeforeEach(func() {
		reg = buildRegistry()
		planet = terranPlanet(reg)
		race = humanRace(reg)
		c = colony.NewColony(1, galaxy.SystemID(0), galaxy.PlanetID(0), "human")
		c.Population = 6
	})

	Describe("ComputeOutput", func() {
		It("computes food proportional to farmers, FoodMod, and population", func() {
			c.Workers = colony.WorkerAllocation{Farmers: 1.0, Workers: 0.0, Scientists: 0.0}
			out, err := colony.ComputeOutput(c, reg, planet, race)
			Expect(err).NotTo(HaveOccurred())
			// food = pop(6) × farmers(1.0) × foodMod(2.0) × bldgMod(1.0) × raceMod(1.0) = 12
			Expect(out.Food).To(BeNumerically("~", 12.0, 0.01))
		})

		It("computes production proportional to workers", func() {
			c.Workers = colony.WorkerAllocation{Farmers: 0.0, Workers: 1.0, Scientists: 0.0}
			out, err := colony.ComputeOutput(c, reg, planet, race)
			Expect(err).NotTo(HaveOccurred())
			// prod = pop(6) × workers(1.0) × prodMod(1.0) × richness(1.0) × bldgMod(1.0) × race(1.0) = 6
			Expect(out.Production).To(BeNumerically("~", 6.0, 0.01))
		})

		It("applies building production bonus", func() {
			c.Buildings = []string{"automated_factory"}
			c.Workers = colony.WorkerAllocation{Farmers: 0.0, Workers: 1.0, Scientists: 0.0}
			out, err := colony.ComputeOutput(c, reg, planet, race)
			Expect(err).NotTo(HaveOccurred())
			// prod × 1.5 = 9
			Expect(out.Production).To(BeNumerically("~", 9.0, 0.01))
		})

		It("applies building food bonus", func() {
			c.Buildings = []string{"hydroponic_farm"}
			c.Workers = colony.WorkerAllocation{Farmers: 1.0, Workers: 0.0, Scientists: 0.0}
			out, err := colony.ComputeOutput(c, reg, planet, race)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Food).To(BeNumerically("~", 18.0, 0.01)) // 12 × 1.5
		})

		It("computes research proportional to scientists", func() {
			c.Workers = colony.WorkerAllocation{Farmers: 0.0, Workers: 0.0, Scientists: 1.0}
			out, err := colony.ComputeOutput(c, reg, planet, race)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Research).To(BeNumerically("~", 6.0, 0.01)) // pop×1.0×1.0×1.0
		})

		It("applies Klackon production bonus", func() {
			klackon, err := reg.Race("klackon")
			Expect(err).NotTo(HaveOccurred())
			c.Workers = colony.WorkerAllocation{Farmers: 0.0, Workers: 1.0, Scientists: 0.0}
			out, err := colony.ComputeOutput(c, reg, planet, klackon)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.Production).To(BeNumerically("~", 9.0, 0.01)) // 6 × 1.5
		})

		It("pop growth caps at MaxPop", func() {
			c.Population = 12 // already at max
			c.Workers = colony.WorkerAllocation{Farmers: 1.0, Workers: 0.0, Scientists: 0.0}
			out, err := colony.ComputeOutput(c, reg, planet, race)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.PopGrowth).To(BeNumerically("~", 0.0, 0.01))
		})

		It("food deficit produces negative pop growth", func() {
			c.Workers = colony.WorkerAllocation{Farmers: 0.0, Workers: 1.0, Scientists: 0.0}
			out, err := colony.ComputeOutput(c, reg, planet, race)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.PopGrowth).To(BeNumerically("<", 0)) // no food at all
		})

		It("Housing building increases max_pop", func() {
			c.Buildings = []string{"housing"}
			c.Workers = colony.WorkerAllocation{Farmers: 1.0, Workers: 0.0, Scientists: 0.0}
			// Now cap is 14 instead of 12. Population of 6 can still grow.
			c.Population = 12 // at original cap, but housing raises it
			out, err := colony.ComputeOutput(c, reg, planet, race)
			Expect(err).NotTo(HaveOccurred())
			Expect(out.PopGrowth).To(BeNumerically(">", 0))
		})
	})

	Describe("Queue", func() {
		It("AdvanceQueue completes an item when progress reaches cost", func() {
			item := colony.QueueItem{
				Target: colony.BuildTarget{Kind: colony.BuildBuilding, ConfigID: "automated_factory"},
				Cost:   120,
			}
			c.Queue = []colony.QueueItem{item}
			target, done, err := colony.AdvanceQueue(c, 120, reg)
			Expect(err).NotTo(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(target.ConfigID).To(Equal("automated_factory"))
			Expect(colony.HasBuilding(c, "automated_factory")).To(BeTrue())
			Expect(len(c.Queue)).To(Equal(0))
		})

		It("AdvanceQueue does not complete early", func() {
			item := colony.QueueItem{
				Target: colony.BuildTarget{Kind: colony.BuildBuilding, ConfigID: "research_lab"},
				Cost:   75,
			}
			c.Queue = []colony.QueueItem{item}
			_, done, _ := colony.AdvanceQueue(c, 50, reg)
			Expect(done).To(BeFalse())
			Expect(colony.HasBuilding(c, "research_lab")).To(BeFalse())
		})

		It("Repeat re-queues the item after completion", func() {
			item := colony.QueueItem{
				Target: colony.BuildTarget{Kind: colony.BuildShip, ConfigID: "colony_ship"},
				Cost:   200,
				Repeat: true,
			}
			c.Queue = []colony.QueueItem{item}
			_, done, err := colony.AdvanceQueue(c, 200, reg)
			Expect(err).NotTo(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(len(c.Queue)).To(Equal(1)) // re-queued
			Expect(c.Queue[0].Progress).To(Equal(0))
		})

		It("AddToQueue fills in Cost from config when not specified", func() {
			item := colony.QueueItem{
				Target: colony.BuildTarget{Kind: colony.BuildBuilding, ConfigID: "housing"},
			}
			Expect(colony.AddToQueue(c, item, reg)).To(Succeed())
			Expect(c.Queue[0].Cost).To(Equal(100))
		})

		It("AddToQueue rejects already-built buildings", func() {
			c.Buildings = []string{"housing"}
			item := colony.QueueItem{Target: colony.BuildTarget{Kind: colony.BuildBuilding, ConfigID: "housing"}}
			Expect(colony.AddToQueue(c, item, reg)).To(HaveOccurred())
		})

		It("RemoveFromQueue removes by index", func() {
			item := colony.QueueItem{
				Target: colony.BuildTarget{Kind: colony.BuildBuilding, ConfigID: "research_lab"},
				Cost:   75,
			}
			c.Queue = []colony.QueueItem{item}
			Expect(colony.RemoveFromQueue(c, 0)).To(Succeed())
			Expect(len(c.Queue)).To(Equal(0))
		})
	})

	Describe("Group", func() {
		It("ApplyGroupTemplate overwrites queue and rally point", func() {
			sid := galaxy.SystemID(7)
			group := &colony.ColonyGroup{
				QueueTemplate: []colony.QueueItem{
					{Target: colony.BuildTarget{Kind: colony.BuildBuilding, ConfigID: "housing"}, Cost: 100},
				},
				RallyPoint: &sid,
			}
			colony.ApplyGroupTemplate(c, group)
			Expect(len(c.Queue)).To(Equal(1))
			Expect(c.Queue[0].Target.ConfigID).To(Equal("housing"))
			Expect(c.RallyPoint).NotTo(BeNil())
			Expect(*c.RallyPoint).To(Equal(sid))
		})

		It("Manager.ApplyGroupToColonies propagates template to tagged colonies", func() {
			m := colony.NewManager()
			c1 := colony.NewColony(0, 0, 0, "human")
			c2 := colony.NewColony(0, 0, 0, "human")
			m.AddColony(c1)
			m.AddColony(c2)
			m.TagColony(c1.ID, "frontier")
			m.TagColony(c2.ID, "core")

			g := m.CreateGroup("frontier-group", []string{"frontier"})
			sid := galaxy.SystemID(5)
			g.RallyPoint = &sid
			g.QueueTemplate = []colony.QueueItem{
				{Target: colony.BuildTarget{Kind: colony.BuildBuilding, ConfigID: "housing"}, Cost: 100},
			}
			m.ApplyGroupToColonies(g)

			Expect(len(c1.Queue)).To(Equal(1))
			Expect(len(c2.Queue)).To(Equal(0)) // not tagged "frontier"
			Expect(c1.RallyPoint).NotTo(BeNil())
			Expect(c2.RallyPoint).To(BeNil())
		})
	})
})
