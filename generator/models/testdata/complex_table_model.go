package models

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/example/complex/models/internal/db"
)

type ComprehensiveExample struct {
	ID                int64
	UuidId            uuid.UUID
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

type CreateComprehensiveExamplePayload struct {
	UuidId            uuid.UUID
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
	data CreateComprehensiveExamplePayload,
) (ComprehensiveExample, error) {
	if err := validate.Struct(data); err != nil {
		return ComprehensiveExample{}, errors.Join(ErrDomainValidation, err)
	}

	row, err := db.New().InsertComprehensiveExample(ctx, dbtx, db.InsertComprehensiveExampleParams{
		ID:                uuid.New(),
		UuidId:            data.UuidId,
		SmallInt:          data.SmallInt,
		RegularInt:        pgtype.Int4{Int32: data.RegularInt, Valid: true},
		BigInt:            data.BigInt,
		DecimalPrecise:    data.DecimalPrecise,
		NumericField:      data.NumericField,
		RealFloat:         pgtype.Float4{Float32: data.RealFloat, Valid: true},
		DoubleFloat:       data.DoubleFloat,
		SmallSerial:       data.SmallSerial,
		BigSerial:         pgtype.Int8{Int64: data.BigSerial, Valid: true},
		FixedChar:         pgtype.Text{String: data.FixedChar, Valid: true},
		VariableChar:      data.VariableChar,
		UnlimitedText:     pgtype.Text{String: data.UnlimitedText, Valid: true},
		TextWithDefault:   pgtype.Text{String: data.TextWithDefault, Valid: true},
		TextNotNull:       data.TextNotNull,
		IsActive:          pgtype.Bool{Bool: data.IsActive, Valid: true},
		IsVerified:        data.IsVerified,
		NullableFlag:      pgtype.Bool{Bool: data.NullableFlag, Valid: true},
		CreatedDate:       pgtype.Date{Time: data.CreatedDate, Valid: true},
		BirthDate:         pgtype.Date{Time: data.BirthDate, Valid: true},
		ExactTime:         pgtype.Time{Time: data.ExactTime, Valid: true},
		TimeWithZone:      pgtype.Timetz{Time: data.TimeWithZone, Valid: true},
		CreatedTimestamp:  pgtype.Timestamp{Time: data.CreatedTimestamp, Valid: true},
		UpdatedTimestamp:  pgtype.Timestamp{Time: data.UpdatedTimestamp, Valid: true},
		TimestampWithZone: pgtype.Timestamptz{Time: data.TimestampWithZone, Valid: true},
		DurationInterval:  pgtype.Interval{Microseconds: data.DurationInterval, Valid: true},
		WorkHours:         pgtype.Interval{Microseconds: data.WorkHours, Valid: true},
		FileData:          data.FileData,
		RequiredBinary:    data.RequiredBinary,
		IpAddress:         pgtype.Inet{IPNet: data.IpAddress, Valid: true},
		IpNetwork:         pgtype.Inet{IPNet: data.IpNetwork, Valid: true},
		MacAddress:        pgtype.Inet{IPNet: data.MacAddress, Valid: true},
		Mac8Address:       pgtype.Inet{IPNet: data.Mac8Address, Valid: true},
		PointLocation:     data.PointLocation,
		LineSegment:       data.LineSegment,
		RectangularBox:    data.RectangularBox,
		PathData:          data.PathData,
		PolygonShape:      data.PolygonShape,
		CircleArea:        data.CircleArea,
		JsonData:          pgtype.JSON{Bytes: data.JsonData, Valid: true},
		JsonbData:         pgtype.JSONB{Bytes: data.JsonbData, Valid: true},
		JsonbNotNull:      pgtype.JSONB{Bytes: data.JsonbNotNull, Valid: true},
		IntegerArray:      pgtype.Array[int32]{Elements: data.IntegerArray, Valid: true},
		TextArray:         pgtype.Array[string]{Elements: data.TextArray, Valid: true},
		MultidimArray:     pgtype.Array[int32]{Elements: data.MultidimArray, Valid: true},
		IntRange:          data.IntRange,
		BigintRange:       data.BigintRange,
		NumericRange:      data.NumericRange,
	})
	if err != nil {
		return ComprehensiveExample{}, err
	}

	return rowToComprehensiveExample(row), nil
}

type UpdateComprehensiveExamplePayload struct {
	ID                uuid.UUID
	UuidId            uuid.UUID
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
	data UpdateComprehensiveExamplePayload,
) (ComprehensiveExample, error) {
	if err := validate.Struct(data); err != nil {
		return ComprehensiveExample{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := db.New().QueryComprehensiveExampleByID(ctx, dbtx, data.ID)
	if err != nil {
		return ComprehensiveExample{}, err
	}

	payload := db.UpdateComprehensiveExampleParams{
		ID:                data.ID,
		UuidId:            currentRow.UuidId,
		SmallInt:          currentRow.SmallInt,
		RegularInt:        currentRow.RegularInt,
		BigInt:            currentRow.BigInt,
		DecimalPrecise:    currentRow.DecimalPrecise,
		NumericField:      currentRow.NumericField,
		RealFloat:         currentRow.RealFloat,
		DoubleFloat:       currentRow.DoubleFloat,
		SmallSerial:       currentRow.SmallSerial,
		BigSerial:         currentRow.BigSerial,
		FixedChar:         currentRow.FixedChar,
		VariableChar:      currentRow.VariableChar,
		UnlimitedText:     currentRow.UnlimitedText,
		TextWithDefault:   currentRow.TextWithDefault,
		TextNotNull:       currentRow.TextNotNull,
		IsActive:          currentRow.IsActive,
		IsVerified:        currentRow.IsVerified,
		NullableFlag:      currentRow.NullableFlag,
		CreatedDate:       currentRow.CreatedDate,
		BirthDate:         currentRow.BirthDate,
		ExactTime:         currentRow.ExactTime,
		TimeWithZone:      currentRow.TimeWithZone,
		CreatedTimestamp:  currentRow.CreatedTimestamp,
		UpdatedTimestamp:  currentRow.UpdatedTimestamp,
		TimestampWithZone: currentRow.TimestampWithZone,
		DurationInterval:  currentRow.DurationInterval,
		WorkHours:         currentRow.WorkHours,
		FileData:          currentRow.FileData,
		RequiredBinary:    currentRow.RequiredBinary,
		IpAddress:         currentRow.IpAddress,
		IpNetwork:         currentRow.IpNetwork,
		MacAddress:        currentRow.MacAddress,
		Mac8Address:       currentRow.Mac8Address,
		PointLocation:     currentRow.PointLocation,
		LineSegment:       currentRow.LineSegment,
		RectangularBox:    currentRow.RectangularBox,
		PathData:          currentRow.PathData,
		PolygonShape:      currentRow.PolygonShape,
		CircleArea:        currentRow.CircleArea,
		JsonData:          currentRow.JsonData,
		JsonbData:         currentRow.JsonbData,
		JsonbNotNull:      currentRow.JsonbNotNull,
		IntegerArray:      currentRow.IntegerArray,
		TextArray:         currentRow.TextArray,
		MultidimArray:     currentRow.MultidimArray,
		IntRange:          currentRow.IntRange,
		BigintRange:       currentRow.BigintRange,
		NumericRange:      currentRow.NumericRange,
	}
	if data.UuidId != uuid.Nil {
		payload.UuidId = data.UuidId
	}
	if true {
		payload.SmallInt = data.SmallInt
	}
	if true {
		payload.RegularInt = pgtype.Int4{Int32: data.RegularInt, Valid: true}
	}
	if true {
		payload.BigInt = data.BigInt
	}
	if true {
		payload.DecimalPrecise = data.DecimalPrecise
	}
	if true {
		payload.NumericField = data.NumericField
	}
	if true {
		payload.RealFloat = pgtype.Float4{Float32: data.RealFloat, Valid: true}
	}
	if true {
		payload.DoubleFloat = data.DoubleFloat
	}
	if true {
		payload.SmallSerial = data.SmallSerial
	}
	if true {
		payload.BigSerial = pgtype.Int8{Int64: data.BigSerial, Valid: true}
	}
	if true {
		payload.FixedChar = pgtype.Text{String: data.FixedChar, Valid: true}
	}
	if true {
		payload.VariableChar = data.VariableChar
	}
	if true {
		payload.UnlimitedText = pgtype.Text{String: data.UnlimitedText, Valid: true}
	}
	if true {
		payload.TextWithDefault = pgtype.Text{String: data.TextWithDefault, Valid: true}
	}
	if true {
		payload.TextNotNull = data.TextNotNull
	}
	if true {
		payload.IsActive = pgtype.Bool{Bool: data.IsActive, Valid: true}
	}
	if true {
		payload.IsVerified = data.IsVerified
	}
	if true {
		payload.NullableFlag = pgtype.Bool{Bool: data.NullableFlag, Valid: true}
	}
	if true {
		payload.CreatedDate = pgtype.Date{Time: data.CreatedDate, Valid: true}
	}
	if true {
		payload.BirthDate = pgtype.Date{Time: data.BirthDate, Valid: true}
	}
	if true {
		payload.ExactTime = pgtype.Time{Time: data.ExactTime, Valid: true}
	}
	if true {
		payload.TimeWithZone = pgtype.Timetz{Time: data.TimeWithZone, Valid: true}
	}
	if true {
		payload.CreatedTimestamp = pgtype.Timestamp{Time: data.CreatedTimestamp, Valid: true}
	}
	if true {
		payload.UpdatedTimestamp = pgtype.Timestamp{Time: data.UpdatedTimestamp, Valid: true}
	}
	if true {
		payload.TimestampWithZone = pgtype.Timestamptz{Time: data.TimestampWithZone, Valid: true}
	}
	if true {
		payload.DurationInterval = pgtype.Interval{Microseconds: data.DurationInterval, Valid: true}
	}
	if true {
		payload.WorkHours = pgtype.Interval{Microseconds: data.WorkHours, Valid: true}
	}
	if true {
		payload.FileData = data.FileData
	}
	if true {
		payload.RequiredBinary = data.RequiredBinary
	}
	if true {
		payload.IpAddress = pgtype.Inet{IPNet: data.IpAddress, Valid: true}
	}
	if true {
		payload.IpNetwork = pgtype.Inet{IPNet: data.IpNetwork, Valid: true}
	}
	if true {
		payload.MacAddress = pgtype.Inet{IPNet: data.MacAddress, Valid: true}
	}
	if true {
		payload.Mac8Address = pgtype.Inet{IPNet: data.Mac8Address, Valid: true}
	}
	if true {
		payload.PointLocation = data.PointLocation
	}
	if true {
		payload.LineSegment = data.LineSegment
	}
	if true {
		payload.RectangularBox = data.RectangularBox
	}
	if true {
		payload.PathData = data.PathData
	}
	if true {
		payload.PolygonShape = data.PolygonShape
	}
	if true {
		payload.CircleArea = data.CircleArea
	}
	if true {
		payload.JsonData = pgtype.JSON{Bytes: data.JsonData, Valid: true}
	}
	if true {
		payload.JsonbData = pgtype.JSONB{Bytes: data.JsonbData, Valid: true}
	}
	if true {
		payload.JsonbNotNull = pgtype.JSONB{Bytes: data.JsonbNotNull, Valid: true}
	}
	if true {
		payload.IntegerArray = pgtype.Array[int32]{Elements: data.IntegerArray, Valid: true}
	}
	if true {
		payload.TextArray = pgtype.Array[string]{Elements: data.TextArray, Valid: true}
	}
	if true {
		payload.MultidimArray = pgtype.Array[int32]{Elements: data.MultidimArray, Valid: true}
	}
	if true {
		payload.IntRange = data.IntRange
	}
	if true {
		payload.BigintRange = data.BigintRange
	}
	if true {
		payload.NumericRange = data.NumericRange
	}

	row, err := db.New().UpdateComprehensiveExample(ctx, dbtx, payload)
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
		db.QueryPaginatedComprehensiveExamplesParams{
			Limit:  pageSize,
			Offset: offset,
		},
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
		UuidId:            row.UuidId,
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
