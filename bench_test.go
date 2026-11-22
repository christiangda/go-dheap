package dheap

import (
	"container/heap"
	"fmt"
	"math/rand/v2"
	"testing"
)

// Benchmarks
//
// Run all of them, with allocation statistics, with:
//
//	go test -run '^$' -bench . -benchmem
//
// The full sweep takes a few minutes. Narrow it down with a pattern, for
// example every benchmark at arity 4, or Push and Pop at one million elements:
//
//	go test -run '^$' -bench '/d=4' -benchmem
//	go test -run '^$' -bench '^Benchmark(Push|Pop)$/d=.*/n=1000000' -benchmem
//
// To compare two versions of the code, run each with -count=10 and feed both
// outputs to benchstat (golang.org/x/perf/cmd/benchstat).
//
// Benchmarks whose iterations process a whole batch of elements (Push, Pop,
// FromSlice, PushHeavy and the container/heap equivalents) report the cost of
// one element as ns/elem. The others perform one operation per iteration, so
// their ns/op is already the cost of one operation. The same goes for the
// Ordered variants of these benchmarks.
//
// All input is prepared before the b.Loop loop, which excludes it from the
// measurement; b.Loop resets the timer when it is first called.

// benchArities are the branching factors every benchmark is run with. 3 is
// the only one that is not a power of two; it shows the cost of the integer
// division in siftUp.
var benchArities = []int{2, 3, 4, 8, 16}

// benchSizes are the heap sizes of the benchmarks that sweep over sizes. With
// 8-byte elements, 1,000 elements (8 KB) fit in the L1 cache, 100,000 (800 KB)
// need L2 or L3, and 1,000,000 (8 MB) exceed the L2 cache of most CPUs and
// the L3 cache of many.
var benchSizes = []int{1_000, 100_000, 1_000_000}

// benchSize is the heap size of the benchmarks that do not sweep over sizes.
const benchSize = 100_000

// benchValues returns n pseudo-random values. The result is the same for
// every call with the same n, so every benchmark sees the same input.
func benchValues(n int) []int {
	r := rand.New(rand.NewPCG(1, 2))
	vals := make([]int, n)
	for i := range vals {
		vals[i] = r.IntN(1 << 30)
	}

	return vals
}

// reportPerElement adds a ns/elem metric to a benchmark whose iterations
// process n elements each.
func reportPerElement(b *testing.B, n int) {
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(n), "ns/elem")
}

// forEachArity runs fn as a sub-benchmark named d=<arity> for every arity in
// benchArities.
func forEachArity(b *testing.B, fn func(b *testing.B, arity int)) {
	for _, arity := range benchArities {
		b.Run(fmt.Sprintf("d=%d", arity), func(b *testing.B) {
			b.ReportAllocs()
			fn(b, arity)
		})
	}
}

// forEachAritySize runs fn as a sub-benchmark named d=<arity>/n=<size> for
// every combination of benchArities and benchSizes.
func forEachAritySize(b *testing.B, fn func(b *testing.B, arity int, vals []int)) {
	for _, n := range benchSizes {
		vals := benchValues(n)

		for _, arity := range benchArities {
			b.Run(fmt.Sprintf("d=%d/n=%d", arity, n), func(b *testing.B) {
				b.ReportAllocs()
				fn(b, arity, vals)
			})
		}
	}
}

// BenchmarkPush measures pushing n random values into an empty heap whose
// backing slice already has enough capacity. With random input most elements
// stop after one or two levels.
func BenchmarkPush(b *testing.B) {
	forEachAritySize(b, func(b *testing.B, arity int, vals []int) {
		h := FromSlice(vals, arity, intLess) // allocates the capacity up front

		for b.Loop() {
			h.Clear()

			for _, v := range vals {
				h.Push(v)
			}
		}

		reportPerElement(b, len(vals))
	})
}

// BenchmarkPushSorted measures the worst case of Push: every new element has
// a higher priority than all others and travels the full height of the tree.
func BenchmarkPushSorted(b *testing.B) {
	forEachArity(b, func(b *testing.B, arity int) {
		h := FromSlice(make([]int, benchSize), arity, intLess) // allocates the capacity up front

		for b.Loop() {
			h.Clear()

			for v := benchSize; v > 0; v-- {
				h.Push(v)
			}
		}

		reportPerElement(b, benchSize)
	})
}

// BenchmarkPop measures draining a heap of n random values. The heap is
// refilled with the timer stopped.
func BenchmarkPop(b *testing.B) {
	forEachAritySize(b, func(b *testing.B, arity int, vals []int) {
		h := FromSlice(vals, arity, intLess) // allocates the capacity up front
		h.Clear()

		for b.Loop() {
			b.StopTimer()

			for _, v := range vals {
				h.Push(v)
			}

			b.StartTimer()

			for h.Len() > 0 {
				h.Pop()
			}
		}

		reportPerElement(b, len(vals))
	})
}

// BenchmarkFromSlice measures building a heap from n random values, including
// the copy of the input.
func BenchmarkFromSlice(b *testing.B) {
	forEachAritySize(b, func(b *testing.B, arity int, vals []int) {
		for b.Loop() {
			FromSlice(vals, arity, intLess)
		}

		reportPerElement(b, len(vals))
	})
}

// BenchmarkPopPush measures the steady state of a queue of n elements: remove
// the root and insert a value with a slightly lower priority. This is the
// pattern of schedulers, timers and event simulations, with one Pop per Push.
func BenchmarkPopPush(b *testing.B) {
	forEachAritySize(b, func(b *testing.B, arity int, vals []int) {
		h := FromSlice(vals, arity, intLess)
		i := 0

		for b.Loop() {
			v, _ := h.Pop()
			h.Push(v + vals[i]&0xffff)

			if i++; i == len(vals) {
				i = 0
			}
		}
	})
}

// BenchmarkPushHeavy measures an asymmetric workload with eight pushes for
// every pop, the kind of load that favours large arities.
func BenchmarkPushHeavy(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		h := FromSlice(vals, arity, intLess) // allocates the capacity up front

		for b.Loop() {
			h.Clear()

			for i, v := range vals {
				h.Push(v)

				if i%8 == 7 {
					h.Pop()
				}
			}
		}

		reportPerElement(b, benchSize)
	})
}

// BenchmarkReplace measures the workload of BenchmarkPopPush done with a
// single Replace call instead of a Pop and a Push.
func BenchmarkReplace(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		h := FromSlice(vals, arity, intLess)
		i := 0

		for b.Loop() {
			v, _ := h.Peek()
			h.Replace(v + vals[i]&0xffff)

			if i++; i == benchSize {
				i = 0
			}
		}
	})
}

// BenchmarkPushPop measures keeping the 1,000 largest values of a random
// stream, the top-k pattern. Most values are rejected by a single comparison.
func BenchmarkPushPop(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		h := FromSlice(vals[:1000], arity, intLess)
		i := 0

		for b.Loop() {
			h.PushPop(vals[i])

			if i++; i == benchSize {
				i = 0
			}
		}
	})
}

// BenchmarkUpdate measures replacing the element at a random index with a
// random value.
func BenchmarkUpdate(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		h := FromSlice(vals, arity, intLess)
		i := 0

		for b.Loop() {
			h.Update(vals[i]%benchSize, vals[benchSize-1-i])

			if i++; i == benchSize {
				i = 0
			}
		}
	})
}

// newTrackedBenchHeap returns a heap of pointers with index tracking and the
// elements it holds, in a fixed order.
func newTrackedBenchHeap(arity int, vals []int) (*DHeap[*tracked], []*tracked) {
	items := make([]*tracked, len(vals))

	for i, v := range vals {
		items[i] = &tracked{priority: v}
	}

	h := FromSlice(items, arity,
		func(a, b *tracked) bool { return a.priority < b.priority },
		WithIndexTracking(func(v *tracked, i int) { v.index = i }),
	)

	return h, items
}

// BenchmarkFix measures changing the priority of a random element in place
// and repairing the heap, the decrease-key and increase-key pattern of graph
// algorithms. It needs index tracking to find the element.
func BenchmarkFix(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		h, items := newTrackedBenchHeap(arity, vals)
		i := 0

		for b.Loop() {
			it := items[i]
			it.priority = vals[benchSize-1-i]
			h.Fix(it.index)

			if i++; i == benchSize {
				i = 0
			}
		}
	})
}

// BenchmarkPopPushTracked is BenchmarkPopPush on a heap of pointers with
// index tracking enabled. Compare it with BenchmarkPopPushTypes/pointer to see
// the cost of the tracking callback alone.
func BenchmarkPopPushTracked(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		h, _ := newTrackedBenchHeap(arity, vals)
		i := 0

		for b.Loop() {
			it, _ := h.Pop()
			it.priority += vals[i] & 0xffff
			h.Push(it)

			if i++; i == benchSize {
				i = 0
			}
		}
	})
}

// The element types of BenchmarkPopPushTypes.
type (
	// pair is a compact struct without pointers: 16 bytes that the garbage
	// collector never has to look at.
	pair struct {
		key int
		id  int
	}

	// record is a struct that contains pointers: 32 bytes that the garbage
	// collector must scan, and whose moves need write barriers while a
	// collection is running.
	record struct {
		key  int
		name string
		meta *int
	}
)

// benchPopPush runs the workload of BenchmarkPopPush for any element type.
// later returns a copy of v whose priority is lowered by delta.
func benchPopPush[T any](b *testing.B, items []T, less func(a, b T) bool, later func(v T, delta int) T) {
	b.ReportAllocs()
	deltas := benchValues(len(items))
	h := FromSlice(items, 4, less)
	i := 0

	for b.Loop() {
		v, _ := h.Pop()
		h.Push(later(v, deltas[i]&0xffff))

		if i++; i == len(items) {
			i = 0
		}
	}
}

// BenchmarkPopPushTypes measures the steady-state workload of
// BenchmarkPopPush at arity 4 for element types of different sizes and
// pointer content. Elements are moved by value, so larger elements cost more
// per level, and a heap of pointers pays a cache miss per comparison instead.
func BenchmarkPopPushTypes(b *testing.B) {
	vals := benchValues(benchSize)

	b.Run("int", func(b *testing.B) {
		benchPopPush(b, vals, intLess, func(v, delta int) int { return v + delta })
	})

	b.Run("struct", func(b *testing.B) {
		items := make([]pair, len(vals))

		for i, v := range vals {
			items[i] = pair{key: v, id: i}
		}

		benchPopPush(b, items,
			func(a, b pair) bool { return a.key < b.key },
			func(v pair, delta int) pair { v.key += delta; return v },
		)
	})

	b.Run("struct_with_pointers", func(b *testing.B) {
		items := make([]record, len(vals))

		for i, v := range vals {
			items[i] = record{key: v, name: "record", meta: &vals[i]}
		}

		benchPopPush(b, items,
			func(a, b record) bool { return a.key < b.key },
			func(v record, delta int) record { v.key += delta; return v },
		)
	})

	b.Run("pointer", func(b *testing.B) {
		items := make([]*pair, len(vals))

		for i, v := range vals {
			items[i] = &pair{key: v, id: i}
		}

		benchPopPush(b, items,
			func(a, b *pair) bool { return a.key < b.key },
			func(v *pair, delta int) *pair { v.key += delta; return v },
		)
	})
}

// The benchmarks below repeat the core benchmarks for Ordered, the heap for
// element types that support the < operator. Compare each one with the
// benchmark of the same name without the Ordered prefix, at n=100000, to see
// what compiling the comparison into the loops is worth.

// BenchmarkOrderedPush is BenchmarkPush at n=100000 for a min-heap created
// with NewMin.
func BenchmarkOrderedPush(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		h := FromSliceMin(vals, arity) // allocates the capacity up front

		for b.Loop() {
			h.Clear()

			for _, v := range vals {
				h.Push(v)
			}
		}

		reportPerElement(b, benchSize)
	})
}

// BenchmarkOrderedPop is BenchmarkPop at n=100000 for a min-heap created with
// NewMin.
func BenchmarkOrderedPop(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		h := FromSliceMin(vals, arity) // allocates the capacity up front
		h.Clear()

		for b.Loop() {
			b.StopTimer()

			for _, v := range vals {
				h.Push(v)
			}

			b.StartTimer()

			for h.Len() > 0 {
				h.Pop()
			}
		}

		reportPerElement(b, benchSize)
	})
}

// BenchmarkOrderedFromSlice is BenchmarkFromSlice at n=100000 for
// FromSliceMin.
func BenchmarkOrderedFromSlice(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		for b.Loop() {
			FromSliceMin(vals, arity)
		}

		reportPerElement(b, benchSize)
	})
}

// BenchmarkOrderedPopPush is BenchmarkPopPush at n=100000 for a min-heap
// created with FromSliceMin.
func BenchmarkOrderedPopPush(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		h := FromSliceMin(vals, arity)
		i := 0

		for b.Loop() {
			v, _ := h.Pop()
			h.Push(v + vals[i]&0xffff)

			if i++; i == benchSize {
				i = 0
			}
		}
	})
}

// BenchmarkOrderedPopPushMax is BenchmarkOrderedPopPush for a max-heap. It
// shows that the direction of the heap costs nothing.
func BenchmarkOrderedPopPushMax(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		h := FromSliceMax(vals, arity)
		i := 0

		for b.Loop() {
			v, _ := h.Pop()
			h.Push(v - vals[i]&0xffff)

			if i++; i == benchSize {
				i = 0
			}
		}
	})
}

// BenchmarkOrderedPushPop is BenchmarkPushPop for a min-heap created with
// FromSliceMin: keeping the 1,000 largest values of a random stream.
func BenchmarkOrderedPushPop(b *testing.B) {
	vals := benchValues(benchSize)
	forEachArity(b, func(b *testing.B, arity int) {
		h := FromSliceMin(vals[:1000], arity)
		i := 0

		for b.Loop() {
			h.PushPop(vals[i])

			if i++; i == benchSize {
				i = 0
			}
		}
	})
}

// BenchmarkOrderedPopPushTypes measures the steady-state workload at arity 4
// for the kinds of element type Ordered supports.
func BenchmarkOrderedPopPushTypes(b *testing.B) {
	vals := benchValues(benchSize)

	b.Run("int", func(b *testing.B) {
		b.ReportAllocs()
		h := FromSliceMin(vals, 4)
		i := 0

		for b.Loop() {
			v, _ := h.Pop()
			h.Push(v + vals[i]&0xffff)

			if i++; i == benchSize {
				i = 0
			}
		}
	})

	b.Run("float64", func(b *testing.B) {
		b.ReportAllocs()
		floats := make([]float64, len(vals))

		for i, v := range vals {
			floats[i] = float64(v)
		}

		h := FromSliceMin(floats, 4)
		i := 0

		for b.Loop() {
			v, _ := h.Pop()
			h.Push(v + float64(vals[i]&0xffff))

			if i++; i == benchSize {
				i = 0
			}
		}
	})

	b.Run("string", func(b *testing.B) {
		b.ReportAllocs()
		// Fixed-width keys, so that every comparison looks at whole strings.
		keys := make([]string, len(vals))

		for i, v := range vals {
			keys[i] = fmt.Sprintf("%012d", v)
		}

		h := FromSliceMin(keys, 4)
		i := 0

		for b.Loop() {
			h.Pop()
			h.Push(keys[i])

			if i++; i == benchSize {
				i = 0
			}
		}
	})
}

// stdHeap adapts a slice of ints to container/heap. It is the reference the
// d-ary heap is compared against.
type stdHeap []int

func (s stdHeap) Len() int           { return len(s) }
func (s stdHeap) Less(i, j int) bool { return s[i] < s[j] }
func (s stdHeap) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s *stdHeap) Push(x any)        { *s = append(*s, x.(int)) }

func (s *stdHeap) Pop() any {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[:n-1]

	return x
}

// BenchmarkContainerHeapPush is BenchmarkPush at n=100000 for the binary heap
// of the standard library.
func BenchmarkContainerHeapPush(b *testing.B) {
	vals := benchValues(benchSize)
	s := make(stdHeap, 0, benchSize)
	b.ReportAllocs()

	for b.Loop() {
		s = s[:0]

		for _, v := range vals {
			heap.Push(&s, v)
		}
	}

	reportPerElement(b, benchSize)
}

// BenchmarkContainerHeapPop is BenchmarkPop at n=100000 for the binary heap of
// the standard library.
func BenchmarkContainerHeapPop(b *testing.B) {
	vals := benchValues(benchSize)
	s := make(stdHeap, 0, benchSize)
	b.ReportAllocs()

	for b.Loop() {
		b.StopTimer()

		for _, v := range vals {
			heap.Push(&s, v)
		}

		b.StartTimer()

		for s.Len() > 0 {
			heap.Pop(&s)
		}
	}

	reportPerElement(b, benchSize)
}

// BenchmarkContainerHeapPopPush is BenchmarkPopPush at n=100000 for the binary
// heap of the standard library.
func BenchmarkContainerHeapPopPush(b *testing.B) {
	vals := benchValues(benchSize)
	s := make(stdHeap, benchSize)
	copy(s, vals)
	heap.Init(&s)

	i := 0
	b.ReportAllocs()

	for b.Loop() {
		v := heap.Pop(&s).(int)
		heap.Push(&s, v+vals[i]&0xffff)

		if i++; i == benchSize {
			i = 0
		}
	}
}
