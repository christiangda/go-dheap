package dheap_test

import (
	"fmt"

	"github.com/christiangda/go-dheap"
)

// A min-heap of integers: the smallest value is popped first.
func Example() {
	h := dheap.New(4, func(a, b int) bool { return a < b })

	for _, v := range []int{5, 2, 8, 1, 9, 3} {
		h.Push(v)
	}

	for !h.IsEmpty() {
		v, _ := h.Pop()
		fmt.Println(v)
	}

	// Output:
	// 1
	// 2
	// 3
	// 5
	// 8
	// 9
}

// Reversing the comparison turns the heap into a max-heap.
func ExampleNew_maxHeap() {
	h := dheap.New(4, func(a, b int) bool { return a > b })

	for _, v := range []int{5, 2, 8, 1, 9, 3} {
		h.Push(v)
	}

	top, _ := h.Peek()
	fmt.Println("largest:", top)

	// Output:
	// largest: 9
}

// Elements can be of any type. The less function decides which field is the
// priority and how ties are broken.
func ExampleNew_structs() {
	type job struct {
		name     string
		priority int
	}

	h := dheap.New(4, func(a, b job) bool {
		if a.priority != b.priority {
			return a.priority > b.priority // higher priority first
		}

		return a.name < b.name // then alphabetically
	})

	h.Push(job{name: "send report", priority: 1})
	h.Push(job{name: "page on-call", priority: 9})
	h.Push(job{name: "rotate logs", priority: 1})
	h.Push(job{name: "restart service", priority: 5})

	for !h.IsEmpty() {
		j, _ := h.Pop()
		fmt.Printf("%d %s\n", j.priority, j.name)
	}

	// Output:
	// 9 page on-call
	// 5 restart service
	// 1 rotate logs
	// 1 send report
}

// FromSlice builds a heap from existing data in linear time and leaves the
// slice it was given untouched.
func ExampleFromSlice() {
	data := []int{7, 3, 9, 1, 4}
	h := dheap.FromSlice(data, 2, func(a, b int) bool { return a < b })

	top, _ := h.Peek()
	fmt.Println("len:", h.Len())
	fmt.Println("min:", top)
	fmt.Println("data:", data)

	// Output:
	// len: 5
	// min: 1
	// data: [7 3 9 1 4]
}

// Peek and Pop report an empty heap through their second result, so the zero
// value of the element type is never ambiguous.
func ExampleDHeap_Pop() {
	h := dheap.New(4, func(a, b int) bool { return a < b })
	h.Push(0)

	v, ok := h.Pop()
	fmt.Println(v, ok)

	v, ok = h.Pop()
	fmt.Println(v, ok)

	// Output:
	// 0 true
	// 0 false
}

// PushPop keeps the k largest values of a stream in a min-heap of size k: the
// root is the smallest of the values kept, and it is evicted whenever a larger
// value arrives.
func ExampleDHeap_PushPop() {
	const k = 3
	h := dheap.New(4, func(a, b int) bool { return a < b })

	for _, v := range []int{4, 9, 1, 7, 3, 8, 2, 6} {
		if h.Len() < k {
			h.Push(v)

			continue
		}

		h.PushPop(v)
	}

	for !h.IsEmpty() {
		v, _ := h.Pop()
		fmt.Println(v)
	}

	// Output:
	// 7
	// 8
	// 9
}

// Replace pops the root and pushes a new element in one step. Here it advances
// a set of counters, always picking the one that is furthest behind.
func ExampleDHeap_Replace() {
	h := dheap.FromSlice([]int{10, 20, 30}, 2, func(a, b int) bool { return a < b })

	for range 4 {
		smallest, _ := h.Peek()
		h.Replace(smallest + 15)
		fmt.Println(smallest)
	}

	// Output:
	// 10
	// 20
	// 25
	// 30
}

// WithIndexTracking lets every element know where it is stored, which makes
// it possible to change the priority of an element, or to remove it, long
// after it was pushed.
func ExampleWithIndexTracking() {
	type task struct {
		name     string
		priority int
		index    int // maintained by the heap; -1 once the task is removed
	}

	h := dheap.New(4,
		func(a, b *task) bool { return a.priority > b.priority },
		dheap.WithIndexTracking(func(t *task, i int) { t.index = i }),
	)

	write := &task{name: "write docs", priority: 1}
	fix := &task{name: "fix bug", priority: 5}
	lunch := &task{name: "lunch", priority: 3}
	for _, t := range []*task{write, fix, lunch} {
		h.Push(t)
	}

	// The docs became urgent: change the priority in place and tell the heap.
	write.priority = 10
	h.Fix(write.index)

	// Lunch is cancelled: remove it from the middle of the heap.
	h.PopAt(lunch.index)
	fmt.Println("lunch index:", lunch.index)

	for !h.IsEmpty() {
		t, _ := h.Pop()
		fmt.Println(t.priority, t.name)
	}

	// Output:
	// lunch index: -1
	// 10 write docs
	// 5 fix bug
}

// Update replaces the element stored at an index with a new value.
func ExampleDHeap_Update() {
	pos := map[string]int{} // name -> index, maintained by the heap

	type entry struct {
		name  string
		score int
	}

	h := dheap.New(2,
		func(a, b entry) bool { return a.score > b.score },
		dheap.WithIndexTracking(func(e entry, i int) { pos[e.name] = i }),
	)

	h.Push(entry{name: "ana", score: 10})
	h.Push(entry{name: "bob", score: 30})
	h.Push(entry{name: "eve", score: 20})

	h.Update(pos["ana"], entry{name: "ana", score: 50})

	for !h.IsEmpty() {
		e, _ := h.Pop()
		fmt.Println(e.name, e.score)
	}

	// Output:
	// ana 50
	// bob 30
	// eve 20
}

// Draining a clone visits the elements in priority order without disturbing
// the original heap.
func ExampleDHeap_Clone() {
	h := dheap.FromSlice([]int{3, 1, 2}, 4, func(a, b int) bool { return a < b })

	c := h.Clone()
	for !c.IsEmpty() {
		v, _ := c.Pop()
		fmt.Println(v)
	}

	fmt.Println("original still holds", h.Len(), "elements")

	// Output:
	// 1
	// 2
	// 3
	// original still holds 3 elements
}

// Clear empties the heap but keeps its memory, so refilling it does not
// allocate.
func ExampleDHeap_Clear() {
	h := dheap.FromSlice([]int{3, 1, 2}, 4, func(a, b int) bool { return a < b })

	h.Clear()
	fmt.Println(h.Len(), h.IsEmpty())

	// Output:
	// 0 true
}

// NewMin is the fast way to get a min-heap of numbers or strings: there is
// no less function to write, and none to call.
func ExampleNewMin() {
	h := dheap.NewMin[int](4)

	for _, v := range []int{5, 2, 8, 1, 9, 3} {
		h.Push(v)
	}

	for !h.IsEmpty() {
		v, _ := h.Pop()
		fmt.Println(v)
	}

	// Output:
	// 1
	// 2
	// 3
	// 5
	// 8
	// 9
}

// NewMax returns the largest element first.
func ExampleNewMax() {
	h := dheap.NewMax[float64](4)

	for _, v := range []float64{2.5, -1, 7.25, 0} {
		h.Push(v)
	}

	top, _ := h.Peek()
	fmt.Println("largest:", top)
	fmt.Println("is max-heap:", h.IsMax())

	// Output:
	// largest: 7.25
	// is max-heap: true
}

// FromSliceMin builds a heap from existing data in linear time. Strings are
// ordered byte by byte, like the < operator orders them.
func ExampleFromSliceMin() {
	words := []string{"pear", "apple", "fig", "banana"}
	h := dheap.FromSliceMin(words, 2)

	for !h.IsEmpty() {
		w, _ := h.Pop()
		fmt.Println(w)
	}

	// Output:
	// apple
	// banana
	// fig
	// pear
}

// FromSliceMax builds a max-heap, here to read scores from best to worst.
func ExampleFromSliceMax() {
	scores := []int{71, 94, 58, 86}
	h := dheap.FromSliceMax(scores, 4)

	best, _ := h.Pop()
	second, _ := h.Pop()
	fmt.Println(best, second)
	fmt.Println("left:", h.Len())

	// Output:
	// 94 86
	// left: 2
}

// A min-heap of bounded size keeps the k largest values of a stream: its
// root is the smallest value kept, and PushPop evicts it whenever a larger
// one arrives.
func ExampleOrdered_PushPop() {
	const k = 3

	h := dheap.NewMin[int](4)

	for _, v := range []int{4, 9, 1, 7, 3, 8, 2, 6} {
		if h.Len() < k {
			h.Push(v)

			continue
		}

		h.PushPop(v)
	}

	for !h.IsEmpty() {
		v, _ := h.Pop()
		fmt.Println(v)
	}

	// Output:
	// 7
	// 8
	// 9
}

// Replace pops the root and pushes a new element in a single pass.
func ExampleOrdered_Replace() {
	h := dheap.FromSliceMin([]int{10, 20, 30}, 2)

	for range 4 {
		smallest, _ := h.Peek()
		h.Replace(smallest + 15)
		fmt.Println(smallest)
	}

	// Output:
	// 10
	// 20
	// 25
	// 30
}
