# MOO2HD Implementation Plan

## Overview

This document outlines the phased implementation plan for MOO2HD, a faithful HD remake of Master of Orion 2. The project has no code yet; this plan defines the build order, key interfaces, and dependencies between systems.

---

## Phase 0 — Project Scaffold (Week 1)

**Goal:** Working build system and empty package skeleton.

### Tasks
- Initialize Go module structure matching the intended directory layout
- Write `Makefile` with targets: `build`, `test`, `lint`, `cross-compile` (Windows/macOS/Linux)
- Add CI-friendly `golint` and `gofmt` checks
- Wire up `ginkgo` + `gomega` test suite bootstrap
- Add `.gitignore` excluding `assets/` (LBX files are not committed)

### Deliverables
- `Makefile`
- Empty packages under `internal/` and `cmd/` with `doc.go` stubs
- `go.sum` with initial dependencies pinned

---

## Phase 1 — LBX Reader (`internal/lbx`) ✓ COMPLETE

**Goal:** Extract all asset types from original LBX archive files.

### Background
LBX is a proprietary archive format used by MicroProse. Each archive contains a table of offsets and a sequence of records that may be sprites, palettes, animations, sound effects, or music.

### Tasks
1. Implement LBX file header parser (magic bytes, record count, offset table)
2. Implement record type detection heuristics (sprite, animation, palette, sound, music)
3. Sprite extractor — decode raw pixel data + palette index into RGBA frames
4. Animation extractor — decode multi-frame sequences with per-frame delays
5. Palette extractor — read 256-color VGA palettes
6. Sound extractor — decode DigiSound/VOC or raw PCM blocks
7. Music extractor — handle MIDI or AdLib OPL data
8. CLI tool `cmd/lbxextract` — dumps all records from a given LBX file to disk

### Key Interfaces
```go
// internal/lbx/lbx.go
type Archive interface {
    Records() []Record
    Record(index int) (Record, error)
}

type RecordType int
const (RecordSprite, RecordAnimation, RecordPalette, RecordSound, RecordMusic RecordType = iota...)

type Record interface {
    Type() RecordType
    Data() []byte
}

type Sprite interface {
    Width, Height int
    Frames() []image.RGBA
}
```

### Test Strategy
- Unit tests against known-good LBX files (ship sprites, planet animations)
- Golden-file tests: extract record N from a specific LBX, compare byte-for-byte to a checked-in expected output

---

## Phase 2 — SVG/Vector Upscaler (`internal/svg`) ✓ COMPLETE

**Goal:** Convert low-res pixel art sprites to resolution-independent vector representations suitable for 1080p–4K rendering.

### Approach
Use a pixel-art upscaling algorithm (e.g., xBRZ or hqx) as a pre-processing step, then trace outlines with a vector tracing library to produce SVG paths. At runtime, the renderer scales SVG to the target resolution.

### Tasks
1. Integrate an xBRZ-style pixel scaler (pure Go or CGo shim)
2. Implement outline tracer: pixel clusters → closed SVG path elements
3. Palette-aware colorization: map palette index to fill/stroke colors
4. Export `image.Image` → SVG document
5. Benchmark: must convert a 64×64 sprite in < 10 ms

### Key Interface
```go
// internal/svg/upscale.go
type Upscaler interface {
    Upscale(src image.Image, palette color.Palette) (*svg.Document, error)
}
```

### Notes
- Pre-generate SVG assets at game startup (or lazily on first render)
- Cache SVG documents to disk keyed by LBX archive + record index + palette hash

---

## Phase 3 — YAML Config & Mod Loader (`configs/`, `internal/config`) (Week 6)

**Goal:** Load base game data and merge mod overrides in directory-name order.

### Tasks
1. Define Go structs for all base config types: `Race`, `Ship`, `Technology`, `Planet`, `Leader`, `Monster`, `Building`
2. Write YAML unmarshaller with validation (required fields, value ranges)
3. Implement mod loader:
   - Scan `mods/` subdirectories in sorted order
   - Deep-merge each mod's YAML over the base config (new keys added, existing keys overridden)
4. Expose a `Config` registry that game systems query

### Directory Layout
```
configs/
  races.yaml
  ships.yaml
  techs.yaml
  planets.yaml
  leaders.yaml
  buildings.yaml
mods/
  01-my-mod/
    races.yaml       # overrides specific race fields
    ships.yaml       # adds new ship hull
```

### Key Interface
```go
// internal/config/config.go
type Registry interface {
    Race(id string) (*Race, error)
    Ship(id string) (*Ship, error)
    Tech(id string) (*Technology, error)
    // ... etc.
}
```

---

## Phase 4 — Core Game State (`internal/game`) (Weeks 7–10)

This phase is the largest and is broken into sub-packages developed roughly in parallel once the config layer is stable.

### 4a — Galaxy Generation (`internal/game/galaxy`)

- Procedural galaxy generator: N star systems (up to 1000), star types, planets per system (up to 5)
- Travel lane graph (Dijkstra-ready adjacency list)
- Seed-reproducible generation (deterministic RNG for save/load and multiplayer sync)
- Data structures must scale: prefer flat slices + index maps over nested maps

### 4b — Colony Management (`internal/game/colony`)

- Colony struct: population, buildings, production queues, morale, food/prod/research outputs
- Build queue with priorities and auto-build rules
- Colony groups: tag multiple colonies, apply build-queue templates and rally points to all
- Rally points: newly built ships auto-route to a designated fleet waypoint

### 4c — Research Tree (`internal/game/research`)

- Tech tree graph loaded from YAML
- Per-race tech availability and bonuses
- Research point accumulation and discovery events

### 4d — Ship Design (`internal/game/ship`)

- Hull + component slot system loaded from YAML
- Design validation (mass limits, power requirements)
- Named designs saved per-player

### 4e — Tactical Combat (`internal/game/combat`)

- Square-grid battlefield (decide based on original game)
- Turn-order initiative system
- Weapon fire, shields, armor, damage propagation
- Auto-resolve fallback (used on timeout and for non-player encounters if desired)

### 4f — Colony Invasion (`internal/game/combat`)

- Ground troop combat (population-based attrition)
- Outcome: colony captured or defended

### 4g — Diplomacy & Espionage (`internal/game/diplo`)

- Relation scores between all faction pairs
- Treaties: non-aggression, trade, research sharing, alliance
- Actions: declare war, offer peace, propose alliance, trade tech, offer colony, pay credits
- Espionage missions: steal tech, sabotage, incite rebellion
- Diplomatic session (enters instanced phase in multiplayer)

### 4h — Leaders (`internal/game/leaders`)

- Leader pool: scientists, admirals, colony governors, spies
- Hiring auction, skill bonuses, assignment to ships/colonies
- Leader events (death, defection)

### 4i — Antaran & Space Monster Events (`internal/game/ai`)

- Scheduled Antaran raids (escalating difficulty)
- Attack Antaran homeworld option if jump gate is built
- Space monster roaming behavior and combat

### 4j — Faction AI (`internal/game/ai`)

- Per-faction personality: expansionist, militarist, researcher, diplomat, industrialist
- Strategic decision loop: colonize, build fleet, research, negotiate, espionage, attack
- Must run in < 1000 ms per faction per turn for up to 8 AI factions

---

## Phase 5 — Star Map UI (`internal/ui`) (Weeks 11–13)

### 5a — Renderer Setup

- Initialize SDL3 via `Zyko0/go-sdl3`
- Resolution: 1080p default, 4K supported; all coordinates in logical pixels scaled by DPI factor
- Render loop: fixed 60 Hz update, uncapped draw

### 5b — Zoom Level System

| Zoom Level | Content |
|---|---|
| Far | Faction-colored Voronoi regions |
| Mid | Colored star icons + travel lanes |
| Close | System names, ship stack icons, planet count badges |

- LOD switch on scroll delta thresholds
- Smooth animated transition between zoom levels

### 5c — Screens

- Star map (main view)
- System / planet detail
- Colony management screen
- Ship design screen
- Research screen
- Fleet movement orders
- Colony groups / rally points
- Diplomacy screen
- Combat screen (square grid)
- Leaders screen
- Turn summary / event log

---

## Phase 6 — Multiplayer (`internal/net`) (Weeks 14–16)

### Turn Protocol (Simultaneous-Turn)

```
Strategic phase (parallel, time-limited ~5 min default):
  - Each player acts independently
  - Client sends partial orders incrementally
  - Server accepts orders until "end turn" signal received or time expires

Resolution phase (server-side):
  - Resolve fleet movement
  - Detect encounters (combat, diplomacy)
  - Queue instanced tactical/diplomacy sessions

Tactical phase (instanced, time-limited):
  - Only involved players participate
  - Non-involved players receive notification only
  - Combat auto-resolves on timeout
  - Diplomacy aborts on timeout
```

### Tasks
1. WebSocket server (`coder/websocket`)
2. Message protocol (JSON or MessagePack) — define all message types
3. Session manager: lobby, game state sync, turn barrier
4. Reconnect handling: player can rejoin mid-game; server replays state delta
5. Spectator mode (receive state updates, no orders)

### Key Message Types
- `JoinLobby`, `LeaveLobby`, `StartGame`
- `SubmitOrders`, `EndTurn`
- `TurnResolved` (broadcast), `EnterCombat`, `EnterDiplomacy`
- `CombatAction`, `DiplomacyProposal`

---

## Phase 7 — Integration & Polish (Weeks 17–20)

- End-to-end single-player smoke test (galaxy gen → colony → research → combat → victory)
- Full multiplayer session test (2-player, all phases)
- Performance profiling: AI loop, rendering, galaxy gen
- Save/load system (serialize game state to gob)
- Audio integration (feed LBX-extracted sounds/music to SDL3 audio)
- Settings screen (resolution, keybindings, sound volume)
- Mod loading smoke tests with a sample mod

---

## Dependency Graph

```
Phase 0 (scaffold)
  └── Phase 1 (LBX reader)
        ├── Phase 2 (upscaler)          ← needs decoded sprites
        └── Phase 3 (config/mods)       ← independent of LBX, can start in parallel
              └── Phase 4 (game state)  ← needs config
                    ├── Phase 5 (UI)    ← needs game state + upscaled assets
                    └── Phase 6 (net)   ← needs game state
                          └── Phase 7 (integration)
```

---

## Open Questions / Decisions Needed

1. **LBX format spec** — Is a reverse-engineered spec available (e.g., from the MOO2 modding community) or do we need to reverse-engineer from scratch? - The LBX format is described at https://moddingwiki.shikadi.net/wiki/LBX_Format and there is a go implementation at https://codeberg.org/bazub/lbxtract-go
2. **Pixel-art upscaling algorithm** — xBRZ, hqx, or a custom ML-based approach? xBRZ is well-understood and has Go ports; ML is higher quality but complex. - Go with xBRZ for now, consider ML later if quality is insufficient.
3. **Combat grid** — Original MOO2 uses a free-placement field with move ranges. Confirm hex vs. square for the remake. - Keep original square layout for fidelity.
4. **Save format** — JSON (human-readable, moddable) vs. gob (compact, Go-native). - Stick with yaml for configs and gob for save files for performance.
5. **Networking library** — `coder/websocket` (actively maintained).
6. **Audio** — SDL3 audio API directly vs. a higher-level library (e.g., `beep`). SDL3 is already a dependency; prefer staying with it. - Yes, stick with SDL if possible.
7. **Maximum galaxy size** — The plan extends to 1000 systems vs. original 72. AI and pathfinding must be validated at scale early (Phase 4j). - Make sure to use concurrency and efficient data structures to handle this.

---

## Immediate Next Steps

1. Run `go mod tidy` and add initial dependencies (`go-sdl3`, `gopkg.in/yaml.v3`, `onsi/ginkgo/v2`, `onsi/gomega`, `coder/websocket`)
2. Create the directory skeleton and stub `doc.go` files
3. Write the `Makefile`
4. Begin Phase 1 (LBX reader) — this unblocks all asset-dependent work
