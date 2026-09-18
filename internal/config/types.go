package config

// LBXPaletteRef identifies a single palette record within an LBX archive.
type LBXPaletteRef struct {
	LBX    string `yaml:"lbx"`    // e.g. "FONTS.LBX"
	Record int    `yaml:"record"` // zero-based record index
}

// LBXPaletteEntry maps a sprite record (identified by its source LBX and index)
// to the ordered list of palette records needed to render it. Palettes are merged
// in order: the first entry provides the base (indices 0–191), subsequent entries
// overlay at their encoded baseIdx (typically the junction range 192+).
type LBXPaletteEntry struct {
	ID       string          `yaml:"id"`
	LBX      string          `yaml:"lbx"`      // source LBX filename, e.g. "SHIPS.LBX"
	Record   int             `yaml:"record"`   // zero-based record index of the sprite
	Palettes []LBXPaletteRef `yaml:"palettes"` // palette sources, applied in order
}

// Race represents a playable or AI species.
type Race struct {
	ID          string      `yaml:"id"`
	Name        string      `yaml:"name"`
	Description string      `yaml:"description,omitempty"`
	Government  string      `yaml:"government,omitempty"` // feudal, dictatorship, democracy, unification
	HomeWorld   PlanetClass `yaml:"home_world"`
	Traits      []string    `yaml:"traits,omitempty"`
	PickTotal   int         `yaml:"pick_total,omitempty"`
	Bonuses     RaceBonuses `yaml:"bonuses,omitempty"`
}

// RaceBonuses holds numeric modifiers for a race.
type RaceBonuses struct {
	Production float64 `yaml:"production,omitempty"`
	Research   float64 `yaml:"research,omitempty"`
	Food       float64 `yaml:"food,omitempty"`
	Morale     float64 `yaml:"morale,omitempty"`
	Ship       float64 `yaml:"ship,omitempty"`
	Espionage  float64 `yaml:"espionage,omitempty"`
}

// PlanetClass enumerates planet environment types.
type PlanetClass string

const (
	PlanetTerran   PlanetClass = "terran"
	PlanetOcean    PlanetClass = "ocean"
	PlanetArid     PlanetClass = "arid"
	PlanetDesert   PlanetClass = "desert"
	PlanetTundra   PlanetClass = "tundra"
	PlanetSwamp    PlanetClass = "swamp"
	PlanetVolcanic PlanetClass = "volcanic"
	PlanetBarren   PlanetClass = "barren"
	PlanetRadiated PlanetClass = "radiated"
	PlanetToxic    PlanetClass = "toxic"
	PlanetGaia     PlanetClass = "gaia"
	PlanetInferno  PlanetClass = "inferno"
	PlanetNone     PlanetClass = "none" // gas giant / uninhabitable
)

// Planet describes the static properties of a planet type or a named system body.
type Planet struct {
	ID          string      `yaml:"id"`
	Class       PlanetClass `yaml:"class"`
	MaxPop      int         `yaml:"max_pop"`
	FoodMod     float64     `yaml:"food_mod,omitempty"`
	ProdMod     float64     `yaml:"prod_mod,omitempty"`
	ResearchMod float64     `yaml:"research_mod,omitempty"`
	Richness    string      `yaml:"richness,omitempty"` // ultra-poor, poor, abundant, rich, ultra-rich
	Gravity     string      `yaml:"gravity,omitempty"`  // low, normal, high
}

// HullSize names a ship hull class.
type HullSize string

const (
	HullFrigate    HullSize = "frigate"
	HullDestroyer  HullSize = "destroyer"
	HullCruiser    HullSize = "cruiser"
	HullBattleship HullSize = "battleship"
	HullTitan      HullSize = "titan"
	HullDoomStar   HullSize = "doom_star"
)

// Ship defines a hull template that players can design ships from.
//
// HP is the base internal systems hit points (structure integrity).
// Structure is the structural hit points that must be depleted before internals
// are damaged — effectively the "hull" layer in MOO2 combat.
// ArmorClass is the base armor value before technology upgrades; each armor tech
// level multiplies this value and adds an additional damage-absorbing layer on
// top of Structure.
// ShieldClass is the base shield class (I, III, V, …); each shield tech level
// sets this value. Shields regenerate each combat round and absorb damage before
// Structure is hit. 0 means no shields installed.
type Ship struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	Hull         HullSize `yaml:"hull"`
	Space        int      `yaml:"space"`
	HP           int      `yaml:"hp"`
	Structure    int      `yaml:"structure"`
	ArmorClass   int      `yaml:"armor_class"`
	ShieldClass  int      `yaml:"shield_class"`
	Speed        int      `yaml:"speed"`
	Cost         int      `yaml:"cost"`
	WeaponSlots  int      `yaml:"weapon_slots"`
	SpecialSlots int      `yaml:"special_slots"`
}

// TechCategory groups technologies thematically.
type TechCategory string

const (
	TechComputers  TechCategory = "computers"
	TechConstruct  TechCategory = "construction"
	TechForceField TechCategory = "force_fields"
	TechPropulsion TechCategory = "propulsion"
	TechWeapons    TechCategory = "weapons"
	TechBiology    TechCategory = "biology"
	TechPhysics    TechCategory = "physics"
	TechChemistry  TechCategory = "chemistry"
	TechSociology  TechCategory = "sociology"
)

// Technology represents one researchable advance in the tech tree.
type Technology struct {
	ID           string       `yaml:"id"`
	Name         string       `yaml:"name"`
	Category     TechCategory `yaml:"category"`
	Level        int          `yaml:"level"`
	Cost         int          `yaml:"cost"`
	Description  string       `yaml:"description,omitempty"`
	Prereqs      []string     `yaml:"prereqs,omitempty"`
	Unlocks      []string     `yaml:"unlocks,omitempty"`
	RaceExcludes []string     `yaml:"race_excludes,omitempty"`
}

// LeaderType distinguishes leader roles.
type LeaderType string

const (
	LeaderAdmiral   LeaderType = "admiral"
	LeaderGovernor  LeaderType = "governor"
	LeaderScientist LeaderType = "scientist"
	LeaderSpy       LeaderType = "spy"
)

// Leader is a hireable character with skills and bonuses.
type Leader struct {
	ID      string     `yaml:"id"`
	Name    string     `yaml:"name"`
	Type    LeaderType `yaml:"type"`
	Level   int        `yaml:"level"`
	Cost    int        `yaml:"cost"`
	Salary  int        `yaml:"salary"`
	Skills  []string   `yaml:"skills,omitempty"`
	Bonuses RaceBonuses `yaml:"bonuses,omitempty"`
}

// Monster defines a roaming space creature.
type Monster struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	HP          int    `yaml:"hp"`
	Attack      int    `yaml:"attack"`
	Defense     int    `yaml:"defense"`
	Speed       int    `yaml:"speed"`
	Description string `yaml:"description,omitempty"`
}

// Building is a structure that can be constructed on a colony.
type Building struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Cost        int      `yaml:"cost"`
	Maintenance int      `yaml:"maintenance"`
	Description string   `yaml:"description,omitempty"`
	Prereqs     []string `yaml:"prereqs,omitempty"`
	Effects     Effects  `yaml:"effects,omitempty"`
}

// ComponentKind categorises what a component does.
type ComponentKind string

const (
	CompWeapon  ComponentKind = "weapon"
	CompShield  ComponentKind = "shield"
	CompArmor   ComponentKind = "armor"
	CompEngine  ComponentKind = "engine"
	CompSpecial ComponentKind = "special"
)

// SlotKind distinguishes weapon hardpoints from special equipment bays.
type SlotKind string

const (
	SlotWeapon  SlotKind = "weapon"
	SlotSpecial SlotKind = "special"
)

// Component is one piece of ship equipment available to install in a design.
// SpaceCost is tons consumed from the hull's Space budget.
// PowerDraw and PowerGen are power units consumed / produced; the design is
// valid only when sum(PowerGen) >= sum(PowerDraw) across all installed components.
// TechPrereq is the config.Technology.ID required to unlock this component;
// empty means always available.
type Component struct {
	ID         string        `yaml:"id"`
	Name       string        `yaml:"name"`
	Kind       ComponentKind `yaml:"kind"`
	SlotKind   SlotKind      `yaml:"slot_kind"`
	SpaceCost  int           `yaml:"space_cost"`
	PowerDraw  int           `yaml:"power_draw"`
	PowerGen   int           `yaml:"power_gen"`
	TechPrereq string        `yaml:"tech_prereq,omitempty"`
}
type Effects struct {
	Production  float64 `yaml:"production,omitempty"`
	Research    float64 `yaml:"research,omitempty"`
	Food        float64 `yaml:"food,omitempty"`
	Morale      float64 `yaml:"morale,omitempty"`
	Defense     float64 `yaml:"defense,omitempty"`
	PopGrowth   float64 `yaml:"pop_growth,omitempty"`
	MaxPop      int     `yaml:"max_pop,omitempty"`
}
