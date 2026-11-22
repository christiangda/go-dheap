// Command dijkstra computes shortest paths with Dijkstra's algorithm, using a
// d-ary heap as the priority queue of vertices to visit.
//
// The example shows two things. First, the elements of the heap are plain
// vertex numbers; their priority lives outside the heap, in the dist slice
// that the less function reads. Second, the position of every vertex in the
// heap is tracked in a side table, the pos slice, so that the algorithm can
// perform a true decrease-key with Fix instead of pushing duplicate entries.
//
// Dijkstra's algorithm is the classic case for a d-ary heap: it performs one
// decrease-key per edge but only one pop per vertex, and a wider heap makes
// the former cheaper at the expense of the latter.
//
// Run it with:
//
//	go run ./docs/examples/dijkstra
package main

import (
	"fmt"
	"math"
	"slices"

	"github.com/christiangda/go-dheap"
)

// edge is a directed connection to the vertex to with a non-negative weight.
type edge struct {
	to     int
	weight int
}

// graph is an adjacency list: graph[v] holds the edges leaving vertex v.
type graph [][]edge

// unreachable is the distance of vertices that cannot be reached.
const unreachable = math.MaxInt

// shortestPaths returns the length of the shortest path from source to every
// vertex of g, and for every vertex the previous vertex on that path (-1 for
// the source and for unreachable vertices).
func shortestPaths(g graph, source int) (dist, prev []int) {
	n := len(g)
	dist = make([]int, n)
	prev = make([]int, n)
	pos := make([]int, n) // pos[v] is the index of v in the heap, or -1
	for v := range n {
		dist[v] = unreachable
		prev[v] = -1
		pos[v] = -1
	}

	// The heap orders vertices by their current distance. Changing dist[v]
	// changes the priority of v, which is why every change is followed by Fix.
	queue := dheap.New(4,
		func(a, b int) bool { return dist[a] < dist[b] },
		dheap.WithIndexTracking(func(v, i int) { pos[v] = i }),
	)

	dist[source] = 0
	queue.Push(source)

	for {
		v, ok := queue.Pop()
		if !ok {
			return dist, prev
		}

		for _, e := range g[v] {
			d := dist[v] + e.weight
			if d >= dist[e.to] {
				continue
			}

			first := dist[e.to] == unreachable
			dist[e.to] = d
			prev[e.to] = v
			if first {
				queue.Push(e.to)
			} else {
				// Decrease-key: the vertex is still queued, because with
				// non-negative weights a vertex that was already popped
				// can never get a shorter distance. Move it up in place.
				queue.Fix(pos[e.to])
			}
		}
	}
}

// path returns the vertices on the shortest path that ends in target.
func path(prev []int, target int) []int {
	var p []int
	for v := target; v != -1; v = prev[v] {
		p = append(p, v)
	}
	slices.Reverse(p)
	return p
}

func main() {
	names := []string{"Lisbon", "Madrid", "Paris", "Rome", "Vienna", "Berlin", "Oslo"}
	const (
		lisbon = iota
		madrid
		paris
		rome
		vienna
		berlin
		oslo
	)

	g := make(graph, len(names))
	connect := func(a, b, hours int) {
		g[a] = append(g[a], edge{to: b, weight: hours})
		g[b] = append(g[b], edge{to: a, weight: hours})
	}
	connect(lisbon, madrid, 10)
	connect(madrid, paris, 13)
	connect(madrid, rome, 22)
	connect(paris, rome, 11)
	connect(paris, berlin, 8)
	connect(rome, vienna, 12)
	connect(vienna, berlin, 8)
	// Oslo has no connection.

	dist, prev := shortestPaths(g, lisbon)

	for v, name := range names {
		if dist[v] == unreachable {
			fmt.Printf("%-7s unreachable\n", name)
			continue
		}

		var route []string
		for _, u := range path(prev, v) {
			route = append(route, names[u])
		}
		fmt.Printf("%-7s %2dh  %v\n", name, dist[v], route)
	}
}
