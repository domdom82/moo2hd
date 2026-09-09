package config

import (
	"errors"
	"fmt"
	"strings"
)

// ValidationErrors collects all problems found during config validation so the
// caller sees every bad entry in one pass rather than stopping at the first.
type ValidationErrors []error

func (ve ValidationErrors) Error() string {
	msgs := make([]string, len(ve))
	for i, e := range ve {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
}

// validate checks every entry in raw and returns a ValidationErrors if any
// entry fails.  A nil return means the config is clean.
func validate(raw *rawConfig) error {
	var errs ValidationErrors
	for _, r := range raw.Races {
		errs = append(errs, validateRace(r)...)
	}
	for _, s := range raw.Ships {
		errs = append(errs, validateShip(s)...)
	}
	for _, t := range raw.Techs {
		errs = append(errs, validateTech(t)...)
	}
	for _, p := range raw.Planets {
		errs = append(errs, validatePlanet(p)...)
	}
	for _, l := range raw.Leaders {
		errs = append(errs, validateLeader(l)...)
	}
	for _, b := range raw.Buildings {
		errs = append(errs, validateBuilding(b)...)
	}
	for _, m := range raw.Monsters {
		errs = append(errs, validateMonster(m)...)
	}
	for _, c := range raw.Components {
		errs = append(errs, validateComponent(c)...)
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func requireString(typ, id, field, val string) error {
	if strings.TrimSpace(val) == "" {
		return fmt.Errorf("%s %q: %s is required", typ, id, field)
	}
	return nil
}

func requirePositive(typ, id, field string, val int) error {
	if val <= 0 {
		return fmt.Errorf("%s %q: %s must be > 0, got %d", typ, id, field, val)
	}
	return nil
}

func requireNonNegative(typ, id, field string, val int) error {
	if val < 0 {
		return fmt.Errorf("%s %q: %s must be >= 0, got %d", typ, id, field, val)
	}
	return nil
}

func requireNonNegativeF(typ, id, field string, val float64) error {
	if val < 0 {
		return fmt.Errorf("%s %q: %s must be >= 0, got %g", typ, id, field, val)
	}
	return nil
}

// collect appends non-nil errors to dst.
func collect(dst *ValidationErrors, errs ...error) {
	for _, e := range errs {
		if e != nil {
			*dst = append(*dst, e)
		}
	}
}

// ── per-type validators ───────────────────────────────────────────────────────

var validGovernments = map[string]bool{
	"feudal": true, "dictatorship": true, "democracy": true, "unification": true,
}

var validPlanetClasses = map[PlanetClass]bool{
	PlanetTerran: true, PlanetOcean: true, PlanetArid: true, PlanetDesert: true,
	PlanetTundra: true, PlanetSwamp: true, PlanetVolcanic: true, PlanetBarren: true,
	PlanetRadiated: true, PlanetToxic: true, PlanetGaia: true, PlanetInferno: true,
	PlanetNone: true,
}

var validRichness = map[string]bool{
	"ultra-poor": true, "poor": true, "abundant": true, "rich": true, "ultra-rich": true,
}

var validGravity = map[string]bool{
	"low": true, "normal": true, "high": true,
}

var validHulls = map[HullSize]bool{
	HullFrigate: true, HullDestroyer: true, HullCruiser: true,
	HullBattleship: true, HullTitan: true, HullDoomStar: true,
}

var validTechCategories = map[TechCategory]bool{
	TechComputers: true, TechConstruct: true, TechForceField: true,
	TechPropulsion: true, TechWeapons: true, TechBiology: true,
	TechPhysics: true, TechChemistry: true, TechSociology: true,
}

var validLeaderTypes = map[LeaderType]bool{
	LeaderAdmiral: true, LeaderGovernor: true, LeaderScientist: true, LeaderSpy: true,
}

func validateRace(r Race) (errs ValidationErrors) {
	collect(&errs, requireString("race", r.ID, "id", r.ID))
	collect(&errs, requireString("race", r.ID, "name", r.Name))
	if !validPlanetClasses[r.HomeWorld] {
		errs = append(errs, fmt.Errorf("race %q: home_world %q is not a valid planet class", r.ID, r.HomeWorld))
	}
	if r.Government != "" && !validGovernments[r.Government] {
		errs = append(errs, fmt.Errorf("race %q: government %q must be one of feudal, dictatorship, democracy, unification", r.ID, r.Government))
	}
	return errs
}

func validateShip(s Ship) (errs ValidationErrors) {
	collect(&errs, requireString("ship", s.ID, "id", s.ID))
	collect(&errs, requireString("ship", s.ID, "name", s.Name))
	if !validHulls[s.Hull] {
		errs = append(errs, fmt.Errorf("ship %q: hull %q is not valid", s.ID, s.Hull))
	}
	collect(&errs,
		requirePositive("ship", s.ID, "space", s.Space),
		requirePositive("ship", s.ID, "hp", s.HP),
		requirePositive("ship", s.ID, "structure", s.Structure),
		requireNonNegative("ship", s.ID, "armor_class", s.ArmorClass),
		requireNonNegative("ship", s.ID, "shield_class", s.ShieldClass),
		requirePositive("ship", s.ID, "speed", s.Speed),
		requirePositive("ship", s.ID, "cost", s.Cost),
		requireNonNegative("ship", s.ID, "weapon_slots", s.WeaponSlots),
		requireNonNegative("ship", s.ID, "special_slots", s.SpecialSlots),
	)
	return errs
}

func validateTech(t Technology) (errs ValidationErrors) {
	collect(&errs, requireString("tech", t.ID, "id", t.ID))
	collect(&errs, requireString("tech", t.ID, "name", t.Name))
	if !validTechCategories[t.Category] {
		errs = append(errs, fmt.Errorf("tech %q: category %q is not valid", t.ID, t.Category))
	}
	collect(&errs,
		requirePositive("tech", t.ID, "level", t.Level),
		requirePositive("tech", t.ID, "cost", t.Cost),
	)
	return errs
}

func validatePlanet(p Planet) (errs ValidationErrors) {
	collect(&errs, requireString("planet", p.ID, "id", p.ID))
	if !validPlanetClasses[p.Class] {
		errs = append(errs, fmt.Errorf("planet %q: class %q is not a valid planet class", p.ID, p.Class))
	}
	collect(&errs, requireNonNegative("planet", p.ID, "max_pop", p.MaxPop))
	collect(&errs,
		requireNonNegativeF("planet", p.ID, "food_mod", p.FoodMod),
		requireNonNegativeF("planet", p.ID, "prod_mod", p.ProdMod),
		requireNonNegativeF("planet", p.ID, "research_mod", p.ResearchMod),
	)
	if p.Richness != "" && !validRichness[p.Richness] {
		errs = append(errs, fmt.Errorf("planet %q: richness %q must be one of ultra-poor, poor, abundant, rich, ultra-rich", p.ID, p.Richness))
	}
	if p.Gravity != "" && !validGravity[p.Gravity] {
		errs = append(errs, fmt.Errorf("planet %q: gravity %q must be one of low, normal, high", p.ID, p.Gravity))
	}
	return errs
}

func validateLeader(l Leader) (errs ValidationErrors) {
	collect(&errs, requireString("leader", l.ID, "id", l.ID))
	collect(&errs, requireString("leader", l.ID, "name", l.Name))
	if !validLeaderTypes[l.Type] {
		errs = append(errs, fmt.Errorf("leader %q: type %q must be one of admiral, governor, scientist, spy", l.ID, l.Type))
	}
	collect(&errs,
		requirePositive("leader", l.ID, "level", l.Level),
		requireNonNegative("leader", l.ID, "cost", l.Cost),
		requireNonNegative("leader", l.ID, "salary", l.Salary),
	)
	return errs
}

func validateBuilding(b Building) (errs ValidationErrors) {
	collect(&errs, requireString("building", b.ID, "id", b.ID))
	collect(&errs, requireString("building", b.ID, "name", b.Name))
	collect(&errs,
		requireNonNegative("building", b.ID, "cost", b.Cost),
		requireNonNegative("building", b.ID, "maintenance", b.Maintenance),
	)
	return errs
}

func validateMonster(m Monster) (errs ValidationErrors) {
	collect(&errs, requireString("monster", m.ID, "id", m.ID))
	collect(&errs, requireString("monster", m.ID, "name", m.Name))
	collect(&errs,
		requirePositive("monster", m.ID, "hp", m.HP),
		requireNonNegative("monster", m.ID, "attack", m.Attack),
		requireNonNegative("monster", m.ID, "defense", m.Defense),
		requirePositive("monster", m.ID, "speed", m.Speed),
	)
	return errs
}

var validComponentKinds = map[ComponentKind]bool{
	CompWeapon: true, CompShield: true, CompArmor: true,
	CompEngine: true, CompSpecial: true,
}

var validSlotKinds = map[SlotKind]bool{
	SlotWeapon: true, SlotSpecial: true,
}

func validateComponent(c Component) (errs ValidationErrors) {
	collect(&errs, requireString("component", c.ID, "id", c.ID))
	collect(&errs, requireString("component", c.ID, "name", c.Name))
	if !validComponentKinds[c.Kind] {
		errs = append(errs, fmt.Errorf("component %q: kind %q must be one of weapon, shield, armor, engine, special", c.ID, c.Kind))
	}
	if !validSlotKinds[c.SlotKind] {
		errs = append(errs, fmt.Errorf("component %q: slot_kind %q must be one of weapon, special", c.ID, c.SlotKind))
	}
	collect(&errs,
		requireNonNegative("component", c.ID, "space_cost", c.SpaceCost),
		requireNonNegative("component", c.ID, "power_draw", c.PowerDraw),
		requireNonNegative("component", c.ID, "power_gen", c.PowerGen),
	)
	return errs
}

// ensure ValidationErrors satisfies the error interface at compile time.
var _ error = ValidationErrors(nil)

// errors.As support: unwrap the slice so callers can inspect individual errors.
func (ve ValidationErrors) Unwrap() []error {
	return []error(ve)
}

// As implements errors.As for ValidationErrors itself.
func (ve ValidationErrors) As(target any) bool {
	if t, ok := target.(*ValidationErrors); ok {
		*t = ve
		return true
	}
	return false
}

// IsValidationError reports whether err (or any error in its chain) is a
// ValidationErrors.
func IsValidationError(err error) bool {
	var ve ValidationErrors
	return errors.As(err, &ve)
}
