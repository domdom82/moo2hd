package research

import (
	"errors"
	"fmt"

	"github.com/domdom82/moo2hd/internal/config"
)

// NewPlayerResearch creates a PlayerResearch for the given race, seeding
// Available with all root techs (those with no prerequisites) and marking
// race-excluded techs as Ineligible.
func NewPlayerResearch(raceID string, reg *config.Registry, tree *Tree) *PlayerResearch {
	bonus, _ := RacialBonus(raceID, reg)
	pr := &PlayerResearch{
		RaceID:     raceID,
		Researched: make(map[string]struct{}),
		Available:  make(map[string]struct{}),
		Ineligible: make(map[string]struct{}),
		Bonus:      bonus,
	}
	race, err := reg.Race(raceID)
	if err == nil {
		for _, t := range tree.Nodes() {
			for _, exc := range t.Tech.RaceExcludes {
				if exc == race.ID {
					pr.Ineligible[t.Tech.ID] = struct{}{}
				}
			}
		}
	}

	for _, n := range tree.Nodes() {
		if _, inel := pr.Ineligible[n.Tech.ID]; inel {
			continue
		}
		if len(n.Prereqs) == 0 {
			pr.Available[n.Tech.ID] = struct{}{}
		}
	}
	return pr
}

// SetCurrentTech sets the technology being researched.
// Returns an error if techID is not in Available.
func SetCurrentTech(pr *PlayerResearch, techID string, tree *Tree) error {
	if tree.Node(techID) == nil {
		return fmt.Errorf("research: tech %q does not exist", techID)
	}
	if _, ok := pr.Available[techID]; !ok {
		return fmt.Errorf("research: tech %q is not available for research", techID)
	}
	pr.CurrentTech = techID
	return nil
}

// Advance credits rp research points toward the current technology.
// When the tech is completed a DiscoveryEvent is returned; otherwise nil.
// Returns an error if no current tech is set.
func Advance(pr *PlayerResearch, rp float64, tree *Tree, reg *config.Registry) (*DiscoveryEvent, error) {
	if pr.CurrentTech == "" {
		return nil, errors.New("research: no current tech set")
	}
	tech, err := reg.Tech(pr.CurrentTech)
	if err != nil {
		return nil, err
	}
	effective := rp * (1 + pr.Bonus)
	pr.Accumulator += effective
	if pr.Accumulator < float64(tech.Cost) {
		return nil, nil
	}
	pr.Accumulator = 0
	completed := pr.CurrentTech
	pr.CurrentTech = ""
	pr.Researched[completed] = struct{}{}
	delete(pr.Available, completed)

	// Recompute Available: any tech whose prereqs are all in Researched.
	var unlocked []string
	for _, n := range tree.Nodes() {
		if _, done := pr.Researched[n.Tech.ID]; done {
			continue
		}
		if _, inel := pr.Ineligible[n.Tech.ID]; inel {
			continue
		}
		if _, avail := pr.Available[n.Tech.ID]; avail {
			continue
		}
		if tree.IsUnlocked(n.Tech.ID, pr.Researched) {
			pr.Available[n.Tech.ID] = struct{}{}
			unlocked = append(unlocked, n.Tech.ID)
		}
	}
	return &DiscoveryEvent{TechID: completed, Unlocked: unlocked}, nil
}

// AvailableList returns all available tech nodes sorted by ID.
func AvailableList(pr *PlayerResearch, tree *Tree) []*Node {
	out := make([]*Node, 0, len(pr.Available))
	for id := range pr.Available {
		if n := tree.Node(id); n != nil {
			out = append(out, n)
		}
	}
	// sort by ID for determinism
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Tech.ID < out[j-1].Tech.ID; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// IsResearched reports whether the given tech has been completed.
func IsResearched(pr *PlayerResearch, id string) bool {
	_, ok := pr.Researched[id]
	return ok
}

// RacialBonus returns the additive research bonus for a race (0 if not found).
// Psilon receives +0.5 (research at 1.5× speed).
func RacialBonus(raceID string, reg *config.Registry) (float64, error) {
	race, err := reg.Race(raceID)
	if err != nil {
		return 0, err
	}
	return race.Bonuses.Research, nil
}
