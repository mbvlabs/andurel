package models

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"time"

	"github.com/example/relations/models/internal/db"
)

type Category struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Description string
	ParentId    uuid.UUID
	SortOrder   int32
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func FindCategory(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) (Category, error) {
	row, err := db.New().QueryCategoryByID(ctx, dbtx, id)
	if err != nil {
		return Category{}, err
	}

	return rowToCategory(row), nil
}

type CreateCategoryData struct {
	Name        string
	Slug        string
	Description string
	ParentId    uuid.UUID
	SortOrder   int32
	IsActive    bool
}

func CreateCategory(
	ctx context.Context,
	dbtx db.DBTX,
	data CreateCategoryData,
) (Category, error) {
	if err := validate.Struct(data); err != nil {
		return Category{}, errors.Join(ErrDomainValidation, err)
	}

	params := db.NewInsertCategoryParams(
		data.Name,
		data.Slug,
		pgtype.Text{String: data.Description, Valid: true},
		data.ParentId,
		data.SortOrder,
		data.IsActive,
	)
	row, err := db.New().InsertCategory(ctx, dbtx, params)
	if err != nil {
		return Category{}, err
	}

	return rowToCategory(row), nil
}

type UpdateCategoryData struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Description string
	ParentId    uuid.UUID
	SortOrder   int32
	IsActive    bool
	UpdatedAt   time.Time
}

func UpdateCategory(
	ctx context.Context,
	dbtx db.DBTX,
	data UpdateCategoryData,
) (Category, error) {
	if err := validate.Struct(data); err != nil {
		return Category{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := db.New().QueryCategoryByID(ctx, dbtx, data.ID)
	if err != nil {
		return Category{}, err
	}

	params := db.NewUpdateCategoryParams(
		data.ID,
		func() string {
			if true {
				return data.Name
			}
			return currentRow.Name
		}(),
		func() string {
			if true {
				return data.Slug
			}
			return currentRow.Slug
		}(),
		func() pgtype.Text {
			if true {
				return pgtype.Text{String: data.Description, Valid: true}
			}
			return currentRow.Description
		}(),
		func() uuid.UUID {
			if data.ParentId != uuid.Nil {
				return data.ParentId
			}
			return currentRow.ParentId
		}(),
		func() int32 {
			if true {
				return data.SortOrder
			}
			return currentRow.SortOrder
		}(),
		func() bool {
			if true {
				return data.IsActive
			}
			return currentRow.IsActive
		}(),
	)

	row, err := db.New().UpdateCategory(ctx, dbtx, params)
	if err != nil {
		return Category{}, err
	}

	return rowToCategory(row), nil
}

func DestroyCategory(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) error {
	return db.New().DeleteCategory(ctx, dbtx, id)
}

func AllCategorys(
	ctx context.Context,
	dbtx db.DBTX,
) ([]Category, error) {
	rows, err := db.New().QueryAllCategorys(ctx, dbtx)
	if err != nil {
		return nil, err
	}

	categorys := make([]Category, len(rows))
	for i, row := range rows {
		categorys[i] = rowToCategory(row)
	}

	return categorys, nil
}

type PaginatedCategorys struct {
	Categorys  []Category
	TotalCount int64
	Page       int64
	PageSize   int64
	TotalPages int64
}

func PaginateCategorys(
	ctx context.Context,
	dbtx db.DBTX,
	page int64,
	pageSize int64,
) (PaginatedCategorys, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	totalCount, err := db.New().CountCategorys(ctx, dbtx)
	if err != nil {
		return PaginatedCategorys{}, err
	}

	rows, err := db.New().QueryPaginatedCategorys(
		ctx,
		dbtx,
		db.NewQueryPaginatedCategorysParams(pageSize, offset),
	)
	if err != nil {
		return PaginatedCategorys{}, err
	}

	categorys := make([]Category, len(rows))
	for i, row := range rows {
		categorys[i] = rowToCategory(row)
	}

	totalPages := (totalCount + int64(pageSize) - 1) / int64(pageSize)

	return PaginatedCategorys{
		Categorys:  categorys,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func rowToCategory(row db.Categorie) Category {
	return Category{
		ID:          row.ID,
		Name:        row.Name,
		Slug:        row.Slug,
		Description: row.Description.String,
		ParentId:    row.ParentId,
		SortOrder:   row.SortOrder,
		IsActive:    row.IsActive,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}

// Categorie loads the Categorie that this Category belongs to
func (category Category) Categorie(
	ctx context.Context,
	dbtx db.DBTX,
) (*Categorie, error) {
	// TODO: Implement many-to-one relation loading
	// This would load the Categorie by category.parent_id
	return nil, fmt.Errorf("Categorie relation not implemented yet")
}
