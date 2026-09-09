package galaxy

import (
	"container/heap"
	"errors"
	"math"
)

// ErrNoPath is returned when ShortestPath cannot reach the destination.
var ErrNoPath = errors.New("galaxy: no path between systems")

// Path holds the result of a pathfinding query.
type Path struct {
	Hops     []SystemID
	Distance float32
}

// ShortestPath finds the shortest lane-path from src to dst using Dijkstra.
// Returns ErrNoPath if dst is unreachable.
func ShortestPath(g *Galaxy, src, dst SystemID) (Path, error) {
	n := len(g.Systems)
	dist := make([]float32, n)
	prev := make([]SystemID, n)
	for i := range dist {
		dist[i] = float32(math.MaxFloat32)
		prev[i] = -1
	}
	dist[src] = 0

	h := &pqHeap{{id: src, cost: 0}}
	heap.Init(h)

	for h.Len() > 0 {
		cur := heap.Pop(h).(pqItem)
		if cur.id == dst {
			break
		}
		if cur.cost > dist[cur.id] {
			continue // stale entry
		}
		for _, lane := range g.Adjacency[cur.id] {
			next := lane.To
			nd := dist[cur.id] + lane.Distance
			if nd < dist[next] {
				dist[next] = nd
				prev[next] = cur.id
				heap.Push(h, pqItem{id: next, cost: nd})
			}
		}
	}

	if dist[dst] == float32(math.MaxFloat32) {
		return Path{}, ErrNoPath
	}

	// Reconstruct path
	var hops []SystemID
	for cur := dst; cur != src; cur = prev[cur] {
		hops = append(hops, cur)
	}
	hops = append(hops, src)
	// reverse
	for i, j := 0, len(hops)-1; i < j; i, j = i+1, j-1 {
		hops[i], hops[j] = hops[j], hops[i]
	}
	return Path{Hops: hops, Distance: dist[dst]}, nil
}

// AllReachable returns all systems reachable from src within maxDist lane-distance,
// mapping SystemID → distance.
func AllReachable(g *Galaxy, src SystemID, maxDist float32) map[SystemID]float32 {
	n := len(g.Systems)
	dist := make([]float32, n)
	for i := range dist {
		dist[i] = float32(math.MaxFloat32)
	}
	dist[src] = 0

	h := &pqHeap{{id: src, cost: 0}}
	heap.Init(h)

	for h.Len() > 0 {
		cur := heap.Pop(h).(pqItem)
		if cur.cost > dist[cur.id] {
			continue
		}
		for _, lane := range g.Adjacency[cur.id] {
			nd := dist[cur.id] + lane.Distance
			if nd <= maxDist && nd < dist[lane.To] {
				dist[lane.To] = nd
				heap.Push(h, pqItem{id: lane.To, cost: nd})
			}
		}
	}

	out := make(map[SystemID]float32)
	for id, d := range dist {
		if d <= maxDist {
			out[SystemID(id)] = d
		}
	}
	return out
}

// ── min-heap for Dijkstra ─────────────────────────────────────────────────────

type pqItem struct {
	id   SystemID
	cost float32
}

type pqHeap []pqItem

func (h pqHeap) Len() int            { return len(h) }
func (h pqHeap) Less(i, j int) bool  { return h[i].cost < h[j].cost }
func (h pqHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *pqHeap) Push(x any)         { *h = append(*h, x.(pqItem)) }
func (h *pqHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
