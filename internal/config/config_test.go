package config_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/domdom82/moo2hd/internal/config"
)

// writeFile creates a file at path with the given content, creating parent dirs.
func writeFile(path, content string) {
	Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
	Expect(os.WriteFile(path, []byte(content), 0o644)).To(Succeed())
}

var _ = Describe("Config", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "moo2hd-config-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	cfgDir := func() string { return filepath.Join(tmpDir, "configs") }
	modsDir := func() string { return filepath.Join(tmpDir, "mods") }

	Describe("Load", func() {
		Context("base config only", func() {
			It("loads races from configs/races.yaml", func() {
				writeFile(filepath.Join(cfgDir(), "races.yaml"), `
races:
  - id: human
    name: Human
    home_world: terran
  - id: psilon
    name: Psilon
    home_world: terran
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				human, err := reg.Race("human")
				Expect(err).NotTo(HaveOccurred())
				Expect(human.Name).To(Equal("Human"))

				psilon, err := reg.Race("psilon")
				Expect(err).NotTo(HaveOccurred())
				Expect(psilon.Name).To(Equal("Psilon"))
			})

			It("loads ships from configs/ships.yaml including structure, armor_class, and shield_class", func() {
				writeFile(filepath.Join(cfgDir(), "ships.yaml"), `
ships:
  - id: cruiser
    name: Cruiser
    hull: cruiser
    space: 200
    hp: 20
    structure: 60
    armor_class: 2
    shield_class: 3
    speed: 2
    cost: 200
    weapon_slots: 6
    special_slots: 3
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				cruiser, err := reg.Ship("cruiser")
				Expect(err).NotTo(HaveOccurred())
				Expect(cruiser.Hull).To(Equal(config.HullCruiser))
				Expect(cruiser.Space).To(Equal(200))
				Expect(cruiser.HP).To(Equal(20))
				Expect(cruiser.Structure).To(Equal(60))
				Expect(cruiser.ArmorClass).To(Equal(2))
				Expect(cruiser.ShieldClass).To(Equal(3))
			})

			It("loads techs and respects category and prereqs", func() {
				writeFile(filepath.Join(cfgDir(), "techs.yaml"), `
techs:
  - id: laser
    name: Laser Cannon
    category: weapons
    level: 1
    cost: 60
    unlocks:
      - gauss
  - id: gauss
    name: Gauss Cannon
    category: weapons
    level: 2
    cost: 200
    prereqs:
      - laser
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				laser, err := reg.Tech("laser")
				Expect(err).NotTo(HaveOccurred())
				Expect(laser.Category).To(Equal(config.TechWeapons))
				Expect(laser.Unlocks).To(ConsistOf("gauss"))

				gauss, err := reg.Tech("gauss")
				Expect(err).NotTo(HaveOccurred())
				Expect(gauss.Prereqs).To(ConsistOf("laser"))
			})

			It("loads planets", func() {
				writeFile(filepath.Join(cfgDir(), "planets.yaml"), `
planets:
  - id: terran
    class: terran
    max_pop: 14
    food_mod: 1.0
    richness: abundant
    gravity: normal
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				terran, err := reg.Planet("terran")
				Expect(err).NotTo(HaveOccurred())
				Expect(terran.MaxPop).To(Equal(14))
				Expect(terran.Richness).To(Equal("abundant"))
			})

			It("loads leaders", func() {
				writeFile(filepath.Join(cfgDir(), "leaders.yaml"), `
leaders:
  - id: napoleon_iv
    name: Napoleon IV
    type: admiral
    level: 3
    cost: 200
    salary: 15
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				l, err := reg.Leader("napoleon_iv")
				Expect(err).NotTo(HaveOccurred())
				Expect(l.Type).To(Equal(config.LeaderAdmiral))
				Expect(l.Level).To(Equal(3))
			})

			It("loads buildings with effects", func() {
				writeFile(filepath.Join(cfgDir(), "buildings.yaml"), `
buildings:
  - id: farm
    name: Hydroponic Farm
    cost: 60
    maintenance: 2
    effects:
      food: 0.25
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				b, err := reg.Building("farm")
				Expect(err).NotTo(HaveOccurred())
				Expect(b.Effects.Food).To(BeNumerically("~", 0.25, 0.001))
			})
		})

		Context("absent directories", func() {
			It("succeeds when configs/ does not exist", func() {
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())
				Expect(reg).NotTo(BeNil())
			})

			It("succeeds when mods/ does not exist", func() {
				writeFile(filepath.Join(cfgDir(), "races.yaml"), `
races:
  - id: human
    name: Human
    home_world: terran
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())
				_, err = reg.Race("human")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("mod overlay — field override", func() {
			It("mod overrides an existing race field", func() {
				writeFile(filepath.Join(cfgDir(), "races.yaml"), `
races:
  - id: human
    name: Human
    home_world: terran
    bonuses:
      morale: 0.1
`)
				writeFile(filepath.Join(modsDir(), "01-buff-humans", "races.yaml"), `
races:
  - id: human
    name: Human (Buffed)
    home_world: terran
    bonuses:
      morale: 0.5
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				human, err := reg.Race("human")
				Expect(err).NotTo(HaveOccurred())
				Expect(human.Name).To(Equal("Human (Buffed)"))
				Expect(human.Bonuses.Morale).To(BeNumerically("~", 0.5, 0.001))
			})
		})

		Context("mod overlay — new entity addition", func() {
			It("mod adds a race that does not exist in base", func() {
				writeFile(filepath.Join(cfgDir(), "races.yaml"), `
races:
  - id: human
    name: Human
    home_world: terran
`)
				writeFile(filepath.Join(modsDir(), "01-new-race", "races.yaml"), `
races:
  - id: custom_race
    name: Custom Race
    home_world: swamp
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				_, err = reg.Race("human")
				Expect(err).NotTo(HaveOccurred())

				custom, err := reg.Race("custom_race")
				Expect(err).NotTo(HaveOccurred())
				Expect(custom.HomeWorld).To(Equal(config.PlanetSwamp))
			})
		})

		Context("mod overlay — sorted application order", func() {
			It("applies mods in directory-name order so later mods win", func() {
				writeFile(filepath.Join(cfgDir(), "races.yaml"), `
races:
  - id: human
    name: Human
    home_world: terran
`)
				writeFile(filepath.Join(modsDir(), "01-first", "races.yaml"), `
races:
  - id: human
    name: Human First Mod
    home_world: terran
`)
				writeFile(filepath.Join(modsDir(), "02-second", "races.yaml"), `
races:
  - id: human
    name: Human Second Mod
    home_world: terran
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				human, err := reg.Race("human")
				Expect(err).NotTo(HaveOccurred())
				Expect(human.Name).To(Equal("Human Second Mod"))
			})
		})

		Context("not-found errors", func() {
			It("returns an error for an unknown race id", func() {
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				_, err = reg.Race("nonexistent")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})

			It("returns an error for an unknown ship id", func() {
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				_, err = reg.Ship("ghost_ship")
				Expect(err).To(HaveOccurred())
			})

			It("returns an error for an unknown tech id", func() {
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				_, err = reg.Tech("warp_9")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("invalid YAML", func() {
			It("returns a parse error for malformed YAML", func() {
				writeFile(filepath.Join(cfgDir(), "races.yaml"), `
races:
  - id: broken
    name: [unclosed bracket
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
			})
		})

		Context("Races() listing", func() {
			It("returns all races sorted by id", func() {
				writeFile(filepath.Join(cfgDir(), "races.yaml"), `
races:
  - id: mrrshan
    name: Mrrshan
    home_world: arid
  - id: bulrathi
    name: Bulrathi
    home_world: terran
  - id: human
    name: Human
    home_world: terran
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				races := reg.Races()
				Expect(races).To(HaveLen(3))
				Expect(races[0].ID).To(Equal("bulrathi"))
				Expect(races[1].ID).To(Equal("human"))
				Expect(races[2].ID).To(Equal("mrrshan"))
			})
		})

		Context("Ships() and Techs() listing", func() {
			It("returns all ships sorted by id", func() {
				writeFile(filepath.Join(cfgDir(), "ships.yaml"), `
ships:
  - id: titan
    name: Titan
    hull: titan
    space: 800
    hp: 130
    structure: 300
    speed: 1
    cost: 1000
    weapon_slots: 15
    special_slots: 6
  - id: frigate
    name: Frigate
    hull: frigate
    space: 50
    hp: 3
    structure: 10
    speed: 3
    cost: 40
    weapon_slots: 2
    special_slots: 1
`)
				reg, err := config.Load(cfgDir(), modsDir())
				Expect(err).NotTo(HaveOccurred())

				ships := reg.Ships()
				Expect(ships).To(HaveLen(2))
				Expect(ships[0].ID).To(Equal("frigate"))
				Expect(ships[1].ID).To(Equal("titan"))
			})
		})

		Context("validation — required fields", func() {
			It("rejects a race with an empty id", func() {
				writeFile(filepath.Join(cfgDir(), "races.yaml"), `
races:
  - id: ""
    name: Nobody
    home_world: terran
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("id is required"))
			})

			It("rejects a race with an empty name", func() {
				writeFile(filepath.Join(cfgDir(), "races.yaml"), `
races:
  - id: nameless
    name: ""
    home_world: terran
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("name is required"))
			})

			It("rejects a ship with an empty id", func() {
				writeFile(filepath.Join(cfgDir(), "ships.yaml"), `
ships:
  - id: ""
    name: Ghost
    hull: frigate
    space: 50
    hp: 3
    structure: 10
    speed: 3
    cost: 40
    weapon_slots: 2
    special_slots: 1
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("id is required"))
			})

			It("rejects a tech with an empty name", func() {
				writeFile(filepath.Join(cfgDir(), "techs.yaml"), `
techs:
  - id: orphan_tech
    name: ""
    category: weapons
    level: 1
    cost: 100
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("name is required"))
			})
		})

		Context("validation — enum values", func() {
			It("rejects a race with an unknown home_world class", func() {
				writeFile(filepath.Join(cfgDir(), "races.yaml"), `
races:
  - id: alien
    name: Alien
    home_world: plasma
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("home_world"))
			})

			It("rejects a race with an unknown government", func() {
				writeFile(filepath.Join(cfgDir(), "races.yaml"), `
races:
  - id: rebel
    name: Rebel
    home_world: terran
    government: anarchy
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("government"))
			})

			It("rejects a ship with an unknown hull type", func() {
				writeFile(filepath.Join(cfgDir(), "ships.yaml"), `
ships:
  - id: hover
    name: Hover Tank
    hull: tank
    space: 50
    hp: 3
    structure: 10
    speed: 3
    cost: 40
    weapon_slots: 2
    special_slots: 1
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("hull"))
			})

			It("rejects a tech with an unknown category", func() {
				writeFile(filepath.Join(cfgDir(), "techs.yaml"), `
techs:
  - id: magic
    name: Magic
    category: arcane
    level: 1
    cost: 100
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("category"))
			})

			It("rejects a planet with an invalid richness value", func() {
				writeFile(filepath.Join(cfgDir(), "planets.yaml"), `
planets:
  - id: weird
    class: terran
    max_pop: 10
    richness: magical
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("richness"))
			})

			It("rejects a leader with an unknown type", func() {
				writeFile(filepath.Join(cfgDir(), "leaders.yaml"), `
leaders:
  - id: jester
    name: Jester
    type: clown
    level: 1
    cost: 50
    salary: 5
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("type"))
			})
		})

		Context("validation — value ranges", func() {
			It("rejects a ship with non-positive space", func() {
				writeFile(filepath.Join(cfgDir(), "ships.yaml"), `
ships:
  - id: null_ship
    name: Null Ship
    hull: frigate
    space: 0
    hp: 3
    structure: 10
    speed: 3
    cost: 40
    weapon_slots: 2
    special_slots: 1
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("space must be > 0"))
			})

			It("rejects a ship with negative armor_class", func() {
				writeFile(filepath.Join(cfgDir(), "ships.yaml"), `
ships:
  - id: rusty
    name: Rusty
    hull: frigate
    space: 50
    hp: 3
    structure: 10
    armor_class: -1
    speed: 3
    cost: 40
    weapon_slots: 2
    special_slots: 1
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("armor_class must be >= 0"))
			})

			It("rejects a tech with non-positive cost", func() {
				writeFile(filepath.Join(cfgDir(), "techs.yaml"), `
techs:
  - id: free_lunch
    name: Free Lunch
    category: physics
    level: 1
    cost: 0
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cost must be > 0"))
			})

			It("rejects a building with negative maintenance", func() {
				writeFile(filepath.Join(cfgDir(), "buildings.yaml"), `
buildings:
  - id: money_pit
    name: Money Pit
    cost: 100
    maintenance: -5
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("maintenance must be >= 0"))
			})

			It("rejects a planet with negative food_mod", func() {
				writeFile(filepath.Join(cfgDir(), "planets.yaml"), `
planets:
  - id: anti_farm
    class: terran
    max_pop: 10
    food_mod: -0.5
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("food_mod must be >= 0"))
			})

			It("collects multiple errors in one pass", func() {
				writeFile(filepath.Join(cfgDir(), "ships.yaml"), `
ships:
  - id: ""
    name: ""
    hull: bad_hull
    space: -10
    hp: 0
    structure: 0
    speed: 0
    cost: 0
    weapon_slots: -1
    special_slots: -1
`)
				_, err := config.Load(cfgDir(), modsDir())
				Expect(err).To(HaveOccurred())
				Expect(config.IsValidationError(err)).To(BeTrue())
				// Should report several problems, not just the first.
				Expect(err.Error()).To(ContainSubstring("id is required"))
				Expect(err.Error()).To(ContainSubstring("hull"))
				Expect(err.Error()).To(ContainSubstring("space must be > 0"))
			})
		})
	})
})
