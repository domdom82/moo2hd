package research_test

import (
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/game/research"
)

// writeFile creates a file inside dir with the given content.
func writeFile(dir, name, content string) {
	GinkgoHelper()
	Expect(os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)).To(Succeed())
}

// buildRegistry builds a config.Registry from a YAML string written into a tmp dir.
func buildRegistry(yaml string) *config.Registry {
	GinkgoHelper()
	dir := GinkgoT().TempDir()
	writeFile(dir, "techs.yaml", yaml)
	reg, err := config.Load(dir, "")
	Expect(err).NotTo(HaveOccurred())
	return reg
}

const simpleTechs = `
techs:
  - id: alpha
    name: Alpha
    category: physics
    level: 1
    cost: 50
  - id: beta
    name: Beta
    category: physics
    level: 2
    cost: 100
    prereqs: [alpha]
  - id: gamma
    name: Gamma
    category: physics
    level: 3
    cost: 200
    prereqs: [beta]
`

const excludedTechs = `
techs:
  - id: alpha
    name: Alpha
    category: physics
    level: 1
    cost: 50
  - id: psilon_only
    name: Psilon Only
    category: physics
    level: 1
    cost: 50
    race_excludes: [human]
`

const racialYaml = `
races:
  - id: human
    name: Human
    home_world: terran
    government: democracy
  - id: psilon
    name: Psilon
    home_world: terran
    government: democracy
    bonuses:
      research: 0.5
techs:
  - id: alpha
    name: Alpha
    category: physics
    level: 1
    cost: 100
`

var _ = Describe("Tree", func() {
	Describe("Build", func() {
		It("builds a DAG from the registry", func() {
			reg := buildRegistry(simpleTechs)
			tree, err := research.Build(reg)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(tree.Nodes())).To(Equal(3))
		})

		It("returns an error for an unknown prereq ID", func() {
			yaml := `
techs:
  - id: alpha
    name: Alpha
    category: physics
    level: 1
    cost: 50
    prereqs: [nonexistent]
`
			reg := buildRegistry(yaml)
			_, err := research.Build(reg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("nonexistent"))
		})

		It("detects a dependency cycle", func() {
			yaml := `
techs:
  - id: a
    name: A
    category: physics
    level: 1
    cost: 50
    prereqs: [b]
  - id: b
    name: B
    category: physics
    level: 2
    cost: 50
    prereqs: [a]
`
			reg := buildRegistry(yaml)
			_, err := research.Build(reg)
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, research.ErrCycle)).To(BeTrue())
		})

		It("ByCategory returns techs in the requested category", func() {
			reg := buildRegistry(simpleTechs)
			tree, _ := research.Build(reg)
			nodes := tree.ByCategory("physics")
			Expect(len(nodes)).To(Equal(3))
		})

		It("TopologicalOrder has each tech after all its prereqs", func() {
			reg := buildRegistry(simpleTechs)
			tree, _ := research.Build(reg)
			order := tree.TopologicalOrder()
			Expect(len(order)).To(Equal(3))
			pos := make(map[string]int, len(order))
			for i, id := range order {
				pos[id] = i
			}
			Expect(pos["alpha"]).To(BeNumerically("<", pos["beta"]))
			Expect(pos["beta"]).To(BeNumerically("<", pos["gamma"]))
		})
	})
})

var _ = Describe("PlayerResearch", func() {
	buildTree := func(yaml string) (*config.Registry, *research.Tree) {
		GinkgoHelper()
		reg := buildRegistry(yaml)
		tree, err := research.Build(reg)
		Expect(err).NotTo(HaveOccurred())
		return reg, tree
	}

	It("seeds Available with root techs only", func() {
		reg, tree := buildTree(simpleTechs)
		pr := research.NewPlayerResearch("", reg, tree)
		Expect(pr.Available).To(HaveKey("alpha"))
		Expect(pr.Available).NotTo(HaveKey("beta"))
		Expect(pr.Available).NotTo(HaveKey("gamma"))
	})

	It("marks race-excluded techs as Ineligible", func() {
		dir := GinkgoT().TempDir()
		writeFile(dir, "data.yaml", excludedTechs+"\nraces:\n  - id: human\n    name: Human\n    home_world: terran\n")
		reg, err := config.Load(dir, "")
		Expect(err).NotTo(HaveOccurred())
		tree, err := research.Build(reg)
		Expect(err).NotTo(HaveOccurred())
		pr := research.NewPlayerResearch("human", reg, tree)
		Expect(pr.Ineligible).To(HaveKey("psilon_only"))
		Expect(pr.Available).NotTo(HaveKey("psilon_only"))
	})

	It("SetCurrentTech rejects a tech not in Available", func() {
		reg, tree := buildTree(simpleTechs)
		pr := research.NewPlayerResearch("", reg, tree)
		err := research.SetCurrentTech(pr, "beta", tree)
		Expect(err).To(HaveOccurred())
	})

	It("SetCurrentTech accepts an available tech", func() {
		reg, tree := buildTree(simpleTechs)
		pr := research.NewPlayerResearch("", reg, tree)
		Expect(research.SetCurrentTech(pr, "alpha", tree)).To(Succeed())
		Expect(pr.CurrentTech).To(Equal("alpha"))
	})

	It("Advance accumulates RP and fires a DiscoveryEvent on completion", func() {
		reg, tree := buildTree(simpleTechs)
		pr := research.NewPlayerResearch("", reg, tree)
		Expect(research.SetCurrentTech(pr, "alpha", tree)).To(Succeed())

		// partial advancement — should not complete
		evt, err := research.Advance(pr, 30, tree, reg)
		Expect(err).NotTo(HaveOccurred())
		Expect(evt).To(BeNil())

		// finish it
		evt, err = research.Advance(pr, 30, tree, reg)
		Expect(err).NotTo(HaveOccurred())
		Expect(evt).NotTo(BeNil())
		Expect(evt.TechID).To(Equal("alpha"))
		Expect(research.IsResearched(pr, "alpha")).To(BeTrue())
	})

	It("newly unlocked techs appear in Available after discovery", func() {
		reg, tree := buildTree(simpleTechs)
		pr := research.NewPlayerResearch("", reg, tree)
		Expect(research.SetCurrentTech(pr, "alpha", tree)).To(Succeed())
		evt, err := research.Advance(pr, 100, tree, reg) // enough to complete
		Expect(err).NotTo(HaveOccurred())
		Expect(evt).NotTo(BeNil())
		Expect(pr.Available).To(HaveKey("beta"))
		Expect(pr.Available).NotTo(HaveKey("gamma")) // still behind beta
	})

	It("Psilon research bonus accumulates 1.5x faster", func() {
		dir := GinkgoT().TempDir()
		writeFile(dir, "data.yaml", racialYaml)
		reg, err := config.Load(dir, "")
		Expect(err).NotTo(HaveOccurred())
		tree, err := research.Build(reg)
		Expect(err).NotTo(HaveOccurred())

		prHuman := research.NewPlayerResearch("human", reg, tree)
		prPsilon := research.NewPlayerResearch("psilon", reg, tree)

		Expect(research.SetCurrentTech(prHuman, "alpha", tree)).To(Succeed())
		Expect(research.SetCurrentTech(prPsilon, "alpha", tree)).To(Succeed())

		// Advance both with 60 RP — human gets 60, psilon gets 90 (60×1.5)
		research.Advance(prHuman, 60, tree, reg)
		research.Advance(prPsilon, 60, tree, reg)

		Expect(prPsilon.Accumulator).To(BeNumerically(">", prHuman.Accumulator))
	})
})
