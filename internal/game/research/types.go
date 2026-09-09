package research

import "github.com/domdom82/moo2hd/internal/config"

// Node wraps a Technology with resolved graph edges.
type Node struct {
	Tech      *config.Technology
	Prereqs   []string // IDs of technologies that must be researched first
	Unlocks   []string // IDs of technologies this one unlocks
	Successor []string // synonym for Unlocks (populated by Build)
}

// Tree is the fully-resolved research DAG.
type Tree struct {
	nodes      map[string]*Node
	byCategory map[config.TechCategory][]*Node
}

// PlayerResearch tracks one player's research state.
type PlayerResearch struct {
	RaceID      string
	Researched  map[string]struct{}
	Available   map[string]struct{} // prereqs met, not yet started or researched
	Ineligible  map[string]struct{} // race_excludes or otherwise locked
	Accumulator float64             // RP accumulated toward CurrentTech
	CurrentTech string              // ID of the tech being researched; empty = idle
	Bonus       float64             // additive RP multiplier from racial traits
}

// DiscoveryEvent is returned when a technology research completes.
type DiscoveryEvent struct {
	TechID  string
	Unlocked []string // IDs of technologies that became available
}
