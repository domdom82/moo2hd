package ship_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/game/ship"
)

func writeFile(dir, name, content string) {
	GinkgoHelper()
	Expect(os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)).To(Succeed())
}

// minimal YAML that produces one scout hull + two components
const testYAML = `
ships:
  - id: scout
    name: Scout
    hull: frigate
    space: 20
    hp: 10
    structure: 5
    armor_class: 0
    shield_class: 0
    speed: 3
    cost: 60
    weapon_slots: 1
    special_slots: 1
components:
  - id: laser_cannon
    name: Laser Cannon
    kind: weapon
    slot_kind: weapon
    space_cost: 5
    power_draw: 1
    power_gen: 0
  - id: nuclear_drive
    name: Nuclear Drive
    kind: engine
    slot_kind: special
    space_cost: 8
    power_draw: 0
    power_gen: 5
    tech_prereq: nuclear_drive
`

func buildRegistry() *config.Registry {
	GinkgoHelper()
	dir := GinkgoT().TempDir()
	writeFile(dir, "data.yaml", testYAML)
	reg, err := config.Load(dir, "")
	Expect(err).NotTo(HaveOccurred())
	return reg
}

var _ = Describe("DesignBook", func() {
	var (
		reg  *config.Registry
		book *ship.DesignBook
	)

	BeforeEach(func() {
		reg = buildRegistry()
		book = ship.NewDesignBook(reg)
	})

	It("NewDesign allocates slots matching the hull definition", func() {
		d, err := book.NewDesign(1, "My Scout", "scout")
		Expect(err).NotTo(HaveOccurred())
		Expect(len(d.Slots)).To(Equal(2)) // 1 weapon + 1 special
		Expect(d.Slots[0].Kind).To(Equal(ship.SlotWeapon))
		Expect(d.Slots[1].Kind).To(Equal(ship.SlotSpecial))
	})

	It("NewDesign returns an error for an unknown hull", func() {
		_, err := book.NewDesign(1, "Bad", "nonexistent")
		Expect(err).To(HaveOccurred())
	})

	It("Install places a weapon component into a weapon slot", func() {
		d, _ := book.NewDesign(1, "Scout", "scout")
		Expect(book.Install(d, 0, "laser_cannon")).To(Succeed())
		Expect(d.Slots[0].ComponentID).To(Equal("laser_cannon"))
	})

	It("Install rejects placing a weapon component into a special slot", func() {
		d, _ := book.NewDesign(1, "Scout", "scout")
		err := book.Install(d, 1, "laser_cannon") // slot 1 is special
		Expect(err).To(HaveOccurred())
	})

	It("Install rejects an unknown component", func() {
		d, _ := book.NewDesign(1, "Scout", "scout")
		err := book.Install(d, 0, "death_beam_xyz")
		Expect(err).To(HaveOccurred())
	})

	It("Remove clears the component", func() {
		d, _ := book.NewDesign(1, "Scout", "scout")
		book.Install(d, 0, "laser_cannon")
		Expect(book.Remove(d, 0)).To(Succeed())
		Expect(d.Slots[0].ComponentID).To(Equal(""))
	})

	Describe("Validate", func() {
		researched := map[string]struct{}{"nuclear_drive": {}}

		It("passes a valid design (laser + nuclear drive)", func() {
			d, _ := book.NewDesign(1, "Scout", "scout")
			book.Install(d, 0, "laser_cannon")
			book.Install(d, 1, "nuclear_drive")
			verr := book.Validate(d, researched)
			Expect(verr).To(BeNil())
			Expect(d.SpaceUsed).To(Equal(5 + 8))
			Expect(d.PowerNet).To(Equal(5 - 1))
		})

		It("fails when space is exceeded", func() {
			// Override hull space to something tiny
			tiny := `
ships:
  - id: tiny_scout
    name: Tiny Scout
    hull: frigate
    space: 3
    hp: 10
    structure: 5
    armor_class: 0
    shield_class: 0
    speed: 3
    cost: 60
    weapon_slots: 1
    special_slots: 0
components:
  - id: laser_cannon
    name: Laser Cannon
    kind: weapon
    slot_kind: weapon
    space_cost: 5
    power_draw: 1
    power_gen: 0
`
			dir := GinkgoT().TempDir()
			writeFile(dir, "data.yaml", tiny)
			r, err := config.Load(dir, "")
			Expect(err).NotTo(HaveOccurred())
			bk := ship.NewDesignBook(r)
			d, _ := bk.NewDesign(1, "Tiny", "tiny_scout")
			bk.Install(d, 0, "laser_cannon")
			verr := bk.Validate(d, nil)
			Expect(verr).NotTo(BeNil())
			Expect(verr.Error()).To(ContainSubstring("space budget"))
		})

		It("fails when power is in deficit", func() {
			d, _ := book.NewDesign(1, "Scout", "scout")
			book.Install(d, 0, "laser_cannon") // draws 1, no generator
			verr := book.Validate(d, nil)
			Expect(verr).NotTo(BeNil())
			Expect(verr.Error()).To(ContainSubstring("power deficit"))
		})

		It("fails when a required tech is not researched", func() {
			d, _ := book.NewDesign(1, "Scout", "scout")
			book.Install(d, 1, "nuclear_drive")
			verr := book.Validate(d, map[string]struct{}{}) // empty research
			Expect(verr).NotTo(BeNil())
			Expect(verr.Error()).To(ContainSubstring("nuclear_drive"))
		})
	})

	Describe("Save / Delete lifecycle", func() {
		researched := map[string]struct{}{"nuclear_drive": {}}

		It("Save assigns a unique ID and stores the design", func() {
			d, _ := book.NewDesign(1, "S1", "scout")
			book.Install(d, 0, "laser_cannon")
			book.Install(d, 1, "nuclear_drive")
			Expect(book.Save(d, researched)).To(Succeed())
			Expect(d.ID).To(BeNumerically(">", 0))
			Expect(len(book.All())).To(Equal(1))
		})

		It("Save rejects an invalid design", func() {
			d, _ := book.NewDesign(1, "Bad", "scout")
			// no engine — power deficit
			book.Install(d, 0, "laser_cannon")
			Expect(book.Save(d, nil)).To(HaveOccurred())
		})

		It("Delete removes a stored design", func() {
			d, _ := book.NewDesign(1, "S1", "scout")
			book.Install(d, 0, "laser_cannon")
			book.Install(d, 1, "nuclear_drive")
			book.Save(d, researched)
			Expect(book.Delete(d.ID)).To(Succeed())
			Expect(len(book.All())).To(Equal(0))
		})

		It("Delete returns an error for unknown ID", func() {
			Expect(book.Delete(999)).To(HaveOccurred())
		})
	})

	Describe("ByHull", func() {
		researched := map[string]struct{}{"nuclear_drive": {}}

		It("returns only designs for the requested hull", func() {
			d1, _ := book.NewDesign(1, "S1", "scout")
			book.Install(d1, 0, "laser_cannon")
			book.Install(d1, 1, "nuclear_drive")
			book.Save(d1, researched)

			d2, _ := book.NewDesign(1, "S2", "scout")
			book.Install(d2, 0, "laser_cannon")
			book.Install(d2, 1, "nuclear_drive")
			book.Save(d2, researched)

			Expect(len(book.ByHull("scout"))).To(Equal(2))
			Expect(len(book.ByHull("battleship"))).To(Equal(0))
		})
	})
})
