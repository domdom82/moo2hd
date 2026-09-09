package ui

import (
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
)

const circleSegments = 16

// unitFan holds the precomputed unit-radius triangle fan:
// index 0 is the centre at origin; indices 1..circleSegments+1 are the ring.
var unitFan [circleSegments + 2]sdl.FPoint

func init() {
	unitFan[0] = sdl.FPoint{X: 0, Y: 0}
	for i := 0; i <= circleSegments; i++ {
		angle := float64(i) / float64(circleSegments) * 2 * math.Pi
		unitFan[i+1] = sdl.FPoint{
			X: float32(math.Cos(angle)),
			Y: float32(math.Sin(angle)),
		}
	}
}

// buildCircleVertices returns a (circleSegments+2)-element triangle fan centred
// at (cx, cy) with the given radius and colour.
// Pass the result directly to renderer.RenderGeometry(nil, verts, nil).
func buildCircleVertices(cx, cy, radius float32, col sdl.FColor) []sdl.Vertex {
	verts := make([]sdl.Vertex, circleSegments+2)
	for i, p := range unitFan {
		verts[i] = sdl.Vertex{
			Position: sdl.FPoint{X: cx + p.X*radius, Y: cy + p.Y*radius},
			Color:    col,
		}
	}
	return verts
}
