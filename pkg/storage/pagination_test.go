package storage

import (
	"slices"
	"testing"
)

func TestPageClampsSelection(t *testing.T) {
	tests := []struct {
		name     string
		page     int64
		pageSize int64
		want     PageRequest
	}{
		{
			name:     "valid",
			page:     2,
			pageSize: 25,
			want:     PageRequest{Page: 2, PageSize: 25, Limit: 25, Offset: 25},
		},
		{
			name:     "zero page starts at one",
			page:     0,
			pageSize: 25,
			want:     PageRequest{Page: 1, PageSize: 25, Limit: 25, Offset: 0},
		},
		{
			name:     "negative page starts at one",
			page:     -3,
			pageSize: 25,
			want:     PageRequest{Page: 1, PageSize: 25, Limit: 25, Offset: 0},
		},
		{
			name:     "zero size defaults",
			page:     3,
			pageSize: 0,
			want:     PageRequest{Page: 3, PageSize: 10, Limit: 10, Offset: 20},
		},
		{
			name:     "size caps at one hundred",
			page:     2,
			pageSize: 500,
			want:     PageRequest{Page: 2, PageSize: 100, Limit: 100, Offset: 100},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Page(test.page, test.pageSize); got != test.want {
				t.Fatalf("Page(%d, %d) = %+v, want %+v", test.page, test.pageSize, got, test.want)
			}
		})
	}
}

func TestNewPaginated(t *testing.T) {
	items := []string{"a", "b"}

	got := NewPaginated(items, 12, Page(2, 5))

	if !slices.Equal(got.Items, items) {
		t.Fatalf("Items = %#v, want %#v", got.Items, items)
	}
	if got.TotalCount != 12 || got.Page != 2 || got.PageSize != 5 || got.TotalPages != 3 {
		t.Fatalf("NewPaginated = %+v", got)
	}
}

func TestNewPaginatedNormalisesZeroPageSize(t *testing.T) {
	got := NewPaginated([]string{}, 0, PageRequest{})

	if got.Page != 1 || got.PageSize != 10 || got.TotalPages != 0 {
		t.Fatalf("NewPaginated = %+v", got)
	}
}
