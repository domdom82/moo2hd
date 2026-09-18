package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// rawConfig is the in-memory shape that YAML files are decoded into before
// being promoted to the typed slices inside Registry.
type rawConfig struct {
	Races       []Race             `yaml:"races,omitempty"`
	Ships       []Ship             `yaml:"ships,omitempty"`
	Techs       []Technology       `yaml:"techs,omitempty"`
	Planets     []Planet           `yaml:"planets,omitempty"`
	Leaders     []Leader           `yaml:"leaders,omitempty"`
	Buildings   []Building         `yaml:"buildings,omitempty"`
	Monsters    []Monster          `yaml:"monsters,omitempty"`
	Components  []Component        `yaml:"components,omitempty"`
	LBXPalettes []LBXPaletteEntry  `yaml:"lbx_palettes,omitempty"`
}

// Registry is the read-only view of fully-merged game configuration.
type Registry struct {
	races       map[string]*Race
	ships       map[string]*Ship
	techs       map[string]*Technology
	planets     map[string]*Planet
	leaders     map[string]*Leader
	buildings   map[string]*Building
	monsters    map[string]*Monster
	components  map[string]*Component
	lbxPalettes map[string]*LBXPaletteEntry // keyed by id
}

// Race returns the race with the given id, or an error if not found.
func (r *Registry) Race(id string) (*Race, error) {
	v, ok := r.races[id]
	if !ok {
		return nil, fmt.Errorf("config: race %q not found", id)
	}
	return v, nil
}

// Ship returns the ship hull with the given id.
func (r *Registry) Ship(id string) (*Ship, error) {
	v, ok := r.ships[id]
	if !ok {
		return nil, fmt.Errorf("config: ship %q not found", id)
	}
	return v, nil
}

// Tech returns the technology with the given id.
func (r *Registry) Tech(id string) (*Technology, error) {
	v, ok := r.techs[id]
	if !ok {
		return nil, fmt.Errorf("config: tech %q not found", id)
	}
	return v, nil
}

// Planet returns the planet definition with the given id.
func (r *Registry) Planet(id string) (*Planet, error) {
	v, ok := r.planets[id]
	if !ok {
		return nil, fmt.Errorf("config: planet %q not found", id)
	}
	return v, nil
}

// Leader returns the leader with the given id.
func (r *Registry) Leader(id string) (*Leader, error) {
	v, ok := r.leaders[id]
	if !ok {
		return nil, fmt.Errorf("config: leader %q not found", id)
	}
	return v, nil
}

// Building returns the building with the given id.
func (r *Registry) Building(id string) (*Building, error) {
	v, ok := r.buildings[id]
	if !ok {
		return nil, fmt.Errorf("config: building %q not found", id)
	}
	return v, nil
}

// Monster returns the monster definition with the given id.
func (r *Registry) Monster(id string) (*Monster, error) {
	v, ok := r.monsters[id]
	if !ok {
		return nil, fmt.Errorf("config: monster %q not found", id)
	}
	return v, nil
}

// Component returns the component with the given id.
func (r *Registry) Component(id string) (*Component, error) {
	v, ok := r.components[id]
	if !ok {
		return nil, fmt.Errorf("config: component %q not found", id)
	}
	return v, nil
}

// Components returns all components sorted by id.
func (r *Registry) Components() []*Component {
	out := make([]*Component, 0, len(r.components))
	for _, v := range r.components {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Races returns all races sorted by id.
func (r *Registry) Races() []*Race {
	out := make([]*Race, 0, len(r.races))
	for _, v := range r.races {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Ships returns all ships sorted by id.
func (r *Registry) Ships() []*Ship {
	out := make([]*Ship, 0, len(r.ships))
	for _, v := range r.ships {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// LBXPaletteFor returns the palette dependency list for the sprite at the given
// LBX file and record index. Returns nil if no entry is configured.
func (r *Registry) LBXPaletteFor(lbxFile string, record int) []LBXPaletteRef {
	for _, e := range r.lbxPalettes {
		if e.LBX == lbxFile && e.Record == record {
			return e.Palettes
		}
	}
	return nil
}

// Techs returns all technologies sorted by id.
func (r *Registry) Techs() []*Technology {
	out := make([]*Technology, 0, len(r.techs))
	for _, v := range r.techs {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Load reads base configs from configDir and then merges mods from modsDir in
// sorted directory-name order. Either directory may be absent; missing dirs are
// silently skipped. Returns a fully merged Registry.
func Load(configDir, modsDir string) (*Registry, error) {
	merged := &rawConfig{}

	if err := loadDir(configDir, merged); err != nil {
		return nil, fmt.Errorf("config: loading base configs from %q: %w", configDir, err)
	}

	mods, err := sortedSubdirs(modsDir)
	if err != nil {
		return nil, fmt.Errorf("config: scanning mods dir %q: %w", modsDir, err)
	}
	for _, mod := range mods {
		if err := loadDir(mod, merged); err != nil {
			return nil, fmt.Errorf("config: loading mod %q: %w", mod, err)
		}
	}

	return buildRegistry(merged), nil
}

// loadDir reads all *.yaml files in dir and merges them into dst.
// A missing dir is treated as empty (no error).
func loadDir(dir string, dst *rawConfig) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// Process files in sorted order for determinism within a single directory.
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var chunk rawConfig
		if err := yaml.Unmarshal(data, &chunk); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := validate(&chunk); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		mergeInto(dst, &chunk)
	}
	return nil
}

// sortedSubdirs returns the full paths of immediate subdirectories of root,
// sorted lexicographically. A missing root is not an error.
func sortedSubdirs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, filepath.Join(root, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// mergeInto overlays src onto dst. For each entity type, entries in src whose
// id already exists in dst replace the existing entry; new ids are appended.
func mergeInto(dst, src *rawConfig) {
	dst.Races       = mergeByID(dst.Races, src.Races, func(r Race) string { return r.ID })
	dst.Ships       = mergeByID(dst.Ships, src.Ships, func(s Ship) string { return s.ID })
	dst.Techs       = mergeByID(dst.Techs, src.Techs, func(t Technology) string { return t.ID })
	dst.Planets     = mergeByID(dst.Planets, src.Planets, func(p Planet) string { return p.ID })
	dst.Leaders     = mergeByID(dst.Leaders, src.Leaders, func(l Leader) string { return l.ID })
	dst.Buildings   = mergeByID(dst.Buildings, src.Buildings, func(b Building) string { return b.ID })
	dst.Monsters    = mergeByID(dst.Monsters, src.Monsters, func(m Monster) string { return m.ID })
	dst.Components  = mergeByID(dst.Components, src.Components, func(c Component) string { return c.ID })
	dst.LBXPalettes = mergeByID(dst.LBXPalettes, src.LBXPalettes, func(e LBXPaletteEntry) string { return e.ID })
}

func mergeByID[T any](dst, src []T, id func(T) string) []T {
	index := make(map[string]int, len(dst))
	for i, v := range dst {
		index[id(v)] = i
	}
	for _, v := range src {
		k := id(v)
		if i, found := index[k]; found {
			dst[i] = v
		} else {
			index[k] = len(dst)
			dst = append(dst, v)
		}
	}
	return dst
}

// buildRegistry converts rawConfig slices into map-keyed Registry.
func buildRegistry(raw *rawConfig) *Registry {
	r := &Registry{
		races:       make(map[string]*Race, len(raw.Races)),
		ships:       make(map[string]*Ship, len(raw.Ships)),
		techs:       make(map[string]*Technology, len(raw.Techs)),
		planets:     make(map[string]*Planet, len(raw.Planets)),
		leaders:     make(map[string]*Leader, len(raw.Leaders)),
		buildings:   make(map[string]*Building, len(raw.Buildings)),
		monsters:    make(map[string]*Monster, len(raw.Monsters)),
		components:  make(map[string]*Component, len(raw.Components)),
		lbxPalettes: make(map[string]*LBXPaletteEntry, len(raw.LBXPalettes)),
	}
	for i := range raw.Races {
		v := raw.Races[i]
		r.races[v.ID] = &v
	}
	for i := range raw.Ships {
		v := raw.Ships[i]
		r.ships[v.ID] = &v
	}
	for i := range raw.Techs {
		v := raw.Techs[i]
		r.techs[v.ID] = &v
	}
	for i := range raw.Planets {
		v := raw.Planets[i]
		r.planets[v.ID] = &v
	}
	for i := range raw.Leaders {
		v := raw.Leaders[i]
		r.leaders[v.ID] = &v
	}
	for i := range raw.Buildings {
		v := raw.Buildings[i]
		r.buildings[v.ID] = &v
	}
	for i := range raw.Monsters {
		v := raw.Monsters[i]
		r.monsters[v.ID] = &v
	}
	for i := range raw.Components {
		v := raw.Components[i]
		r.components[v.ID] = &v
	}
	for i := range raw.LBXPalettes {
		v := raw.LBXPalettes[i]
		r.lbxPalettes[v.ID] = &v
	}
	return r
}
