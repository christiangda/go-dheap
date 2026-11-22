# 📚 go-dheap documentation

| Document | What it covers |
| --- | --- |
| 🚦 [priority-queue.md](priority-queue.md) | How to build priority queues that use the heaps as their storage, pattern by pattern |
| ⚙️ [design.md](design.md) | How the heap works inside: the slice layout, why it is fast, how to choose the arity, benchmarks |
| 📖 [API reference][pkg] | Every exported name, with runnable examples |

## 🧪 Runnable examples

Each directory under [examples/](examples/) is a small, complete program.

| Example | Shows | Run |
| --- | --- | --- |
| [priorityqueue](examples/priorityqueue/main.go) | A generic queue with priorities and first-in, first-out tie-breaking | `go run ./docs/examples/priorityqueue` |
| [indexedqueue](examples/indexedqueue/main.go) | Handles: re-prioritize and cancel queued items with `WithIndexTracking` | `go run ./docs/examples/indexedqueue` |
| [dijkstra](examples/dijkstra/main.go) | Shortest paths with a true decrease-key and a side table of positions | `go run ./docs/examples/dijkstra` |
| [topk](examples/topk/main.go) | The best *k* elements of a stream in O(k) memory with `PushPop` | `go run ./docs/examples/topk` |
| [kwaymerge](examples/kwaymerge/main.go) | Merging sorted inputs with a heap of cursors and `Replace` | `go run ./docs/examples/kwaymerge` |
| [median](examples/median/main.go) | A running median with two `Ordered` heaps, a max-heap and a min-heap | `go run ./docs/examples/median` |

The API reference has shorter examples for every method. They live in
[example_test.go](../example_test.go) and are verified by `go test`.

[pkg]: https://pkg.go.dev/github.com/christiangda/go-dheap
