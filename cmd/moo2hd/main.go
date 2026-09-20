// Command moo2hd is the main game binary entry point.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/domdom82/moo2hd/internal/config"
	"github.com/domdom82/moo2hd/internal/game/colony"
	"github.com/domdom82/moo2hd/internal/game/galaxy"
	"github.com/domdom82/moo2hd/internal/ui"
)

const (
	windowW = 1920
	windowH = 1080

	// localRace is the hard-coded player race for development.
	localRace = "human"
)

func main() {
	defer binsdl.Load().Unload()
	defer sdl.Quit()

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		log.Fatal(err)
	}

	glxOpt := galaxy.OptionsForSize(galaxy.GalaxySizeHuge)
	glxOpt.Seed = uint64(time.Now().UnixNano())

	g, err := galaxy.NewGenerator(glxOpt).Generate()
	if err != nil {
		log.Fatal(err)
	}

	// Seed colonies: every planet in every claimed system belongs to its
	// system's faction. Faction 0 (blue) is the human player.
	//
	// TEST ONLY: 20% of multi-planet systems get a second faction owning the
	// last planet, so mixed-colour Voronoi blobs can be verified visually.
	raceForFaction := func(f int) string {
		if f == 0 {
			return localRace
		}
		return fmt.Sprintf("faction-%d", f)
	}

	mgr := colony.NewManager()
	for i := range g.Systems {
		sys := &g.Systems[i]
		if sys.Faction < 0 {
			continue
		}
		race := raceForFaction(sys.Faction)
		mixedSystem := len(sys.Planets) > 1 && sys.ID%5 == 0 // TEST: every 5th multi-planet system
		for j, pid := range sys.Planets {
			p := g.Planet(pid)
			r := race
			if mixedSystem && j == len(sys.Planets)-1 {
				// Give the last planet to the neighbouring faction (+1, wrapping at 8).
				r = raceForFaction((sys.Faction + 1) % 8)
			}
			c := colony.NewColony(0, sys.ID, pid, r)
			c.Population = float64(p.MaxPop) / 2.0
			mgr.AddColony(c)
		}
	}

	window, renderer, err := sdl.CreateWindowAndRenderer("MOO2HD", windowW, windowH, sdl.WINDOW_RESIZABLE)
	if err != nil {
		log.Fatal(err)
	}
	defer window.Destroy()
	defer renderer.Destroy()

	cam := ui.NewCamera(windowW, windowH, glxOpt.Width, glxOpt.Height)
	font := ui.NewFontManager(renderer, "assets/fonts/DejaVuSans.ttf", 14)
	defer font.Close()

	reg, err := config.Load("configs/", "mods/")
	if err != nil {
		log.Printf("warning: could not load config: %v", err)
	}
	bg, err := ui.NewBackgroundManager(renderer, "assets/", reg, glxOpt.Seed)
	if err != nil {
		log.Printf("warning: could not load star backgrounds: %v", err)
	}
	defer bg.Close()

	nm, err := ui.NewNebulaManager(renderer, "assets/", reg)
	if err != nil {
		log.Printf("warning: could not load nebula textures: %v", err)
	}
	defer nm.Close()

	sm := ui.NewStarMap(g, cam, font, bg, nm)
	sm.SetColonyData(mgr, localRace)
	ih := ui.NewInputHandler(sm.Camera(), glxOpt.Width, glxOpt.Height)
	sm.Bind(ih)

	lastTick := sdl.Ticks()

	sdl.RunLoop(func() error {
		now := sdl.Ticks()
		dt := float32(now-lastTick) / 1000.0
		lastTick = now

		var event sdl.Event
		for sdl.PollEvent(&event) {
			var err error
			if sm.HasActiveScreen() {
				err = sm.HandleScreen(&event)
			} else {
				err = ih.Handle(&event)
			}
			if err != nil {
				return err
			}
		}

		ih.Update(dt)
		sm.Update(dt)
		return sm.Draw(renderer)
	})
}
