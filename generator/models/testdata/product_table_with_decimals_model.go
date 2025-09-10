package models

import (
	"context"
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/example/shop/models/internal/db"
)

type Product struct {
	ID          int32
	Name        string
	Price       float64
	Description string
	CategoryId  int32
	InStock     bool
	Metadata    []byte
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func FindProduct(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) (Product, error) {
	row, err := db.New().QueryProductByID(ctx, dbtx, id)
	if err != nil {
		return Product{}, err
	}

	return rowToProduct(row), nil
}

type CreateProductPayload struct {
	Name        string
	Price       float64
	Description string
	CategoryId  int32
	InStock     bool
	Metadata    []byte
}

func CreateProduct(
	ctx context.Context,
	dbtx db.DBTX,
	data CreateProductPayload,
) (Product, error) {
	if err := validate.Struct(data); err != nil {
		return Product{}, errors.Join(ErrDomainValidation, err)
	}

	row, err := db.New().InsertProduct(ctx, dbtx, db.InsertProductParams{
		ID:          uuid.New(),
		Name:        data.Name,
		Price:       data.Price,
		Description: pgtype.Text{String: data.Description, Valid: true},
		CategoryId:  data.CategoryId,
		InStock:     pgtype.Bool{Bool: data.InStock, Valid: true},
		Metadata:    data.Metadata,
	})
	if err != nil {
		return Product{}, err
	}

	return rowToProduct(row), nil
}

type UpdateProductPayload struct {
	ID          uuid.UUID
	Name        string
	Price       float64
	Description string
	CategoryId  int32
	InStock     bool
	Metadata    []byte
	UpdatedAt   time.Time
}

func UpdateProduct(
	ctx context.Context,
	dbtx db.DBTX,
	data UpdateProductPayload,
) (Product, error) {
	if err := validate.Struct(data); err != nil {
		return Product{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := db.New().QueryProductByID(ctx, dbtx, data.ID)
	if err != nil {
		return Product{}, err
	}

	payload := db.UpdateProductParams{
		ID:          data.ID,
		Name:        currentRow.Name,
		Price:       currentRow.Price,
		Description: currentRow.Description,
		CategoryId:  currentRow.CategoryId,
		InStock:     currentRow.InStock,
		Metadata:    currentRow.Metadata,
	}
	if true {
		payload.Name = data.Name
	}
	if true {
		payload.Price = data.Price
	}
	if true {
		payload.Description = pgtype.Text{String: data.Description, Valid: true}
	}
	if true {
		payload.CategoryId = data.CategoryId
	}
	if true {
		payload.InStock = pgtype.Bool{Bool: data.InStock, Valid: true}
	}
	if true {
		payload.Metadata = data.Metadata
	}

	row, err := db.New().UpdateProduct(ctx, dbtx, payload)
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
	return db.New().DeleteProduct(ctx, dbtx, id)
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
		ID:          row.ID,
		Name:        row.Name,
		Price:       row.Price,
		Description: row.Description.String,
		CategoryId:  row.CategoryId,
		InStock:     row.InStock.Bool,
		Metadata:    row.Metadata,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}
