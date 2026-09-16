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

// pieSlice describes one coloured wedge of a pie chart circle.
type pieSlice struct {
	fraction float32    // proportion of the full circle [0, 1]
	col      sdl.FColor // fill colour for this wedge
}

// buildPieVertices returns vertex and index buffers for a filled circle split
// into proportional coloured wedge segments. Slices are laid out
// counter-clockwise starting at angle 0. Each slice gets at least one triangle.
// For a single full-circle slice this is equivalent to buildCircleVertices.
func buildPieVertices(cx, cy, radius float32, slices []pieSlice) ([]sdl.Vertex, []int32) {
	if len(slices) == 0 {
		return nil, nil
	}

	// Assign integer segment counts to each slice, ensuring each gets at least 1
	// and the total sums to exactly circleSegments.
	counts := make([]int, len(slices))
	remaining := circleSegments
	for i, s := range slices {
		n := int(s.fraction*float32(circleSegments) + 0.5)
		if n < 1 {
			n = 1
		}
		counts[i] = n
		remaining -= n
	}
	// Distribute leftover segments (rounding artefacts) to the largest slice.
	if remaining != 0 {
		biggest := 0
		for i := 1; i < len(counts); i++ {
			if counts[i] > counts[biggest] {
				biggest = i
			}
		}
		counts[biggest] += remaining
	}

	// Each triangle gets 3 unshared vertices to avoid colour bleed at
	// slice boundaries.
	verts := make([]sdl.Vertex, circleSegments*3)
	indices := make([]int32, circleSegments*3)
	for i := range circleSegments * 3 {
		indices[i] = int32(i)
	}

	triIdx := 0
	segIdx := 0 // running segment index into the full circle
	for s, cnt := range counts {
		col := slices[s].col
		for t := 0; t < cnt; t++ {
			a1 := float64(segIdx) / float64(circleSegments) * 2 * math.Pi
			a2 := float64(segIdx+1) / float64(circleSegments) * 2 * math.Pi
			base := triIdx * 3
			verts[base+0] = sdl.Vertex{Position: sdl.FPoint{X: cx, Y: cy}, Color: col}
			verts[base+1] = sdl.Vertex{
				Position: sdl.FPoint{X: cx + radius*float32(math.Cos(a1)), Y: cy + radius*float32(math.Sin(a1))},
				Color:    col,
			}
			verts[base+2] = sdl.Vertex{
				Position: sdl.FPoint{X: cx + radius*float32(math.Cos(a2)), Y: cy + radius*float32(math.Sin(a2))},
				Color:    col,
			}
			triIdx++
			segIdx++
		}
	}

	return verts, indices
}
