package scheduler

// expiryOrder uses a min-heap. Unknown expiry sorts after known expiry; equal
// timestamps use batch ID for deterministic allocation.
func expiryOrder(indices []int, batches []Batch) []int {
	less := func(a, b int) bool {
		x, y := batches[a], batches[b]
		if x.ExpiresAt.IsZero() != y.ExpiresAt.IsZero() {
			return !x.ExpiresAt.IsZero()
		}
		if !x.ExpiresAt.Equal(y.ExpiresAt) {
			return x.ExpiresAt.Before(y.ExpiresAt)
		}
		return x.ID < y.ID
	}
	for i := len(indices)/2 - 1; i >= 0; i-- {
		siftDown(indices, i, len(indices), less)
	}
	ordered := make([]int, 0, len(indices))
	for len(indices) > 0 {
		ordered = append(ordered, indices[0])
		last := indices[len(indices)-1]
		indices = indices[:len(indices)-1]
		if len(indices) > 0 {
			indices[0] = last
			siftDown(indices, 0, len(indices), less)
		}
	}
	return ordered
}

func siftDown(heap []int, root, size int, less func(int, int) bool) {
	for {
		child := root*2 + 1
		if child >= size {
			return
		}
		if child+1 < size && less(heap[child+1], heap[child]) {
			child++
		}
		if !less(heap[child], heap[root]) {
			return
		}
		heap[root], heap[child] = heap[child], heap[root]
		root = child
	}
}
