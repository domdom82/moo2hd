package galaxy

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// Options controls how a galaxy is generated.
type Options struct {
	Seed         uint64
	SystemCount  int     // 5–1000; default 70
	FactionCount int     // 1–8; default 8
	Width        float32 // map extent in arbitrary units; default 1000
	Height       float32
	MinDistance  float32 // minimum separation between systems; default 40
}

func (o *Options) setDefaults() {
	if o.SystemCount == 0 {
		o.SystemCount = 70
	}
	if o.FactionCount == 0 {
		o.FactionCount = 8
	}
	if o.Width == 0 {
		o.Width = 1000
	}
	if o.Height == 0 {
		o.Height = 1000
	}
	if o.MinDistance == 0 {
		o.MinDistance = 40
	}
}

// Generator produces Galaxy values deterministically from a seed.
type Generator struct {
	opts Options
	rng  *rand.Rand
}

// NewGenerator creates a Generator from the given options.
func NewGenerator(opts Options) *Generator {
	opts.setDefaults()
	rng := rand.New(rand.NewPCG(opts.Seed, opts.Seed^0xdeadbeef))
	return &Generator{opts: opts, rng: rng}
}

// Generate creates a new Galaxy. Calling Generate twice with the same seed
// yields identical results.
func (g *Generator) Generate() (*Galaxy, error) {
	n := g.opts.SystemCount
	if n < 2 {
		return nil, fmt.Errorf("galaxy: SystemCount must be >= 2, got %d", n)
	}
	if n > 1000 {
		return nil, fmt.Errorf("galaxy: SystemCount must be <= 1000, got %d", n)
	}

	systems := g.placeSystems(n)
	g.assignFactions(systems)
	planets := g.generatePlanets(systems)
	adj := g.assignSpecials(systems)

	nameMap := make(map[string]SystemID, len(systems))
	for i, s := range systems {
		nameMap[s.Name] = SystemID(i)
	}

	return &Galaxy{
		Seed:         g.opts.Seed,
		Systems:      systems,
		Planets:      planets,
		Adjacency:    adj,
		systemByName: nameMap,
	}, nil
}

// placeSystems distributes n systems with minimum separation across the map.
func (g *Generator) placeSystems(n int) []System {
	systems := make([]System, 0, n)
	maxAttempts := n * 200

	for len(systems) < n && maxAttempts > 0 {
		maxAttempts--
		x := g.rng.Float32() * g.opts.Width
		y := g.rng.Float32() * g.opts.Height
		if !g.tooClose(x, y, systems) {
			id := SystemID(len(systems))
			systems = append(systems, System{
				ID:         id,
				Name:       systemName(int(id)),
				X:          x,
				Y:          y,
				Star:       g.randomStar(),
				Size:       g.randomSize(),
				WormholeTo: -1,
			})
		}
	}
	return systems
}

func (g *Generator) tooClose(x, y float32, systems []System) bool {
	minD2 := g.opts.MinDistance * g.opts.MinDistance
	for _, s := range systems {
		dx := x - s.X
		dy := y - s.Y
		if dx*dx+dy*dy < minD2 {
			return true
		}
	}
	return false
}

// assignFactions seeds one homeworld per faction at canonical map positions
// (corners, mid-edges, centre) then assigns every system to the nearest seed,
// producing contiguous territorial blobs suitable for the far zoom view.
func (g *Generator) assignFactions(systems []System) {
	n := g.opts.FactionCount
	if n <= 0 {
		return
	}
	w, h := g.opts.Width, g.opts.Height
	pad := float32(0.1) // keep seeds 10% in from each edge

	// Canonical seed positions for up to 8 factions.
	// Order: 4 corners, 4 mid-edges. Centre only if n==1.
	allSeeds := [][2]float32{
		{pad * w, pad * h},             // top-left
		{(1 - pad) * w, pad * h},       // top-right
		{pad * w, (1 - pad) * h},       // bottom-left
		{(1 - pad) * w, (1 - pad) * h}, // bottom-right
		{0.5 * w, pad * h},             // top-mid
		{0.5 * w, (1 - pad) * h},       // bottom-mid
		{pad * w, 0.5 * h},             // left-mid
		{(1 - pad) * w, 0.5 * h},       // right-mid
	}

	seeds := allSeeds[:n]
	for i := range systems {
		best := 0
		bestD := float32(math.MaxFloat32)
		for f, s := range seeds {
			dx := systems[i].X - s[0]
			dy := systems[i].Y - s[1]
			if d := dx*dx + dy*dy; d < bestD {
				bestD = d
				best = f
			}
		}
		systems[i].Faction = best
	}
}

// assignSpecials rolls a special property for each system according to the
// MOO2 distribution, then pairs wormhole systems. It returns the adjacency
// slice (non-nil only for wormhole pairs, stored bidirectionally at distance 1).
func (g *Generator) assignSpecials(systems []System) [][]Lane {
	n := len(systems)
	adj := make([][]Lane, n)

	specials := []Special{
		SpecialNone, SpecialPlanet, SpecialWormhole,
		SpecialShipDebris, SpecialPirateCache, SpecialLostHero,
	}
	weights := []int{78, 10, 5, 2, 2, 2} // sum = 99; close enough to 100

	// First pass: roll each system's special independently.
	for i := range systems {
		systems[i].Special = pickWeighted(g.rng, specials, weights)
	}

	// Second pass: pair wormhole systems.
	// eligible = indices with no special yet (will be used as wormhole targets).
	// We iterate wormhole candidates in order; for each we pick a random
	// eligible partner that has not yet been assigned any special.
	eligible := make([]int, 0, n)
	for i := range systems {
		if systems[i].Special == SpecialNone {
			eligible = append(eligible, i)
		}
	}

	for i := range systems {
		if systems[i].Special != SpecialWormhole {
			continue
		}
		// Skip systems that were already assigned as a wormhole target in a
		// previous iteration (their WormholeTo is already set).
		if systems[i].WormholeTo >= 0 {
			continue
		}
		if len(eligible) == 0 {
			// No free partner — downgrade this system.
			systems[i].Special = SpecialNone
			continue
		}
		// Pick a random eligible partner.
		pick := g.rng.IntN(len(eligible))
		j := eligible[pick]
		// Remove j from eligible by swapping with the last element.
		eligible[pick] = eligible[len(eligible)-1]
		eligible = eligible[:len(eligible)-1]

		// Link the pair.
		systems[i].WormholeTo = SystemID(j)
		systems[j].WormholeTo = SystemID(i)
		systems[j].Special = SpecialWormhole

		// Add bidirectional adjacency with distance=1 (one turn travel).
		adj[i] = append(adj[i], Lane{To: SystemID(j), Distance: 1})
		adj[j] = append(adj[j], Lane{To: SystemID(i), Distance: 1})
	}

	return adj
}

// generatePlanets creates planets for all systems and attaches their IDs.
func (g *Generator) generatePlanets(systems []System) []Planet {
	planets := make([]Planet, 0, len(systems)*3)
	for i := range systems {
		slots := g.slotsForSize(systems[i].Size)
		for slot := 1; slot <= slots; slot++ {
			pid := PlanetID(len(planets))
			p := Planet{
				ID:       pid,
				SystemID: SystemID(i),
				Slot:     slot,
				Class:    g.randomPlanetClass(),
				Richness: g.randomRichness(),
				Gravity:  g.randomGravity(),
				MaxPop:   g.rng.IntN(13) + 3, // 3–15
				Size:     g.rng.IntN(5) + 1,  // 1–5
			}
			planets = append(planets, p)
			systems[i].Planets = append(systems[i].Planets, pid)
		}
	}
	return planets
}

func (g *Generator) slotsForSize(sz SystemSize) int {
	switch sz {
	case SizeSmall:
		return 1 + g.rng.IntN(2) // 1–2
	case SizeLarge:
		return 3 + g.rng.IntN(3) // 3–5
	default:
		return 2 + g.rng.IntN(2) // 2–3
	}
}

func (g *Generator) randomStar() StarType {
	types := []StarType{StarYellow, StarBlue, StarWhite, StarOrange, StarRed, StarBrown, StarNeutron, StarBlackHole}
	weights := []int{30, 10, 15, 20, 15, 5, 3, 2} // rough MOO2-like distribution
	return pickWeighted(g.rng, types, weights)
}

func (g *Generator) randomSize() SystemSize {
	switch g.rng.IntN(3) {
	case 0:
		return SizeSmall
	case 1:
		return SizeLarge
	default:
		return SizeMedium
	}
}

var planetClasses = []string{
	"terran", "ocean", "arid", "desert", "tundra", "swamp",
	"volcanic", "barren", "radiated", "toxic", "inferno", "none",
}

var richnesses = []string{"ultra-poor", "poor", "abundant", "rich", "ultra-rich"}
var gravities = []string{"low", "normal", "high"}

func (g *Generator) randomPlanetClass() string {
	return planetClasses[g.rng.IntN(len(planetClasses))]
}

func (g *Generator) randomRichness() string {
	return richnesses[g.rng.IntN(len(richnesses))]
}

func (g *Generator) randomGravity() string {
	return gravities[g.rng.IntN(len(gravities))]
}

func pickWeighted[T any](rng *rand.Rand, items []T, weights []int) T {
	total := 0
	for _, w := range weights {
		total += w
	}
	r := rng.IntN(total)
	for i, w := range weights {
		r -= w
		if r < 0 {
			return items[i]
		}
	}
	return items[len(items)-1]
}

// systemName returns a deterministic name for a system index.
var systemNames = []string{
	"Sol", "Vega", "Altair", "Deneb", "Sirius", "Capella", "Betelgeuse",
	"Arcturus", "Antares", "Spica", "Fomalhaut",
	"Regulus", "Achernar", "Hadar", "Acrux", "Mimosa", "Gacrux",
	"Shaula", "Sargas", "Kaus", "Nunki", "Zuben", "Zavijava", "Zaniah",
	"Vindemiatrix", "Minelauva", "Syrma", "Izar", "Nekkar", "Seginus", "Alkalurops",
	"Muphrid", "Rastaban", "Eltanin", "Grumium", "Kuma", "Altais",
	"Gianfar", "Tyl", "Aldhibah", "Alwaid", "Nodus", "Edasich", "Alpheratz",
	"Mirach", "Almach", "Algol", "Menkalinan", "Almaaz",
	"Alhena", "Mebsuta", "Mekbuda", "Alzirr", "Adhafera", "Rasalhague",
	"Sabik", "Kornephoros", "Formalhaut", "Zubenelgenubi", "Zubeneschamali", "Graffias", "Rasalgethi",
	"Kaus Australis", "Kaus Media", "Kaus Borealis", "Ascella", "Albaldah", "Alshain", "Acrab", "Dschubba",
	"Nebula", "Helios", "Lacaille", "Alphard", "Alnair", "Alsephina", "Alshat", "Alfirk", "Alrescha",
	"Alniyat", "Algedi", "Dabih", "Ras Algethi", "Mirza", "Yed", "Warikomi", "Laan",
	"Meklon", "Moro", "Pax", "Draconis", "Lir", "Frere", "Taben", "Leng", "Elkis",
	"Rosa", "Yhe", "Nazin", "San", "Mihr", "Orion", "Oba", "Sssla", "Gemat", "Pleides", "Oberon", "Min",
	"Zib", "Yue", "Malec", "Opus", "Gigas", "Wag", "Distaff", "Ursa",
	"Xoth", "Vela", "Bier", "Slkim", "Roch", "Gnol", "Erebus", "Hades", "Juno", "Kronos", "Leto", "Nereus", "Phobos",
	"Rhea", "Saturn", "Titan", "Umbra", "Vesta", "Wotan", "Xena", "Ymir", "Zeus", "Alcyone", "Atlas", "Calypso",
	"Dione", "Enceladus", "Hyperion", "Janus", "Mimas", "Pandora", "Phoebe", "Prometheus", "Tethys", "Telesto",
	"Helene", "Epimetheus", "Himalia", "Elara", "Pasiphae", "Sinope",
	"Carme", "Ananke", "Leda", "Themisto", "Callirrhoe", "Megaclite", "Taygete", "Chaldene", "Harpalyke", "Kalyke",
	"Io", "Europa", "Ganymede", "Amalthea", "Nova", "Aether", "Nyx", "Ere", "Hebe", "Thebe", "Adrastea",
	"Iocaste", "Erinome", "Isonoe", "Praxidike", "Autonoe", "Thyone", "Taurus",
	"Hermippe", "Aitne", "Eurydome", "Euanthe", "Euporie", "Orthosie", "Sponde", "Kale", "Pasithee",
	"Centauri", "Hydra", "Perseus", "Cassiopeia", "Cygnus", "Lyra", "Aquila", "Ptolemy", "Paranar", "Vox", "Parma", "Siru",
	"Uzith", "Paxn", "Ecuar", "Lirr", "Yedeus", "Kif", "Zin", "Nirb", "Goi", "Taoast", "Vijzar", "Ras", "Yher",
	"Yenbaran", "Voxh", "Woza", "Saliba", "Wool", "Sanomeda", "Wegat", "Ellq", "Turrktos", "Urflae", "Rex", "Quiurus",
	"Yadb", "Kaes", "Huttis", "Zibam", "Ajieh", "Yueh", "Buemis", "Ummllus", "Obarion", "Wagi", "Rhai", "Larx",
	"Minra", "Orflar", "Tain", "Zak", "Tahon", "Hapbo", "Okitis", "Weilon", "Tyrei", "Ravm", "Quek", "Inakda", "Thog", "Naam", "Damea",
	"Pavo", "Iras", "Ceti", "Iremoora", "Irra", "Ishi", "Velan", "Vegaer", "Izara", "Seki", "Jabos", "Vaht", "Yian", "Vagnium",
	"Jobetes", "Choom", "Zonaus", "Juga", "Juka", "Sadr", "Juzala", "Kaff", "Kalbla", "Kahtn", "Unuklla", "Kakes", "Kalib",
	"Moyosto", "Pernus", "Enzu", "Naosus", "Narnla", "Kled", "Nathsa", "Knaaa", "Urnar", "Erosria", "Komius", "Mulano", "Kothhais",
	"Saak", "Okabi", "Exisuri", "Ezore", "Ezra", "Laann", "Tali", "Foo", "Fahde", "Okda", "Adib", "Tais", "Lengzin",
	"Ogka", "Caphria", "Ferassa", "Rubariom", "Linxrdia", "Finnau", "Lonela", "Loraa", "Sirus", "Luraa", "Xeen", "Lyae", "AyilLyra",
	"Maazo", "Ukko", "Suud", "Magi", "Wolf", "Mahu", "Olor", "Sung", "Crux", "Rosaan", "Adenin", "Rochi", "Suji", "Yuzh", "Wain",
	"Udha", "Oomaius", "Opuss", "Smon", "Ubarn", "Zirrff", "Deli", "Muru", "Skaio", "Geph", "Getaa", "Rialsa", "Ghavs", "Rheael",
	"Gion", "Yothis", "Moroch", "Shihm", "Gote", "Grusna", "Rana", "Gularn", "Ramono", "Barvra", "Guad", "Osae", "Oshiad", "Wazna",
	"Haliia", "Rabe", "Pund", "Trax", "Hapion", "Voore", "Hasenus", "Ymarne", "Hayk", "Tonds", "Bieron", "Niru", "Hera", "Tome",
	"Tiam", "Shen", "Nobi", "Diab", "Biots", "Shatr", "Dolza", "Aran", "Sahu", "Alar", "Thok", "Ibis", "Boloe", "Ilya", "Immaus",
	"Malecaut", "Malusa", "Wesen", "Celox", "Abeen", "Sukra", "Matar", "Media", "Wesatgo", "Anraqo", "Mensa", "Cerva", "Cetuse",
	"Slkima", "Mirak", "Mishay", "Mizar", "Chiba", "Ching", "Shira", "Simak", "Munic", "Chort", "Nasak", "Nagar", "Necht",
	"Sentes", "Sella", "Selial", "Nilusis", "Wabir", "Waage", "Nisusm", "Wasat", "Notonn", "Segel", "Obacas", "Arkab", "Crius", "Saxon",
	"Croix", "Croth", "Satyraut", "Omega", "Sarwa", "Orbis", "Oriab", "Argusmi", "Cygni", "Cursa", "Otawa", "Scera", "Padus", "Sarti",
	"Saiph", "Parmag", "Sahil", "Dagon", "Delta", "Saganes", "Perenis", "Sadakon", "Dabanel", "Dannu", "Pesci", "Phakt", "Dante",
	"Pherd", "Ryoun", "Ruzam", "Phyco", "Waghi", "Zeraha", "Rotan", "Plinyorea", "Rlyeh", "Rigel", "Ribat", "Demhe", "Dendo", "Remus",
	"Rebarus", "Ramad", "Denebk", "Rahab", "Qablu", "Qatar", "Derke", "Achir", "Zweig", "Virgoa", "Dhira", "Aroch", "Draco", "Zaoth",
	"Zaman", "Vespam", "Dumke", "Ecber", "Varak", "Vardi", "Elkisus", "Atari", "Enoch", "Ensis", "Uxmai", "Atsui", "Ursus", "Audax",
	"Ungal", "Esper", "Etana", "Fakar", "Zaban", "Axion", "Felis", "Fides", "Firma", "Fixas", "Urkab", "Fovea", "Akrab",
	"Gadjoa", "Galos", "Gatto", "Udarapha", "Genam", "Gammaon", "Baham", "Balik", "Tycho", "Tukan", "Gorra",
	"Tsuke", "Tsugi", "Tsath", "Habor", "Hague", "Tsangs", "Hally", "Berel", "Zubana", "Benat", "Ynarl", "Helos", "Thraxr",
	"Thraa", "Hewel", "Hindea", "Zosma", "Hontes", "Horne", "Horus", "Hoshi", "Yifnehan", "Thoth", "Inari", "Indus", "Thiba", "Alcor",
	"Borea", "Thaur", "Ixion", "Boshi", "Jacob", "Testa", "Janib", "Jinga", "Tejat", "Jugum", "Justaa", "Katab", "Bulanen", "Bunda",
	"Keats", "Keeta", "Kelev", "Telum", "Taurio", "Kluhn", "Knyan", "Burin", "Alepha", "Yekubi", "Alphar", "Krebs", "Zobna", "Yarak",
	"Talas", "Ladon", "Laius", "Canis", "Xolas", "Lepus", "Xenona", "Tabena", "Lince", "Lomar", "Lupus", "Zinge", "Sutuls",
	"Abbith", "Acamar", "Anchat", "Arkham", "Arlyehm", "Aurora", "Azimon", "Baalbo", "Bagdei", "Bestia", "Birdon", "Bogina", "Bootes",
	"Brutum", "Butain", "Cadmus", "Caeleb", "Carinax", "Castor", "Celtsi", "Cephee", "Charon", "Chelae", "Corona", "Corvus", "Cressas",
	"Cryptos", "Degiri", "Desmos", "Dilgan", "Dorado", "Drakka", "Dromos", "Dilfim", "Elysia", "Ercole", "Erulus", "Faelis", "Fafnir",
	"Fascia", "Fische", "Fullen", "Fundusm", "Fuseki", "Gabbar", "Gharne", "Ghamus", "Gibber", "Girtab", "Gurion", "Hammis", "Hamete",
	"Harag", "Hasami", "Hastur", "Hiraki", "Hircus", "Hyades", "Hydrae", "Icarus", "Invaka", "Ishtar", "Jabbar", "Jataka",
	"Joseki", "Joshua", "Kadath", "Kailis", "Kakari", "Kakata", "Khumba", "Kochba", "Kolath", "Komokun", "Kosumi", "Krakena",
	"Ktyngae", "Kyusho", "Lamash", "Lawdon", "Lerion", "Lesath", "Lurnis", "Lycaon", "Maalor", "Madira", "Magaria", "Magnus", "Manica",
	"Manzil", "Markab", "Menita", "Menalo", "Merope", "Milaff", "Miract", "Mirror", "Mochos", "Monkar", "Monius", "Morrig", "Mulban",
	"Mutlat", "Nessusm", "Nimbus", "Ninsar", "Niphla", "Noctuan", "Nordia", "Nortes", "Obelus", "Oberona", "Oculus", "Ogeima",
	"Olorunnis", "Origen", "Orloge", "Osiris", "Passer", "Patera", "Pavoner", "Perseas", "Perseo", "Phecca", "Pictor", "Pindarma", "Pipiri",
	"Pollus", "Pollux", "Praxis", "Propus", "Pythones", "Quayal", "Radiff", "Rayden", "Revati", "Rhilus", "Rishka", "Riwand", "Rukubi",
	"Sabaki", "Samson", "Sarfah", "Sargon", "Schiff", "Scheat", "Schwan", "Seidon", "Semeai", "Septumi", "Sharru", "Shicho", "Shwing",
	"Siktut", "Simius", "Situla", "Sonans", "Stalaz", "Sulcus", "Syrius", "Tenuki", "Tesuji", "Tewari", "Thales", "Thuban", "Tigrisd",
	"Tinnan", "Trifid", "Trigon", "Tucana", "Tyriusus", "Ubboth", "Ulgher", "Ulthar", "Urania", "Ursulas", "Ussika", "Valkyr", "Vector",
	"Vhoorl", "Virgil", "Volans", "Vulcan", "Vuples", "Whynil", "Wilderr", "Willow", "Wirius", "Yarrow", "Yildum", "Zaurak",
	"Zenithnd", "Zibbat", "Zichos", "Zirkel", "Zoctan", "Zubrahr", "Arietis", "Pegasus", "Zhadoom", "Miranda", "Minerva", "Dauphin",
	"Zhardan", "Camelus", "Darrian", "Corolla", "Corbeau", "Quandra", "Pelenor", "Phantos", "Quercia", "Pharmaz", "Phobeus", "Proxima",
	"Proteus", "Souchis", "Mariner", "Procyon", "Proctor", "Yaddith", "Proclus", "Priapus", "Statius", "Niallar", "Phoenix",
	"Porrima", "Ponnuki", "Stellio", "Rhombus", "Marindi", "Polaris", "Maretta", "Risabam", "Celaeno", "Mane", "Go", "Poculum",
	"Romulas", "Xanthus", "Madorla", "Pistrix", "Piatuos", "Numinos", "Octipes", "Olathoe", "Leonids", "Lemuria", "Laniger", "Lacerta",
	"Omicron", "Kulthos", "Tambiru", "Tammech", "Carcosa", "Serapis", "Serpens", "Tarazad", "Tarcuta", "Tatiana", "Sarnathi", "Aquilae",
	"Shaghar", "Khrissa", "Shalako", "Templum", "Bussola", "Tericas", "Jaculum", "Bungula", "Bubulus", "Ivielda", "Inganok",
	"Thesbia", "Ceginus", "Ovillus", "Hyboria", "Muscida", "Xengara", "Thuggon", "Musator", "Shimari", "Blucher", "Alacast", "Shinogi",
	"Tirstar", "Mulubat", "Hawking", "Toranor", "Paladiaus", "Montone", "Trethon", "San San", "Alaozar", "Triones", "Tripton", "Tristan",
	"Guradas", "Gryphon", "Barcada", "Gontzol", "Gladius", "Geodesy", "Xiclotl", "Babylon", "Gehenna",
	"Baaltis", "Gazelle", "Galileo", "Xiphias", "Fortuna", "Fluvius", "Avellar", "Algeiba", "Escalon", "Zarijan", "Erigone",
	"Actaeus", "Scorpio", "Canopus", "Epsilon", "Endoria", "Elvarad", "Electra", "Elcorno", "Einhorn", "Mithras", "Valusia", "Echidna",
	"Varinia", "Artemis", "Dunwich", "Dunatis", "Veneris", "Drossel", "Sagitta", "Dramasa", "Takamoku", "Scheader", "Scutulums", "Cathuria",
	"Nubilium", "Neptunus", "Nephthys", "Xendalla", "Mutatrix", "Monstrum", "Steinbok", "Callisto", "Magdalen", "Syntaxis", "Tantalus",
	"Klystron", "Tarandus", "Cabrilla", "Jordanus", "Browntes", "Incedius", "Herschel", "Hermidon", "Brachium", "Herculis", "Tindalos",
	"Heracles", "Hanekomi", "Gorgonis", "Tyrannus", "Galapago", "Eridanus", "Asterion", "Ascellus", "Volantis", "Denubius",
	"Profugus", "Proditor", "Collassa", "Reticuli", "Cimmeria", "Pomptina", "Polarion", "Chorazin", "Chay Foo", "Plastrum",
	"Rosemund", "Sabazius", "Perizoma", "Paradise", "Panthera", "Palaemon", "Saltator", "Zothique", "Scaliger", "Bethmoora",
	"Psi Gamma", "Quadrupes", "Celephais", "Kit Alpha", "Aldebaran", "Sarkomand", "Tau Cygni", "Hadramaut", "Antarktos",
	"Leviathan", "Beta Ceti", "Concordia", "Andromeda", "Rutilicus", "Commoriom", "Vitruvious", "Tormantius", "Hyperborea", "Poseidonis",
	"Einstein", "Hippocampus", "Cthugha", "Yuggoth", "Iapetus", "Cthulhu", "Nyarlathotep", "Shub-Niggurath", "Azathoth", "Tsathoggua",
	"Yog-Sothoth", "Nodens", "Atlach-Nacha", "Cthylla", "Ghatanothoa", "Abhoth", "Bokrug", "Chaugnar Faugn",
	"Eihort", "Glaaki", "Ithaqua", "Kthanid", "Lloigor", "Mnomquah", "Quachil Uttaus", "Rhan-Tegoth", "Shantak",
	"Yidhra", "Zhar", "Zvilpogghua", "Riemann", "Poincare", "Hilbert", "Noether", "Turing", "Godel", "Cantor", "Ramanujan", "Fermat", "Euler",
	"Gauss", "Lagrange", "Laplace", "Newton", "Kepler", "Copernicus", "Hubble", "Curie",
	"Feynman", "Bohr", "Planck", "Dirac", "Heisenberg", "Schrodinger", "Maxwell", "Faraday", "Tesla",
	"Archimedes", "Pythagoras", "Euclid", "Aristotle", "Plato", "Socrates", "Confucius", "Laozi", "Sun Tzu", "Mencius",
	"Rumi", "Hafez", "Omar Khayyam", "Shakespeare", "Dante Alighieri", "Homer",
	"Beethoven", "Mozart", "Bach", "Chopin", "Tchaikovsky", "Vivaldi",
	"Picasso", "Van Gogh", "Da Vinci", "Michelangelo", "Rembrandt", "Monet",
	"Warhol", "Dali", "Kahlo", "Hokusai", "Klimt", "Matisse", "Pollock", "Rothko", "Basquiat",
	"Rodin", "Brancusi", "Giacometti", "Calder", "Moore", "Hepworth",
	"Nevelson", "Oldenburg", "Smith", "Chagall", "Miro", "Klee",
	"Magritte", "Duchamp", "Man Ray", "Rauschenberg", "Johns", "Lichtenstein",
	"Rothenberg", "Haring", "Koons", "Hirst", "Murakami", "Yayoi Kusama", "Takashi Murakami", "Banksy",
	"Tatooine", "Alderaan", "Hoth", "Dagobah", "Endor", "Naboo", "Coruscant", "Mustafar", "Kamino", "Geonosis", "Kashyyyk",
	"Yavin", "Bespin", "Jakku", "Scarif", "Crait", "Ahch-To", "Exegol", "D'Qar", "Takodana", "Starkiller Base",
	"Felucia", "Ryloth", "Sullust", "Muunilinst", "Malastare", "Saleucami", "Ord Mantell", "Mon Cala",
	"Corellia", "Kessel", "Lothal", "Dathomir", "Ilum", "Jedha", "Eadu",
	"Cantonica", "Kijimi", "Mimban", "Rodia", "Nal Hutta", "Nar Shaddaa",
	"Ferenginar", "Qo'noS", "Romulus", "Cardassia", "Bajor", "Trill", "Andoria", "Tellar",
	"Tirith", "Eregion", "Gondor", "Rohan", "Mordor", "Isengard", "Helm", "Minas", "Fangorn", "Lothlorien", "Mirkwood",
	"Rivendell", "Shire", "Bree", "Weathertop", "Edoras", "Dol Guldur", "Barad-dur", "Cirith Ungol", "Doom", "Osgiliath",
	"Morgul", "Oine", "Gimli", "Legolas", "Aragorn", "Boromir", "Frodo", "Sam", "Merry", "Pippin", "Gandalf",
	"Elrond", "Galadriel", "Saruman", "Sauron", "Gollum", "Bilbo", "Thorin", "Balin", "Dwalin", "Kili", "Fili",
	"Bombur", "Bifur", "Bofur", "Dori", "Nori", "Ori", "Smaug", "Azog", "Bolg", "Radagast",
	"Thranduil", "Tauriel", "Beorn", "Bard", "Erebor", "Dale", "Esgaroth",
	"Rhosgobel", "Dol Amroth", "Pelargir", "Umbar", "Harad", "Khand", "Rhûn",
	"Angmar", "Forodwaith", "Dunland", "Lossarnach", "Anfalas", "Lamedon", "Lebennin", "Dorwinion", "Eriador",
	"Sabaton", "Slayer", "Metallica", "Maiden", "Anthrax", "Judas", "Sabbath", "Pantera", "Slipknot", "Sevenfold",
	"Rammstein", "Korn", "Godsmack", "Shinedown", "Seether", "Grace", "Roach", "Chevelle", "Volbeat", "Halestorm", "Evanescence",
	"Paramore", "Shredder", "Krang", "Sevendust",
	"Mesa", "Gaia", "Ares", "Apollo", "Hermes", "Athena", "Poseidon", "Demeter", "Hephaestus", "Dionysus",
	"Aphrodite", "Cronus", "Uranus", "Thanatos", "Hypnos", "Morpheus", "Nemesis", "Tyche", "Nike",
	"Anubis", "Ra", "Isis", "Set", "Bastet", "Sekhmet", "Hathor",
	"Ptah", "Sobek", "Amun", "Khnum", "Taweret", "Bes", "Anuket", "Hapi", "Khepri",
	"Ma'at", "Seshat", "Serqet", "Geb", "Nut", "Shu", "Tefnut",
	"Loki", "Heimdallr", "Tyr", "Baldr", "Njord", "Skadi", "Bragi",
	"Idun", "Hodr", "Vidar", "Vili", "Ve", "Sif", "Forseti", "Ullr", "Hela",
	"Jormungandr", "Fenrir", "Sleipnir", "Ratatoskr", "Nidhogg",
	"Yggdrasil", "Bifrost", "Mimir", "Huginn", "Muninn", "Frey", "Gefjon", "Hnoss", "Gullveig",
	"Skuld", "Verdandi", "Urd", "Valkyrie", "Einherjar", "Ragnarok",
	"Midgard", "Asgard", "Vanaheim", "Alfheim", "Svartalfheim", "Jotunheim", "Niflheim", "Muspelheim",
	"Helheim", "Vigrid", "Hvergelmir", "Yggdrasill", "Gjallarhorn",
	"Fimbulwinter", "Surtur", "Skoll", "Hati", "Garmr",
	"Frigg", "Freyja", "Odin", "Thor", "Hel", "Audhumla",
	"Moria", "Gondolin", "Nargothrond", "Doriath", "Beleriand", "Angband", "Tol Eressea", "Aman", "Valinor", "Taniquetil",
	"Manwe", "Varda", "Ulmo", "Aulë", "Yavanna", "Nienna", "Mandos", "Lorien", "Tulkas",
	"Melkor", "Ungoliant", "Glaurung", "Ancalagon", "Thangorodrim",
	"Feanor", "Fingolfin", "Finarfin", "Maedhros", "Maglor", "Celegorm",
	"Curufin", "Caranthir", "Amrod", "Amras", "Thingol", "Melian",
	"Earendil", "Elwing", "Tuor", "Idril", "Glorfindel", "Ecthelion", "Gil-galad", "Círdan",
	"Finrod", "Angrod", "Aegnor", "Orodreth", "Beleg", "Beren", "Luthien",
	"Raynor", "Kerrigan", "Zeratul", "Artanis", "Tassadar", "Fenix", "Stukov", "Alarak", "Karax", "Vorazun",
	"Rexxar", "Chen", "Li-Ming", "Medivh", "Deckard", "Samuro", "Valeera", "Maiev", "Illidan", "Malfurion",
	"Thrall", "Jaina", "Anduin", "Sylvanas", "Garrosh", "Varian", "Tyrande", "Vol'jin", "Gul'dan",
	"Kel'Thuzad", "Anub'arak", "Cho'gall", "Ner'zhul",
	"Ajur", "Char", "Korhal", "Mar Sara", "Shakuras", "Tarsonis", "Umojan", "Zerus", "Bel'Shir", "Braxis", "Coda", "Dusk", "Eldarim",
	"Goroth", "Haven", "Jovian", "Korriban", "Lunara", "Nexus", "Oblivion",
	"Quarantine", "Ravager", "Sanctum", "Tomb", "Ulduar", "Vortex", "Xel'Naga", "Ysera",
	"Zul'Gurub", "Zul'Aman", "Zul'Drak", "Zul'Farrak",
	"Zebes", "Tallon", "Brinstar", "Norfair", "Maridia", "Crateria", "Phendrana", "Chozo",
	"Tourian", "Ridley", "Kraid", "Metroid", "Samus", "Phazon", "Crater", "Magmoor", "Orpheon", "Burenia",
	"Reagan", "Bush", "Clinton", "Obama", "Trump", "Biden", "Roosevelt", "Kennedy", "Johnson", "Nixon",
	"Ford", "Carter", "Washington", "Adams", "Jefferson", "Madison", "Monroe", "Jackson", "Van Buren",
	"Harrison", "Tyler", "Polk", "Taylor", "Fillmore", "Pierce", "Buchanan",
	"Lincoln", "Grant", "Hayes", "Garfield", "Arthur", "McKinley",
	"Taft", "Wilson", "Harding", "Coolidge", "Hoover", "Truman", "Eisenhower",
	"Kirk", "Picard", "Sisko", "Janeway", "Archer", "Burnham", "Riker", "Data", "Worf", "Troi", "Crusher",
	"Spock", "McCoy", "Scotty", "Uhura", "Chekov", "Sulu", "La Forge", "Dax", "Kira", "Quark",
	"Nog", "Bashir", "Odo", "Weyoun", "Garak", "Martok",
	"Cook", "Drake", "Halsey", "Hopper", "Kirkland", "Miller", "Nye", "Sagan", "Tyson", "Watson", "Goodall",
	"Magellan", "Columbus", "Vespucci", "Cabot", "Balboa", "Pizarro", "Cortez",
}

// Prior, Posterior, and other suffixes are used to distinguish multiple systems in the same star cluster or binary system.

// Minor, Major, and other prefixes are used to distinguish multiple systems in the same star cluster or binary system.

// A, B, C, etc. are used to distinguish multiple stars in a system.

func systemName(i int) string {
	if i < len(systemNames) {
		return systemNames[i]
	}
	return fmt.Sprintf("System-%d", i)
}
