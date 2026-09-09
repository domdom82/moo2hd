# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MOO2HD is a faithful HD remake of Master of Orion 2 (1996) with modern extensions. The game uses the original LBX asset files (graphics, animations, sounds, music) — the player must own a copy. All config is YAML; mods work by dropping YAML overrides into a `mods/` subdirectory.

## Technology Choices (Planned / To Be Decided)

No code exists yet. When starting implementation, prefer:
- **Language**: Go (cross-platform, open-source friendly, good performance)
- **Rendering**: SDL3 or a game framework that supports 1080p–4K and SVG-like vector scaling
- **Networking**: WebSockets for simultaneous-turn multiplayer
- **Config**: YAML everywhere (game data, mods, server config)
- **Build**: A single `Makefile` or equivalent that can cross-compile for Windows, macOS, Linux

## Intended Project Structure

```
moo2hd/
├── assets/           # runtime-loaded originals (LBX files, not committed)
├── mods/             # mod subdirs, loaded in order; each may contain YAML overrides
├── internal/
│   ├── lbx/          # LBX file reader/extractor
│   ├── svg/          # low-res → vector upscaler
│   ├── game/         # core game state, rules, AI
│   │   ├── galaxy/   # star map, systems, planets
│   │   ├── colony/   # colony management, build queues, groups, rally points
│   │   ├── combat/   # tactical space combat
│   │   ├── research/ # tech tree
│   │   ├── diplo/    # diplomacy & espionage
│   │   └── ai/       # faction AI (expansionist, militarist, researcher, etc.)
│   ├── net/          # multiplayer server, simultaneous-turn protocol
│   └── ui/           # star map renderer, all game screens
├── cmd/
│   ├── moo2hd/       # game binary entry point
│   └── lbxextract/   # standalone LBX extraction tool
└── config/           # base YAML game data (races, ships, techs, etc.)
```

## Key Architectural Constraints

**Multiplayer turn model** — Simultaneous turns, not sequential:
1. *Strategic phase*: all players act in parallel; configurable time limit (default 5 min); server waits for all "end turn" signals.
2. *Resolution phase*: server resolves fleet movement, detects encounters, queues tactical/diplomacy instances.
3. *Tactical phase*: instanced — only involved players enter combat or diplomacy sessions; others are notified but unaffected. Both types are time-limited; combat auto-resolves on timeout, diplomacy aborts.

**Star map zoom levels** — Level-of-detail must be computed per zoom tier:
- Far: faction-colored regions only
- Mid: colored star systems + travel lanes
- Close: system names, iconified ship stacks, planet count

**Mod loading** — YAML config files in `mods/` subdirs are merged over base `configs/` values in directory-name order. Any base config key can be overridden; new entries can be added.

**LBX reader** — Must extract sprites, palettes, animations, sounds, and music from original LBX archive format. Output feeds both the SVG upscaler and the runtime renderer.

**Maximum galaxy size** — Up to 1000 star systems × 5 planets each. Data structures and AI must scale accordingly (original cap was ~72 systems).

## Game Feature Checklist (for tracking implementation progress)

Core engine: LBX reader · SVG upscaler · YAML config + mod loader · zoomable star map renderer  
Single-player loop: galaxy gen · colony management · auto-build queues · colony groups · rally points · research tree · ship design · tactical combat · colony invasion · diplomacy · espionage · leaders · Antarans · space monsters · faction AI  
Multiplayer: simultaneous-turn server · instanced tactical/diplomacy phases · time-limited turns · lobby/session config

## Developer Notes
- Use ginkgo for unit testing and gomega for assertions in Go.
- Use gofmt and golint for code formatting and linting.
- Use Go modules for dependency management.
- Use SDL3 for rendering and input handling.
- Use Zyko0/go-sdl3 for SDL3 bindings in Go.
- Use WebSockets for multiplayer networking.
- Use YAML for configuration and modding.
- Use Makefile for building and cross-compiling the project.

## Game Design Notes
- The game mechanics should be based on the latest unofficial patch 1.50
- Find documentation [here](https://moo2mod.com/patch/GAME_MANUAL.PDF)
- Find a detailed book on the game mechanics [here](https://moo2mod.com/doc/game/manual.html)