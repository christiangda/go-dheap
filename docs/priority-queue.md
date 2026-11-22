# 🚦 Building priority queues on top of `DHeap`

A heap is the storage engine of a priority queue: it keeps the most important
element at hand and everything else in good enough order to find the next one
quickly. The heaps of this package deliberately stop there. They know nothing
about priorities, handles or tie-breaking, because those differ from queue to
queue.

This guide shows how to add them. Every section is backed by a complete
program under [examples/](examples/) that you can run and copy from.

| I need… | Pattern | Example |
| --- | --- | --- |
| "Highest priority first", FIFO among equals | [Stable queue](#1--a-stable-priority-queue) | [priorityqueue](examples/priorityqueue/main.go) |
| To change the priority of a queued item or cancel it | [Indexed queue](#2--changing-and-removing-queued-items) | [indexedqueue](examples/indexedqueue/main.go) |
| Shortest paths, A\*, Prim | [Decrease-key](#3--decrease-key-dijkstras-algorithm) | [dijkstra](examples/dijkstra/main.go) |
| The best *k* of a large or endless stream | [Bounded heap](#4--top-k-with-a-bounded-heap) | [topk](examples/topk/main.go) |
| To merge sorted inputs | [Cursor heap](#5--k-way-merge) | [kwaymerge](examples/kwaymerge/main.go) |
| A median or percentile that follows a stream | [Two heaps](#6--two-heaps-of-numbers-a-running-median) | [median](examples/median/main.go) |

## 🧭 Which heap type?

The package has two, and picking one is the first decision:

| Your elements are… | Use | Created with |
| --- | --- | --- |
| Numbers or strings that *are* the priority | `Ordered` | `NewMin`, `NewMax`, `FromSliceMin`, `FromSliceMax` |
| Structs, pointers or IDs ordered by a field or by outside data | `DHeap` | `New`, `FromSlice` |
| Anything you need to update or remove while it is queued | `DHeap` | `New` with `WithIndexTracking` |

`Ordered` is the faster one, because it compares with the `<` operator instead
of calling a function. Most priority queues carry a payload next to the
priority, though, and those are `DHeap` territory. Sections 1 to 5 use
`DHeap`; [section 6](#6--two-heaps-of-numbers-a-running-median) uses
`Ordered`.

## The one rule: `less` decides who goes first

`less(a, b)` answers a single question: *must `a` leave the queue before `b`?*
Everything about the queue's behaviour is encoded there.

```go
// Smallest number first (min-heap).
dheap.New(4, func(a, b int) bool { return a < b })

// Largest number first (max-heap).
dheap.New(4, func(a, b int) bool { return a > b })

// Earliest deadline first; among equal deadlines, the cheaper job.
dheap.New(4, func(a, b job) bool {
    if !a.deadline.Equal(b.deadline) {
        return a.deadline.Before(b.deadline)
    }
    return a.cost < b.cost
})
```

Two properties are worth remembering:

- `less` must be a consistent strict ordering: never `less(a, a)`, and never
  both `less(a, b)` and `less(b, a)`.
- Elements for which neither `less(a, b)` nor `less(b, a)` holds are ties, and
  a heap returns ties in **no particular order**. If the order of ties
  matters, break them inside `less`, as the next section does.

## 1. 📬 A stable priority queue

*Full program: [examples/priorityqueue](examples/priorityqueue/main.go)*

The most common queue: a value plus a priority, highest priority first, and
first-in, first-out among values of the same priority. The heap stores a small
entry that carries the value, its priority, and a sequence number that makes
every entry unique:

```go
type PriorityQueue[T any] struct {
    heap *dheap.DHeap[entry[T]] // the storage
    seq  uint64                 // sequence number of the next entry
}

type entry[T any] struct {
    value    T
    priority int
    seq      uint64 // insertion order, used to break ties
}

func NewPriorityQueue[T any]() *PriorityQueue[T] {
    less := func(a, b entry[T]) bool {
        if a.priority != b.priority {
            return a.priority > b.priority // higher priority first
        }
        return a.seq < b.seq // then older entries first
    }
    return &PriorityQueue[T]{heap: dheap.New(4, less)}
}

func (q *PriorityQueue[T]) Enqueue(value T, priority int) {
    q.heap.Push(entry[T]{value: value, priority: priority, seq: q.seq})
    q.seq++
}

func (q *PriorityQueue[T]) Dequeue() (T, bool) {
    e, ok := q.heap.Pop()
    return e.value, ok
}
```

The queue is a thin wrapper: it owns the policy (what a priority is, how ties
are resolved, what the method names are) and delegates the storage.

```console
$ go run ./docs/examples/priorityqueue
6 tasks queued, next: "production is down"

production is down
database is full
answer email
review pull request
update dependencies
water the plants
```

Note how the two urgent tasks and the three normal ones come out in the order
they went in.

> 💡 **Values or pointers?** Storing entries by value, as above, keeps them in
> one contiguous block of memory, which is what makes the heap fast. Prefer it
> for entries up to a few machine words. Store pointers when entries are
> large, or when you need handles to them (next section).

## 2. 🎯 Changing and removing queued items

*Full program: [examples/indexedqueue](examples/indexedqueue/main.go)*

Timers get cancelled, jobs get escalated, orders get amended. All of these
need to reach an element in the *middle* of the heap, and `DHeap` has the
methods for it:

| Method | Use it to… |
| --- | --- |
| `Fix(i)` | repair the heap after you changed the priority of element `i` in place |
| `Update(i, v)` | replace element `i` with a new value `v` |
| `PopAt(i)` | remove element `i` |

They take the element's **index in the heap's slice**, and that index changes
every time the heap reorders itself. Searching for the element is not an
option: it would cost O(n) per change, and `DHeap` does not expose its slice.
Instead, let the heap tell you where each element is, and record it in one of
two places:

| Strategy | Where the index is stored | Cost per move | When |
| --- | --- | --- | --- |
| 📌 Inside the element | A field of the element, which must be a pointer | One store | The default choice; gives you handles |
| 🗺️ In a side table | A slice or map keyed by an ID of the element | A slice store or map write | Elements are values or IDs, or you cannot change their type |

`WithIndexTracking` makes the heap call your function whenever an element
comes to rest at a new index, and with `-1` when it leaves the heap. Storing
the index inside the element turns the element into a **handle**:

```go
type Item[T any] struct {
    Value    T
    priority int
    index    int // position in the heap, -1 when not queued; written by the heap
}

func NewQueue[T any]() *Queue[T] {
    return &Queue[T]{
        heap: dheap.New(4,
            func(a, b *Item[T]) bool { return a.priority < b.priority },
            dheap.WithIndexTracking(func(it *Item[T], i int) { it.index = i }),
        ),
    }
}

// Push returns the handle.
func (q *Queue[T]) Push(value T, priority int) *Item[T] {
    it := &Item[T]{Value: value, priority: priority, index: -1}
    q.heap.Push(it)
    return it
}

// Update changes the priority of a queued item in O(log n).
func (q *Queue[T]) Update(it *Item[T], priority int) bool {
    if it.index < 0 {
        return false // not queued any more
    }
    it.priority = priority
    return q.heap.Fix(it.index)
}

// Remove takes an item out of the queue in O(log n), wherever it is.
func (q *Queue[T]) Remove(it *Item[T]) bool {
    if it.index < 0 {
        return false
    }
    _, ok := q.heap.PopAt(it.index)
    return ok
}
```

Because the heap reports `-1` on removal, a handle always knows whether its
item is still queued. That makes double cancellation harmless:

```console
$ go run ./docs/examples/indexedqueue
cancel timeout:  true
cancel it again: false

t= 50ms  retry request
t=100ms  flush metrics
t=275ms  collect garbage
t=300ms  refresh token

retry still queued: false
```

> ⚠️ **Always go through `Fix` or `Update`.** Changing a field that `less`
> reads without telling the heap silently breaks the ordering. `IsValid` can
> detect that in tests.

> ⚠️ **Keep the callback trivial.** It runs for every element the heap moves.
> It must not call back into the heap.

### The alternative: lazy deletion

If cancellations are rare, you can skip index tracking altogether: mark the
item as cancelled, leave it in the heap, and discard cancelled items when they
surface in `Pop`. It is simpler and keeps the hot path free of callbacks, at
the price of a heap that holds dead entries until they reach the root.

## 3. 🧭 Decrease-key: Dijkstra's algorithm

*Full program: [examples/dijkstra](examples/dijkstra/main.go)*

Graph searches are where d-ary heaps earn their keep. Dijkstra's algorithm
lowers a priority up to once per **edge** (a sift-up, which gets cheaper as
the arity grows) but pops only once per **vertex** (a sift-down, which gets
more expensive). On graphs with many more edges than vertices, a wider heap
shifts the cost to where there is less work.

The example also shows the *side table* strategy. The heap stores bare vertex
numbers; priorities and positions live in two slices next to it:

```go
dist := make([]int, n) // dist[v]: best known distance to v
pos := make([]int, n)  // pos[v]: index of v in the heap, or -1

queue := dheap.New(4,
    func(a, b int) bool { return dist[a] < dist[b] },       // priority lives outside
    dheap.WithIndexTracking(func(v, i int) { pos[v] = i }), // so does the position
)
```

Relaxing an edge then becomes a true decrease-key, with no duplicate entries
in the queue:

```go
for _, e := range g[v] {
    d := dist[v] + e.weight
    if d >= dist[e.to] {
        continue
    }

    first := dist[e.to] == unreachable
    dist[e.to] = d
    if first {
        queue.Push(e.to)
    } else {
        queue.Fix(pos[e.to]) // decrease-key
    }
}
```

```console
$ go run ./docs/examples/dijkstra
Lisbon   0h  [Lisbon]
Madrid  10h  [Lisbon Madrid]
Paris   23h  [Lisbon Madrid Paris]
Rome    32h  [Lisbon Madrid Rome]
Vienna  39h  [Lisbon Madrid Paris Berlin Vienna]
Berlin  31h  [Lisbon Madrid Paris Berlin]
Oslo    unreachable
```

> 💡 The order matters: update `dist` **first**, then call `Fix`. The heap
> compares elements by calling `less`, which reads `dist`.

## 4. 🏆 Top-k with a bounded heap

*Full program: [examples/topk](examples/topk/main.go)*

To keep the *k* largest values of a stream, store them in a **min**-heap.
That sounds backwards until you look at the root: it is the smallest of the
values kept, the one to throw out when something better arrives.

```go
h := dheap.New(4, less) // smallest kept value at the root
for v := range seq {
    if h.Len() < k {
        h.Push(v)
        continue
    }
    h.PushPop(v) // insert v, evict the smallest
}
```

`PushPop` is what makes this fast. If `v` is not better than the root, which
is the case for almost every value once the heap has warmed up, it returns
after **one comparison** without touching the heap. Otherwise it replaces the
root and repairs the heap in a single pass. The heap never grows beyond *k*
elements, so memory is O(k) however long the stream is.

```console
$ go run ./docs/examples/topk
17 distinct words, the 5 most frequent:
1. the   11
2. dog   4
3. fox   4
4. and   3
5. quick 3

3 largest numbers: [99 91 76]
```

## 5. 🔀 K-way merge

*Full program: [examples/kwaymerge](examples/kwaymerge/main.go)*

Merging *k* sorted inputs needs, at every step, the smallest of the *k*
values at the front of each input. Put one cursor per input into a heap and
that value is always at the root:

```go
h := dheap.FromSlice(cursors, 4, less) // O(k) to build

for {
    c, ok := h.Peek()
    if !ok {
        return merged
    }
    merged = append(merged, value(c))

    c.next++
    if c.next < len(inputs[c.input]) {
        h.Replace(c) // advance the cursor: one sift-down
    } else {
        h.Pop() // input exhausted
    }
}
```

`Replace` swaps the root for the advanced cursor and sifts it down once. A
`Pop` followed by a `Push` would traverse the tree twice. Breaking ties by
input number inside `less` keeps the merge stable.

```console
$ go run ./docs/examples/kwaymerge
shard 0: [1 4 9 16 25]
shard 1: [2 3 5 7 11 13]
shard 2: []
shard 3: [4 8 12 16 20]
shard 4: [6]
merged:  [1 2 3 4 4 5 6 7 8 9 11 12 13 16 16 20 25]
```

## 6. 📈 Two heaps of numbers: a running median

*Full program: [examples/median](examples/median/main.go)*

When the queue holds bare numbers, there is nothing to wrap and no `less` to
write: `Ordered` is the storage, and it is faster than `DHeap` for this kind
of element.

The classic use of *two* heaps is a median that keeps up with a stream. Split
the values seen so far into a smaller half and a larger half. Keep the smaller
half in a **max**-heap and the larger half in a **min**-heap, and the median
is always at one of the two roots:

```text
 smaller half (max-heap)      larger half (min-heap)
       1  3  [5]          |        [8]  9  12
              ▲ root                ▲ root
```

```go
type RunningMedian struct {
    lower *dheap.Ordered[float64] // max-heap: the smaller half
    upper *dheap.Ordered[float64] // min-heap: the larger half
}

func NewRunningMedian() *RunningMedian {
    return &RunningMedian{
        lower: dheap.NewMax[float64](4),
        upper: dheap.NewMin[float64](4),
    }
}

func (m *RunningMedian) Add(v float64) {
    if m.lower.Len() == m.upper.Len() {
        // The lower half must grow. Pass v through the upper half first, so
        // that the value moving down is the smallest of the large values.
        m.lower.Push(m.upper.PushPop(v))

        return
    }

    m.upper.Push(m.lower.PushPop(v))
}
```

`PushPop` does the balancing: it inserts the new value into one half and
hands back that half's root, which then moves to the other half. Adding a
value costs O(log n) and reading the median O(1).

```console
$ go run ./docs/examples/median
latency  median
   12ms   12.0ms
    5ms    8.5ms
   48ms   12.0ms
    9ms   10.5ms
    3ms    9.0ms
   27ms   10.5ms
    8ms    9.0ms
  150ms   10.5ms
   11ms   11.0ms
    6ms   10.0ms

10 samples
```

Note how the 150 ms outlier barely moves the median, which is the reason to
track a median instead of an average.

## 🧰 Checklist for your own queue

- **Pick the heap type first.** `Ordered` for bare numbers and strings,
  `DHeap` for everything else.
- **Wrap, don't expose.** Keep the heap in an unexported field and give your
  queue the methods and names that fit your domain.
- **Put the whole policy in `less`**, including tie-breaking.
- **Pick the right single-pass operation:** `Replace` when you pop and push
  together, `PushPop` when you push and maybe pop, `FromSlice` when you
  already have the elements.
- **Use an arity of 4** unless measurements say otherwise, and keep it a power
  of two. See [design.md](design.md#-choosing-the-arity).
- **Add your own locking** if several goroutines share the queue; the heaps
  have none.
- **Reuse the queue.** `Clear` keeps the memory, so a refilled queue does not
  allocate.
