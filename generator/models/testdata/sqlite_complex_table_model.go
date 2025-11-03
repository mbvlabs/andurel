package models

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/example/sqlite/models/internal/db"
)

type Product struct {
	ID                    uuid.UUID
	IntField              int64
	IntegerField          int64
	TinyintField          int64
	SmallintField         int64
	MediumintField        int64
	BigintField           int64
	UnsignedBigintField   int64
	Int2Field             int64
	Int8Field             int64
	BooleanField          bool
	BoolField             bool
	CharacterField        string
	VarcharField          string
	VaryingCharacterField string
	NcharField            string
	NativeCharacterField  string
	NvarcharField         string
	TextField             string
	ClobField             string
	CharField             string
	RealField             float64
	DoubleField           float64
	DoublePrecisionField  float64
	FloatField            float64
	NumericField          float64
	DecimalField          float64
	DecField              float64
	BlobField             []byte
	DateAsText            time.Time
	DatetimeAsText        time.Time
	TimestampField        time.Time
	TimeField             time.Time
	RequiredText          string
	RequiredInt           int64
	DefaultText           string
	DefaultInt            int64
	DefaultReal           float64
	DefaultBool           bool
	DefaultTimestamp      time.Time
	PositiveInt           int64
	EmailText             string
}

func FindProduct(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) (Product, error) {
	row, err := queries.QueryProductByID(ctx, dbtx, id.String())
	if err != nil {
		return Product{}, err
	}

	result, err := rowToProduct(row)
	if err != nil {
		return Product{}, err
	}
	return result, nil
}

type CreateProductData struct {
	IntField              int64
	IntegerField          int64
	TinyintField          int64
	SmallintField         int64
	MediumintField        int64
	BigintField           int64
	UnsignedBigintField   int64
	Int2Field             int64
	Int8Field             int64
	BooleanField          bool
	BoolField             bool
	CharacterField        string
	VarcharField          string
	VaryingCharacterField string
	NcharField            string
	NativeCharacterField  string
	NvarcharField         string
	TextField             string
	ClobField             string
	CharField             string
	RealField             float64
	DoubleField           float64
	DoublePrecisionField  float64
	FloatField            float64
	NumericField          float64
	DecimalField          float64
	DecField              float64
	BlobField             []byte
	DateAsText            time.Time
	DatetimeAsText        time.Time
	TimestampField        time.Time
	TimeField             time.Time
	RequiredText          string
	RequiredInt           int64
	DefaultText           string
	DefaultInt            int64
	DefaultReal           float64
	DefaultBool           bool
	DefaultTimestamp      time.Time
	PositiveInt           int64
	EmailText             string
}

func CreateProduct(
	ctx context.Context,
	dbtx db.DBTX,
	data CreateProductData,
) (Product, error) {
	if err := validate.Struct(data); err != nil {
		return Product{}, errors.Join(ErrDomainValidation, err)
	}

	params := db.CreateInsertProductParams()
	row, err := queries.InsertProduct(ctx, dbtx, params)
	if err != nil {
		return Product{}, err
	}

	result, err := rowToProduct(row)
	if err != nil {
		return Product{}, err
	}
	return result, nil
}

type UpdateProductData struct {
	ID                    uuid.UUID
	IntField              int64
	IntegerField          int64
	TinyintField          int64
	SmallintField         int64
	MediumintField        int64
	BigintField           int64
	UnsignedBigintField   int64
	Int2Field             int64
	Int8Field             int64
	BooleanField          bool
	BoolField             bool
	CharacterField        string
	VarcharField          string
	VaryingCharacterField string
	NcharField            string
	NativeCharacterField  string
	NvarcharField         string
	TextField             string
	ClobField             string
	CharField             string
	RealField             float64
	DoubleField           float64
	DoublePrecisionField  float64
	FloatField            float64
	NumericField          float64
	DecimalField          float64
	DecField              float64
	BlobField             []byte
	DateAsText            time.Time
	DatetimeAsText        time.Time
	TimestampField        time.Time
	TimeField             time.Time
	RequiredText          string
	RequiredInt           int64
	DefaultText           string
	DefaultInt            int64
	DefaultReal           float64
	DefaultBool           bool
	DefaultTimestamp      time.Time
	PositiveInt           int64
	EmailText             string
}

func UpdateProduct(
	ctx context.Context,
	dbtx db.DBTX,
	data UpdateProductData,
) (Product, error) {
	if err := validate.Struct(data); err != nil {
		return Product{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := queries.QueryProductByID(ctx, dbtx, data.ID.String())
	if err != nil {
		return Product{}, err
	}

	params := db.CreateUpdateProductParams()

	row, err := queries.UpdateProduct(ctx, dbtx, params)
	if err != nil {
		return Product{}, err
	}

	result, err := rowToProduct(row)
	if err != nil {
		return Product{}, err
	}
	return result, nil
}

func DestroyProduct(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) error {
	return queries.DeleteProduct(ctx, dbtx, id.String())
}

func AllProducts(
	ctx context.Context,
	dbtx db.DBTX,
) ([]Product, error) {
	rows, err := queries.QueryAllProducts(ctx, dbtx)
	if err != nil {
		return nil, err
	}

	products := make([]Product, len(rows))
	for i, row := range rows {
		result, err := rowToProduct(row)
		if err != nil {
			return nil, err
		}
		products[i] = result
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

	totalCount, err := queries.CountProducts(ctx, dbtx)
	if err != nil {
		return PaginatedProducts{}, err
	}

	rows, err := queries.QueryPaginatedProducts(
		ctx,
		dbtx,
		db.CreateQueryPaginatedProductsParams(pageSize, offset),
	)
	if err != nil {
		return PaginatedProducts{}, err
	}

	products := make([]Product, len(rows))
	for i, row := range rows {
		result, err := rowToProduct(row)
		if err != nil {
			return PaginatedProducts{}, err
		}
		products[i] = result
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

func rowToProduct(row db.Product) (Product, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return Product{}, err
	}

	return Product{
		ID:                    id,
		IntField:              row.IntField.Int64,
		IntegerField:          row.IntegerField.Int64,
		TinyintField:          row.TinyintField.Int64,
		SmallintField:         row.SmallintField.Int64,
		MediumintField:        row.MediumintField.Int64,
		BigintField:           row.BigintField.Int64,
		UnsignedBigintField:   row.UnsignedBigintField.Int64,
		Int2Field:             row.Int2Field.Int64,
		Int8Field:             row.Int8Field.Int64,
		BooleanField:          row.BooleanField.Bool,
		BoolField:             row.BoolField.Bool,
		CharacterField:        row.CharacterField.String,
		VarcharField:          row.VarcharField.String,
		VaryingCharacterField: row.VaryingCharacterField.String,
		NcharField:            row.NcharField.String,
		NativeCharacterField:  row.NativeCharacterField.String,
		NvarcharField:         row.NvarcharField.String,
		TextField:             row.TextField.String,
		ClobField:             row.ClobField.String,
		CharField:             row.CharField.String,
		RealField:             row.RealField.Float64,
		DoubleField:           row.DoubleField.Float64,
		DoublePrecisionField:  row.DoublePrecisionField.Float64,
		FloatField:            row.FloatField.Float64,
		NumericField:          row.NumericField.Float64,
		DecimalField:          row.DecimalField.Float64,
		DecField:              row.DecField.Float64,
		BlobField:             row.BlobField,
		DateAsText:            row.DateAsText.Time,
		DatetimeAsText:        row.DatetimeAsText.Time,
		TimestampField:        row.TimestampField.Time,
		TimeField:             row.TimeField.Time,
		RequiredText:          row.RequiredText,
		RequiredInt:           row.RequiredInt,
		DefaultText:           row.DefaultText.String,
		DefaultInt:            row.DefaultInt.Int64,
		DefaultReal:           row.DefaultReal.Float64,
		DefaultBool:           row.DefaultBool.Bool,
		DefaultTimestamp:      row.DefaultTimestamp.Time,
		PositiveInt:           row.PositiveInt.Int64,
		EmailText:             row.EmailText.String,
	}, nil
}
