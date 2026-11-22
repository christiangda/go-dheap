// Command priorityqueue shows how to build a stable priority queue on top of
// a d-ary heap.
//
// A heap on its own returns elements of equal priority in an unspecified
// order. The queue in this example adds a sequence number to every entry and
// uses it as a tie-breaker, so that elements with the same priority leave the
// queue in the order they entered it.
//
// Run it with:
//
//	go run ./docs/examples/priorityqueue
package main

import (
	"fmt"

	"github.com/christiangda/go-dheap"
)

// PriorityQueue is a queue that returns the value with the highest priority
// first, and values of equal priority in first-in, first-out order.
//
// The zero value is not usable; create a queue with NewPriorityQueue.
type PriorityQueue[T any] struct {
	// heap is the storage of the queue. The queue itself only adds the
	// bookkeeping the heap does not do: the tie-breaking sequence number.
	heap *dheap.DHeap[entry[T]]

	// seq is the sequence number of the next entry.
	seq uint64
}

// entry is what the queue stores in the heap for every value.
type entry[T any] struct {
	value    T
	priority int
	seq      uint64 // insertion order, used to break ties
}

// NewPriorityQueue returns an empty queue.
func NewPriorityQueue[T any]() *PriorityQueue[T] {
	less := func(a, b entry[T]) bool {
		if a.priority != b.priority {
			return a.priority > b.priority // higher priority first
		}
		return a.seq < b.seq // then older entries first
	}

	// Arity 4 is a good default: the tree is half as deep as a binary heap
	// and the four children of a node are neighbours in memory.
	return &PriorityQueue[T]{heap: dheap.New(4, less)}
}

// Enqueue adds value to the queue with the given priority.
func (q *PriorityQueue[T]) Enqueue(value T, priority int) {
	q.heap.Push(entry[T]{value: value, priority: priority, seq: q.seq})
	q.seq++
}

// Dequeue removes and returns the value with the highest priority. The
// boolean result is false if the queue is empty.
func (q *PriorityQueue[T]) Dequeue() (T, bool) {
	e, ok := q.heap.Pop()
	return e.value, ok
}

// Peek returns the value with the highest priority without removing it. The
// boolean result is false if the queue is empty.
func (q *PriorityQueue[T]) Peek() (T, bool) {
	e, ok := q.heap.Peek()
	return e.value, ok
}

// Len returns the number of values in the queue.
func (q *PriorityQueue[T]) Len() int {
	return q.heap.Len()
}

func main() {
	const (
		low    = 1
		normal = 5
		urgent = 10
	)

	q := NewPriorityQueue[string]()
	q.Enqueue("answer email", normal)
	q.Enqueue("water the plants", low)
	q.Enqueue("production is down", urgent)
	q.Enqueue("review pull request", normal)
	q.Enqueue("database is full", urgent)
	q.Enqueue("update dependencies", normal)

	next, _ := q.Peek()
	fmt.Printf("%d tasks queued, next: %q\n\n", q.Len(), next)

	for q.Len() > 0 {
		task, _ := q.Dequeue()
		fmt.Println(task)
	}
}
