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

	g, err := galaxy.NewGenerator(galaxy.Options{Seed: 42, SystemCount: 70}).Generate()
	if err != nil {
		log.Fatal(err)
	}

	window, renderer, err := sdl.CreateWindowAndRenderer("MOO2HD", windowW, windowH, sdl.WINDOW_RESIZABLE)
	if err != nil {
		log.Fatal(err)
	}
	defer window.Destroy()
	defer renderer.Destroy()

	cam := ui.NewCamera(windowW, windowH)
	font := ui.NewFontManager(renderer, "assets/fonts/DejaVuSans.ttf", 14)
	defer font.Close()
	sm := ui.NewStarMap(g, cam, font)
	ih := ui.NewInputHandler(sm.Camera(), 1000, 1000)

	sdl.RunLoop(func() error {
		var event sdl.Event
		for sdl.PollEvent(&event) {
			if err := ih.Handle(&event); err != nil {
				return err
			}
		}
		return sm.Draw(renderer)
	})
}
