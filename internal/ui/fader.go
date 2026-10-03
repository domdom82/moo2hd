package ui

import "github.com/Zyko0/go-sdl3/sdl"

type faderState int

const (
	faderIdle      faderState = iota
	faderFadingOut            // alpha 0→255
	faderFadingIn             // alpha 255→0
)

// ScreenFader drives a fade-out → callback → fade-in transition.
// Callers draw their scene first, then call fader.Draw to overlay the black veil.
// The transition is fully data-driven: no goroutines, no channels.
type ScreenFader struct {
	state      faderState
	alpha      float32 // 0.0–255.0
	speed      float32 // alpha units per second
	onMidpoint func()  // called once when screen is fully black
}

// NewScreenFader creates a fader that completes each half of the transition in
// durationS seconds (fade-out takes durationS, fade-in takes durationS).
func NewScreenFader(durationS float32) *ScreenFader {
	return &ScreenFader{speed: 255.0 / durationS}
}

// Start begins a fade-out → onMidpoint → fade-in sequence.
// If a transition is already running it is restarted from the beginning.
func (f *ScreenFader) Start(onMidpoint func()) {
	f.state = faderFadingOut
	f.alpha = 0
	f.onMidpoint = onMidpoint
}

// Active reports whether a transition is currently running.
func (f *ScreenFader) Active() bool {
	return f.state != faderIdle
}

// Update advances the transition by dt seconds.
func (f *ScreenFader) Update(dt float32) {
	switch f.state {
	case faderFadingOut:
		f.alpha += f.speed * dt
		if f.alpha >= 255 {
			f.alpha = 255
			if f.onMidpoint != nil {
				f.onMidpoint()
				f.onMidpoint = nil
			}
			f.state = faderFadingIn
		}
	case faderFadingIn:
		f.alpha -= f.speed * dt
		if f.alpha <= 0 {
			f.alpha = 0
			f.state = faderIdle
		}
	}
}

// Draw overlays a black rectangle at the current fade alpha.
// Call this after drawing your scene content so the veil sits on top.
func (f *ScreenFader) Draw(r *sdl.Renderer) {
	if f.state == faderIdle {
		return
	}
	w, h, err := r.RenderOutputSize()
	if err != nil {
		return
	}
	r.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
	r.SetDrawColor(0, 0, 0, uint8(f.alpha))
	r.RenderFillRect(&sdl.FRect{X: 0, Y: 0, W: float32(w), H: float32(h)})
	r.SetDrawBlendMode(sdl.BLENDMODE_NONE)
}
