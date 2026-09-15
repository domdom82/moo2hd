// Command moo2hd is the main game binary entry point.
package main

import (
	"log"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/domdom82/moo2hd/internal/game/galaxy"
	"github.com/domdom82/moo2hd/internal/ui"
)

const (
	windowW = 1920
	windowH = 1080
)

func main() {
	defer binsdl.Load().Unload()
	defer sdl.Quit()

	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		log.Fatal(err)
	}

	glxOpt := galaxy.OptionsForSize(galaxy.GalaxySizeHuge)
	glxOpt.Seed = 42

	g, err := galaxy.NewGenerator(glxOpt).Generate()
	if err != nil {
		log.Fatal(err)
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
	sm := ui.NewStarMap(g, cam, font)
	ih := ui.NewInputHandler(sm.Camera(), glxOpt.Width, glxOpt.Height)
	sm.Bind(ih)

	lastTick := sdl.Ticks()

	sdl.RunLoop(func() error {
		now := sdl.Ticks()
		dt := float32(now-lastTick) / 1000.0
		lastTick = now

		var event sdl.Event
		for sdl.PollEvent(&event) {
			if err := ih.Handle(&event); err != nil {
				return err
			}
		}

		ih.Update(dt)
		sm.Update(dt)
		return sm.Draw(renderer)
	})
}
