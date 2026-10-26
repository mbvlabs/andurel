package storage

import "math"

const (
	defaultPageSize = 10
	maxPageSize     = 100
	// maxOffset is the largest page offset an int32 SQL parameter can carry.
	maxOffset = math.MaxInt32
)

// PageRequest is a clamped page selection ready for a query.
type PageRequest struct {
	Page     int64
	PageSize int64
	Limit    int32
	Offset   int32
}

// Page clamps page and pageSize to a valid selection. The page starts at 1,
// pageSize defaults to 10, pageSize is capped at 100, and the offset is kept
// inside int32 so a huge page cannot wrap into a negative SQL OFFSET.
func Page(page, pageSize int64) PageRequest {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	maxPage := maxOffset/pageSize + 1
	if page > maxPage {
		page = maxPage
	}

	return PageRequest{
		Page:     page,
		PageSize: pageSize,
		Limit:    int32(pageSize),
		Offset:   int32((page - 1) * pageSize),
	}
}

// Paginated is one page of items with the total count and page metadata.
type Paginated[T any] struct {
	Items      []T
	TotalCount int64
	Page       int64
	PageSize   int64
	TotalPages int64
}

// NewPaginated combines a page of items with its total count and request.
func NewPaginated[T any](items []T, total int64, req PageRequest) Paginated[T] {
	if req.PageSize < 1 {
		req = Page(req.Page, req.PageSize)
	}

	totalPages := (total + req.PageSize - 1) / req.PageSize

	return Paginated[T]{
		Items:      items,
		TotalCount: total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}
}
