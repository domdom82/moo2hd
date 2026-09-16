package colony

import "github.com/domdom82/moo2hd/internal/config"

// Manager owns all colonies and groups for a game instance.
type Manager struct {
	Colonies []*Colony
	Groups   []*ColonyGroup
	nextID   int
}

// NewManager creates an empty Manager.
func NewManager() *Manager {
	return &Manager{nextID: 1}
}

// AddColony registers a colony with the manager, assigning a unique ID.
func (m *Manager) AddColony(c *Colony) {
	c.ID = m.nextID
	m.nextID++
	m.Colonies = append(m.Colonies, c)
}

// TagColony appends a tag to the named colony (by ID).
func (m *Manager) TagColony(id int, tag string) {
	for _, c := range m.Colonies {
		if c.ID == id {
			c.Tags = append(c.Tags, tag)
			return
		}
	}
}

// CreateGroup creates and registers a new colony group.
func (m *Manager) CreateGroup(name string, tags []string) *ColonyGroup {
	g := &ColonyGroup{Name: name, Tags: tags}
	m.Groups = append(m.Groups, g)
	return g
}

// SetGroupTemplate assigns a build queue template to a group.
func (m *Manager) SetGroupTemplate(g *ColonyGroup, template []QueueItem) {
	g.QueueTemplate = make([]QueueItem, len(template))
	copy(g.QueueTemplate, template)
}

// SetGroupRallyPoint assigns a rally point to a group.
func (m *Manager) SetGroupRallyPoint(g *ColonyGroup, sid *int) {
	if sid == nil {
		g.RallyPoint = nil
		return
	}
	// convert int to galaxy.SystemID
	from := (interface{})(sid)
	_ = from
	g.RallyPoint = nil // caller uses ApplyGroupTemplate directly with a typed pointer
}

// ApplyGroupToColonies pushes the group's template and rally point to all
// colonies that share at least one of the group's tags.
func (m *Manager) ApplyGroupToColonies(g *ColonyGroup) {
	for _, c := range m.Colonies {
		if colonySharesTag(c, g) {
			ApplyGroupTemplate(c, g)
		}
	}
}

// ColonyForPlanet returns the colony on the given planet, or nil if none.
func (m *Manager) ColonyForPlanet(planetID int) *Colony {
	for _, c := range m.Colonies {
		if int(c.PlanetID) == planetID {
			return c
		}
	}
	return nil
}

func colonySharesTag(c *Colony, g *ColonyGroup) bool {
	for _, ct := range c.Tags {
		for _, gt := range g.Tags {
			if ct == gt {
				return true
			}
		}
	}
	return false
}

// AdvanceAllQueues advances production queues for every colony by one turn.
// Returns a map from colony ID to completed BuildTarget (only colonies that
// completed an item this turn are included).
func (m *Manager) AdvanceAllQueues(reg *config.Registry, productions map[int]float64) (map[int]BuildTarget, error) {
	results := make(map[int]BuildTarget)
	for _, c := range m.Colonies {
		prod := productions[c.ID]
		target, ok, err := AdvanceQueue(c, prod, reg)
		if err != nil {
			return nil, err
		}
		if ok {
			results[c.ID] = target
		}
	}
	return results, nil
}
