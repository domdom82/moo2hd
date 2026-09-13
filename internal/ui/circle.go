package ui

import (
	"math"

	"github.com/Zyko0/go-sdl3/sdl"
)

const circleSegments = 32

// unitFan holds the precomputed unit-radius points:
// index 0 is the centre at origin; indices 1..circleSegments+1 are the ring.
var unitFan [circleSegments + 2]sdl.FPoint

// circleIndices encodes the triangle fan as explicit index triples so SDL3
// renders it correctly regardless of primitive topology assumptions.
var circleIndices [circleSegments * 3]int32

func init() {
	unitFan[0] = sdl.FPoint{X: 0, Y: 0}
	for i := 0; i <= circleSegments; i++ {
		angle := float64(i) / float64(circleSegments) * 2 * math.Pi
		unitFan[i+1] = sdl.FPoint{
			X: float32(math.Cos(angle)),
			Y: float32(math.Sin(angle)),
		}
	}
	for i := range circleSegments {
		circleIndices[i*3+0] = 0
		circleIndices[i*3+1] = int32(i + 1)
		circleIndices[i*3+2] = int32(i + 2)
	}
}

// buildCircleVertices returns the vertex buffer for a filled circle centred at
// (cx, cy) with the given radius and colour. Use with circleIndices:
//
//	r.RenderGeometry(nil, verts, circleIndices[:])
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
