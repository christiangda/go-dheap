# ⚙️ Design and performance

How `DHeap` stores its elements, why that is fast, and how to pick the arity
for your workload.

- [The layout: a tree without pointers](#-the-layout-a-tree-without-pointers)
- [Why a slice?](#-why-a-slice)
- [How Push and Pop work](#-how-push-and-pop-work)
- [Decisions on the hot path](#-decisions-on-the-hot-path)
- [Choosing the arity](#-choosing-the-arity)
- [Compared with container/heap](#-compared-with-containerheap)
- [Running the benchmarks](#-running-the-benchmarks)

## 🌳 The layout: a tree without pointers

A d-ary heap is a tree in which every node has up to *d* children and no child
orders before its parent. `DHeap` never builds that tree. It stores the
elements in a single slice, level by level, and *computes* where the relatives
of an element are:

```text
A heap with arity d = 3 and 13 elements

                      [0]
          ┌────────────┼────────────┐
         [1]          [2]          [3]
      ┌───┼───┐    ┌───┼───┐    ┌───┼───┐
     [4] [5] [6]  [7] [8] [9] [10][11][12]

The same heap in memory

   index:   0 │ 1   2   3 │ 4   5   6 │ 7   8   9 │ 10  11  12
            ▲   └───┬───┘   └───┬───┘   └───┬───┘   └────┬────┘
          root   children    children    children     children
                  of 0         of 1        of 2          of 3
```

For the element at index `i`:

| Relative | Index |
| --- | --- |
| Parent | `(i - 1) / d` |
| First child | `i*d + 1` |
| Last child | `i*d + d` |

That is the whole data structure: one slice, an arity and a comparison
function.

## 🧠 Why a slice?

**It saves memory.** The heap stores the elements and nothing else. There are
no node objects, no child or parent pointers and no allocation per element.
The only overhead is the spare capacity that `append` keeps for growth.

For comparison, a pointer-based 4-ary tree of `int` needs four child pointers
next to every 8-byte value. That is 40 bytes per node, which Go's allocator
rounds up to 48: six times the size of the element, plus one allocation per
element for the garbage collector to track.

**It is kind to the CPU cache.** Memory is slow and caches are small, so what
matters is how many *different* cache lines an operation touches.

- The children of a node are **adjacent**. With 8-byte elements and an arity
  of 4, all four children span 32 bytes: one cache line, or two when they
  straddle a boundary. Choosing the best child, the most expensive step of
  `Pop`, therefore costs one or two cache misses instead of four.
- The top levels of the tree live at the **start of the slice**. Every
  operation passes through them, so they stay in the L1 cache.
- A contiguous block is what the hardware prefetcher understands, which makes
  `FromSlice` and `Clear` close to a linear scan.

**It is kind to the garbage collector.** A slice of pointer-free elements such
as `int` or a small struct is a single object that the collector never has to
scan. A tree of nodes is one object per element.

The costs of this layout are worth knowing as well:

- Elements are **moved by value**. Large elements are expensive to shift
  around, so store pointers or indices to them instead. The measurements
  below show the effect.
- Parent and child are **far apart** in a large heap. Walking down the tree
  jumps through memory, one potential cache miss per level. This is precisely
  why a larger arity helps: fewer levels, fewer jumps.

## 🔁 How Push and Pop work

**Push** appends the new element at the end of the slice, making it the last
leaf, and then *sifts it up*: while the element orders before its parent, the
parent moves down one level. This takes at most one comparison per level:
`log_d n` in total.

**Pop** takes the root, moves the last leaf into its place, and *sifts it
down*: on every level it finds the best of up to *d* children and, if that
child orders before the element, moves the child up. This takes up to *d*
comparisons per level: `d · log_d n` in total.

That asymmetry is the trade-off behind the arity. A larger *d* makes the tree
shallower, which is pure gain for `Push`, while `Pop` trades fewer levels for
more comparisons on each of them.

| Arity | Levels for 1,000,000 elements | Comparisons per Push (worst case) | Comparisons per Pop (worst case) |
| --- | --- | --- | --- |
| 2 | 20 | 20 | 40 |
| 4 | 10 | 10 | 40 |
| 8 | 7 | 7 | 56 |
| 16 | 5 | 5 | 80 |

## 🔧 Decisions on the hot path

**Hold and shift instead of swapping.** The textbook loop swaps an element
with its parent or child on every level, which is two writes per level. Both
sift loops keep the moving element aside, shift the elements it passes by one
level, and store it once at the end: one write per level.

**A shift instead of a division.** Finding the parent needs a division by the
arity, and integer division is among the slowest instructions a CPU has. When
the arity is a power of two, the heap uses a shift instead. On the test
machine this alone halves the cost of `Push`; compare the columns for d=3 and
d=4 in the tables below.

**No allocations after warm-up.** Every operation works in place. Once the
slice has grown to its working size, nothing allocates, and `Clear` keeps the
memory for reuse. A test (`TestSteadyStateDoesNotAllocate`) guards this.

**Single-pass compound operations.** `Replace` and `PushPop` repair the heap
with one sift-down where a `Pop` followed by a `Push` would need a sift-down
and a sift-up. `FromSlice` builds a heap in O(n) with Floyd's algorithm
instead of O(n log n) pushes.

**No reference is left behind.** Slots vacated by `Pop`, `PopAt` and `Clear`
are zeroed, so a heap of pointers does not keep removed elements alive.

**Ties never move.** Both loops stop on the first comparison that is not
strictly "orders before", so equal elements do not trade places.

**Tracking costs next to nothing when unused.** Without a callback, index
tracking is a nil check that the branch predictor gets right every time.

One thing was tried and rejected: the "bottom-up" variant of `Pop`, which
sinks the hole to a leaf without comparing against the moving element and
then sifts up. It made draining a heap about 10% faster but the steady-state
pop-and-push workload 20–40% slower, so it is not used.

### The remaining cost: the comparison callback

`less` is a function value, so every comparison is an indirect call that the
compiler cannot inline. This is the price of supporting any element type and
any ordering with one implementation, and it is the largest remaining cost of
`Pop`. Keep `less` small, and avoid captures of large structures by value.

## 🧮 Choosing the arity

Measured with `int` elements and `less` = `a < b` on an Intel Core i7-7700HQ
(6 MB L3 cache), Go 1.27, linux/amd64. Nanoseconds per element; lower is
better; the best value of each row is in bold. Your numbers will differ, the
pattern should not.

**Push**: pushing *n* random values.

| n | d=2 | d=3 | d=4 | d=8 | d=16 |
| --- | --- | --- | --- | --- | --- |
| 1,000 | 19.4 | 32.2 | 14.2 | 10.4 | **9.9** |
| 100,000 | 24.6 | 39.0 | 14.2 | 12.4 | **10.0** |
| 1,000,000 | 26.3 | 41.8 | 16.3 | 13.1 | **11.8** |

**Pop**: draining a heap of *n* random values.

| n | d=2 | d=3 | d=4 | d=8 | d=16 |
| --- | --- | --- | --- | --- | --- |
| 1,000 | **100.2** | 109.0 | 115.0 | 128.8 | 186.8 |
| 100,000 | **165.3** | 175.3 | 181.0 | 232.7 | 368.7 |
| 1,000,000 | 387.9 | 668.6 | **245.2** | 326.9 | 387.0 |

**Pop + Push**: steady state of a queue holding *n* elements.

| n | d=2 | d=3 | d=4 | d=8 | d=16 |
| --- | --- | --- | --- | --- | --- |
| 1,000 | **101.8** | 125.6 | 113.1 | 140.4 | 216.1 |
| 100,000 | **158.9** | 164.1 | 162.8 | 203.7 | 304.1 |
| 1,000,000 | **166.8** | 191.2 | 196.2 | 241.9 | 346.9 |

**FromSlice**: building a heap from *n* random values.

| n | d=2 | d=3 | d=4 | d=8 | d=16 |
| --- | --- | --- | --- | --- | --- |
| 1,000 | 13.2 | 9.6 | 11.7 | 6.6 | **6.1** |
| 100,000 | 17.3 | 13.8 | 12.2 | 11.9 | **7.6** |
| 1,000,000 | 25.4 | 15.1 | 12.1 | 9.4 | **7.3** |

What to take from this:

- 🚀 **`Push` and `FromSlice` get faster with a larger arity.** Going from 2
  to 4 makes `Push` 30–40% faster; 8 and 16 add a little more.
- ⚖️ **`Pop` gets slower with a larger arity while the heap fits in the
  cache**: by 10–15% from 2 to 4, by 30–40% at 8, and it doubles at 16.
- 🧊 **Draining a heap that outgrew the cache favours a shallower tree.**
  Emptying one million elements is about 35% faster with arity 4 than with 2.
  A steady-state queue of that size is different: its activity stays near the
  top of the tree, which is cached, and arity 2 keeps its small lead.
- ➗ **Avoid arities that are not powers of two.** Arity 3 pays for a division
  on every level of every `Push`, which makes it slower there than both of
  its neighbours.

Recommendations:

| Workload | Arity |
| --- | --- |
| No idea yet, mixed operations | **4** |
| Many more pushes than pops; many `Fix` calls that raise priorities (Dijkstra, A\*, Prim) | **8** or **16** |
| One pop per push, or mostly pops | **2** or **4** |
| Filling and then draining a very large heap | **4** or **8** |

Then measure with your own element type and comparison. The benchmarks in
[bench_test.go](../bench_test.go) are a good starting point.

### The effect of the element type

The steady-state workload at arity 4 with 100,000 elements, for different
element types:

| Element | Size | ns/op | Why |
| --- | --- | --- | --- |
| `int` | 8 bytes | 175 | The baseline |
| struct of two ints | 16 bytes | 195 | Twice the bytes to move per level |
| struct with a string and a pointer | 32 bytes | 231 | Four times the bytes; the collector must scan the slice |
| pointer to a struct | 8 bytes | 246 | Cheap to move, but every comparison follows a pointer to memory that is not adjacent |
| pointer to a struct, with index tracking | 8 bytes | 247 | The callback adds no measurable cost on top |

Small values in the slice beat pointers to values elsewhere, even when the
values are a few words large.

## 🆚 Compared with `container/heap`

The standard library's heap is binary and works through an interface. The same
machine and input, 100,000 `int` elements:

| Operation | `container/heap` | `DHeap` d=2 | `DHeap` d=4 | `DHeap` d=8 |
| --- | --- | --- | --- | --- |
| Push, ns per element | 44.1 | 24.6 | 14.2 | **12.4** |
| Pop, ns per element | 207.5 | **165.3** | 181.0 | 232.7 |
| Pop + Push, ns per pair | 184.8 | **158.9** | 162.8 | 203.7 |
| Allocations per Push or Pop of an `int` | 1 | **0** | **0** | **0** |

`container/heap` passes elements as `any`, which allocates for most values
that are not pointers. `DHeap` is generic, so elements are never boxed.

## 🏃 Running the benchmarks

```bash
# Everything, with allocation statistics. Takes a few minutes.
go test -run '^$' -bench . -benchmem

# Every benchmark at arity 4.
go test -run '^$' -bench '/d=4' -benchmem

# Push and Pop at one million elements.
go test -run '^$' -bench '^Benchmark(Push|Pop)$/d=.*/n=1000000' -benchmem

# Comparing two versions of the code.
go test -run '^$' -bench . -count 10 > old.txt   # on the old version
go test -run '^$' -bench . -count 10 > new.txt   # on the new version
go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt
```

| Benchmark | Measures |
| --- | --- |
| `Push`, `Pop`, `FromSlice`, `PopPush` | The core operations, for every arity and for heaps of 1,000, 100,000 and 1,000,000 elements |
| `PushSorted` | The worst case of `Push`: every element rises to the root |
| `PushHeavy` | An asymmetric load of eight pushes per pop |
| `Replace`, `PushPop` | The single-pass compound operations |
| `Fix`, `Update` | Changing elements in the middle of the heap |
| `PopPushTracked` | The cost of index tracking |
| `PopPushTypes` | Element types of different sizes and pointer content |
| `ContainerHeap*` | The standard library as a reference |
