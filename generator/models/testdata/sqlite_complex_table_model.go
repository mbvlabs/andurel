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

	row, err := db.New().InsertProduct(ctx, dbtx, db.InsertProductParams{
		ID:                    uuid.New().String(),
		IntField:              sql.NullInt64{Int64: data.IntField, Valid: true},
		IntegerField:          sql.NullInt64{Int64: data.IntegerField, Valid: true},
		TinyintField:          sql.NullInt64{Int64: data.TinyintField, Valid: true},
		SmallintField:         sql.NullInt64{Int64: data.SmallintField, Valid: true},
		MediumintField:        sql.NullInt64{Int64: data.MediumintField, Valid: true},
		BigintField:           sql.NullInt64{Int64: data.BigintField, Valid: true},
		UnsignedBigintField:   sql.NullInt64{Int64: data.UnsignedBigintField, Valid: true},
		Int2Field:             sql.NullInt64{Int64: data.Int2Field, Valid: true},
		Int8Field:             sql.NullInt64{Int64: data.Int8Field, Valid: true},
		BooleanField:          sql.NullBool{Bool: data.BooleanField, Valid: true},
		BoolField:             sql.NullBool{Bool: data.BoolField, Valid: true},
		CharacterField:        sql.NullString{String: data.CharacterField, Valid: true},
		VarcharField:          sql.NullString{String: data.VarcharField, Valid: true},
		VaryingCharacterField: sql.NullString{String: data.VaryingCharacterField, Valid: true},
		NcharField:            sql.NullString{String: data.NcharField, Valid: true},
		NativeCharacterField:  sql.NullString{String: data.NativeCharacterField, Valid: true},
		NvarcharField:         sql.NullString{String: data.NvarcharField, Valid: true},
		TextField:             sql.NullString{String: data.TextField, Valid: true},
		ClobField:             sql.NullString{String: data.ClobField, Valid: true},
		CharField:             sql.NullString{String: data.CharField, Valid: true},
		RealField:             sql.NullFloat64{Float64: data.RealField, Valid: true},
		DoubleField:           sql.NullFloat64{Float64: data.DoubleField, Valid: true},
		DoublePrecisionField:  sql.NullFloat64{Float64: data.DoublePrecisionField, Valid: true},
		FloatField:            sql.NullFloat64{Float64: data.FloatField, Valid: true},
		NumericField:          sql.NullFloat64{Float64: data.NumericField, Valid: true},
		DecimalField:          sql.NullFloat64{Float64: data.DecimalField, Valid: true},
		DecField:              sql.NullFloat64{Float64: data.DecField, Valid: true},
		BlobField:             data.BlobField,
		DateAsText:            sql.NullTime{Time: data.DateAsText, Valid: true},
		DatetimeAsText:        sql.NullTime{Time: data.DatetimeAsText, Valid: true},
		TimestampField:        sql.NullTime{Time: data.TimestampField, Valid: true},
		TimeField:             sql.NullTime{Time: data.TimeField, Valid: true},
		RequiredText:          data.RequiredText,
		RequiredInt:           data.RequiredInt,
		DefaultText:           sql.NullString{String: data.DefaultText, Valid: true},
		DefaultInt:            sql.NullInt64{Int64: data.DefaultInt, Valid: true},
		DefaultReal:           sql.NullFloat64{Float64: data.DefaultReal, Valid: true},
		DefaultBool:           sql.NullBool{Bool: data.DefaultBool, Valid: true},
		DefaultTimestamp:      sql.NullTime{Time: data.DefaultTimestamp, Valid: true},
		PositiveInt:           sql.NullInt64{Int64: data.PositiveInt, Valid: true},
		EmailText:             sql.NullString{String: data.EmailText, Valid: true},
	})
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

	params := db.UpdateProductParams{
		ID:                    data.ID.String(),
		IntField:              currentRow.IntField,
		IntegerField:          currentRow.IntegerField,
		TinyintField:          currentRow.TinyintField,
		SmallintField:         currentRow.SmallintField,
		MediumintField:        currentRow.MediumintField,
		BigintField:           currentRow.BigintField,
		UnsignedBigintField:   currentRow.UnsignedBigintField,
		Int2Field:             currentRow.Int2Field,
		Int8Field:             currentRow.Int8Field,
		BooleanField:          currentRow.BooleanField,
		BoolField:             currentRow.BoolField,
		CharacterField:        currentRow.CharacterField,
		VarcharField:          currentRow.VarcharField,
		VaryingCharacterField: currentRow.VaryingCharacterField,
		NcharField:            currentRow.NcharField,
		NativeCharacterField:  currentRow.NativeCharacterField,
		NvarcharField:         currentRow.NvarcharField,
		TextField:             currentRow.TextField,
		ClobField:             currentRow.ClobField,
		CharField:             currentRow.CharField,
		RealField:             currentRow.RealField,
		DoubleField:           currentRow.DoubleField,
		DoublePrecisionField:  currentRow.DoublePrecisionField,
		FloatField:            currentRow.FloatField,
		NumericField:          currentRow.NumericField,
		DecimalField:          currentRow.DecimalField,
		DecField:              currentRow.DecField,
		BlobField:             currentRow.BlobField,
		DateAsText:            currentRow.DateAsText,
		DatetimeAsText:        currentRow.DatetimeAsText,
		TimestampField:        currentRow.TimestampField,
		TimeField:             currentRow.TimeField,
		RequiredText:          currentRow.RequiredText,
		RequiredInt:           currentRow.RequiredInt,
		DefaultText:           currentRow.DefaultText,
		DefaultInt:            currentRow.DefaultInt,
		DefaultReal:           currentRow.DefaultReal,
		DefaultBool:           currentRow.DefaultBool,
		DefaultTimestamp:      currentRow.DefaultTimestamp,
		PositiveInt:           currentRow.PositiveInt,
		EmailText:             currentRow.EmailText,
	}
	if true {
		params.IntField = sql.NullInt64{Int64: data.IntField, Valid: true}
	}
	if true {
		params.IntegerField = sql.NullInt64{Int64: data.IntegerField, Valid: true}
	}
	if true {
		params.TinyintField = sql.NullInt64{Int64: data.TinyintField, Valid: true}
	}
	if true {
		params.SmallintField = sql.NullInt64{Int64: data.SmallintField, Valid: true}
	}
	if true {
		params.MediumintField = sql.NullInt64{Int64: data.MediumintField, Valid: true}
	}
	if true {
		params.BigintField = sql.NullInt64{Int64: data.BigintField, Valid: true}
	}
	if true {
		params.UnsignedBigintField = sql.NullInt64{Int64: data.UnsignedBigintField, Valid: true}
	}
	if true {
		params.Int2Field = sql.NullInt64{Int64: data.Int2Field, Valid: true}
	}
	if true {
		params.Int8Field = sql.NullInt64{Int64: data.Int8Field, Valid: true}
	}
	if true {
		params.BooleanField = sql.NullBool{Bool: data.BooleanField, Valid: true}
	}
	if true {
		params.BoolField = sql.NullBool{Bool: data.BoolField, Valid: true}
	}
	if true {
		params.CharacterField = sql.NullString{String: data.CharacterField, Valid: true}
	}
	if true {
		params.VarcharField = sql.NullString{String: data.VarcharField, Valid: true}
	}
	if true {
		params.VaryingCharacterField = sql.NullString{String: data.VaryingCharacterField, Valid: true}
	}
	if true {
		params.NcharField = sql.NullString{String: data.NcharField, Valid: true}
	}
	if true {
		params.NativeCharacterField = sql.NullString{String: data.NativeCharacterField, Valid: true}
	}
	if true {
		params.NvarcharField = sql.NullString{String: data.NvarcharField, Valid: true}
	}
	if true {
		params.TextField = sql.NullString{String: data.TextField, Valid: true}
	}
	if true {
		params.ClobField = sql.NullString{String: data.ClobField, Valid: true}
	}
	if true {
		params.CharField = sql.NullString{String: data.CharField, Valid: true}
	}
	if true {
		params.RealField = sql.NullFloat64{Float64: data.RealField, Valid: true}
	}
	if true {
		params.DoubleField = sql.NullFloat64{Float64: data.DoubleField, Valid: true}
	}
	if true {
		params.DoublePrecisionField = sql.NullFloat64{Float64: data.DoublePrecisionField, Valid: true}
	}
	if true {
		params.FloatField = sql.NullFloat64{Float64: data.FloatField, Valid: true}
	}
	if true {
		params.NumericField = sql.NullFloat64{Float64: data.NumericField, Valid: true}
	}
	if true {
		params.DecimalField = sql.NullFloat64{Float64: data.DecimalField, Valid: true}
	}
	if true {
		params.DecField = sql.NullFloat64{Float64: data.DecField, Valid: true}
	}
	if true {
		params.BlobField = data.BlobField
	}
	if true {
		params.DateAsText = sql.NullTime{Time: data.DateAsText, Valid: true}
	}
	if true {
		params.DatetimeAsText = sql.NullTime{Time: data.DatetimeAsText, Valid: true}
	}
	if true {
		params.TimestampField = sql.NullTime{Time: data.TimestampField, Valid: true}
	}
	if true {
		params.TimeField = sql.NullTime{Time: data.TimeField, Valid: true}
	}
	if true {
		params.RequiredText = data.RequiredText
	}
	if true {
		params.RequiredInt = data.RequiredInt
	}
	if true {
		params.DefaultText = sql.NullString{String: data.DefaultText, Valid: true}
	}
	if true {
		params.DefaultInt = sql.NullInt64{Int64: data.DefaultInt, Valid: true}
	}
	if true {
		params.DefaultReal = sql.NullFloat64{Float64: data.DefaultReal, Valid: true}
	}
	if true {
		params.DefaultBool = sql.NullBool{Bool: data.DefaultBool, Valid: true}
	}
	if true {
		params.DefaultTimestamp = sql.NullTime{Time: data.DefaultTimestamp, Valid: true}
	}
	if true {
		params.PositiveInt = sql.NullInt64{Int64: data.PositiveInt, Valid: true}
	}
	if true {
		params.EmailText = sql.NullString{String: data.EmailText, Valid: true}
	}

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
