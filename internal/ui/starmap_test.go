package ui_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/domdom82/moo2hd/internal/game/galaxy"
	"github.com/domdom82/moo2hd/internal/ui"
)

var _ = Describe("StarMap helpers", func() {
	Describe("StarColor", func() {
		allTypes := []galaxy.StarType{
			galaxy.StarYellow, galaxy.StarBlue, galaxy.StarWhite,
			galaxy.StarOrange, galaxy.StarRed, galaxy.StarBrown,
			galaxy.StarBlackHole,
		}

		It("returns a distinct color for every StarType", func() {
			seen := map[[3]float32]bool{}
			for _, t := range allTypes {
				c := ui.StarColor(t)
				key := [3]float32{c.R, c.G, c.B}
				Expect(seen[key]).To(BeFalse(), "duplicate color for star type %q", t)
				seen[key] = true
			}
		})

		It("returns alpha=1.0 for all star types", func() {
			for _, t := range allTypes {
				c := ui.StarColor(t)
				Expect(c.A).To(BeNumerically("~", float32(1.0), 0.001))
			}
		})
	})

	Describe("FactionColor", func() {
		It("returns 8 distinct colors for indices 0–7", func() {
			seen := map[[4]float32]bool{}
			for i := range 8 {
				c := ui.FactionColor(i)
				key := [4]float32{c.R, c.G, c.B, c.A}
				Expect(seen[key]).To(BeFalse(), "duplicate faction color at index %d", i)
				seen[key] = true
			}
		})

		It("wraps at 8 (index 0 == index 8)", func() {
			Expect(ui.FactionColor(0)).To(Equal(ui.FactionColor(8)))
		})

		It("all faction colors have non-zero alpha", func() {
			for i := range 8 {
				Expect(ui.FactionColor(i).A).To(BeNumerically(">", float32(0)))
			}
		})
	})

	Describe("BackgroundColor", func() {
		It("has non-zero alpha", func() {
			Expect(ui.BackgroundColor().A).To(BeNumerically(">", float32(0)))
		})
	})
})
