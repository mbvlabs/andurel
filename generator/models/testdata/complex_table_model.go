package models

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"time"

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
	row, err := db.New().QueryComprehensiveExampleByID(ctx, dbtx, id)
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

	params := db.NewInsertComprehensiveExampleParams(
		data.UuidID,
		data.SmallInt,
		pgtype.Int4{Int32: data.RegularInt, Valid: true},
		data.BigInt,
		data.DecimalPrecise,
		data.NumericField,
		pgtype.Float4{Float32: data.RealFloat, Valid: true},
		data.DoubleFloat,
		data.SmallSerial,
		pgtype.Int8{Int64: data.BigSerial, Valid: true},
		pgtype.Text{String: data.FixedChar, Valid: true},
		data.VariableChar,
		pgtype.Text{String: data.UnlimitedText, Valid: true},
		pgtype.Text{String: data.TextWithDefault, Valid: true},
		data.TextNotNull,
		pgtype.Bool{Bool: data.IsActive, Valid: true},
		data.IsVerified,
		pgtype.Bool{Bool: data.NullableFlag, Valid: true},
		pgtype.Date{Time: data.CreatedDate, Valid: true},
		pgtype.Date{Time: data.BirthDate, Valid: true},
		pgtype.Time{Time: data.ExactTime, Valid: true},
		pgtype.Timetz{Time: data.TimeWithZone, Valid: true},
		pgtype.Timestamp{Time: data.CreatedTimestamp, Valid: true},
		pgtype.Timestamp{Time: data.UpdatedTimestamp, Valid: true},
		pgtype.Timestamptz{Time: data.TimestampWithZone, Valid: true},
		pgtype.Interval{Microseconds: data.DurationInterval, Valid: true},
		pgtype.Interval{Microseconds: data.WorkHours, Valid: true},
		data.FileData,
		data.RequiredBinary,
		pgtype.Inet{IPNet: data.IpAddress, Valid: true},
		pgtype.Inet{IPNet: data.IpNetwork, Valid: true},
		pgtype.Inet{IPNet: data.MacAddress, Valid: true},
		pgtype.Inet{IPNet: data.Mac8Address, Valid: true},
		data.PointLocation,
		data.LineSegment,
		data.RectangularBox,
		data.PathData,
		data.PolygonShape,
		data.CircleArea,
		pgtype.JSON{Bytes: data.JsonData, Valid: true},
		pgtype.JSONB{Bytes: data.JsonbData, Valid: true},
		pgtype.JSONB{Bytes: data.JsonbNotNull, Valid: true},
		pgtype.Array[int32]{Elements: data.IntegerArray, Valid: true},
		pgtype.Array[string]{Elements: data.TextArray, Valid: true},
		pgtype.Array[int32]{Elements: data.MultidimArray, Valid: true},
		data.IntRange,
		data.BigintRange,
		data.NumericRange,
	)
	row, err := db.New().InsertComprehensiveExample(ctx, dbtx, params)
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

	currentRow, err := db.New().QueryComprehensiveExampleByID(ctx, dbtx, data.ID)
	if err != nil {
		return ComprehensiveExample{}, err
	}

	params := db.NewUpdateComprehensiveExampleParams(
		data.ID,
		func() uuid.UUID {
			if data.UuidID != uuid.Nil {
				return data.UuidID
			}
			return currentRow.UuidID
		}(),
		func() int16 {
			if true {
				return data.SmallInt
			}
			return currentRow.SmallInt
		}(),
		func() pgtype.Int4 {
			if true {
				return pgtype.Int4{Int32: data.RegularInt, Valid: true}
			}
			return currentRow.RegularInt
		}(),
		func() int64 {
			if true {
				return data.BigInt
			}
			return currentRow.BigInt
		}(),
		func() pgtype.Numeric {
			if true {
				return data.DecimalPrecise
			}
			return currentRow.DecimalPrecise
		}(),
		func() pgtype.Numeric {
			if true {
				return data.NumericField
			}
			return currentRow.NumericField
		}(),
		func() pgtype.Float4 {
			if true {
				return pgtype.Float4{Float32: data.RealFloat, Valid: true}
			}
			return currentRow.RealFloat
		}(),
		func() float64 {
			if true {
				return data.DoubleFloat
			}
			return currentRow.DoubleFloat
		}(),
		func() int16 {
			if true {
				return data.SmallSerial
			}
			return currentRow.SmallSerial
		}(),
		func() pgtype.Int8 {
			if true {
				return pgtype.Int8{Int64: data.BigSerial, Valid: true}
			}
			return currentRow.BigSerial
		}(),
		func() pgtype.Text {
			if true {
				return pgtype.Text{String: data.FixedChar, Valid: true}
			}
			return currentRow.FixedChar
		}(),
		func() string {
			if true {
				return data.VariableChar
			}
			return currentRow.VariableChar
		}(),
		func() pgtype.Text {
			if true {
				return pgtype.Text{String: data.UnlimitedText, Valid: true}
			}
			return currentRow.UnlimitedText
		}(),
		func() pgtype.Text {
			if true {
				return pgtype.Text{String: data.TextWithDefault, Valid: true}
			}
			return currentRow.TextWithDefault
		}(),
		func() string {
			if true {
				return data.TextNotNull
			}
			return currentRow.TextNotNull
		}(),
		func() pgtype.Bool {
			if true {
				return pgtype.Bool{Bool: data.IsActive, Valid: true}
			}
			return currentRow.IsActive
		}(),
		func() bool {
			if true {
				return data.IsVerified
			}
			return currentRow.IsVerified
		}(),
		func() pgtype.Bool {
			if true {
				return pgtype.Bool{Bool: data.NullableFlag, Valid: true}
			}
			return currentRow.NullableFlag
		}(),
		func() pgtype.Date {
			if true {
				return pgtype.Date{Time: data.CreatedDate, Valid: true}
			}
			return currentRow.CreatedDate
		}(),
		func() pgtype.Date {
			if true {
				return pgtype.Date{Time: data.BirthDate, Valid: true}
			}
			return currentRow.BirthDate
		}(),
		func() pgtype.Time {
			if true {
				return pgtype.Time{Time: data.ExactTime, Valid: true}
			}
			return currentRow.ExactTime
		}(),
		func() pgtype.Timetz {
			if true {
				return pgtype.Timetz{Time: data.TimeWithZone, Valid: true}
			}
			return currentRow.TimeWithZone
		}(),
		func() pgtype.Timestamp {
			if true {
				return pgtype.Timestamp{Time: data.CreatedTimestamp, Valid: true}
			}
			return currentRow.CreatedTimestamp
		}(),
		func() pgtype.Timestamp {
			if true {
				return pgtype.Timestamp{Time: data.UpdatedTimestamp, Valid: true}
			}
			return currentRow.UpdatedTimestamp
		}(),
		func() pgtype.Timestamptz {
			if true {
				return pgtype.Timestamptz{Time: data.TimestampWithZone, Valid: true}
			}
			return currentRow.TimestampWithZone
		}(),
		func() pgtype.Interval {
			if true {
				return pgtype.Interval{Microseconds: data.DurationInterval, Valid: true}
			}
			return currentRow.DurationInterval
		}(),
		func() pgtype.Interval {
			if true {
				return pgtype.Interval{Microseconds: data.WorkHours, Valid: true}
			}
			return currentRow.WorkHours
		}(),
		func() pgtype.Bytea {
			if true {
				return data.FileData
			}
			return currentRow.FileData
		}(),
		func() []byte {
			if true {
				return data.RequiredBinary
			}
			return currentRow.RequiredBinary
		}(),
		func() pgtype.Inet {
			if true {
				return pgtype.Inet{IPNet: data.IpAddress, Valid: true}
			}
			return currentRow.IpAddress
		}(),
		func() pgtype.CIDR {
			if true {
				return pgtype.Inet{IPNet: data.IpNetwork, Valid: true}
			}
			return currentRow.IpNetwork
		}(),
		func() pgtype.Macaddr {
			if true {
				return pgtype.Inet{IPNet: data.MacAddress, Valid: true}
			}
			return currentRow.MacAddress
		}(),
		func() pgtype.Macaddr8 {
			if true {
				return pgtype.Inet{IPNet: data.Mac8Address, Valid: true}
			}
			return currentRow.Mac8Address
		}(),
		func() pgtype.Point {
			if true {
				return data.PointLocation
			}
			return currentRow.PointLocation
		}(),
		func() pgtype.Lseg {
			if true {
				return data.LineSegment
			}
			return currentRow.LineSegment
		}(),
		func() pgtype.Box {
			if true {
				return data.RectangularBox
			}
			return currentRow.RectangularBox
		}(),
		func() pgtype.Path {
			if true {
				return data.PathData
			}
			return currentRow.PathData
		}(),
		func() pgtype.Polygon {
			if true {
				return data.PolygonShape
			}
			return currentRow.PolygonShape
		}(),
		func() pgtype.Circle {
			if true {
				return data.CircleArea
			}
			return currentRow.CircleArea
		}(),
		func() pgtype.JSON {
			if true {
				return pgtype.JSON{Bytes: data.JsonData, Valid: true}
			}
			return currentRow.JsonData
		}(),
		func() pgtype.JSONB {
			if true {
				return pgtype.JSONB{Bytes: data.JsonbData, Valid: true}
			}
			return currentRow.JsonbData
		}(),
		func() pgtype.JSONB {
			if true {
				return pgtype.JSONB{Bytes: data.JsonbNotNull, Valid: true}
			}
			return currentRow.JsonbNotNull
		}(),
		func() pgtype.Array[int32] {
			if true {
				return pgtype.Array[int32]{Elements: data.IntegerArray, Valid: true}
			}
			return currentRow.IntegerArray
		}(),
		func() pgtype.Array[string] {
			if true {
				return pgtype.Array[string]{Elements: data.TextArray, Valid: true}
			}
			return currentRow.TextArray
		}(),
		func() pgtype.Array[int32] {
			if true {
				return pgtype.Array[int32]{Elements: data.MultidimArray, Valid: true}
			}
			return currentRow.MultidimArray
		}(),
		func() pgtype.Int4range {
			if true {
				return data.IntRange
			}
			return currentRow.IntRange
		}(),
		func() pgtype.Int8range {
			if true {
				return data.BigintRange
			}
			return currentRow.BigintRange
		}(),
		func() pgtype.Numrange {
			if true {
				return data.NumericRange
			}
			return currentRow.NumericRange
		}(),
	)

	row, err := db.New().UpdateComprehensiveExample(ctx, dbtx, params)
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
	return db.New().DeleteComprehensiveExample(ctx, dbtx, id)
}

func AllComprehensiveExamples(
	ctx context.Context,
	dbtx db.DBTX,
) ([]ComprehensiveExample, error) {
	rows, err := db.New().QueryAllComprehensiveExamples(ctx, dbtx)
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

	totalCount, err := db.New().CountComprehensiveExamples(ctx, dbtx)
	if err != nil {
		return PaginatedComprehensiveExamples{}, err
	}

	rows, err := db.New().QueryPaginatedComprehensiveExamples(
		ctx,
		dbtx,
		db.NewQueryPaginatedComprehensiveExamplesParams(pageSize, offset),
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
