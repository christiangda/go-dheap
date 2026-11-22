// Command median computes the median of a stream of numbers as they arrive,
// using two heaps of an ordered type.
//
// The numbers seen so far are split into two halves. A max-heap holds the
// smaller half, so its root is the largest of the small numbers. A min-heap
// holds the larger half, so its root is the smallest of the large numbers.
// The median is then always at one of the two roots, or between them.
//
//	smaller half (max-heap)      larger half (min-heap)
//	      1  3  [5]          |        [8]  9  12
//	             ▲ root                ▲ root
//
// Because the elements are plain numbers, the example uses dheap.Ordered,
// created with NewMax and NewMin. It needs no less function and is faster
// than a DHeap for this kind of element.
//
// Run it with:
//
//	go run ./docs/examples/median
package main

import (
	"fmt"

	"github.com/christiangda/go-dheap"
)

// RunningMedian keeps the median of all the values added to it.
//
// The zero value is not usable; create one with NewRunningMedian.
type RunningMedian struct {
	// lower holds the smaller half of the values. Its root is their maximum.
	lower *dheap.Ordered[float64]

	// upper holds the larger half of the values. Its root is their minimum.
	// It never holds more values than lower, and at most one fewer.
	upper *dheap.Ordered[float64]
}

// NewRunningMedian returns a RunningMedian without any values.
func NewRunningMedian() *RunningMedian {
	return &RunningMedian{
		lower: dheap.NewMax[float64](4),
		upper: dheap.NewMin[float64](4),
	}
}

// Add records the value v in O(log n) time.
func (m *RunningMedian) Add(v float64) {
	if m.lower.Len() == m.upper.Len() {
		// The lower half must grow. Pass v through the upper half first, so
		// that the value moving down is the smallest of the large values.
		m.lower.Push(m.upper.PushPop(v))

		return
	}

	// The upper half must grow. Pass v through the lower half first, so that
	// the value moving up is the largest of the small values.
	m.upper.Push(m.lower.PushPop(v))
}

// Median returns the median of the values added so far in O(1) time. The
// boolean result is false if no value was added.
func (m *RunningMedian) Median() (float64, bool) {
	low, ok := m.lower.Peek()
	if !ok {
		return 0, false
	}

	if m.lower.Len() > m.upper.Len() {
		return low, true // An odd number of values: the middle one.
	}

	high, _ := m.upper.Peek()

	return (low + high) / 2, true // An even number: between the middle two.
}

// Len returns the number of values added so far.
func (m *RunningMedian) Len() int {
	return m.lower.Len() + m.upper.Len()
}

func main() {
	latencies := []float64{12, 5, 48, 9, 3, 27, 8, 150, 11, 6}

	m := NewRunningMedian()

	fmt.Println("latency  median")

	for _, ms := range latencies {
		m.Add(ms)

		median, _ := m.Median()
		fmt.Printf("%5.0fms  %5.1fms\n", ms, median)
	}

	fmt.Printf("\n%d samples\n", m.Len())
}
