package hashset

// Set is a data structure that can store elements and has no repeated values.
// A set is backed by a hash table(Go's map), and is not safe for concurrent use
// by multiple goroutines without additional locking or coordination.
type Set[T comparable] map[T]struct{}

// Make builds a hash set.
func Make[T comparable]() Set[T] {
	return make(Set[T])
}

// Insert adds items to the set.
func (s Set[T]) Insert(items ...T) {
	for _, item := range items {
		s[item] = struct{}{}
	}
}

// List returns the contents as a slice.
func (s Set[T]) List() []T {
	res := make([]T, 0, len(s))

	for key := range s {
		res = append(res, key)
	}

	return res
}
