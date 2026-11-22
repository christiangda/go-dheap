package dheap

import (
	"cmp"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// orderedKinds are the two flavours of Ordered, with the constructors under
// test and the comparison a reference implementation would sort with.
var orderedKinds = []struct {
	name      string
	isMax     bool
	newHeap   func(arity int) *Ordered[int]
	fromSlice func(data []int, arity int) *Ordered[int]
	cmp       func(a, b int) int
}{
	{name: "min", isMax: false, newHeap: NewMin[int], fromSlice: FromSliceMin[int], cmp: ascending},
	{name: "max", isMax: true, newHeap: NewMax[int], fromSlice: FromSliceMax[int], cmp: descending},
}

// orderedInvariantError checks the heap ordering independently of
// Ordered.IsValid and of ordersBefore. It walks the tree from the parents
// down and compares with the plain operators. It returns nil if the heap is
// well formed. It must not be used with NaNs.
func orderedInvariantError[T cmp.Ordered](h *Ordered[T]) error {
	n := len(h.data)

	for parent := 0; parent < n; parent++ {
		first := parent*h.arity + 1

		for child := first; child < first+h.arity && child < n; child++ {
			violated := h.data[child] < h.data[parent]

			if h.desc {
				violated = h.data[child] > h.data[parent]
			}

			if violated {
				return fmt.Errorf("heap ordering violated: data[%d] = %v must be popped before its parent data[%d] = %v",
					child, h.data[child], parent, h.data[parent])
			}
		}
	}

	return nil
}

// drainOrdered pops every element of h and returns them in pop order.
func drainOrdered[T cmp.Ordered](h *Ordered[T]) []T {
	var got []T

	for {
		v, ok := h.Pop()
		if !ok {
			return got
		}

		got = append(got, v)
	}
}

func TestOrderedConstructorsPanic(t *testing.T) {
	const want = "dheap: arity must be at least 2"

	constructors := []struct {
		name string
		call func(arity int)
	}{
		{name: "NewMin", call: func(arity int) { NewMin[int](arity) }},
		{name: "NewMax", call: func(arity int) { NewMax[int](arity) }},
		{name: "FromSliceMin", call: func(arity int) { FromSliceMin([]int{1}, arity) }},
		{name: "FromSliceMax", call: func(arity int) { FromSliceMax([]int{1}, arity) }},
	}

	for _, c := range constructors {
		for _, arity := range []int{1, 0, -4} {
			t.Run(fmt.Sprintf("%s/arity=%d", c.name, arity), func(t *testing.T) {
				defer func() {
					if got := recover(); got != want {
						t.Errorf("%s(arity=%d) panicked with %v, want %q", c.name, arity, got, want)
					}
				}()

				c.call(arity)
			})
		}
	}
}

func TestOrderedNew(t *testing.T) {
	for _, kind := range orderedKinds {
		for _, arity := range testArities {
			h := kind.newHeap(arity)

			if got := h.Arity(); got != arity {
				t.Errorf("%s heap: Arity() = %d, want %d", kind.name, got, arity)
			}

			if got := h.IsMax(); got != kind.isMax {
				t.Errorf("%s heap: IsMax() = %t, want %t", kind.name, got, kind.isMax)
			}

			if got := h.Len(); got != 0 {
				t.Errorf("%s heap: Len() = %d, want 0", kind.name, got)
			}

			if !h.IsEmpty() {
				t.Errorf("%s heap: IsEmpty() = false, want true", kind.name)
			}

			if !h.IsValid() {
				t.Errorf("%s heap: IsValid() = false, want true", kind.name)
			}

			if got, want := h.shift, New(arity, intLess).shift; got != want {
				t.Errorf("%s heap with arity %d: shift = %d, want %d", kind.name, arity, got, want)
			}
		}
	}
}

func TestOrderedPushThenPopReturnsSortedOrder(t *testing.T) {
	for _, kind := range orderedKinds {
		for _, arity := range testArities {
			for _, n := range boundarySizes(arity) {
				t.Run(fmt.Sprintf("%s/d=%d/n=%d", kind.name, arity, n), func(t *testing.T) {
					vals := randomInts(n)
					h := kind.newHeap(arity)

					for i, v := range vals {
						h.Push(v)

						if got := h.Len(); got != i+1 {
							t.Fatalf("Len() after %d pushes = %d, want %d", i+1, got, i+1)
						}
					}

					if err := orderedInvariantError(h); err != nil {
						t.Fatalf("after %d pushes: %v", n, err)
					}

					if got := h.IsEmpty(); got != (n == 0) {
						t.Errorf("IsEmpty() after %d pushes = %t, want %t", n, got, n == 0)
					}

					got, want := drainOrdered(h), sortedCopy(vals, kind.cmp)
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

func TestOrderedFromSlice(t *testing.T) {
	for _, kind := range orderedKinds {
		for _, arity := range testArities {
			for _, n := range boundarySizes(arity) {
				t.Run(fmt.Sprintf("%s/d=%d/n=%d", kind.name, arity, n), func(t *testing.T) {
					vals := randomInts(n)
					input := slices.Clone(vals)

					h := kind.fromSlice(input, arity)

					if !slices.Equal(input, vals) {
						t.Errorf("the constructor modified its input: got %v, want %v", input, vals)
					}

					if got := h.Len(); got != n {
						t.Errorf("Len() = %d, want %d", got, n)
					}

					if got := h.IsMax(); got != kind.isMax {
						t.Errorf("IsMax() = %t, want %t", got, kind.isMax)
					}

					if err := orderedInvariantError(h); err != nil {
						t.Fatalf("after building from %v: %v", vals, err)
					}

					// The heap must not alias the caller's slice.
					for i := range input {
						input[i] = -1
					}

					got, want := drainOrdered(h), sortedCopy(vals, kind.cmp)
					if !slices.Equal(got, want) {
						t.Errorf("popped elements = %v, want %v", got, want)
					}
				})
			}
		}
	}
}

func TestOrderedFromSliceNil(t *testing.T) {
	h := FromSliceMin[int](nil, 4)
	if got := h.Len(); got != 0 {
		t.Errorf("FromSliceMin(nil).Len() = %d, want 0", got)
	}

	h.Push(1)

	if got, ok := h.Pop(); got != 1 || !ok {
		t.Errorf("Pop() = %d, %t, want 1, true", got, ok)
	}
}

func TestOrderedPeek(t *testing.T) {
	minHeap, maxHeap := NewMin[int](3), NewMax[int](3)

	if got, ok := minHeap.Peek(); got != 0 || ok {
		t.Errorf("Peek() on empty min-heap = %d, %t, want 0, false", got, ok)
	}

	vals := []int{5, 9, 3, 7, 1, 8}
	for i, v := range vals {
		minHeap.Push(v)
		maxHeap.Push(v)

		seen := vals[:i+1]

		if got, ok := minHeap.Peek(); got != slices.Min(seen) || !ok {
			t.Errorf("min-heap Peek() after pushing %v = %d, %t, want %d, true", seen, got, ok, slices.Min(seen))
		}

		if got, ok := maxHeap.Peek(); got != slices.Max(seen) || !ok {
			t.Errorf("max-heap Peek() after pushing %v = %d, %t, want %d, true", seen, got, ok, slices.Max(seen))
		}
	}

	if got := minHeap.Len(); got != len(vals) {
		t.Errorf("Len() after Peek = %d, want %d; Peek must not remove elements", got, len(vals))
	}
}

func TestOrderedPopEmpty(t *testing.T) {
	h := NewMin[string](2)
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

func TestOrderedReplace(t *testing.T) {
	for _, kind := range orderedKinds {
		t.Run(kind.name+"/empty heap", func(t *testing.T) {
			h := kind.newHeap(4)
			if got, ok := h.Replace(7); got != 0 || ok {
				t.Errorf("Replace(7) on empty heap = %d, %t, want 0, false", got, ok)
			}

			if got := drainOrdered(h); !slices.Equal(got, []int{7}) {
				t.Errorf("heap after Replace(7) = %v, want [7]", got)
			}
		})
	}

	initial := []int{9, 1, 7, 3, 5}

	for _, kind := range orderedKinds {
		for _, arity := range testArities {
			for _, v := range []int{-10, 1, 6, 9, 100} { // below, equal to, between and above the elements
				t.Run(fmt.Sprintf("%s/d=%d/v=%d", kind.name, arity, v), func(t *testing.T) {
					h := kind.fromSlice(initial, arity)

					// Replace always returns the old root, whatever v is.
					sorted := sortedCopy(initial, kind.cmp)
					wantPrev := sorted[0]
					wantHeap := sortedCopy(append(sorted[1:], v), kind.cmp)

					if got, ok := h.Replace(v); got != wantPrev || !ok {
						t.Errorf("Replace(%d) = %d, %t, want %d, true", v, got, ok, wantPrev)
					}

					if err := orderedInvariantError(h); err != nil {
						t.Errorf("after Replace(%d): %v", v, err)
					}

					if got := drainOrdered(h); !slices.Equal(got, wantHeap) {
						t.Errorf("heap after Replace(%d) = %v, want %v", v, got, wantHeap)
					}
				})
			}
		}
	}
}

func TestOrderedPushPop(t *testing.T) {
	for _, kind := range orderedKinds {
		t.Run(kind.name+"/empty heap", func(t *testing.T) {
			h := kind.newHeap(4)
			if got := h.PushPop(7); got != 7 {
				t.Errorf("PushPop(7) on empty heap = %d, want 7", got)
			}

			if got := h.Len(); got != 0 {
				t.Errorf("Len() after PushPop on empty heap = %d, want 0", got)
			}
		})
	}

	initial := []int{9, 1, 7, 3, 5}

	for _, kind := range orderedKinds {
		for _, arity := range testArities {
			for _, v := range []int{-10, 1, 6, 9, 100} { // below, equal to, between and above the elements
				t.Run(fmt.Sprintf("%s/d=%d/v=%d", kind.name, arity, v), func(t *testing.T) {
					h := kind.fromSlice(initial, arity)

					// PushPop behaves like a Push followed by a Pop.
					all := sortedCopy(append(slices.Clone(initial), v), kind.cmp)
					want, wantHeap := all[0], all[1:]

					if got := h.PushPop(v); got != want {
						t.Errorf("PushPop(%d) = %d, want %d", v, got, want)
					}

					if err := orderedInvariantError(h); err != nil {
						t.Errorf("after PushPop(%d): %v", v, err)
					}

					if got := drainOrdered(h); !slices.Equal(got, wantHeap) {
						t.Errorf("heap after PushPop(%d) = %v, want %v", v, got, wantHeap)
					}
				})
			}
		}
	}
}

// TestOrderedPushPopLeavesHeapUntouched checks the fast path of PushPop: a
// value that would be popped straight away must not reorder the heap, not
// even when it ties with the root.
func TestOrderedPushPopLeavesHeapUntouched(t *testing.T) {
	minHeap := FromSliceMin([]int{5, 6, 7, 8, 9}, 2)
	before := slices.Clone(minHeap.data)

	for _, v := range []int{4, 5} {
		if got := minHeap.PushPop(v); got != v {
			t.Errorf("min-heap PushPop(%d) = %d, want %d", v, got, v)
		}

		if !slices.Equal(minHeap.data, before) {
			t.Errorf("min-heap PushPop(%d) changed the layout to %v, want %v", v, minHeap.data, before)
		}
	}

	maxHeap := FromSliceMax([]int{5, 6, 7, 8, 9}, 2)
	before = slices.Clone(maxHeap.data)

	for _, v := range []int{10, 9} {
		if got := maxHeap.PushPop(v); got != v {
			t.Errorf("max-heap PushPop(%d) = %d, want %d", v, got, v)
		}

		if !slices.Equal(maxHeap.data, before) {
			t.Errorf("max-heap PushPop(%d) changed the layout to %v, want %v", v, maxHeap.data, before)
		}
	}
}

func TestOrderedClear(t *testing.T) {
	h := NewMin[string](4)

	for _, v := range randomInts(100) {
		h.Push(fmt.Sprint(v))
	}

	capBefore := cap(h.data)

	h.Clear()

	if got := h.Len(); got != 0 {
		t.Errorf("Len() after Clear = %d, want 0", got)
	}

	if got := cap(h.data); got != capBefore {
		t.Errorf("capacity after Clear = %d, want %d", got, capBefore)
	}

	for i, s := range h.data[:capBefore] {
		if s != "" {
			t.Fatalf("Clear left the string %q at index %d; elements must be released", s, i)
		}
	}

	// The heap stays usable.
	h.Push("b")
	h.Push("a")

	if got, ok := h.Pop(); got != "a" || !ok {
		t.Errorf("Pop() after Clear and two pushes = %q, %t, want %q, true", got, ok, "a")
	}
}

func TestOrderedPopReleasesStrings(t *testing.T) {
	h := FromSliceMin([]string{"d", "b", "f", "a", "e"}, 2)
	h.Pop()

	if got := h.Len(); got != 4 {
		t.Fatalf("Len() after Pop = %d, want 4", got)
	}

	if got := h.data[:5][4]; got != "" {
		t.Errorf("Pop left the string %q past the length; elements must be released", got)
	}
}

func TestOrderedClone(t *testing.T) {
	for _, kind := range orderedKinds {
		t.Run(kind.name, func(t *testing.T) {
			vals := randomInts(100)
			h := kind.fromSlice(vals, 4)

			c := h.Clone()
			if got, want := c.Arity(), h.Arity(); got != want {
				t.Errorf("Clone().Arity() = %d, want %d", got, want)
			}

			if got, want := c.IsMax(), h.IsMax(); got != want {
				t.Errorf("Clone().IsMax() = %t, want %t", got, want)
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

			if err := orderedInvariantError(c); err != nil {
				t.Errorf("clone after changes: %v", err)
			}

			got, want := drainOrdered(h), sortedCopy(append(slices.Clone(vals), -2), kind.cmp)
			if !slices.Equal(got, want) {
				t.Errorf("original after changing the clone = %v, want %v", got, want)
			}
		})
	}
}

func TestOrderedIsValid(t *testing.T) {
	for _, kind := range orderedKinds {
		for _, arity := range testArities {
			for _, n := range boundarySizes(arity) {
				// IsValid must agree with the independent check on arbitrary
				// data, most of which is not a heap.
				h := kind.newHeap(arity)
				h.data = randomInts(n)

				if got, want := h.IsValid(), orderedInvariantError(h) == nil; got != want {
					t.Errorf("%s heap, d=%d, n=%d: IsValid() = %t on arbitrary data, want %t", kind.name, arity, n, got, want)
				}

				h.heapify()

				if got, err := h.IsValid(), orderedInvariantError(h); !got || err != nil {
					t.Errorf("%s heap, d=%d, n=%d: after heapify, IsValid() = %t and invariant check = %v, want true and <nil>",
						kind.name, arity, n, got, err)
				}
			}
		}
	}

	// A single element out of place must be detected at every position.
	for _, arity := range testArities {
		for i := 1; i < 40; i++ {
			h := FromSliceMin(randomInts(40), arity)
			h.data[i] = -1 // Now smaller than its parent.

			if h.IsValid() {
				t.Errorf("d=%d: IsValid() = true after corrupting index %d, want false", arity, i)
			}
		}
	}
}

// TestOrderedRandomOperationsAgainstModel runs a long random sequence of
// operations and compares the heap with a sorted slice after every step.
func TestOrderedRandomOperationsAgainstModel(t *testing.T) {
	for _, kind := range orderedKinds {
		for _, arity := range testArities {
			t.Run(fmt.Sprintf("%s/d=%d", kind.name, arity), func(t *testing.T) {
				r := rand.New(rand.NewPCG(uint64(arity), 123))
				h := kind.newHeap(arity)

				var model []int // always sorted in pop order

				insert := func(v int) {
					at, _ := slices.BinarySearchFunc(model, v, kind.cmp)
					model = slices.Insert(model, at, v)
				}

				for step := range 20000 {
					v := r.IntN(500)

					switch op := r.IntN(10); {
					case op < 4:
						h.Push(v)
						insert(v)

					case op < 7:
						got, ok := h.Pop()
						if ok != (len(model) > 0) {
							t.Fatalf("step %d: Pop() ok = %t with %d elements", step, ok, len(model))
						}

						if ok {
							if got != model[0] {
								t.Fatalf("step %d: Pop() = %d, want %d", step, got, model[0])
							}

							model = model[1:]
						}

					case op == 7:
						got, ok := h.Replace(v)
						if ok {
							if got != model[0] {
								t.Fatalf("step %d: Replace(%d) = %d, want %d", step, v, got, model[0])
							}

							model = model[1:]
						}

						insert(v)

					case op == 8:
						insert(v)

						want := model[0]
						if got := h.PushPop(v); got != want {
							t.Fatalf("step %d: PushPop(%d) = %d, want %d", step, v, got, want)
						}

						model = model[1:]

					case op == 9 && r.IntN(200) == 0:
						h.Clear()
						model = model[:0]
					}

					if got, want := h.Len(), len(model); got != want {
						t.Fatalf("step %d: Len() = %d, want %d", step, got, want)
					}

					if err := orderedInvariantError(h); err != nil {
						t.Fatalf("step %d: %v", step, err)
					}

					if len(model) > 0 {
						if got, _ := h.Peek(); got != model[0] {
							t.Fatalf("step %d: Peek() = %d, want %d", step, got, model[0])
						}
					}
				}

				if got := drainOrdered(h); !slices.Equal(got, model) {
					t.Errorf("final heap content = %v, want %v", got, model)
				}
			})
		}
	}
}

// TestOrderedMatchesDHeap checks the claim in the documentation that an
// Ordered heap returns the same elements in the same order as a DHeap with
// the corresponding less function.
func TestOrderedMatchesDHeap(t *testing.T) {
	for _, arity := range testArities {
		vals := randomInts(2000)

		minOrdered, minFunc := FromSliceMin(vals, arity), FromSlice(vals, arity, intLess)
		if got, want := drainOrdered(minOrdered), drain(minFunc); !slices.Equal(got, want) {
			t.Errorf("d=%d: min-heap popped %v, a DHeap with a < b popped %v", arity, got, want)
		}

		maxOrdered, maxFunc := FromSliceMax(vals, arity), FromSlice(vals, arity, intGreater)
		if got, want := drainOrdered(maxOrdered), drain(maxFunc); !slices.Equal(got, want) {
			t.Errorf("d=%d: max-heap popped %v, a DHeap with a > b popped %v", arity, got, want)
		}
	}
}

func TestOrderedElementTypes(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		words := []string{"pear", "apple", "fig", "", "banana", "apple", "Zucchini"}

		want := slices.Sorted(slices.Values(words))
		if got := drainOrdered(FromSliceMin(words, 3)); !slices.Equal(got, want) {
			t.Errorf("min-heap of strings popped %q, want %q", got, want)
		}

		slices.Reverse(want)

		if got := drainOrdered(FromSliceMax(words, 3)); !slices.Equal(got, want) {
			t.Errorf("max-heap of strings popped %q, want %q", got, want)
		}
	})

	t.Run("unsigned integers", func(t *testing.T) {
		vals := []uint8{200, 0, 255, 17, 128, 1}

		want := slices.Sorted(slices.Values(vals))
		if got := drainOrdered(FromSliceMin(vals, 2)); !slices.Equal(got, want) {
			t.Errorf("min-heap of uint8 popped %v, want %v", got, want)
		}
	})

	t.Run("floats", func(t *testing.T) {
		vals := []float64{2.5, math.Inf(1), -1, 0, math.Inf(-1), 1e-9, -1}

		want := slices.Sorted(slices.Values(vals))
		if got := drainOrdered(FromSliceMin(vals, 4)); !slices.Equal(got, want) {
			t.Errorf("min-heap of float64 popped %v, want %v", got, want)
		}
	})
}

// TestOrderedNaN checks the documented place of NaN in the order: before
// every other value, so first out of a min-heap and last out of a max-heap.
func TestOrderedNaN(t *testing.T) {
	nan := math.NaN()
	vals := []float64{3, nan, -1, math.Inf(-1), nan, 2, math.Inf(1)}

	for _, arity := range testArities {
		got := drainOrdered(FromSliceMin(vals, arity))
		if !math.IsNaN(got[0]) || !math.IsNaN(got[1]) {
			t.Errorf("d=%d: min-heap popped %v, want the two NaNs first", arity, got)
		}

		if want := []float64{math.Inf(-1), -1, 2, 3, math.Inf(1)}; !slices.Equal(got[2:], want) {
			t.Errorf("d=%d: min-heap popped %v after the NaNs, want %v", arity, got[2:], want)
		}

		got = drainOrdered(FromSliceMax(vals, arity))
		if !math.IsNaN(got[5]) || !math.IsNaN(got[6]) {
			t.Errorf("d=%d: max-heap popped %v, want the two NaNs last", arity, got)
		}

		if want := []float64{math.Inf(1), 3, 2, -1, math.Inf(-1)}; !slices.Equal(got[:5], want) {
			t.Errorf("d=%d: max-heap popped %v before the NaNs, want %v", arity, got[:5], want)
		}
	}
}

// TestOrderedSteadyStateDoesNotAllocate guards the main performance property
// of the heap: once the backing slice has grown, no operation allocates.
func TestOrderedSteadyStateDoesNotAllocate(t *testing.T) {
	for _, arity := range testArities {
		h := FromSliceMin(randomInts(1000), arity)
		h.Push(0) // Force the one growth of the backing slice up front.
		h.Pop()

		allocs := testing.AllocsPerRun(100, func() {
			v, _ := h.Pop()
			h.Push(v + 7)
			h.Replace(v + 3)
			h.PushPop(v + 5)
			h.Peek()
		})

		if allocs != 0 {
			t.Errorf("d=%d: steady-state operations allocate %v times per run, want 0", arity, allocs)
		}
	}
}
