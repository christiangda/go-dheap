package dheap

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
)

// testArities covers binary heaps, powers of two (shift path), other arities
// (division path), and an arity larger than most of the test inputs.
var testArities = []int{2, 3, 4, 5, 8, 16, 17, 64}

// boundarySizes returns the heap sizes at which the shape of a tree of the
// given arity changes: empty, a lone root, a root with a partial, a full and
// an overfull first level, and the same around the second level. Most
// off-by-one mistakes in a heap show up at one of these sizes.
func boundarySizes(arity int) []int {
	sizes := []int{0, 1, 2, arity - 1, arity, arity + 1, arity + 2, arity * arity, arity*arity + 1, arity*arity + arity + 1, 1000}
	slices.Sort(sizes)

	return slices.Compact(sizes)
}

// invariantError checks the heap ordering independently of DHeap.IsValid. It
// walks the tree from the parents down: for every node it visits the children
// that exist and verifies that none of them orders before the node. It returns
// nil if the heap is well formed.
func invariantError[T any](h *DHeap[T]) error {
	n := len(h.data)
	for parent := 0; parent < n; parent++ {
		first := parent*h.arity + 1
		for child := first; child < first+h.arity && child < n; child++ {
			if h.less(h.data[child], h.data[parent]) {
				return fmt.Errorf("heap ordering violated: data[%d] = %v orders before its parent data[%d] = %v",
					child, h.data[child], parent, h.data[parent])
			}
		}
	}

	return nil
}

func intLess(a, b int) bool    { return a < b }
func intGreater(a, b int) bool { return a > b }

// randomInts returns n pseudo-random values that include duplicates. The
// result is the same for every run.
func randomInts(n int) []int {
	r := rand.New(rand.NewPCG(uint64(n), 42))
	vals := make([]int, n)
	for i := range vals {
		vals[i] = r.IntN(n/2 + 1)
	}

	return vals
}

// drain pops every element of h and returns them in pop order.
func drain[T any](h *DHeap[T]) []T {
	var got []T

	for {
		v, ok := h.Pop()
		if !ok {
			return got
		}

		got = append(got, v)
	}
}

func sortedCopy(vals []int, cmp func(a, b int) int) []int {
	want := slices.Clone(vals)
	slices.SortFunc(want, cmp)

	return want
}

func ascending(a, b int) int  { return a - b }
func descending(a, b int) int { return b - a }

func TestNewPanics(t *testing.T) {
	tests := []struct {
		name  string
		arity int
		less  func(a, b int) bool
		want  string
	}{
		{name: "arity 1", arity: 1, less: intLess, want: "dheap: arity must be at least 2"},
		{name: "arity 0", arity: 0, less: intLess, want: "dheap: arity must be at least 2"},
		{name: "negative arity", arity: -4, less: intLess, want: "dheap: arity must be at least 2"},
		{name: "nil less", arity: 2, less: nil, want: "dheap: less function must not be nil"},
	}

	constructors := []struct {
		name string
		call func(arity int, less func(a, b int) bool)
	}{
		{name: "New", call: func(arity int, less func(a, b int) bool) { New(arity, less) }},
		{name: "FromSlice", call: func(arity int, less func(a, b int) bool) { FromSlice([]int{1}, arity, less) }},
	}

	for _, c := range constructors {
		for _, tc := range tests {
			t.Run(c.name+"/"+tc.name, func(t *testing.T) {
				defer func() {
					if got := recover(); got != tc.want {
						t.Errorf("%s(arity=%d) panicked with %v, want %q", c.name, tc.arity, got, tc.want)
					}
				}()
				c.call(tc.arity, tc.less)
			})
		}
	}
}

func TestNew(t *testing.T) {
	for _, arity := range testArities {
		h := New(arity, intLess)
		if got := h.Arity(); got != arity {
			t.Errorf("New(%d, less).Arity() = %d, want %d", arity, got, arity)
		}

		if got := h.Len(); got != 0 {
			t.Errorf("New(%d, less).Len() = %d, want 0", arity, got)
		}

		if !h.IsEmpty() {
			t.Errorf("New(%d, less).IsEmpty() = false, want true", arity)
		}

		if !h.IsValid() {
			t.Errorf("New(%d, less).IsValid() = false, want true", arity)
		}
	}
}

func TestShift(t *testing.T) {
	tests := []struct {
		arity int
		want  uint
	}{
		{arity: 2, want: 1},
		{arity: 3, want: 0},
		{arity: 4, want: 2},
		{arity: 6, want: 0},
		{arity: 8, want: 3},
		{arity: 1024, want: 10},
		{arity: 1000, want: 0},
	}

	for _, tc := range tests {
		if got := New(tc.arity, intLess).shift; got != tc.want {
			t.Errorf("New(%d, less).shift = %d, want %d", tc.arity, got, tc.want)
		}
	}
}

func TestPushThenPopReturnsSortedOrder(t *testing.T) {
	orders := []struct {
		name string
		less func(a, b int) bool
		cmp  func(a, b int) int
	}{
		{name: "min", less: intLess, cmp: ascending},
		{name: "max", less: intGreater, cmp: descending},
	}

	for _, order := range orders {
		for _, arity := range testArities {
			for _, n := range boundarySizes(arity) {
				t.Run(fmt.Sprintf("%s/d=%d/n=%d", order.name, arity, n), func(t *testing.T) {
					vals := randomInts(n)
					h := New(arity, order.less)

					for i, v := range vals {
						h.Push(v)

						if got := h.Len(); got != i+1 {
							t.Fatalf("Len() after %d pushes = %d, want %d", i+1, got, i+1)
						}
					}

					if err := invariantError(h); err != nil {
						t.Fatalf("after %d pushes: %v", n, err)
					}

					if got := h.IsEmpty(); got != (n == 0) {
						t.Errorf("IsEmpty() after %d pushes = %t, want %t", n, got, n == 0)
					}

					got, want := drain(h), sortedCopy(vals, order.cmp)
					if !slices.Equal(got, want) {
						t.Errorf("popped elements = %v, want %v", got, want)
					}

					if !h.IsEmpty() {
						t.Errorf("IsEmpty() after draining = false, want true")
					}
				})
			}
		}
	}
}

func TestFromSlice(t *testing.T) {
	for _, arity := range testArities {
		for _, n := range boundarySizes(arity) {
			t.Run(fmt.Sprintf("d=%d/n=%d", arity, n), func(t *testing.T) {
				vals := randomInts(n)
				input := slices.Clone(vals)

				h := FromSlice(input, arity, intLess)

				if !slices.Equal(input, vals) {
					t.Errorf("FromSlice modified its input: got %v, want %v", input, vals)
				}

				if got := h.Len(); got != n {
					t.Errorf("FromSlice(%d elements).Len() = %d, want %d", n, got, n)
				}

				if err := invariantError(h); err != nil {
					t.Fatalf("after FromSlice(%v): %v", vals, err)
				}

				// The heap must not alias the caller's slice.
				for i := range input {
					input[i] = -1
				}

				got, want := drain(h), sortedCopy(vals, ascending)
				if !slices.Equal(got, want) {
					t.Errorf("popped elements = %v, want %v", got, want)
				}
			})
		}
	}
}

func TestFromSliceNil(t *testing.T) {
	h := FromSlice(nil, 4, intLess)
	if got := h.Len(); got != 0 {
		t.Errorf("FromSlice(nil).Len() = %d, want 0", got)
	}

	h.Push(1)

	if got, ok := h.Pop(); got != 1 || !ok {
		t.Errorf("Pop() = %d, %t, want 1, true", got, ok)
	}
}

func TestPeek(t *testing.T) {
	h := New(3, intLess)
	if got, ok := h.Peek(); got != 0 || ok {
		t.Errorf("Peek() on empty heap = %d, %t, want 0, false", got, ok)
	}

	wantMin := 0

	for i, v := range []int{5, 9, 3, 7, 1, 8} {
		h.Push(v)

		if i == 0 || v < wantMin {
			wantMin = v
		}

		if got, ok := h.Peek(); got != wantMin || !ok {
			t.Errorf("Peek() after pushing %d = %d, %t, want %d, true", v, got, ok, wantMin)
		}
	}

	if got := h.Len(); got != 6 {
		t.Errorf("Len() after Peek = %d, want 6; Peek must not remove elements", got)
	}
}

func TestPopEmpty(t *testing.T) {
	h := New(2, func(a, b string) bool { return a < b })
	if got, ok := h.Pop(); got != "" || ok {
		t.Errorf("Pop() on empty heap = %q, %t, want %q, false", got, ok, "")
	}

	// A heap that was filled and drained behaves like a new one.
	h.Push("a")
	h.Pop()

	if got, ok := h.Pop(); got != "" || ok {
		t.Errorf("Pop() on drained heap = %q, %t, want %q, false", got, ok, "")
	}
}

func TestReplace(t *testing.T) {
	t.Run("empty heap", func(t *testing.T) {
		h := New(4, intLess)
		if got, ok := h.Replace(7); got != 0 || ok {
			t.Errorf("Replace(7) on empty heap = %d, %t, want 0, false", got, ok)
		}

		if got := drain(h); !slices.Equal(got, []int{7}) {
			t.Errorf("heap after Replace(7) = %v, want [7]", got)
		}
	})

	tests := []struct {
		name     string
		v        int
		wantPrev int
		wantHeap []int
	}{
		{name: "lower priority than every element", v: 100, wantPrev: 1, wantHeap: []int{3, 5, 7, 9, 100}},
		{name: "in the middle", v: 6, wantPrev: 1, wantHeap: []int{3, 5, 6, 7, 9}},
		{name: "higher priority than the root", v: 0, wantPrev: 1, wantHeap: []int{0, 3, 5, 7, 9}},
		{name: "equal to the root", v: 1, wantPrev: 1, wantHeap: []int{1, 3, 5, 7, 9}},
	}

	for _, arity := range testArities {
		for _, tc := range tests {
			t.Run(fmt.Sprintf("d=%d/%s", arity, tc.name), func(t *testing.T) {
				h := FromSlice([]int{9, 1, 7, 3, 5}, arity, intLess)
				if got, ok := h.Replace(tc.v); got != tc.wantPrev || !ok {
					t.Errorf("Replace(%d) = %d, %t, want %d, true", tc.v, got, ok, tc.wantPrev)
				}

				if err := invariantError(h); err != nil {
					t.Errorf("after Replace(%d): %v", tc.v, err)
				}

				if got := drain(h); !slices.Equal(got, tc.wantHeap) {
					t.Errorf("heap after Replace(%d) = %v, want %v", tc.v, got, tc.wantHeap)
				}
			})
		}
	}
}

func TestPushPop(t *testing.T) {
	t.Run("empty heap", func(t *testing.T) {
		h := New(4, intLess)
		if got := h.PushPop(7); got != 7 {
			t.Errorf("PushPop(7) on empty heap = %d, want 7", got)
		}

		if got := h.Len(); got != 0 {
			t.Errorf("Len() after PushPop on empty heap = %d, want 0", got)
		}
	})

	tests := []struct {
		name     string
		v        int
		want     int
		wantHeap []int
	}{
		{name: "lower priority than every element", v: 100, want: 1, wantHeap: []int{3, 5, 7, 9, 100}},
		{name: "in the middle", v: 6, want: 1, wantHeap: []int{3, 5, 6, 7, 9}},
		{name: "higher priority than the root", v: 0, want: 0, wantHeap: []int{1, 3, 5, 7, 9}},
		{name: "equal to the root", v: 1, want: 1, wantHeap: []int{1, 3, 5, 7, 9}},
	}

	for _, arity := range testArities {
		for _, tc := range tests {
			t.Run(fmt.Sprintf("d=%d/%s", arity, tc.name), func(t *testing.T) {
				h := FromSlice([]int{9, 1, 7, 3, 5}, arity, intLess)
				if got := h.PushPop(tc.v); got != tc.want {
					t.Errorf("PushPop(%d) = %d, want %d", tc.v, got, tc.want)
				}

				if err := invariantError(h); err != nil {
					t.Errorf("after PushPop(%d): %v", tc.v, err)
				}

				if got := drain(h); !slices.Equal(got, tc.wantHeap) {
					t.Errorf("heap after PushPop(%d) = %v, want %v", tc.v, got, tc.wantHeap)
				}
			})
		}
	}
}

func TestPopAtOutOfRange(t *testing.T) {
	h := FromSlice([]int{1, 2, 3}, 2, intLess)

	for _, i := range []int{-1, 3, 4, 1 << 40} {
		if got, ok := h.PopAt(i); got != 0 || ok {
			t.Errorf("PopAt(%d) on heap of 3 = %d, %t, want 0, false", i, got, ok)
		}
	}

	if got := h.Len(); got != 3 {
		t.Errorf("Len() after out-of-range PopAt = %d, want 3", got)
	}

	if got, ok := New(2, intLess).PopAt(0); got != 0 || ok {
		t.Errorf("PopAt(0) on empty heap = %d, %t, want 0, false", got, ok)
	}
}

func TestPopAt(t *testing.T) {
	for _, arity := range testArities {
		for _, n := range boundarySizes(arity) {
			if n > 300 {
				continue // Removing every index of a large heap adds time, not coverage.
			}

			vals := randomInts(n)

			for i := range n {
				t.Run(fmt.Sprintf("d=%d/n=%d/i=%d", arity, n, i), func(t *testing.T) {
					h := FromSlice(vals, arity, intLess)
					want := h.data[i]

					got, ok := h.PopAt(i)
					if got != want || !ok {
						t.Fatalf("PopAt(%d) = %d, %t, want %d, true", i, got, ok, want)
					}

					if err := invariantError(h); err != nil {
						t.Fatalf("after PopAt(%d): %v", i, err)
					}

					wantRest := sortedCopy(vals, ascending)
					at, _ := slices.BinarySearch(wantRest, want)
					wantRest = slices.Delete(wantRest, at, at+1)
					if gotRest := drain(h); !slices.Equal(gotRest, wantRest) {
						t.Errorf("heap after PopAt(%d) = %v, want %v", i, gotRest, wantRest)
					}
				})
			}
		}
	}
}

// TestPopAtMovesReplacementUp covers the case where the leaf that fills the
// gap has a higher priority than the parent of the gap, which can only happen
// when the two are in different subtrees.
func TestPopAtMovesReplacementUp(t *testing.T) {
	//         0
	//     10     1
	//   11  12  2  3
	h := FromSlice([]int{0, 10, 1, 11, 12, 2, 3}, 2, intLess)
	if got := h.data; !slices.Equal(got, []int{0, 10, 1, 11, 12, 2, 3}) {
		t.Fatalf("unexpected layout %v; the test needs updating", got)
	}

	if got, ok := h.PopAt(3); got != 11 || !ok { // 3 replaces 11 and must rise above 10.
		t.Fatalf("PopAt(3) = %d, %t, want 11, true", got, ok)
	}

	if err := invariantError(h); err != nil {
		t.Errorf("after PopAt(3): %v", err)
	}

	if got, want := drain(h), []int{0, 1, 2, 3, 10, 12}; !slices.Equal(got, want) {
		t.Errorf("heap after PopAt(3) = %v, want %v", got, want)
	}
}

func TestFixAndUpdateOutOfRange(t *testing.T) {
	h := FromSlice([]int{1, 2, 3}, 2, intLess)

	for _, i := range []int{-1, 3, 4, 1 << 40} {
		if h.Fix(i) {
			t.Errorf("Fix(%d) on heap of 3 = true, want false", i)
		}

		if h.Update(i, 0) {
			t.Errorf("Update(%d, 0) on heap of 3 = true, want false", i)
		}
	}

	if got, want := drain(h), []int{1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("heap after out-of-range Fix and Update = %v, want %v", got, want)
	}
}

func TestFix(t *testing.T) {
	type item struct{ priority int }
	less := func(a, b *item) bool { return a.priority < b.priority }

	for _, arity := range testArities {
		for _, newPriority := range []int{-1, 25, 1000} { // rises, stays near, sinks
			for i := range 50 {
				t.Run(fmt.Sprintf("d=%d/i=%d/priority=%d", arity, i, newPriority), func(t *testing.T) {
					h := New(arity, less)

					for _, v := range randomInts(50) {
						h.Push(&item{priority: v})
					}

					h.data[i].priority = newPriority
					if !h.Fix(i) {
						t.Fatalf("Fix(%d) = false, want true", i)
					}

					if err := invariantError(h); err != nil {
						t.Errorf("after Fix(%d): %v", i, err)
					}

					if got := h.Len(); got != 50 {
						t.Errorf("Len() after Fix(%d) = %d, want 50", i, got)
					}
				})
			}
		}
	}
}

func TestUpdate(t *testing.T) {
	for _, arity := range testArities {
		for _, v := range []int{-1, 25, 1000} { // rises, stays near, sinks
			for i := range 50 {
				t.Run(fmt.Sprintf("d=%d/i=%d/v=%d", arity, i, v), func(t *testing.T) {
					vals := randomInts(50)
					h := FromSlice(vals, arity, intLess)
					old := h.data[i]

					if !h.Update(i, v) {
						t.Fatalf("Update(%d, %d) = false, want true", i, v)
					}

					if err := invariantError(h); err != nil {
						t.Fatalf("after Update(%d, %d): %v", i, v, err)
					}

					want := sortedCopy(vals, ascending)
					at, _ := slices.BinarySearch(want, old)
					want[at] = v
					slices.Sort(want)

					if got := drain(h); !slices.Equal(got, want) {
						t.Errorf("heap after Update(%d, %d) = %v, want %v", i, v, got, want)
					}
				})
			}
		}
	}
}

func TestClear(t *testing.T) {
	h := New(4, func(a, b *int) bool { return *a < *b })

	for _, v := range randomInts(100) {
		h.Push(&v)
	}

	capBefore := cap(h.data)

	h.Clear()

	if got := h.Len(); got != 0 {
		t.Errorf("Len() after Clear = %d, want 0", got)
	}

	if got := cap(h.data); got != capBefore {
		t.Errorf("capacity after Clear = %d, want %d", got, capBefore)
	}

	for i, p := range h.data[:capBefore] {
		if p != nil {
			t.Fatalf("Clear left a reference at index %d; elements must be released", i)
		}
	}

	// The heap stays usable.
	one, two := 1, 2
	h.Push(&two)
	h.Push(&one)

	if got, ok := h.Pop(); !ok || *got != 1 {
		t.Errorf("Pop() after Clear and two pushes did not return 1")
	}
}

func TestRemovalReleasesReferences(t *testing.T) {
	less := func(a, b *int) bool { return *a < *b }
	fill := func() *DHeap[*int] {
		h := New(2, less)

		for _, v := range []int{4, 2, 6, 1, 5} {
			h.Push(&v)
		}

		return h
	}

	removals := []struct {
		name   string
		remove func(h *DHeap[*int])
	}{
		{name: "Pop", remove: func(h *DHeap[*int]) { h.Pop() }},
		{name: "PopAt first", remove: func(h *DHeap[*int]) { h.PopAt(0) }},
		{name: "PopAt middle", remove: func(h *DHeap[*int]) { h.PopAt(2) }},
		{name: "PopAt last", remove: func(h *DHeap[*int]) { h.PopAt(4) }},
	}

	for _, tc := range removals {
		t.Run(tc.name, func(t *testing.T) {
			h := fill()
			tc.remove(h)

			if got := h.Len(); got != 4 {
				t.Fatalf("Len() after %s = %d, want 4", tc.name, got)
			}

			if got := h.data[:5][4]; got != nil {
				t.Errorf("%s left a reference past the length; elements must be released", tc.name)
			}
		})
	}
}

func TestClone(t *testing.T) {
	vals := randomInts(100)
	h := FromSlice(vals, 4, intLess)

	c := h.Clone()
	if got, want := c.Arity(), h.Arity(); got != want {
		t.Errorf("Clone().Arity() = %d, want %d", got, want)
	}

	if got, want := c.shift, h.shift; got != want {
		t.Errorf("Clone().shift = %d, want %d", got, want)
	}

	if !slices.Equal(c.data, h.data) {
		t.Errorf("Clone() holds %v, want %v", c.data, h.data)
	}

	// Changes to the clone must not reach the original, and vice versa.
	c.Push(-1)
	c.Pop()
	c.Pop()
	h.Push(-2)

	if got, want := h.Len(), len(vals)+1; got != want {
		t.Errorf("Len() of original after changing the clone = %d, want %d", got, want)
	}

	if got, want := c.Len(), len(vals)-1; got != want {
		t.Errorf("Len() of clone after changing the original = %d, want %d", got, want)
	}

	if !h.IsValid() || !c.IsValid() {
		t.Errorf("IsValid() = %t (original), %t (clone), want true, true", h.IsValid(), c.IsValid())
	}

	if got, want := drain(h), sortedCopy(append(slices.Clone(vals), -2), ascending); !slices.Equal(got, want) {
		t.Errorf("original after changing the clone = %v, want %v", got, want)
	}
}

func TestCloneEmpty(t *testing.T) {
	c := New(3, intLess).Clone()
	c.Push(2)
	c.Push(1)

	if got, want := drain(c), []int{1, 2}; !slices.Equal(got, want) {
		t.Errorf("clone of empty heap after two pushes = %v, want %v", got, want)
	}
}

func TestIsValidDetectsViolations(t *testing.T) {
	for _, arity := range testArities {
		for i := 1; i < 40; i++ {
			h := FromSlice(randomInts(40), arity, intLess)
			if !h.IsValid() {
				t.Fatalf("d=%d: IsValid() on fresh heap = false, want true", arity)
			}

			h.data[i] = -1 // Now smaller than its parent.
			if h.IsValid() {
				t.Errorf("d=%d: IsValid() = true after corrupting index %d, want false", arity, i)
			}
		}
	}
}

// TestIsValidAgreesWithInvariantCheck compares IsValid, which walks from each
// child to its parent, with invariantError, which walks from each parent to
// its children, on slices that are mostly not heaps.
func TestIsValidAgreesWithInvariantCheck(t *testing.T) {
	for _, arity := range testArities {
		for _, n := range boundarySizes(arity) {
			h := New(arity, intLess)
			h.data = randomInts(n) // arbitrary order
			if got, want := h.IsValid(), invariantError(h) == nil; got != want {
				t.Errorf("d=%d, n=%d: IsValid() = %t on arbitrary data, want %t", arity, n, got, want)
			}

			h.heapify()

			if got, err := h.IsValid(), invariantError(h); !got || err != nil {
				t.Errorf("d=%d, n=%d: after heapify, IsValid() = %t and invariant check = %v, want true and <nil>", arity, n, got, err)
			}
		}
	}
}

// TestEqualElementsAreNotMoved checks the stop conditions of both sift
// loops: an element must never trade places with an element of the same
// priority, which would be wasted work.
func TestEqualElementsAreNotMoved(t *testing.T) {
	type item struct{ priority, id int }
	less := func(a, b item) bool { return a.priority < b.priority }

	for _, arity := range testArities {
		moves := 0
		h := New(arity, less, WithIndexTracking(func(item, int) { moves++ }))

		// siftUp: every push must leave the new element where it was appended.
		const n = 100
		for id := range n {
			h.Push(item{priority: 1, id: id})
		}

		for i, v := range h.data {
			if v.id != i {
				t.Fatalf("d=%d: element %d is at index %d; pushing equal elements must not reorder them", arity, v.id, i)
			}
		}

		if moves != n {
			t.Errorf("d=%d: %d pushes of equal elements reported %d index changes, want %d", arity, n, moves, n)
		}

		// siftDown: a new root that ties with its children must stay at the root.
		h.Replace(item{priority: 1, id: -1})

		if got := h.data[0].id; got != -1 {
			t.Errorf("d=%d: Replace with an equal element left element %d at the root, want the new element", arity, got)
		}
	}
}

// tracked is an element that records its own position, the typical use of
// WithIndexTracking.
type tracked struct {
	priority int
	index    int
}

func newTrackedHeap(arity int) *DHeap[*tracked] {
	return New(arity,
		func(a, b *tracked) bool { return a.priority < b.priority },
		WithIndexTracking(func(v *tracked, i int) { v.index = i }),
	)
}

// checkTracking verifies that every element in h knows its index and that
// every element in removed was told that it left the heap.
func checkTracking(t *testing.T, op string, h *DHeap[*tracked], removed []*tracked) {
	t.Helper()

	for i, v := range h.data {
		if v.index != i {
			t.Fatalf("after %s: element at index %d recorded index %d", op, i, v.index)
		}
	}

	for _, v := range removed {
		if v.index != -1 {
			t.Fatalf("after %s: removed element recorded index %d, want -1", op, v.index)
		}
	}

	if err := invariantError(h); err != nil {
		t.Fatalf("after %s: %v", op, err)
	}
}

func TestIndexTracking(t *testing.T) {
	for _, arity := range testArities {
		t.Run(fmt.Sprintf("d=%d", arity), func(t *testing.T) {
			r := rand.New(rand.NewPCG(uint64(arity), 7))
			h := newTrackedHeap(arity)
			var removed []*tracked

			for step := range 5000 {
				var op string

				switch r.IntN(8) {
				case 0, 1, 2:
					op = "Push"
					h.Push(&tracked{priority: r.IntN(1000), index: -2})
				case 3:
					op = "Pop"

					if v, ok := h.Pop(); ok {
						removed = append(removed, v)
					}
				case 4:
					op = "PopAt"

					if h.Len() > 0 {
						v, _ := h.PopAt(r.IntN(h.Len()))
						removed = append(removed, v)
					}
				case 5:
					op = "Fix"

					if h.Len() > 0 {
						i := r.IntN(h.Len())
						h.data[i].priority = r.IntN(1000)
						h.Fix(i)
					}
				case 6:
					op = "Update"

					if h.Len() > 0 {
						i := r.IntN(h.Len())
						removed = append(removed, h.data[i])
						h.Update(i, &tracked{priority: r.IntN(1000), index: -2})
					}
				case 7:
					if r.IntN(2) == 0 {
						op = "Replace"

						if prev, ok := h.Replace(&tracked{priority: r.IntN(1000), index: -2}); ok {
							removed = append(removed, prev)
						}
					} else {
						op = "PushPop"
						v := &tracked{priority: r.IntN(1000), index: -1}
						removed = append(removed, h.PushPop(v))
					}
				}

				checkTracking(t, fmt.Sprintf("step %d (%s)", step, op), h, removed)
			}

			removed = append(removed, h.data...)
			h.Clear()
			checkTracking(t, "Clear", h, removed)
		})
	}
}

func TestIndexTrackingUpdateWithSameElement(t *testing.T) {
	h := newTrackedHeap(4)
	items := make([]*tracked, 20)
	for i := range items {
		items[i] = &tracked{priority: i}
		h.Push(items[i])
	}

	// Passing the element that is already stored must leave it tracked.
	it := items[15]
	it.priority = -1
	if !h.Update(it.index, it) {
		t.Fatalf("Update(%d, same element) = false, want true", it.index)
	}

	if it.index != 0 {
		t.Errorf("element recorded index %d after becoming the root, want 0", it.index)
	}

	checkTracking(t, "Update with same element", h, nil)
}

func TestIndexTrackingFromSlice(t *testing.T) {
	for _, arity := range testArities {
		items := make([]*tracked, 100)
		for i, v := range randomInts(len(items)) {
			items[i] = &tracked{priority: v, index: -2}
		}

		calls := 0
		h := FromSlice(items, arity,
			func(a, b *tracked) bool { return a.priority < b.priority },
			WithIndexTracking(func(v *tracked, i int) { v.index = i; calls++ }),
		)
		checkTracking(t, fmt.Sprintf("FromSlice(d=%d)", arity), h, nil)

		if calls != len(items) {
			t.Errorf("FromSlice(d=%d) reported %d index changes, want %d (one per element)", arity, calls, len(items))
		}
	}
}

func TestCloneDoesNotInheritIndexTracking(t *testing.T) {
	h := newTrackedHeap(2)

	for _, v := range randomInts(50) {
		h.Push(&tracked{priority: v})
	}

	drain(h.Clone())
	checkTracking(t, "draining a clone", h, nil)
}

// TestRandomOperationsAgainstModel runs a long random sequence of operations
// and compares the heap with a sorted slice after every step.
func TestRandomOperationsAgainstModel(t *testing.T) {
	for _, arity := range testArities {
		t.Run(fmt.Sprintf("d=%d", arity), func(t *testing.T) {
			r := rand.New(rand.NewPCG(uint64(arity), 99))
			h := New(arity, intLess)
			var model []int // always sorted

			insert := func(v int) {
				at, _ := slices.BinarySearch(model, v)
				model = slices.Insert(model, at, v)
			}

			remove := func(v int) {
				at, found := slices.BinarySearch(model, v)
				if !found {
					t.Fatalf("heap returned %d, which the model does not contain", v)
				}

				model = slices.Delete(model, at, at+1)
			}

			for step := range 20000 {
				v := r.IntN(500)

				switch op := r.IntN(10); {
				case op < 4:
					h.Push(v)
					insert(v)
				case op < 6:
					got, ok := h.Pop()
					if ok != (len(model) > 0) {
						t.Fatalf("step %d: Pop() ok = %t with %d elements", step, ok, len(model))
					}

					if ok {
						if got != model[0] {
							t.Fatalf("step %d: Pop() = %d, want %d", step, got, model[0])
						}

						remove(got)
					}
				case op == 6:
					got, ok := h.Replace(v)
					if ok {
						if got != model[0] {
							t.Fatalf("step %d: Replace(%d) = %d, want %d", step, v, got, model[0])
						}

						remove(got)
					}

					insert(v)
				case op == 7:
					insert(v)
					want := model[0]
					if got := h.PushPop(v); got != want {
						t.Fatalf("step %d: PushPop(%d) = %d, want %d", step, v, got, want)
					}

					remove(want)
				case op == 8 && h.Len() > 0:
					got, _ := h.PopAt(r.IntN(h.Len()))
					remove(got)
				case op == 9 && h.Len() > 0:
					i := r.IntN(h.Len())
					remove(h.data[i])
					h.Update(i, v)
					insert(v)
				}

				if got, want := h.Len(), len(model); got != want {
					t.Fatalf("step %d: Len() = %d, want %d", step, got, want)
				}

				if err := invariantError(h); err != nil {
					t.Fatalf("step %d: %v", step, err)
				}

				if len(model) > 0 {
					if got, _ := h.Peek(); got != model[0] {
						t.Fatalf("step %d: Peek() = %d, want %d", step, got, model[0])
					}
				}
			}

			if got := drain(h); !slices.Equal(got, model) {
				t.Errorf("final heap content = %v, want %v", got, model)
			}
		})
	}
}

// TestLargeElements makes sure that sifting copies whole elements, not just a
// prefix of them.
func TestLargeElements(t *testing.T) {
	type large struct {
		key     int
		payload [16]int
	}

	h := New(4, func(a, b large) bool { return a.key < b.key })

	for _, v := range randomInts(500) {
		e := large{key: v}
		for i := range e.payload {
			e.payload[i] = v
		}

		h.Push(e)
	}

	prev := -1

	for _, e := range drain(h) {
		if e.key < prev {
			t.Fatalf("popped key %d after %d; elements are out of order", e.key, prev)
		}

		prev = e.key

		for i, p := range e.payload {
			if p != e.key {
				t.Fatalf("element with key %d has payload[%d] = %d; the element was corrupted", e.key, i, p)
			}
		}
	}
}

// TestSteadyStateDoesNotAllocate guards the main performance property of the
// heap: once the backing slice has grown, no operation allocates.
func TestSteadyStateDoesNotAllocate(t *testing.T) {
	for _, arity := range testArities {
		h := FromSlice(randomInts(1000), arity, intLess)
		h.Push(0) // Force the one growth of the backing slice up front.
		h.Pop()

		allocs := testing.AllocsPerRun(100, func() {
			v, _ := h.Pop()
			h.Push(v + 7)
			h.Replace(v + 3)
			h.PushPop(v + 5)
			h.Update(h.Len()/2, v+1)
			h.Fix(h.Len() / 3)
			w, _ := h.PopAt(h.Len() / 2)
			h.Push(w)
			h.Peek()
		})

		if allocs != 0 {
			t.Errorf("d=%d: steady-state operations allocate %v times per run, want 0", arity, allocs)
		}
	}
}
