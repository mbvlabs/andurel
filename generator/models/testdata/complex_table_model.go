package models

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/example/complex/models/internal/db"
)

type ComprehensiveExample struct {
	ID                uuid.UUID
	UuidID            uuid.UUID
	SmallInt          int16
	RegularInt        int32
	BigInt            int64
	DecimalPrecise    float64
	NumericField      float64
	RealFloat         float32
	DoubleFloat       float64
	SmallSerial       int16
	BigSerial         int64
	FixedChar         string
	VariableChar      string
	UnlimitedText     string
	TextWithDefault   string
	TextNotNull       string
	IsActive          bool
	IsVerified        bool
	NullableFlag      bool
	CreatedDate       time.Time
	BirthDate         time.Time
	ExactTime         time.Time
	TimeWithZone      time.Time
	CreatedTimestamp  time.Time
	UpdatedTimestamp  time.Time
	TimestampWithZone time.Time
	DurationInterval  string
	WorkHours         string
	FileData          []byte
	RequiredBinary    []byte
	IpAddress         string
	IpNetwork         string
	MacAddress        string
	Mac8Address       string
	PointLocation     string
	LineSegment       string
	RectangularBox    string
	PathData          string
	PolygonShape      string
	CircleArea        string
	JsonData          []byte
	JsonbData         []byte
	JsonbNotNull      []byte
	IntegerArray      []int32
	TextArray         []string
	MultidimArray     []int32
	IntRange          string
	BigintRange       string
	NumericRange      string
}

func FindComprehensiveExample(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) (ComprehensiveExample, error) {
	row, err := queries.QueryComprehensiveExampleByID(ctx, dbtx, id)
	if err != nil {
		return ComprehensiveExample{}, err
	}

	return rowToComprehensiveExample(row), nil
}

type CreateComprehensiveExampleData struct {
	UuidID            uuid.UUID
	SmallInt          int16
	RegularInt        int32
	BigInt            int64
	DecimalPrecise    float64
	NumericField      float64
	RealFloat         float32
	DoubleFloat       float64
	SmallSerial       int16
	BigSerial         int64
	FixedChar         string
	VariableChar      string
	UnlimitedText     string
	TextWithDefault   string
	TextNotNull       string
	IsActive          bool
	IsVerified        bool
	NullableFlag      bool
	CreatedDate       time.Time
	BirthDate         time.Time
	ExactTime         time.Time
	TimeWithZone      time.Time
	CreatedTimestamp  time.Time
	UpdatedTimestamp  time.Time
	TimestampWithZone time.Time
	DurationInterval  string
	WorkHours         string
	FileData          []byte
	RequiredBinary    []byte
	IpAddress         string
	IpNetwork         string
	MacAddress        string
	Mac8Address       string
	PointLocation     string
	LineSegment       string
	RectangularBox    string
	PathData          string
	PolygonShape      string
	CircleArea        string
	JsonData          []byte
	JsonbData         []byte
	JsonbNotNull      []byte
	IntegerArray      []int32
	TextArray         []string
	MultidimArray     []int32
	IntRange          string
	BigintRange       string
	NumericRange      string
}

func CreateComprehensiveExample(
	ctx context.Context,
	dbtx db.DBTX,
	data CreateComprehensiveExampleData,
) (ComprehensiveExample, error) {
	if err := validate.Struct(data); err != nil {
		return ComprehensiveExample{}, errors.Join(ErrDomainValidation, err)
	}

	params := db.CreateInsertComprehensiveExampleParams()
	row, err := queries.InsertComprehensiveExample(ctx, dbtx, params)
	if err != nil {
		return ComprehensiveExample{}, err
	}

	return rowToComprehensiveExample(row), nil
}

type UpdateComprehensiveExampleData struct {
	ID                uuid.UUID
	UuidID            uuid.UUID
	SmallInt          int16
	RegularInt        int32
	BigInt            int64
	DecimalPrecise    float64
	NumericField      float64
	RealFloat         float32
	DoubleFloat       float64
	SmallSerial       int16
	BigSerial         int64
	FixedChar         string
	VariableChar      string
	UnlimitedText     string
	TextWithDefault   string
	TextNotNull       string
	IsActive          bool
	IsVerified        bool
	NullableFlag      bool
	CreatedDate       time.Time
	BirthDate         time.Time
	ExactTime         time.Time
	TimeWithZone      time.Time
	CreatedTimestamp  time.Time
	UpdatedTimestamp  time.Time
	TimestampWithZone time.Time
	DurationInterval  string
	WorkHours         string
	FileData          []byte
	RequiredBinary    []byte
	IpAddress         string
	IpNetwork         string
	MacAddress        string
	Mac8Address       string
	PointLocation     string
	LineSegment       string
	RectangularBox    string
	PathData          string
	PolygonShape      string
	CircleArea        string
	JsonData          []byte
	JsonbData         []byte
	JsonbNotNull      []byte
	IntegerArray      []int32
	TextArray         []string
	MultidimArray     []int32
	IntRange          string
	BigintRange       string
	NumericRange      string
}

func UpdateComprehensiveExample(
	ctx context.Context,
	dbtx db.DBTX,
	data UpdateComprehensiveExampleData,
) (ComprehensiveExample, error) {
	if err := validate.Struct(data); err != nil {
		return ComprehensiveExample{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := queries.QueryComprehensiveExampleByID(ctx, dbtx, data.ID)
	if err != nil {
		return ComprehensiveExample{}, err
	}

	params := db.CreateUpdateComprehensiveExampleParams()

	row, err := queries.UpdateComprehensiveExample(ctx, dbtx, params)
	if err != nil {
		return ComprehensiveExample{}, err
	}

	return rowToComprehensiveExample(row), nil
}

func DestroyComprehensiveExample(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) error {
	return queries.DeleteComprehensiveExample(ctx, dbtx, id)
}

func AllComprehensiveExamples(
	ctx context.Context,
	dbtx db.DBTX,
) ([]ComprehensiveExample, error) {
	rows, err := queries.QueryAllComprehensiveExamples(ctx, dbtx)
	if err != nil {
		return nil, err
	}

	comprehensiveexamples := make([]ComprehensiveExample, len(rows))
	for i, row := range rows {
		comprehensiveexamples[i] = rowToComprehensiveExample(row)
	}

	return comprehensiveexamples, nil
}

type PaginatedComprehensiveExamples struct {
	ComprehensiveExamples []ComprehensiveExample
	TotalCount            int64
	Page                  int64
	PageSize              int64
	TotalPages            int64
}

func PaginateComprehensiveExamples(
	ctx context.Context,
	dbtx db.DBTX,
	page int64,
	pageSize int64,
) (PaginatedComprehensiveExamples, error) {
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

	totalCount, err := queries.CountComprehensiveExamples(ctx, dbtx)
	if err != nil {
		return PaginatedComprehensiveExamples{}, err
	}

	rows, err := queries.QueryPaginatedComprehensiveExamples(
		ctx,
		dbtx,
		db.CreateQueryPaginatedComprehensiveExamplesParams(pageSize, offset),
	)
	if err != nil {
		return PaginatedComprehensiveExamples{}, err
	}

	comprehensiveexamples := make([]ComprehensiveExample, len(rows))
	for i, row := range rows {
		comprehensiveexamples[i] = rowToComprehensiveExample(row)
	}

	totalPages := (totalCount + int64(pageSize) - 1) / int64(pageSize)

	return PaginatedComprehensiveExamples{
		ComprehensiveExamples: comprehensiveexamples,
		TotalCount:            totalCount,
		Page:                  page,
		PageSize:              pageSize,
		TotalPages:            totalPages,
	}, nil
}

func rowToComprehensiveExample(row db.ComprehensiveExample) ComprehensiveExample {
	return ComprehensiveExample{
		ID:                row.ID,
		UuidID:            row.UuidID,
		SmallInt:          row.SmallInt,
		RegularInt:        row.RegularInt.Int32,
		BigInt:            row.BigInt,
		DecimalPrecise:    row.DecimalPrecise,
		NumericField:      row.NumericField,
		RealFloat:         row.RealFloat.Float32,
		DoubleFloat:       row.DoubleFloat,
		SmallSerial:       row.SmallSerial,
		BigSerial:         row.BigSerial.Int64,
		FixedChar:         row.FixedChar.String,
		VariableChar:      row.VariableChar,
		UnlimitedText:     row.UnlimitedText.String,
		TextWithDefault:   row.TextWithDefault.String,
		TextNotNull:       row.TextNotNull,
		IsActive:          row.IsActive.Bool,
		IsVerified:        row.IsVerified,
		NullableFlag:      row.NullableFlag.Bool,
		CreatedDate:       row.CreatedDate.Time,
		BirthDate:         row.BirthDate.Time,
		ExactTime:         row.ExactTime.Time,
		TimeWithZone:      row.TimeWithZone.Time,
		CreatedTimestamp:  row.CreatedTimestamp.Time,
		UpdatedTimestamp:  row.UpdatedTimestamp.Time,
		TimestampWithZone: row.TimestampWithZone.Time,
		DurationInterval:  row.DurationInterval.Microseconds,
		WorkHours:         row.WorkHours.Microseconds,
		FileData:          row.FileData,
		RequiredBinary:    row.RequiredBinary,
		IpAddress:         row.IpAddress.IPNet.String(),
		IpNetwork:         row.IpNetwork.IPNet.String(),
		MacAddress:        row.MacAddress.IPNet.String(),
		Mac8Address:       row.Mac8Address.IPNet.String(),
		PointLocation:     string(row.PointLocation.Bytes),
		LineSegment:       string(row.LineSegment.Bytes),
		RectangularBox:    string(row.RectangularBox.Bytes),
		PathData:          string(row.PathData.Bytes),
		PolygonShape:      string(row.PolygonShape.Bytes),
		CircleArea:        string(row.CircleArea.Bytes),
		JsonData:          row.JsonData.Bytes,
		JsonbData:         row.JsonbData.Bytes,
		JsonbNotNull:      row.JsonbNotNull.Bytes,
		IntegerArray:      row.IntegerArray.Elements,
		TextArray:         row.TextArray.Elements,
		MultidimArray:     row.MultidimArray.Elements,
		IntRange:          string(row.IntRange.Bytes),
		BigintRange:       string(row.BigintRange.Bytes),
		NumericRange:      string(row.NumericRange.Bytes),
	}
}
