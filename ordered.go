package dheap

import (
	"cmp"
	"math/bits"
	"slices"
)

// Ordered is a d-ary heap for element types that support the < operator:
// integers, floating-point numbers and strings. It is either a min-heap,
// created with [NewMin] or [FromSliceMin], or a max-heap, created with
// [NewMax] or [FromSliceMax].
//
// Ordered does the same job as a [DHeap] whose less function is a < b or
// a > b, considerably faster. A DHeap has to call its less function for every
// comparison. Ordered compares with the < operator, which the compiler places
// directly inside the loops.
//
// Use Ordered when the elements themselves are the priorities. Use [DHeap]
// when elements are ordered by one of their fields or by anything else that
// needs a less function, and when elements have to be updated or removed by
// index.
//
// A floating-point NaN orders before every other value, as it does for
// [cmp.Less]. NaNs are therefore popped first from a min-heap and last from a
// max-heap.
//
// The zero value is not usable. An Ordered must not be copied after first use;
// use [Ordered.Clone] instead. An Ordered is not safe for concurrent use.
type Ordered[T cmp.Ordered] struct {
	// data holds the implicit tree in level order, exactly like DHeap.data.
	data []T

	// arity is the maximum number of children per node. It is at least 2.
	arity int

	// shift is log2(arity) if arity is a power of two, and 0 otherwise. A
	// non-zero shift lets siftUp replace the division by arity with a shift.
	shift uint

	// desc is true for a max-heap, in which larger elements are popped first.
	desc bool
}

// NewMin returns an empty min-heap in which every node has up to arity
// children. [Ordered.Pop] returns the smallest element first.
//
// NewMin panics if arity is less than 2.
func NewMin[T cmp.Ordered](arity int) *Ordered[T] {
	return newOrdered[T](arity, false)
}

// NewMax returns an empty max-heap in which every node has up to arity
// children. [Ordered.Pop] returns the largest element first.
//
// NewMax panics if arity is less than 2.
func NewMax[T cmp.Ordered](arity int) *Ordered[T] {
	return newOrdered[T](arity, true)
}

// FromSliceMin returns a min-heap that holds the elements of data. Building
// the heap this way takes O(n) time, which is faster than pushing the
// elements one by one.
//
// The heap works on its own copy of data, so the caller's slice is neither
// modified nor retained.
//
// FromSliceMin panics if arity is less than 2.
func FromSliceMin[T cmp.Ordered](data []T, arity int) *Ordered[T] {
	h := newOrdered[T](arity, false)
	h.data = slices.Clone(data)
	h.heapify()

	return h
}

// FromSliceMax returns a max-heap that holds the elements of data. Building
// the heap this way takes O(n) time, which is faster than pushing the
// elements one by one.
//
// The heap works on its own copy of data, so the caller's slice is neither
// modified nor retained.
//
// FromSliceMax panics if arity is less than 2.
func FromSliceMax[T cmp.Ordered](data []T, arity int) *Ordered[T] {
	h := newOrdered[T](arity, true)
	h.data = slices.Clone(data)
	h.heapify()

	return h
}

// newOrdered validates the arity and returns an empty heap. The heap is a
// max-heap if desc is true and a min-heap otherwise.
func newOrdered[T cmp.Ordered](arity int, desc bool) *Ordered[T] {
	if arity < 2 {
		panic("dheap: arity must be at least 2")
	}

	h := &Ordered[T]{
		arity: arity,
		desc:  desc,
	}

	if arity&(arity-1) == 0 { // arity is a power of two
		h.shift = uint(bits.TrailingZeros(uint(arity)))
	}

	return h
}

// Arity returns the branching factor d of the heap, the maximum number of
// children per node.
func (h *Ordered[T]) Arity() int {
	return h.arity
}

// IsMax reports whether the heap is a max-heap, which pops its largest
// element first. It returns false for a min-heap.
func (h *Ordered[T]) IsMax() bool {
	return h.desc
}

// Len returns the number of elements in the heap.
func (h *Ordered[T]) Len() int {
	return len(h.data)
}

// IsEmpty reports whether the heap contains no elements.
func (h *Ordered[T]) IsEmpty() bool {
	return len(h.data) == 0
}

// Peek returns the element that [Ordered.Pop] would return, without removing
// it, in O(1) time. The boolean result is false if the heap is empty.
func (h *Ordered[T]) Peek() (T, bool) {
	if len(h.data) == 0 {
		var zero T

		return zero, false
	}

	return h.data[0], true
}

// Push inserts v into the heap in O(log_d n) time.
func (h *Ordered[T]) Push(v T) {
	h.data = append(h.data, v)
	h.siftUp(len(h.data) - 1)
}

// Pop removes and returns the smallest element of a min-heap, or the largest
// element of a max-heap, in O(d log_d n) time. The boolean result is false if
// the heap is empty.
func (h *Ordered[T]) Pop() (T, bool) {
	var zero T

	n := len(h.data)
	if n == 0 {
		return zero, false
	}

	// Move the last leaf to the root and let it sink to its place.
	last := n - 1
	top := h.data[0]
	h.data[0] = h.data[last]
	h.data[last] = zero // Do not keep a string reachable past the length.
	h.data = h.data[:last]

	if last > 0 {
		h.siftDown(0)
	}

	return top, true
}

// Replace removes and returns the element that [Ordered.Pop] would return and
// inserts v. It restores the heap with a single sift-down pass, which makes it
// cheaper than a Pop followed by a [Ordered.Push].
//
// Unlike [Ordered.PushPop], Replace returns the old root even if v would be
// popped before it. If the heap is empty, Replace only inserts v and the
// boolean result is false.
func (h *Ordered[T]) Replace(v T) (T, bool) {
	if len(h.data) == 0 {
		h.data = append(h.data, v)

		var zero T

		return zero, false
	}

	prev := h.data[0]
	h.data[0] = v
	h.siftDown(0)

	return prev, true
}

// PushPop inserts v and then removes and returns the element that
// [Ordered.Pop] would return. It restores the heap with a single sift-down
// pass, which makes it cheaper than a [Ordered.Push] followed by a Pop.
//
// If no element in the heap would be popped before v, PushPop returns v and
// leaves the heap untouched. This makes PushPop the natural operation for
// keeping the k largest values of a stream in a min-heap of size k, or the k
// smallest in a max-heap.
func (h *Ordered[T]) PushPop(v T) T {
	if len(h.data) == 0 || !ordersBefore(h.data[0], v, h.desc) {
		return v
	}

	top := h.data[0]
	h.data[0] = v
	h.siftDown(0)

	return top
}

// Clear removes all elements. The heap keeps its allocated capacity, so it
// can be refilled without allocating.
func (h *Ordered[T]) Clear() {
	clear(h.data) // Release the memory that strings refer to.
	h.data = h.data[:0]
}

// Clone returns a copy of the heap that shares no storage with h, so that
// pushing to or popping from one does not affect the other.
func (h *Ordered[T]) Clone() *Ordered[T] {
	return &Ordered[T]{
		data:  slices.Clone(h.data),
		arity: h.arity,
		shift: h.shift,
		desc:  h.desc,
	}
}

// IsValid reports whether every element satisfies the heap ordering with
// respect to its parent. It takes O(n) time.
//
// The methods of Ordered never leave a heap invalid. IsValid is meant for
// tests and assertions, not for regular operation.
func (h *Ordered[T]) IsValid() bool {
	data, arity, desc := h.data, h.arity, h.desc

	for i := 1; i < len(data); i++ {
		if ordersBefore(data[i], data[(i-1)/arity], desc) {
			return false
		}
	}

	return true
}

// ordersBefore reports whether a must be popped before b: whether a is less
// than b in a min-heap, or greater than b in a max-heap, where desc is true.
//
// The function is small enough for the compiler to inline it, which turns
// every comparison in the sift loops into a few machine instructions. This is
// what makes Ordered faster than DHeap.
//
// cmp.Less is used instead of a bare < so that floating-point NaNs have a
// fixed place in the order. For every other type the two compile to the same
// code.
func ordersBefore[T cmp.Ordered](a, b T, desc bool) bool {
	if desc {
		a, b = b, a
	}

	return cmp.Less(a, b)
}

// heapify establishes the heap ordering over arbitrary data in O(n) time
// using Floyd's algorithm: it sifts down every internal node, starting with
// the parent of the last leaf and ending with the root.
func (h *Ordered[T]) heapify() {
	n := len(h.data)
	if n < 2 {
		return
	}

	for i := (n - 2) / h.arity; i >= 0; i-- {
		h.siftDown(i)
	}
}

// siftUp moves the element at index i toward the root until its parent no
// longer orders after it.
//
// It is the loop of DHeap.siftUp with the call to less replaced by
// ordersBefore, and without index tracking. See there for why the element is
// held aside instead of being swapped, and why the parent is found with a
// shift when possible.
func (h *Ordered[T]) siftUp(i int) {
	data, desc := h.data, h.desc
	arity, shift := h.arity, h.shift

	item := data[i]

	for i > 0 {
		var parent int

		if shift != 0 {
			parent = (i - 1) >> shift
		} else {
			parent = (i - 1) / arity
		}

		if !ordersBefore(item, data[parent], desc) {
			break
		}

		data[i] = data[parent]
		i = parent
	}

	data[i] = item
}

// siftDown moves the element at index i toward the leaves until none of its
// children orders before it.
//
// It is the loop of DHeap.siftDown with the calls to less replaced by
// ordersBefore, and without index tracking.
func (h *Ordered[T]) siftDown(i int) {
	data, desc := h.data, h.desc
	arity := h.arity
	n := len(data)

	item := data[i]

	for {
		first := i*arity + 1
		if first >= n {
			break // The element reached a leaf.
		}

		// Find the child that must be popped first. The last internal node
		// may have fewer than arity children.
		best := first

		for j, end := first+1, min(first+arity, n); j < end; j++ {
			if ordersBefore(data[j], data[best], desc) {
				best = j
			}
		}

		if !ordersBefore(data[best], item, desc) {
			break
		}

		data[i] = data[best]
		i = best
	}

	data[i] = item
}
