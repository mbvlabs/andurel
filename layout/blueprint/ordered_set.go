package blueprint

import (
	"sort"
)

// OrderedSet maintains a collection of unique strings in insertion order with
// optional canonical sorting.
type OrderedSet struct {
	items []string
	index map[string]int
}

// NewOrderedSet creates an empty ordered set.
func NewOrderedSet() *OrderedSet {
	return &OrderedSet{
		items: make([]string, 0),
		index: make(map[string]int),
	}
}

// Add inserts an item into the set if not already present. Returns true if the
// item was added, false if it already existed.
func (os *OrderedSet) Add(item string) bool {
	if _, exists := os.index[item]; exists {
		return false
	}

	os.index[item] = len(os.items)
	os.items = append(os.items, item)
	return true
}

// Contains reports whether the item is in the set.
func (os *OrderedSet) Contains(item string) bool {
	_, exists := os.index[item]
	return exists
}

// Items returns a copy of the current items in insertion order.
func (os *OrderedSet) Items() []string {
	result := make([]string, len(os.items))
	copy(result, os.items)
	return result
}

// Len returns the number of unique items in the set.
func (os *OrderedSet) Len() int {
	return len(os.items)
}

// SortedItems returns a sorted copy of the items.
func (os *OrderedSet) SortedItems() []string {
	items := os.Items()
	sort.Strings(items)
	return items
}

// Merge adds all items from another ordered set while maintaining uniqueness.
func (os *OrderedSet) Merge(other *OrderedSet) {
	if other == nil {
		return
	}

	for _, item := range other.items {
		os.Add(item)
	}
}
