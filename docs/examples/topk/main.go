// Command topk finds the k largest elements of a stream using a heap of
// bounded size.
//
// The trick is to invert the order: to keep the k largest values, store them
// in a min-heap. Its root is then the smallest of the values kept, which is
// exactly the one to evict when a larger value arrives. The heap never holds
// more than k elements, so the memory use is O(k) and the running time
// O(n log k), no matter how long the stream is.
//
// DHeap.PushPop does the compare-and-evict step in one call, and it does no
// work at all for values that are too small to make the cut.
//
// Run it with:
//
//	go run ./docs/examples/topk
package main

import (
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/christiangda/go-dheap"
)

// wordCount is a word and the number of times it occurs.
type wordCount struct {
	word  string
	count int
}

// fewer reports whether a occurs less often than b. Ties are broken in
// reverse alphabetical order so that the result is deterministic.
func fewer(a, b wordCount) bool {
	if a.count != b.count {
		return a.count < b.count
	}

	return a.word > b.word
}

// topK returns the k elements of seq that order last according to less,
// largest first.
func topK[T any](seq iter.Seq[T], k int, less func(a, b T) bool) []T {
	if k <= 0 {
		return nil
	}

	// The smallest element kept is at the root.
	h := dheap.New(4, less)

	for v := range seq {
		if h.Len() < k {
			h.Push(v)

			continue
		}

		// Insert v and evict the smallest element. If v is the smallest,
		// PushPop hands it straight back and leaves the heap alone.
		h.PushPop(v)
	}

	// Popping yields the smallest first, so fill the result from the back.
	top := make([]T, h.Len())
	for i := len(top) - 1; i >= 0; i-- {
		top[i], _ = h.Pop()
	}

	return top
}

const text = `
the quick brown fox jumps over the lazy dog and the dog barks at the fox
while the quick cat watches the dog and the fox from over the wall and
the lazy cat sleeps while the brown dog jumps at the quick fox`

func main() {
	counts := map[string]int{}

	for word := range strings.FieldsSeq(text) {
		counts[word]++
	}

	// Adapt the map to a sequence of wordCount values.
	words := func(yield func(wordCount) bool) {
		for word, count := range counts {
			if !yield(wordCount{word: word, count: count}) {
				return
			}
		}
	}

	fmt.Printf("%d distinct words, the 5 most frequent:\n", len(counts))

	for i, wc := range topK(words, 5, fewer) {
		fmt.Printf("%d. %-5s %d\n", i+1, wc.word, wc.count)
	}

	// The same function finds the largest numbers of a slice.
	numbers := []int{42, 7, 99, 13, 58, 76, 3, 91, 27, 64}
	largest := topK(slices.Values(numbers), 3, func(a, b int) bool { return a < b })
	fmt.Println("\n3 largest numbers:", largest)
}
