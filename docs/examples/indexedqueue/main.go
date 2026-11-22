// Command indexedqueue shows how to build a priority queue whose entries can
// be re-prioritized or cancelled after they were queued.
//
// Both operations need to know where an entry currently sits inside the heap.
// The queue asks the heap to report that with dheap.WithIndexTracking and
// stores the index in the entry itself. The entry returned by Push then acts
// as a handle for Update and Remove.
//
// Run it with:
//
//	go run ./docs/examples/indexedqueue
package main

import (
	"fmt"

	"github.com/christiangda/go-dheap"
)

// Item is an entry of a Queue. Push returns it as a handle that identifies
// the entry in later calls to Update and Remove.
type Item[T any] struct {
	Value T

	priority int

	// index is the position of the item in the heap, or -1 if the item is not
	// queued. It is written by the heap, never by the queue.
	index int
}

// Priority returns the current priority of the item.
func (it *Item[T]) Priority() int { return it.priority }

// Queued reports whether the item is still waiting in its queue.
func (it *Item[T]) Queued() bool { return it.index >= 0 }

// Queue is a priority queue that returns the item with the lowest priority
// value first and supports changing and removing queued items.
//
// The zero value is not usable; create a queue with NewQueue.
type Queue[T any] struct {
	heap *dheap.DHeap[*Item[T]]
}

// NewQueue returns an empty queue.
func NewQueue[T any]() *Queue[T] {
	return &Queue[T]{
		heap: dheap.New(4,
			func(a, b *Item[T]) bool { return a.priority < b.priority },
			// The heap calls this function every time an item changes
			// position, and with -1 when the item leaves the heap.
			dheap.WithIndexTracking(func(it *Item[T], i int) { it.index = i }),
		),
	}
}

// Push adds value with the given priority and returns its handle.
func (q *Queue[T]) Push(value T, priority int) *Item[T] {
	it := &Item[T]{Value: value, priority: priority, index: -1}
	q.heap.Push(it)

	return it
}

// Pop removes and returns the item with the lowest priority value. The
// boolean result is false if the queue is empty.
func (q *Queue[T]) Pop() (*Item[T], bool) {
	return q.heap.Pop()
}

// Update changes the priority of a queued item in O(log n) time. It reports
// whether the item was still queued.
func (q *Queue[T]) Update(it *Item[T], priority int) bool {
	if !it.Queued() {
		return false
	}

	it.priority = priority

	return q.heap.Fix(it.index) // the item moves up or down as needed
}

// Remove takes a queued item out of the queue in O(log n) time, wherever it
// is. It reports whether the item was still queued.
func (q *Queue[T]) Remove(it *Item[T]) bool {
	if !it.Queued() {
		return false
	}

	_, ok := q.heap.PopAt(it.index)

	return ok
}

// Len returns the number of queued items.
func (q *Queue[T]) Len() int {
	return q.heap.Len()
}

func main() {
	// A timer wheel in miniature: the priority is the time, in milliseconds,
	// at which a timer fires.
	timers := NewQueue[string]()

	timers.Push("flush metrics", 100)
	retry := timers.Push("retry request", 250)
	timeout := timers.Push("request timeout", 400)
	timers.Push("refresh token", 300)
	gc := timers.Push("collect garbage", 900)

	// The request succeeded: its timeout is no longer needed.
	fmt.Println("cancel timeout: ", timers.Remove(timeout))
	fmt.Println("cancel it again:", timers.Remove(timeout))

	// The retry is rescheduled to run first, garbage collection is pulled in.
	timers.Update(retry, 50)
	timers.Update(gc, 275)

	fmt.Println()

	for timers.Len() > 0 {
		it, _ := timers.Pop()
		fmt.Printf("t=%3dms  %s\n", it.Priority(), it.Value)
	}

	// A popped item knows that it left the queue.
	fmt.Println()
	fmt.Println("retry still queued:", retry.Queued())
}
