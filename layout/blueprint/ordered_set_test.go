package blueprint

import (
	"testing"
)

func TestOrderedSet_Add(t *testing.T) {
	os := NewOrderedSet()

	// First add should return true
	if !os.Add("first") {
		t.Error("expected Add to return true for new item")
	}

	// Duplicate add should return false
	if os.Add("first") {
		t.Error("expected Add to return false for duplicate item")
	}

	// Second unique add should return true
	if !os.Add("second") {
		t.Error("expected Add to return true for new item")
	}

	if os.Len() != 2 {
		t.Errorf("expected length 2, got %d", os.Len())
	}
}

func TestOrderedSet_Contains(t *testing.T) {
	os := NewOrderedSet()
	os.Add("exists")

	if !os.Contains("exists") {
		t.Error("expected Contains to return true for existing item")
	}

	if os.Contains("missing") {
		t.Error("expected Contains to return false for non-existing item")
	}
}

func TestOrderedSet_Items(t *testing.T) {
	os := NewOrderedSet()
	os.Add("third")
	os.Add("first")
	os.Add("second")

	items := os.Items()

	// Should maintain insertion order
	expected := []string{"third", "first", "second"}
	if len(items) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(items))
	}

	for i, exp := range expected {
		if items[i] != exp {
			t.Errorf("at index %d: expected %s, got %s", i, exp, items[i])
		}
	}

	// Verify it's a copy
	items[0] = "modified"
	if os.Items()[0] == "modified" {
		t.Error("Items() should return a copy, not original slice")
	}
}

func TestOrderedSet_SortedItems(t *testing.T) {
	os := NewOrderedSet()
	os.Add("zebra")
	os.Add("apple")
	os.Add("banana")

	sorted := os.SortedItems()

	expected := []string{"apple", "banana", "zebra"}
	if len(sorted) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(sorted))
	}

	for i, exp := range expected {
		if sorted[i] != exp {
			t.Errorf("at index %d: expected %s, got %s", i, exp, sorted[i])
		}
	}
}

func TestOrderedSet_Merge(t *testing.T) {
	os1 := NewOrderedSet()
	os1.Add("a")
	os1.Add("b")

	os2 := NewOrderedSet()
	os2.Add("b") // duplicate
	os2.Add("c")
	os2.Add("d")

	os1.Merge(os2)

	// Should have unique items from both sets
	if os1.Len() != 4 {
		t.Errorf("expected length 4 after merge, got %d", os1.Len())
	}

	expected := []string{"a", "b", "c", "d"}
	items := os1.Items()

	for _, exp := range expected {
		found := false
		for _, item := range items {
			if item == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected item %s not found after merge", exp)
		}
	}
}

func TestOrderedSet_MergeNil(t *testing.T) {
	os := NewOrderedSet()
	os.Add("test")

	os.Merge(nil)

	if os.Len() != 1 {
		t.Errorf("expected length to remain 1 after merging nil, got %d", os.Len())
	}
}
