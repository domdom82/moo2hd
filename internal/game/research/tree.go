package research

import (
	"errors"
	"fmt"
	"sort"

	"github.com/domdom82/moo2hd/internal/config"
)

// ErrCycle is returned when the technology graph contains a dependency cycle.
var ErrCycle = errors.New("research: technology dependency cycle detected")

// Build constructs a Tree from the given Registry.
// It validates that all prereq/unlock IDs exist and that the DAG is acyclic.
func Build(reg *config.Registry) (*Tree, error) {
	techs := reg.Techs()
	nodes := make(map[string]*Node, len(techs))
	for _, t := range techs {
		tc := *t
		nodes[t.ID] = &Node{Tech: &tc}
	}

	// Wire prereqs and unlocks; validate IDs exist.
	for _, n := range nodes {
		n.Prereqs = make([]string, len(n.Tech.Prereqs))
		copy(n.Prereqs, n.Tech.Prereqs)
		for _, id := range n.Prereqs {
			if _, ok := nodes[id]; !ok {
				return nil, fmt.Errorf("research: tech %q prereq %q not found", n.Tech.ID, id)
			}
		}
		n.Unlocks = make([]string, len(n.Tech.Unlocks))
		copy(n.Unlocks, n.Tech.Unlocks)
		for _, id := range n.Unlocks {
			if _, ok := nodes[id]; !ok {
				return nil, fmt.Errorf("research: tech %q unlocks %q not found", n.Tech.ID, id)
			}
		}
		n.Successor = n.Unlocks
	}

	// Cycle detection — DFS three-colour (white=0, grey=1, black=2).
	color := make(map[string]int, len(nodes))
	var dfs func(id string) error
	dfs = func(id string) error {
		if color[id] == 2 {
			return nil
		}
		if color[id] == 1 {
			return fmt.Errorf("%w: %s", ErrCycle, id)
		}
		color[id] = 1
		for _, pre := range nodes[id].Prereqs {
			if err := dfs(pre); err != nil {
				return err
			}
		}
		color[id] = 2
		return nil
	}
	for id := range nodes {
		if err := dfs(id); err != nil {
			return nil, err
		}
	}

	byCat := make(map[config.TechCategory][]*Node)
	for _, n := range nodes {
		cat := n.Tech.Category
		byCat[cat] = append(byCat[cat], n)
	}
	for cat := range byCat {
		sort.Slice(byCat[cat], func(i, j int) bool {
			return byCat[cat][i].Tech.ID < byCat[cat][j].Tech.ID
		})
	}

	return &Tree{nodes: nodes, byCategory: byCat}, nil
}

// Node returns the node for the given tech ID, or nil if not found.
func (t *Tree) Node(id string) *Node {
	return t.nodes[id]
}

// Nodes returns all nodes sorted by ID.
func (t *Tree) Nodes() []*Node {
	out := make([]*Node, 0, len(t.nodes))
	for _, n := range t.nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tech.ID < out[j].Tech.ID })
	return out
}

// ByCategory returns all nodes in the given category, sorted by ID.
func (t *Tree) ByCategory(cat config.TechCategory) []*Node {
	return t.byCategory[cat]
}

// IsUnlocked reports whether techID is unlockable given the researched set.
// A tech is unlockable when all of its Prereqs are in researched.
func (t *Tree) IsUnlocked(techID string, researched map[string]struct{}) bool {
	n, ok := t.nodes[techID]
	if !ok {
		return false
	}
	for _, pre := range n.Prereqs {
		if _, done := researched[pre]; !done {
			return false
		}
	}
	return true
}

// TopologicalOrder returns all tech IDs in a valid topological order
// (each tech appears after all its prerequisites).
func (t *Tree) TopologicalOrder() []string {
	visited := make(map[string]bool)
	var order []string
	var visit func(id string)
	visit = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		for _, pre := range t.nodes[id].Prereqs {
			visit(pre)
		}
		order = append(order, id)
	}
	// Visit in deterministic sorted order.
	ids := make([]string, 0, len(t.nodes))
	for id := range t.nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		visit(id)
	}
	return order
}
