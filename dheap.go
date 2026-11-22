// Package dheap implements a generic d-ary heap, a priority queue stored as an
// implicit tree in a slice, in which every node has up to d children.
//
// # Ordering
//
// A heap is ordered by the less function given to [New] or [FromSlice]. The
// element with the highest priority, the one [DHeap.Pop] returns first, is the
// one that less orders before all others. A less of a < b therefore gives a
// min-heap and a less of a > b gives a max-heap:
//
//	minHeap := dheap.New(4, func(a, b int) bool { return a < b })
//	maxHeap := dheap.New(4, func(a, b int) bool { return a > b })
//
// Elements that compare equal are returned in an unspecified order. To get a
// stable order, break ties inside less, for example with an insertion counter.
//
// # Choosing the arity
//
// The arity d is the number of children per node. A larger arity makes the
// tree shallower, which makes [DHeap.Push] cheaper, but [DHeap.Pop] has to
// compare more children on every level:
//
//	Push, siftUp:   O(log_d n) comparisons
//	Pop, siftDown:  O(d log_d n) comparisons
//
// An arity of 4 is a good default. Compared with a binary heap, Push is
// considerably faster, and Pop is slightly slower while the heap fits in the
// CPU caches and faster once it does not, because the tree is half as deep.
// Workloads that push much more than they pop benefit from 8 or more, and
// workloads dominated by Pop on small heaps may prefer 2. The benchmarks of
// this package cover all of these cases; measure before settling on a value.
//
// Prefer an arity that is a power of two. The parent of a node is then found
// with a shift instead of an integer division, which makes Push noticeably
// faster.
//
// # Memory layout
//
// The tree is implicit. The elements live in a single slice in level order,
// and the position of a parent or a child is computed from an index instead of
// being followed through a pointer. This has two consequences:
//
//   - It is compact. The heap stores nothing but the elements themselves: no
//     node objects, no child pointers, and no allocation per element.
//   - It is cache friendly. The children of a node are adjacent in memory, so
//     the comparisons made on one level of the tree touch one or two cache
//     lines.
//
// Elements are moved by value, so small element types make the fastest heaps.
// For large elements, consider storing pointers or indices instead.
//
// # Index-based operations
//
// [DHeap.PopAt], [DHeap.Fix] and [DHeap.Update] address an element by its index
// in the underlying slice. Indices change whenever the heap is reordered, so
// these methods are only practical together with [WithIndexTracking], which
// reports every index change to a callback. That is the building block for
// priority queues that support changing the priority of a queued element or
// removing it, such as the queue used by Dijkstra's algorithm.
//
// # Concurrency
//
// A [DHeap] is not safe for concurrent use. Callers that share one between
// goroutines must synchronize access themselves.
package dheap

import (
	"math/bits"
	"slices"
)

// DHeap is a d-ary heap of elements of type T.
//
// The zero value is not usable; create a DHeap with [New] or [FromSlice].
// A DHeap must not be copied after first use; use [DHeap.Clone] instead.
type DHeap[T any] struct {
	// data holds the implicit tree in level order. The root is data[0], the
	// children of data[i] are data[i*arity+1] through data[i*arity+arity], and
	// the parent of data[i] is data[(i-1)/arity].
	data []T

	// arity is the maximum number of children per node. It is at least 2.
	arity int

	// shift is log2(arity) if arity is a power of two, and 0 otherwise. A
	// non-zero shift lets siftUp replace the division by arity with a shift.
	shift uint

	// less reports whether a must be popped before b.
	less func(a, b T) bool

	// onMove, if not nil, is called with an element and its new index every
	// time the element is stored at a different index, and with index -1 when
	// the element leaves the heap. See WithIndexTracking.
	onMove func(v T, i int)
}

// An Option configures a [DHeap] created by [New] or [FromSlice].
type Option[T any] func(*DHeap[T])

// WithIndexTracking returns an [Option] that makes the heap report the
// position of its elements to fn.
//
// The heap calls fn(v, i) whenever element v comes to rest at index i of the
// underlying slice, and fn(v, -1) when v leaves the heap through
// [DHeap.Pop], [DHeap.PopAt], [DHeap.Replace], [DHeap.PushPop],
// [DHeap.Update] or [DHeap.Clear]. An element may be reported more than once
// with the same index.
//
// Recording the index, typically in a field of the element itself or in a
// side table such as a map, is what makes [DHeap.PopAt], [DHeap.Fix] and
// [DHeap.Update] usable:
//
//	type task struct {
//		priority int
//		index    int // maintained by the heap
//	}
//
//	h := dheap.New(4,
//		func(a, b *task) bool { return a.priority < b.priority },
//		dheap.WithIndexTracking(func(t *task, i int) { t.index = i }),
//	)
//
// fn runs on the hot path of every operation, so it should be cheap, and it
// must not call methods of the heap.
func WithIndexTracking[T any](fn func(v T, i int)) Option[T] {
	return func(h *DHeap[T]) {
		h.onMove = fn
	}
}

// New returns an empty heap in which every node has up to arity children and
// in which less(a, b) reports whether a must be popped before b.
//
// New panics if arity is less than 2 or if less is nil, because either one is
// a programming error that no caller could handle at run time.
func New[T any](arity int, less func(a, b T) bool, opts ...Option[T]) *DHeap[T] {
	h := newDHeap(arity, less)
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// FromSlice returns a heap that holds the elements of data. Building the heap
// this way takes O(n) time, which is faster than pushing the elements one by
// one.
//
// The heap works on its own copy of data, so the caller's slice is neither
// modified nor retained.
//
// FromSlice panics if arity is less than 2 or if less is nil.
func FromSlice[T any](data []T, arity int, less func(a, b T) bool, opts ...Option[T]) *DHeap[T] {
	h := newDHeap(arity, less)
	h.data = slices.Clone(data)
	h.heapify()

	// The options are applied after heapify so that index tracking reports
	// every element exactly once, at its final position.
	for _, opt := range opts {
		opt(h)
	}
	if h.onMove != nil {
		for i, v := range h.data {
			h.onMove(v, i)
		}
	}
	return h
}

// newDHeap validates the configuration and returns an empty heap without any
// options applied.
func newDHeap[T any](arity int, less func(a, b T) bool) *DHeap[T] {
	if arity < 2 {
		panic("dheap: arity must be at least 2")
	}
	if less == nil {
		panic("dheap: less function must not be nil")
	}

	h := &DHeap[T]{
		arity: arity,
		less:  less,
	}
	if arity&(arity-1) == 0 { // arity is a power of two
		h.shift = uint(bits.TrailingZeros(uint(arity)))
	}
	return h
}

// Arity returns the branching factor d of the heap, the maximum number of
// children per node.
func (h *DHeap[T]) Arity() int {
	return h.arity
}

// Len returns the number of elements in the heap.
func (h *DHeap[T]) Len() int {
	return len(h.data)
}

// IsEmpty reports whether the heap contains no elements.
func (h *DHeap[T]) IsEmpty() bool {
	return len(h.data) == 0
}

// Peek returns the highest-priority element without removing it, in O(1)
// time. The boolean result is false if the heap is empty.
func (h *DHeap[T]) Peek() (T, bool) {
	if len(h.data) == 0 {
		var zero T
		return zero, false
	}
	return h.data[0], true
}

// Push inserts v into the heap in O(log_d n) time.
func (h *DHeap[T]) Push(v T) {
	h.data = append(h.data, v)
	h.siftUp(len(h.data) - 1)
}

// Pop removes and returns the highest-priority element in O(d log_d n) time.
// The boolean result is false if the heap is empty.
func (h *DHeap[T]) Pop() (T, bool) {
	var zero T
	n := len(h.data)
	if n == 0 {
		return zero, false
	}

	// Move the last leaf to the root and let it sink to its place.
	last := n - 1
	top := h.data[0]
	h.data[0] = h.data[last]
	h.data[last] = zero // Do not keep the element reachable past the length.
	h.data = h.data[:last]
	if last > 0 {
		h.siftDown(0)
	}

	if h.onMove != nil {
		h.onMove(top, -1)
	}
	return top, true
}

// Replace removes and returns the highest-priority element and inserts v. It
// restores the heap with a single sift-down pass, which makes it cheaper than
// a [DHeap.Pop] followed by a [DHeap.Push].
//
// Unlike [DHeap.PushPop], Replace returns the old root even if v has a higher
// priority. If the heap is empty, Replace only inserts v and the boolean
// result is false.
func (h *DHeap[T]) Replace(v T) (T, bool) {
	if len(h.data) == 0 {
		h.Push(v)
		var zero T
		return zero, false
	}

	prev := h.data[0]
	h.data[0] = v
	h.siftDown(0)

	if h.onMove != nil {
		h.onMove(prev, -1)
	}
	return prev, true
}

// PushPop inserts v and then removes and returns the highest-priority
// element. It restores the heap with a single sift-down pass, which makes it
// cheaper than a [DHeap.Push] followed by a [DHeap.Pop].
//
// If no element in the heap has a higher priority than v, PushPop returns v
// and leaves the heap untouched. This makes PushPop the natural operation for
// keeping the best k elements of a stream in a heap of bounded size.
func (h *DHeap[T]) PushPop(v T) T {
	if len(h.data) == 0 || !h.less(h.data[0], v) {
		return v
	}

	top := h.data[0]
	h.data[0] = v
	h.siftDown(0)

	if h.onMove != nil {
		h.onMove(top, -1)
	}
	return top
}

// PopAt removes and returns the element at index i of the underlying slice in
// O(d log_d n) time. The boolean result is false if i is out of range.
//
// See [WithIndexTracking] for how to learn the index of an element.
func (h *DHeap[T]) PopAt(i int) (T, bool) {
	var zero T
	n := len(h.data)
	if i < 0 || i >= n {
		return zero, false
	}

	// Fill the gap with the last leaf, which may have to move either way.
	last := n - 1
	removed := h.data[i]
	h.data[i] = h.data[last]
	h.data[last] = zero // Do not keep the element reachable past the length.
	h.data = h.data[:last]
	if i < last {
		h.fix(i)
	}

	if h.onMove != nil {
		h.onMove(removed, -1)
	}
	return removed, true
}

// Fix restores the heap ordering after the priority of the element at index
// i has changed, in O(d log_d n) time. It reports whether i is in range.
//
// Call Fix after mutating an element in place in a way that affects less.
// See [WithIndexTracking] for how to learn the index of an element.
func (h *DHeap[T]) Fix(i int) bool {
	if i < 0 || i >= len(h.data) {
		return false
	}
	h.fix(i)
	return true
}

// Update replaces the element at index i with v and restores the heap
// ordering in O(d log_d n) time. It reports whether i is in range.
//
// See [WithIndexTracking] for how to learn the index of an element.
func (h *DHeap[T]) Update(i int, v T) bool {
	if i < 0 || i >= len(h.data) {
		return false
	}

	old := h.data[i]
	h.data[i] = v
	if h.onMove != nil {
		// Report the removal first: old and v may be the same element.
		h.onMove(old, -1)
	}
	h.fix(i)
	return true
}

// Clear removes all elements. The heap keeps its allocated capacity, so it
// can be refilled without allocating.
func (h *DHeap[T]) Clear() {
	if h.onMove != nil {
		for _, v := range h.data {
			h.onMove(v, -1)
		}
	}
	clear(h.data) // Release the references held by the elements.
	h.data = h.data[:0]
}

// Clone returns a copy of the heap that shares no storage with h, so that
// pushing to or popping from one does not affect the other. The elements
// themselves are copied by assignment: if T is a pointer type, both heaps
// refer to the same values.
//
// The clone does not inherit the [WithIndexTracking] callback. Indices keep
// describing positions in h, which makes it safe to drain a clone in order to
// visit the elements of h in priority order.
func (h *DHeap[T]) Clone() *DHeap[T] {
	return &DHeap[T]{
		data:  slices.Clone(h.data),
		arity: h.arity,
		shift: h.shift,
		less:  h.less,
	}
}

// IsValid reports whether every element satisfies the heap ordering with
// respect to its parent. It takes O(n) time.
//
// A heap only becomes invalid if elements are mutated in place without a call
// to [DHeap.Fix], or if less is not a consistent ordering. IsValid is meant
// for tests and assertions, not for regular operation.
func (h *DHeap[T]) IsValid() bool {
	data, arity, less := h.data, h.arity, h.less
	for i := 1; i < len(data); i++ {
		if less(data[i], data[(i-1)/arity]) {
			return false
		}
	}
	return true
}

// heapify establishes the heap ordering over arbitrary data in O(n) time
// using Floyd's algorithm: it sifts down every internal node, starting with
// the parent of the last leaf and ending with the root.
func (h *DHeap[T]) heapify() {
	n := len(h.data)
	if n < 2 {
		return
	}
	for i := (n - 2) / h.arity; i >= 0; i-- {
		h.siftDown(i)
	}
}

// fix restores the heap ordering around index i, which must be in range.
func (h *DHeap[T]) fix(i int) {
	// An element that moved up took the place of an ancestor, and an ancestor
	// orders before everything below it. Only an element that stayed where it
	// was can still need to move down.
	if h.siftUp(i) == i {
		h.siftDown(i)
	}
}

// siftUp moves the element at index i toward the root until its parent no
// longer orders after it, and returns the final index of the element.
//
// The loop does not swap the element with its parent on every level. It keeps
// the element aside, shifts the parents it passes one level down, and stores
// the element once when its place is found. This does one write per level
// where swapping does two.
//
// The fields of h are copied into locals because the compiler cannot prove
// that the call to less leaves h untouched, and would otherwise reload them on
// every iteration.
func (h *DHeap[T]) siftUp(i int) int {
	data, less, onMove := h.data, h.less, h.onMove
	arity, shift := h.arity, h.shift

	item := data[i]
	for i > 0 {
		// Integer division is one of the slowest instructions of a CPU, and
		// this is the only one on the push path. The branch is predicted
		// perfectly because shift never changes.
		var parent int
		if shift != 0 {
			parent = (i - 1) >> shift
		} else {
			parent = (i - 1) / arity
		}

		if !less(item, data[parent]) {
			break
		}
		data[i] = data[parent]
		if onMove != nil {
			onMove(data[i], i)
		}
		i = parent
	}
	data[i] = item
	if onMove != nil {
		onMove(item, i)
	}
	return i
}

// siftDown moves the element at index i toward the leaves until none of its
// children orders before it, and returns the final index of the element.
//
// It keeps the element aside and shifts children up, for the same reasons as
// siftUp.
func (h *DHeap[T]) siftDown(i int) int {
	data, less, onMove := h.data, h.less, h.onMove
	arity := h.arity
	n := len(data)

	item := data[i]
	for {
		first := i*arity + 1
		if first >= n {
			break // The element reached a leaf.
		}

		// Find the child with the highest priority. The last internal node
		// may have fewer than arity children.
		best := first
		for j, end := first+1, min(first+arity, n); j < end; j++ {
			if less(data[j], data[best]) {
				best = j
			}
		}

		if !less(data[best], item) {
			break
		}
		data[i] = data[best]
		if onMove != nil {
			onMove(data[i], i)
		}
		i = best
	}
	data[i] = item
	if onMove != nil {
		onMove(item, i)
	}
	return i
}
