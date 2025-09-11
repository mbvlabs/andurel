package models

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"time"

	"github.com/example/sqlite/models/internal/db"
)

type Product struct {
	ID           int64
	Uuid         string
	Name         string
	Description  string
	Price        float64
	Weight       float64
	Quantity     int64
	InStock      bool
	Tags         string
	Metadata     []byte
	CreatedDate  time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CategoryId   int64
	IsFeatured   bool
	DiscountRate float64
}

func FindProduct(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) (Product, error) {
	row, err := db.New().QueryProductByID(ctx, dbtx, id.String())
	if err != nil {
		return Product{}, err
	}

	return rowToProduct(row), nil
}

type CreateProductData struct {
	Uuid         string
	Name         string
	Description  string
	Price        float64
	Weight       float64
	Quantity     int64
	InStock      bool
	Tags         string
	Metadata     []byte
	CreatedDate  time.Time
	CategoryId   int64
	IsFeatured   bool
	DiscountRate float64
}

func CreateProduct(
	ctx context.Context,
	dbtx db.DBTX,
	data CreateProductData,
) (Product, error) {
	if err := validate.Struct(data); err != nil {
		return Product{}, errors.Join(ErrDomainValidation, err)
	}

	row, err := db.New().InsertProduct(ctx, dbtx, db.InsertProductParams{
		ID:           uuid.New().String(),
		Uuid:         data.Uuid,
		Name:         data.Name,
		Description:  sql.NullString{String: data.Description, Valid: true},
		Price:        sql.NullFloat64{Float64: data.Price, Valid: true},
		Weight:       sql.NullFloat64{Float64: data.Weight, Valid: true},
		Quantity:     data.Quantity,
		InStock:      sql.NullBool{Bool: data.InStock, Valid: true},
		Tags:         sql.NullString{String: data.Tags, Valid: true},
		Metadata:     data.Metadata,
		CreatedDate:  sql.NullTime{Time: data.CreatedDate, Valid: true},
		CategoryId:   sql.NullInt64{Int64: data.CategoryId, Valid: true},
		IsFeatured:   sql.NullBool{Bool: data.IsFeatured, Valid: true},
		DiscountRate: sql.NullFloat64{Float64: data.DiscountRate, Valid: true},
	})
	if err != nil {
		return Product{}, err
	}

	return rowToProduct(row), nil
}

type UpdateProductData struct {
	ID           uuid.UUID
	Uuid         string
	Name         string
	Description  string
	Price        float64
	Weight       float64
	Quantity     int64
	InStock      bool
	Tags         string
	Metadata     []byte
	CreatedDate  time.Time
	UpdatedAt    time.Time
	CategoryId   int64
	IsFeatured   bool
	DiscountRate float64
}

func UpdateProduct(
	ctx context.Context,
	dbtx db.DBTX,
	data UpdateProductData,
) (Product, error) {
	if err := validate.Struct(data); err != nil {
		return Product{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := db.New().QueryProductByID(ctx, dbtx, data.ID.String())
	if err != nil {
		return Product{}, err
	}

	params := db.UpdateProductParams{
		ID:           data.ID.String(),
		Uuid:         currentRow.Uuid,
		Name:         currentRow.Name,
		Description:  currentRow.Description,
		Price:        currentRow.Price,
		Weight:       currentRow.Weight,
		Quantity:     currentRow.Quantity,
		InStock:      currentRow.InStock,
		Tags:         currentRow.Tags,
		Metadata:     currentRow.Metadata,
		CreatedDate:  currentRow.CreatedDate,
		CategoryId:   currentRow.CategoryId,
		IsFeatured:   currentRow.IsFeatured,
		DiscountRate: currentRow.DiscountRate,
	}
	if true {
		params.Uuid = data.Uuid
	}
	if true {
		params.Name = data.Name
	}
	if true {
		params.Description = sql.NullString{String: data.Description, Valid: true}
	}
	if true {
		params.Price = sql.NullFloat64{Float64: data.Price, Valid: true}
	}
	if true {
		params.Weight = sql.NullFloat64{Float64: data.Weight, Valid: true}
	}
	if true {
		params.Quantity = data.Quantity
	}
	if true {
		params.InStock = sql.NullBool{Bool: data.InStock, Valid: true}
	}
	if true {
		params.Tags = sql.NullString{String: data.Tags, Valid: true}
	}
	if true {
		params.Metadata = data.Metadata
	}
	if true {
		params.CreatedDate = sql.NullTime{Time: data.CreatedDate, Valid: true}
	}
	if true {
		params.CategoryId = sql.NullInt64{Int64: data.CategoryId, Valid: true}
	}
	if true {
		params.IsFeatured = sql.NullBool{Bool: data.IsFeatured, Valid: true}
	}
	if true {
		params.DiscountRate = sql.NullFloat64{Float64: data.DiscountRate, Valid: true}
	}

	row, err := db.New().UpdateProduct(ctx, dbtx, params)
	if err != nil {
		return Product{}, err
	}

	return rowToProduct(row), nil
}

func DestroyProduct(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) error {
	return db.New().DeleteProduct(ctx, dbtx, id.String())
}

func AllProducts(
	ctx context.Context,
	dbtx db.DBTX,
) ([]Product, error) {
	rows, err := db.New().QueryAllProducts(ctx, dbtx)
	if err != nil {
		return nil, err
	}

	products := make([]Product, len(rows))
	for i, row := range rows {
		products[i] = rowToProduct(row)
	}

	return products, nil
}

type PaginatedProducts struct {
	Products   []Product
	TotalCount int64
	Page       int64
	PageSize   int64
	TotalPages int64
}

func PaginateProducts(
	ctx context.Context,
	dbtx db.DBTX,
	page int64,
	pageSize int64,
) (PaginatedProducts, error) {
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

	totalCount, err := db.New().CountProducts(ctx, dbtx)
	if err != nil {
		return PaginatedProducts{}, err
	}

	rows, err := db.New().QueryPaginatedProducts(
		ctx,
		dbtx,
		db.QueryPaginatedProductsParams{
			Limit:  pageSize,
			Offset: offset,
		},
	)
	if err != nil {
		return PaginatedProducts{}, err
	}

	products := make([]Product, len(rows))
	for i, row := range rows {
		products[i] = rowToProduct(row)
	}

	totalPages := (totalCount + int64(pageSize) - 1) / int64(pageSize)

	return PaginatedProducts{
		Products:   products,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func rowToProduct(row db.Product) Product {
	return Product{
		ID:           row.ID,
		Uuid:         row.Uuid,
		Name:         row.Name,
		Description:  row.Description.String,
		Price:        row.Price.Float64,
		Weight:       row.Weight.Float64,
		Quantity:     row.Quantity,
		InStock:      row.InStock.Bool,
		Tags:         row.Tags.String,
		Metadata:     row.Metadata,
		CreatedDate:  row.CreatedDate.Time,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt.Time,
		CategoryId:   row.CategoryId.Int64,
		IsFeatured:   row.IsFeatured.Bool,
		DiscountRate: row.DiscountRate.Float64,
	}
}
