package ui

import (
	"os"

	"github.com/Zyko0/go-sdl3/bin/binttf"
	"github.com/Zyko0/go-sdl3/sdl"
	"github.com/Zyko0/go-sdl3/ttf"
)

// FontManager wraps SDL3_ttf and provides a simple DrawText API.
// It degrades gracefully to SDL3's built-in 8×8 DebugText when no font
// file is available — all callers are unaffected by this fallback.
type FontManager struct {
	renderer  *sdl.Renderer
	unloadLib func() // nil when no TTF library was loaded
	font      *ttf.Font
	engine    *ttf.TextEngine
	texts     map[string]*ttf.Text
	lastCol   map[string]sdl.Color
}

// NewFontManager loads SDL3_ttf and opens fontPath at the given point size.
// If fontPath is "" or the file does not exist, it returns a FontManager
// that falls back to DebugText for all DrawText calls.
func NewFontManager(renderer *sdl.Renderer, fontPath string, ptSize float32) *FontManager {
	fm := &FontManager{
		renderer: renderer,
		texts:    make(map[string]*ttf.Text),
		lastCol:  make(map[string]sdl.Color),
	}
	if fontPath == "" {
		return fm
	}
	if _, err := os.Stat(fontPath); err != nil {
		return fm // font file absent — use DebugText fallback
	}
	lib := binttf.Load()
	if err := ttf.Init(); err != nil {
		lib.Unload()
		return fm
	}
	font, err := ttf.OpenFont(fontPath, ptSize)
	if err != nil {
		ttf.Quit()
		lib.Unload()
		return fm
	}
	engine, err := ttf.CreateRendererTextEngine(renderer)
	if err != nil {
		font.Close()
		ttf.Quit()
		lib.Unload()
		return fm
	}
	// Capture lib in a closure so it stays alive until Close() is called.
	fm.unloadLib = func() {
		ttf.Quit()
		lib.Unload()
	}
	fm.font = font
	fm.engine = engine
	return fm
}

// DrawText renders str at (x, y) using the TTF engine when available,
// otherwise falls back to SDL3's built-in DebugText.
func (fm *FontManager) DrawText(str string, x, y float32, col sdl.Color) error {
	if fm.font == nil {
		fm.renderer.SetDrawColor(col.R, col.G, col.B, col.A)
		return fm.renderer.DebugText(x, y, str)
	}
	obj, ok := fm.texts[str]
	if !ok {
		var err error
		obj, err = fm.engine.CreateText(fm.font, str)
		if err != nil {
			return err
		}
		fm.texts[str] = obj
	}
	// Update color only when it changes, avoiding unnecessary GPU state churn.
	if prev, seen := fm.lastCol[str]; !seen || prev != col {
		obj.SetColor(col.R, col.G, col.B, col.A)
		fm.lastCol[str] = col
	}
	return obj.DrawRenderer(x, y)
}

// Close frees all TTF resources. Safe to call when no font was loaded.
func (fm *FontManager) Close() {
	for _, obj := range fm.texts {
		obj.Destroy()
	}
	if fm.engine != nil {
		fm.engine.DestroyRenderer()
	}
	if fm.font != nil {
		fm.font.Close()
	}
	if fm.unloadLib != nil {
		fm.unloadLib()
	}
}
