# 🌳 go-dheap

[![main branch](https://github.com/christiangda/go-dheap/actions/workflows/main.yml/badge.svg)](https://github.com/christiangda/go-dheap/actions/workflows/main.yml)
[![CodeQL](https://github.com/christiangda/go-dheap/actions/workflows/codeql.yml/badge.svg)](https://github.com/christiangda/go-dheap/actions/workflows/codeql.yml)
[![Release](https://github.com/christiangda/go-dheap/actions/workflows/release.yml/badge.svg)](https://github.com/christiangda/go-dheap/actions/workflows/release.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/christiangda/go-dheap.svg)](https://pkg.go.dev/github.com/christiangda/go-dheap)
[![Go Report Card](https://goreportcard.com/badge/github.com/christiangda/go-dheap)](https://goreportcard.com/report/github.com/christiangda/go-dheap)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/christiangda/go-dheap?style=plastic)
[![release](https://img.shields.io/github/release/christiangda/go-dheap/all.svg)](https://github.com/christiangda/go-dheap/releases)
[![license](https://img.shields.io/github/license/christiangda/go-dheap.svg)](https://github.com/christiangda/go-dheap/blob/main/LICENSE)

A fast, generic **d-ary heap** for Go: the storage engine for priority queues,
schedulers, timers, top-k selection and graph algorithms.

A d-ary heap is a binary heap with a dial. Instead of two children, every node
has *d*. Turn the dial up and the tree gets shallower, so inserting gets
cheaper; turn it down and removing gets cheaper. You pick the trade-off that
fits your workload.

**📦 Zero dependencies** — only the Go standard library. See [go.mod](go.mod).

## ✨ Features

- 🧬 **Generic** — `DHeap[T]` holds any type, with no interfaces to implement and no boxing
- 🎛️ **Any arity** — binary, 4-ary, 8-ary… chosen per heap at run time
- 🔀 **Any order** — min-heap, max-heap or multi-key ordering through one `less` function
- ⚡ **Built for speed** — a flat slice, one write per level, shifts instead of divisions, zero allocations in steady state
- 🧠 **Cache and memory friendly** — no nodes, no pointers; siblings share a cache line
- 🎯 **Indexed operations** — `Fix`, `Update` and `PopAt` with built-in index tracking, for decrease-key and cancellation
- 🚄 **Single-pass shortcuts** — `Replace`, `PushPop` and O(n) `FromSlice`
- ✅ **Thoroughly tested** — 100% statement coverage, randomized tests against a reference model, runnable examples
- 📦 **Zero dependencies** — standard library only

## 📑 Table of contents

- [Installation](#-installation)
- [Quick start](#-quick-start)
- [Why a slice?](#-why-a-slice)
- [Choosing the arity](#-choosing-the-arity)
- [API at a glance](#-api-at-a-glance)
- [Priority queues](#-priority-queues)
- [Performance](#-performance)
- [Testing](#-testing)
- [Versioning and releases](#-versioning-and-releases)
- [Documentation](#-documentation)
- [Contributing](#-contributing)
- [License](#-license)

## 📥 Installation

```bash
go get github.com/christiangda/go-dheap@latest
```

Requires Go 1.24 or later.

## 🚀 Quick start

```go
package main

import (
    "fmt"

    "github.com/christiangda/go-dheap"
)

func main() {
    // A 4-ary min-heap: the smallest value comes out first.
    h := dheap.New(4, func(a, b int) bool { return a < b })

    for _, v := range []int{5, 2, 8, 1, 9, 3} {
        h.Push(v)
    }

    top, _ := h.Peek()
    fmt.Println("smallest:", top) // smallest: 1

    for !h.IsEmpty() {
        v, _ := h.Pop()
        fmt.Print(v, " ") // 1 2 3 5 8 9
    }
}
```

The `less` function is the whole ordering policy. Flip it for a max-heap, or
compare several fields for anything more elaborate:

```go
type job struct {
    name     string
    priority int
}

// Highest priority first, then alphabetically.
jobs := dheap.New(4, func(a, b job) bool {
    if a.priority != b.priority {
        return a.priority > b.priority
    }
    return a.name < b.name
})
```

Already have the data? Build the heap in linear time:

```go
h := dheap.FromSlice(data, 4, func(a, b int) bool { return a < b })
```

`Peek` and `Pop` return `(value, ok)`, so an empty heap is never confused with
a zero value.

## 🧠 Why a slice?

`DHeap` never builds a tree of nodes. It keeps the elements in **one slice**,
level by level, and computes where the parent and the children of an element
are from its index:

```text
                      [0]
          ┌────────────┼────────────┐                 arity d = 3
         [1]          [2]          [3]
      ┌───┼───┐    ┌───┼───┐    ┌───┼───┐             parent(i)     = (i - 1) / d
     [4] [5] [6]  [7] [8] [9] [10][11][12]            firstChild(i) = i*d + 1

 slice:  0 │ 1  2  3 │ 4  5  6 │ 7  8  9 │ 10 11 12
```

This buys two things:

- 💾 **Space.** The heap stores your elements and nothing else: no node
  objects, no child pointers, no allocation per element. A pointer-based
  4-ary tree of `int` would spend 48 bytes per element; the slice spends 8.
- 🏎️ **Speed through the CPU cache.** The children of a node sit next to each
  other in memory, so picking the best child, the expensive step of `Pop`,
  touches one or two cache lines instead of chasing *d* pointers. The top of
  the tree lives at the start of the slice and stays in the L1 cache. And a
  slice of pointer-free elements is a single object the garbage collector
  never has to scan.

The flip side: elements are moved by value, so keep them small, or store
pointers or indices to large ones. More in [docs/design.md](docs/design.md).

## 🧮 Choosing the arity

| Operation | Comparisons | A larger arity makes it… |
| --- | --- | --- |
| `Push` (sift up) | `log_d n` | 🚀 faster: the tree is shallower |
| `Pop` (sift down) | `d · log_d n` | 🐢 slower: more children to compare per level |

- 👉 **Start with 4.** It makes `Push` 30–40% faster than a binary heap and
  costs `Pop` 10–15%.
- 📈 **Use 8 or 16** when pushes or priority changes outnumber pops, as in
  Dijkstra's algorithm.
- 📉 **Use 2** when pops dominate.
- ➗ **Stick to powers of two.** The heap then finds parents with a shift
  instead of a division, which roughly halves the cost of `Push`.

Then measure with your own data: `go test -bench '/d=(2|4|8)'`. The numbers
behind this advice are in [docs/design.md](docs/design.md#-choosing-the-arity).

## 🧩 API at a glance

| | Function | What it does | Time |
| --- | --- | --- | --- |
| 🏗️ | `New(arity, less, opts...)` | Creates an empty heap | O(1) |
| 🏗️ | `FromSlice(data, arity, less, opts...)` | Builds a heap from a copy of `data` | O(n) |
| ➕ | `Push(v)` | Inserts an element | O(log_d n) |
| ➖ | `Pop()` | Removes and returns the top element | O(d log_d n) |
| 👀 | `Peek()` | Returns the top element without removing it | O(1) |
| 🔄 | `Replace(v)` | Pops the top and pushes `v` in one pass | O(d log_d n) |
| 🔄 | `PushPop(v)` | Pushes `v` and pops the top in one pass | O(d log_d n) |
| 🎯 | `Fix(i)` | Repairs the heap after element `i` changed | O(d log_d n) |
| 🎯 | `Update(i, v)` | Replaces element `i` with `v` | O(d log_d n) |
| 🎯 | `PopAt(i)` | Removes element `i` | O(d log_d n) |
| 📏 | `Len()`, `IsEmpty()`, `Arity()` | Size and configuration | O(1) |
| 🧹 | `Clear()` | Removes everything, keeps the memory | O(n) |
| 👯 | `Clone()` | Returns an independent copy | O(n) |
| 🩺 | `IsValid()` | Verifies the heap ordering, for tests | O(n) |
| 📌 | `WithIndexTracking(fn)` | Option: reports every index change to `fn` | — |

`New` and `FromSlice` panic if the arity is below 2 or `less` is nil. A
`DHeap` is not safe for concurrent use; add your own locking if goroutines
share one.

Full reference with runnable examples: [pkg.go.dev](https://pkg.go.dev/github.com/christiangda/go-dheap).

## 🚦 Priority queues

A heap is the storage of a priority queue; the queue adds the policy. The
guide **[Building priority queues on top of DHeap](docs/priority-queue.md)**
walks through five patterns, each with a complete program:

| Pattern | Example |
| --- | --- |
| 📬 Stable queue: highest priority first, FIFO among equals | [priorityqueue](docs/examples/priorityqueue/main.go) |
| 🎯 Handles: re-prioritize and cancel queued items | [indexedqueue](docs/examples/indexedqueue/main.go) |
| 🧭 Dijkstra's shortest paths with a true decrease-key | [dijkstra](docs/examples/dijkstra/main.go) |
| 🏆 Top-k of a stream in O(k) memory | [topk](docs/examples/topk/main.go) |
| 🔀 K-way merge of sorted inputs | [kwaymerge](docs/examples/kwaymerge/main.go) |

A taste of the second one. `WithIndexTracking` lets every element know where
it is, which turns it into a handle you can update or remove later:

```go
type task struct {
    name     string
    priority int
    index    int // maintained by the heap; -1 once removed
}

h := dheap.New(4,
    func(a, b *task) bool { return a.priority > b.priority },
    dheap.WithIndexTracking(func(t *task, i int) { t.index = i }),
)

docs := &task{name: "write docs", priority: 1}
h.Push(docs)
// ... push more tasks ...

docs.priority = 10 // it became urgent
h.Fix(docs.index)  // O(log n), wherever it is in the heap

h.PopAt(docs.index) // or cancel it altogether
```

## 📊 Performance

100,000 `int` elements on an Intel Core i7-7700HQ, Go 1.27, linux/amd64.
Lower is better.

| Operation | `container/heap` | `DHeap` d=2 | `DHeap` d=4 | `DHeap` d=8 |
| --- | --- | --- | --- | --- |
| Push, ns per element | 44.1 | 24.6 | 14.2 | **12.4** |
| Pop, ns per element | 207.5 | **165.3** | 181.0 | 232.7 |
| Pop + Push, ns per pair | 184.8 | **158.9** | 162.8 | 203.7 |
| Allocations per Push or Pop | 1 | **0** | **0** | **0** |

Numbers vary between machines; run the benchmarks on yours:

```bash
go test -run '^$' -bench . -benchmem          # everything (a few minutes)
go test -run '^$' -bench '/d=4' -benchmem     # only arity 4
```

The suite sweeps arities 2 to 16 and heap sizes from 1,000 to 1,000,000
elements, covers asymmetric workloads and different element types, and
includes `container/heap` as a reference. How the heap gets its speed, and
the complete results, are in [docs/design.md](docs/design.md).

## 🧪 Testing

```bash
go test -race -cover ./...
```

The tests check every operation for arities 2 to 64 at the sizes where the
shape of the tree changes (0, 1, 2, d-1, d, d+1, d², d²+1, …), replay long
random sequences of operations against a sorted-slice reference model,
validate the heap invariant with a checker that is independent of the
implementation, and assert that steady-state operations do not allocate.

## 🔖 Versioning and releases

The module follows [semantic versioning](https://semver.org/). Pushing a tag
runs the tests and publishes a GitHub release with generated notes:

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

## 📚 Documentation

- 📖 [API reference](https://pkg.go.dev/github.com/christiangda/go-dheap) — every exported name, with examples
- 🚦 [Priority queue guide](docs/priority-queue.md) — building queues on top of `DHeap`
- ⚙️ [Design and performance](docs/design.md) — the layout, the hot path, choosing the arity, benchmarks
- 🧪 [Example programs](docs/examples/) — five complete, runnable programs

## 🤝 Contributing

Issues and pull requests are welcome. Before opening a pull request, please
run:

```bash
gofmt -l .                                   # prints nothing when the code is formatted
go vet ./...
go test -race ./...
go test -run '^$' -bench . -benchtime 1x .   # benchmarks still run
```

For changes to the sift loops, include before and after results from
[benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat).

## 📄 License

[Apache License 2.0](LICENSE)
