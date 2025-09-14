package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"time"

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
	row, err := db.New().QueryProductByID(ctx, dbtx, id.String())
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

	params := db.NewInsertProductParams(
		sql.NullInt64{Int64: data.IntField, Valid: true},
		sql.NullInt64{Int64: data.IntegerField, Valid: true},
		sql.NullInt64{Int64: data.TinyintField, Valid: true},
		sql.NullInt64{Int64: data.SmallintField, Valid: true},
		sql.NullInt64{Int64: data.MediumintField, Valid: true},
		sql.NullInt64{Int64: data.BigintField, Valid: true},
		sql.NullInt64{Int64: data.UnsignedBigintField, Valid: true},
		sql.NullInt64{Int64: data.Int2Field, Valid: true},
		sql.NullInt64{Int64: data.Int8Field, Valid: true},
		sql.NullBool{Bool: data.BooleanField, Valid: true},
		sql.NullBool{Bool: data.BoolField, Valid: true},
		sql.NullString{String: data.CharacterField, Valid: true},
		sql.NullString{String: data.VarcharField, Valid: true},
		sql.NullString{String: data.VaryingCharacterField, Valid: true},
		sql.NullString{String: data.NcharField, Valid: true},
		sql.NullString{String: data.NativeCharacterField, Valid: true},
		sql.NullString{String: data.NvarcharField, Valid: true},
		sql.NullString{String: data.TextField, Valid: true},
		sql.NullString{String: data.ClobField, Valid: true},
		sql.NullString{String: data.CharField, Valid: true},
		sql.NullFloat64{Float64: data.RealField, Valid: true},
		sql.NullFloat64{Float64: data.DoubleField, Valid: true},
		sql.NullFloat64{Float64: data.DoublePrecisionField, Valid: true},
		sql.NullFloat64{Float64: data.FloatField, Valid: true},
		sql.NullFloat64{Float64: data.NumericField, Valid: true},
		sql.NullFloat64{Float64: data.DecimalField, Valid: true},
		sql.NullFloat64{Float64: data.DecField, Valid: true},
		data.BlobField,
		sql.NullTime{Time: data.DateAsText, Valid: true},
		sql.NullTime{Time: data.DatetimeAsText, Valid: true},
		sql.NullTime{Time: data.TimestampField, Valid: true},
		sql.NullTime{Time: data.TimeField, Valid: true},
		data.RequiredText,
		data.RequiredInt,
		sql.NullString{String: data.DefaultText, Valid: true},
		sql.NullInt64{Int64: data.DefaultInt, Valid: true},
		sql.NullFloat64{Float64: data.DefaultReal, Valid: true},
		sql.NullBool{Bool: data.DefaultBool, Valid: true},
		sql.NullTime{Time: data.DefaultTimestamp, Valid: true},
		sql.NullInt64{Int64: data.PositiveInt, Valid: true},
		sql.NullString{String: data.EmailText, Valid: true},
	)
	row, err := db.New().InsertProduct(ctx, dbtx, params)
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

	currentRow, err := db.New().QueryProductByID(ctx, dbtx, data.ID.String())
	if err != nil {
		return Product{}, err
	}

	params := db.NewUpdateProductParams(
		data.ID.String(),
		func() sql.NullInt64 {
			if true {
				return sql.NullInt64{Int64: data.IntField, Valid: true}
			}
			return currentRow.IntField
		}(),
		func() sql.NullInt64 {
			if true {
				return sql.NullInt64{Int64: data.IntegerField, Valid: true}
			}
			return currentRow.IntegerField
		}(),
		func() sql.NullInt64 {
			if true {
				return sql.NullInt64{Int64: data.TinyintField, Valid: true}
			}
			return currentRow.TinyintField
		}(),
		func() sql.NullInt64 {
			if true {
				return sql.NullInt64{Int64: data.SmallintField, Valid: true}
			}
			return currentRow.SmallintField
		}(),
		func() sql.NullInt64 {
			if true {
				return sql.NullInt64{Int64: data.MediumintField, Valid: true}
			}
			return currentRow.MediumintField
		}(),
		func() sql.NullInt64 {
			if true {
				return sql.NullInt64{Int64: data.BigintField, Valid: true}
			}
			return currentRow.BigintField
		}(),
		func() sql.NullInt64 {
			if true {
				return sql.NullInt64{Int64: data.UnsignedBigintField, Valid: true}
			}
			return currentRow.UnsignedBigintField
		}(),
		func() sql.NullInt64 {
			if true {
				return sql.NullInt64{Int64: data.Int2Field, Valid: true}
			}
			return currentRow.Int2Field
		}(),
		func() sql.NullInt64 {
			if true {
				return sql.NullInt64{Int64: data.Int8Field, Valid: true}
			}
			return currentRow.Int8Field
		}(),
		func() sql.NullBool {
			if true {
				return sql.NullBool{Bool: data.BooleanField, Valid: true}
			}
			return currentRow.BooleanField
		}(),
		func() sql.NullBool {
			if true {
				return sql.NullBool{Bool: data.BoolField, Valid: true}
			}
			return currentRow.BoolField
		}(),
		func() sql.NullString {
			if true {
				return sql.NullString{String: data.CharacterField, Valid: true}
			}
			return currentRow.CharacterField
		}(),
		func() sql.NullString {
			if true {
				return sql.NullString{String: data.VarcharField, Valid: true}
			}
			return currentRow.VarcharField
		}(),
		func() sql.NullString {
			if true {
				return sql.NullString{String: data.VaryingCharacterField, Valid: true}
			}
			return currentRow.VaryingCharacterField
		}(),
		func() sql.NullString {
			if true {
				return sql.NullString{String: data.NcharField, Valid: true}
			}
			return currentRow.NcharField
		}(),
		func() sql.NullString {
			if true {
				return sql.NullString{String: data.NativeCharacterField, Valid: true}
			}
			return currentRow.NativeCharacterField
		}(),
		func() sql.NullString {
			if true {
				return sql.NullString{String: data.NvarcharField, Valid: true}
			}
			return currentRow.NvarcharField
		}(),
		func() sql.NullString {
			if true {
				return sql.NullString{String: data.TextField, Valid: true}
			}
			return currentRow.TextField
		}(),
		func() sql.NullString {
			if true {
				return sql.NullString{String: data.ClobField, Valid: true}
			}
			return currentRow.ClobField
		}(),
		func() sql.NullString {
			if true {
				return sql.NullString{String: data.CharField, Valid: true}
			}
			return currentRow.CharField
		}(),
		func() sql.NullFloat64 {
			if true {
				return sql.NullFloat64{Float64: data.RealField, Valid: true}
			}
			return currentRow.RealField
		}(),
		func() sql.NullFloat64 {
			if true {
				return sql.NullFloat64{Float64: data.DoubleField, Valid: true}
			}
			return currentRow.DoubleField
		}(),
		func() sql.NullFloat64 {
			if true {
				return sql.NullFloat64{Float64: data.DoublePrecisionField, Valid: true}
			}
			return currentRow.DoublePrecisionField
		}(),
		func() sql.NullFloat64 {
			if true {
				return sql.NullFloat64{Float64: data.FloatField, Valid: true}
			}
			return currentRow.FloatField
		}(),
		func() sql.NullFloat64 {
			if true {
				return sql.NullFloat64{Float64: data.NumericField, Valid: true}
			}
			return currentRow.NumericField
		}(),
		func() sql.NullFloat64 {
			if true {
				return sql.NullFloat64{Float64: data.DecimalField, Valid: true}
			}
			return currentRow.DecimalField
		}(),
		func() sql.NullFloat64 {
			if true {
				return sql.NullFloat64{Float64: data.DecField, Valid: true}
			}
			return currentRow.DecField
		}(),
		func() []byte {
			if true {
				return data.BlobField
			}
			return currentRow.BlobField
		}(),
		func() sql.NullTime {
			if true {
				return sql.NullTime{Time: data.DateAsText, Valid: true}
			}
			return currentRow.DateAsText
		}(),
		func() sql.NullTime {
			if true {
				return sql.NullTime{Time: data.DatetimeAsText, Valid: true}
			}
			return currentRow.DatetimeAsText
		}(),
		func() sql.NullTime {
			if true {
				return sql.NullTime{Time: data.TimestampField, Valid: true}
			}
			return currentRow.TimestampField
		}(),
		func() sql.NullTime {
			if true {
				return sql.NullTime{Time: data.TimeField, Valid: true}
			}
			return currentRow.TimeField
		}(),
		func() string {
			if true {
				return data.RequiredText
			}
			return currentRow.RequiredText
		}(),
		func() int64 {
			if true {
				return data.RequiredInt
			}
			return currentRow.RequiredInt
		}(),
		func() sql.NullString {
			if true {
				return sql.NullString{String: data.DefaultText, Valid: true}
			}
			return currentRow.DefaultText
		}(),
		func() sql.NullInt64 {
			if true {
				return sql.NullInt64{Int64: data.DefaultInt, Valid: true}
			}
			return currentRow.DefaultInt
		}(),
		func() sql.NullFloat64 {
			if true {
				return sql.NullFloat64{Float64: data.DefaultReal, Valid: true}
			}
			return currentRow.DefaultReal
		}(),
		func() sql.NullBool {
			if true {
				return sql.NullBool{Bool: data.DefaultBool, Valid: true}
			}
			return currentRow.DefaultBool
		}(),
		func() sql.NullTime {
			if true {
				return sql.NullTime{Time: data.DefaultTimestamp, Valid: true}
			}
			return currentRow.DefaultTimestamp
		}(),
		func() sql.NullInt64 {
			if true {
				return sql.NullInt64{Int64: data.PositiveInt, Valid: true}
			}
			return currentRow.PositiveInt
		}(),
		func() sql.NullString {
			if true {
				return sql.NullString{String: data.EmailText, Valid: true}
			}
			return currentRow.EmailText
		}(),
	)

	row, err := db.New().UpdateProduct(ctx, dbtx, params)
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

	totalCount, err := db.New().CountProducts(ctx, dbtx)
	if err != nil {
		return PaginatedProducts{}, err
	}

	rows, err := db.New().QueryPaginatedProducts(
		ctx,
		dbtx,
		db.NewQueryPaginatedProductsParams(pageSize, offset),
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
