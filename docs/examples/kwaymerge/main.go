// Command kwaymerge merges several sorted sequences into one, the core of
// external sorting, of log-structured storage engines and of merging
// time-ordered event streams.
//
// The heap holds one cursor per input, ordered by the value the cursor points
// at. The root is therefore always the smallest value not yet written. After
// writing it, the cursor is advanced and put back with DHeap.Replace, which
// repairs the heap in a single pass instead of the two passes of a Pop
// followed by a Push.
//
// Run it with:
//
//	go run ./docs/examples/kwaymerge
package main

import (
	"fmt"

	"github.com/christiangda/go-dheap"
)

// cursor is a position in one of the inputs.
type cursor struct {
	input int // which input the cursor reads from
	next  int // index of the current value in that input
}

// merge returns the values of all inputs in ascending order. Every input must
// already be sorted in ascending order. Equal values keep the order of their
// inputs, so the merge is stable.
func merge(inputs ...[]int) []int {
	value := func(c cursor) int { return inputs[c.input][c.next] }

	less := func(a, b cursor) bool {
		if va, vb := value(a), value(b); va != vb {
			return va < vb
		}

		return a.input < b.input // stable: earlier inputs first
	}

	// Start with a cursor at the beginning of every non-empty input. FromSlice
	// builds the heap in linear time.
	var cursors []cursor
	total := 0

	for i, in := range inputs {
		total += len(in)

		if len(in) > 0 {
			cursors = append(cursors, cursor{input: i})
		}
	}

	h := dheap.FromSlice(cursors, 4, less)

	merged := make([]int, 0, total)

	for {
		c, ok := h.Peek()
		if !ok {
			return merged
		}

		merged = append(merged, value(c))

		c.next++

		if c.next < len(inputs[c.input]) {
			h.Replace(c) // the input has more values: advance its cursor
		} else {
			h.Pop() // the input is exhausted: drop its cursor
		}
	}
}

func main() {
	shards := [][]int{
		{1, 4, 9, 16, 25},
		{2, 3, 5, 7, 11, 13},
		{},
		{4, 8, 12, 16, 20},
		{6},
	}

	for i, s := range shards {
		fmt.Printf("shard %d: %v\n", i, s)
	}

	fmt.Println("merged: ", merge(shards...))
}
