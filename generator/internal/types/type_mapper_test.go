package types

import (
	"testing"
)

func TestMapSQLTypeToGo_NonNullableTypes(t *testing.T) {
	tests := []struct {
		name         string
		sqlType      string
		expectedGo   string
		expectedSQLC string
		expectedPkg  string
	}{
		{"varchar", "varchar", "string", "string", ""},
		{"text", "text", "string", "string", ""},
		{"char", "char", "string", "string", ""},

		{"uuid", "uuid", "uuid.UUID", "uuid.UUID", "github.com/google/uuid"},

		{"boolean", "boolean", "bool", "bool", ""},
		{"bool", "bool", "bool", "bool", ""},

		{"integer", "integer", "int32", "int32", ""},
		{"int", "int", "int32", "int32", ""},
		{"int4", "int4", "int32", "int32", ""},
		{"serial", "serial", "int32", "int32", ""},
		{"bigint", "bigint", "int64", "int64", ""},
		{"int8", "int8", "int64", "int64", ""},
		{"bigserial", "bigserial", "int64", "int64", ""},
		{"smallint", "smallint", "int16", "int16", ""},
		{"int2", "int2", "int16", "int16", ""},
		{
			"smallserial",
			"smallserial",
			"int16",
			"int16",
			"",
		},

		{"real", "real", "float32", "float32", ""},
		{"float4", "float4", "float32", "float32", ""},
		{
			"double precision",
			"double precision",
			"float64",
			"float64",
			"",
		},
		{"float8", "float8", "float64", "float64", ""},
		{
			"decimal",
			"decimal",
			"float64",
			"pgtype.Numeric",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"numeric",
			"numeric",
			"float64",
			"pgtype.Numeric",
			"github.com/jackc/pgx/v5/pgtype",
		},

		{
			"timestamp",
			"timestamp",
			"time.Time",
			"pgtype.Timestamp",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"timestamp without time zone",
			"timestamp without time zone",
			"time.Time",
			"pgtype.Timestamp",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"timestamptz",
			"timestamptz",
			"time.Time",
			"pgtype.Timestamptz",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"timestamp with time zone",
			"timestamp with time zone",
			"time.Time",
			"pgtype.Timestamptz",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{"date", "date", "time.Time", "pgtype.Date", "github.com/jackc/pgx/v5/pgtype"},
		{"time", "time", "time.Time", "pgtype.Time", "github.com/jackc/pgx/v5/pgtype"},

		{"bytea", "bytea", "[]byte", "[]byte", ""},
		{"jsonb", "jsonb", "[]byte", "pgtype.JSONB", "github.com/jackc/pgx/v5/pgtype"},
		{"json", "json", "[]byte", "pgtype.JSON", "github.com/jackc/pgx/v5/pgtype"},
	}

	tm := NewTypeMapper("postgresql")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goType, sqlcType, pkg, err := tm.MapSQLTypeToGo(tt.sqlType, false)
			if err != nil {
				t.Fatalf("MapSQLTypeToGo(%s, false) error = %v", tt.sqlType, err)
			}

			if goType != tt.expectedGo {
				t.Errorf(
					"MapSQLTypeToGo(%s, false) goType = %s, want %s",
					tt.sqlType,
					goType,
					tt.expectedGo,
				)
			}

			if sqlcType != tt.expectedSQLC {
				t.Errorf(
					"MapSQLTypeToGo(%s, false) sqlcType = %s, want %s",
					tt.sqlType,
					sqlcType,
					tt.expectedSQLC,
				)
			}

			if pkg != tt.expectedPkg {
				t.Errorf(
					"MapSQLTypeToGo(%s, false) package = %s, want %s",
					tt.sqlType,
					pkg,
					tt.expectedPkg,
				)
			}
		})
	}
}

func TestMapSQLTypeToGo_NullableTypes(t *testing.T) {
	tests := []struct {
		name         string
		sqlType      string
		expectedGo   string
		expectedSQLC string
		expectedPkg  string
	}{
		{
			"varchar nullable",
			"varchar",
			"string",
			"pgtype.Text",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{"text nullable", "text", "string", "pgtype.Text", "github.com/jackc/pgx/v5/pgtype"},

		{
			"boolean nullable",
			"boolean",
			"bool",
			"pgtype.Bool",
			"github.com/jackc/pgx/v5/pgtype",
		},

		{
			"integer nullable",
			"integer",
			"int32",
			"pgtype.Int4",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"bigint nullable",
			"bigint",
			"int64",
			"pgtype.Int8",
			"github.com/jackc/pgx/v5/pgtype",
		},

		{
			"decimal nullable",
			"decimal",
			"float64",
			"pgtype.Numeric",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"numeric nullable",
			"numeric",
			"float64",
			"pgtype.Numeric",
			"github.com/jackc/pgx/v5/pgtype",
		},

		{
			"timestamp nullable",
			"timestamp",
			"time.Time",
			"pgtype.Timestamp",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"timestamptz nullable",
			"timestamptz",
			"time.Time",
			"pgtype.Timestamptz",
			"github.com/jackc/pgx/v5/pgtype",
		},

		{"uuid nullable", "uuid", "uuid.UUID", "pgtype.UUID", "github.com/jackc/pgx/v5/pgtype"},
	}

	tm := NewTypeMapper("postgresql")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goType, sqlcType, pkg, err := tm.MapSQLTypeToGo(tt.sqlType, true)
			if err != nil {
				t.Fatalf("MapSQLTypeToGo(%s, true) error = %v", tt.sqlType, err)
			}

			if goType != tt.expectedGo {
				t.Errorf(
					"MapSQLTypeToGo(%s, true) goType = %s, want %s",
					tt.sqlType,
					goType,
					tt.expectedGo,
				)
			}

			if sqlcType != tt.expectedSQLC {
				t.Errorf(
					"MapSQLTypeToGo(%s, true) sqlcType = %s, want %s",
					tt.sqlType,
					sqlcType,
					tt.expectedSQLC,
				)
			}

			if pkg != tt.expectedPkg {
				t.Errorf(
					"MapSQLTypeToGo(%s, true) package = %s, want %s",
					tt.sqlType,
					pkg,
					tt.expectedPkg,
				)
			}
		})
	}
}

func TestGenerateConversionFromDB(t *testing.T) {
	tests := []struct {
		name         string
		fieldName    string
		sqlcType     string
		goType       string
		expectedCode string
	}{
		{"pgtype.Text", "Name", "pgtype.Text", "pgtype.Text", "row.Name.String"},
		{"pgtype.Bool", "IsActive", "pgtype.Bool", "pgtype.Bool", "row.IsActive.Bool"},
		{"pgtype.Int2", "SmallAge", "pgtype.Int2", "pgtype.Int2", "row.SmallAge.Int16"},
		{"pgtype.Int4", "Age", "pgtype.Int4", "pgtype.Int4", "row.Age.Int32"},
		{"pgtype.Int8", "BigAge", "pgtype.Int8", "pgtype.Int8", "row.BigAge.Int64"},
		{"pgtype.Float4", "SmallPrice", "pgtype.Float4", "pgtype.Float4", "row.SmallPrice.Float32"},
		{"pgtype.Float8", "Price", "pgtype.Float8", "pgtype.Float8", "row.Price.Float64"},
		{"uuid direct", "ID", "uuid.UUID", "uuid.UUID", "row.ID"},
		{"pgtype.UUID", "RefID", "pgtype.UUID", "uuid.UUID", "uuid.UUID(row.RefID.Bytes)"},
		{
			"pgtype.Timestamptz",
			"CreatedAt",
			"pgtype.Timestamptz",
			"pgtype.Timestamptz",
			"row.CreatedAt.Time",
		},
		{
			"pgtype.Timestamp",
			"UpdatedAt",
			"pgtype.Timestamp",
			"pgtype.Timestamp",
			"row.UpdatedAt.Time",
		},
		{"pgtype.Numeric", "Amount", "pgtype.Numeric", "pgtype.Numeric", "row.Amount"},
		{"pgtype.JSONB", "Metadata", "pgtype.JSONB", "pgtype.JSONB", "row.Metadata"},
	}

	tm := NewTypeMapper("postgresql")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.GenerateConversionFromDB(tt.fieldName, tt.sqlcType, tt.goType)
			if result != tt.expectedCode {
				t.Errorf("GenerateConversionFromDB(%s, %s, %s) = %s, want %s",
					tt.fieldName, tt.sqlcType, tt.goType, result, tt.expectedCode)
			}
		})
	}
}

func TestGenerateConversionToDB(t *testing.T) {
	tests := []struct {
		name         string
		sqlcType     string
		goType       string
		valueExpr    string
		expectedCode string
	}{
		{
			"pgtype.Text",
			"pgtype.Text",
			"pgtype.Text",
			"data.Name",
			"pgtype.Text{String: data.Name, Valid: true}",
		},
		{
			"pgtype.Bool",
			"pgtype.Bool",
			"pgtype.Bool",
			"data.IsActive",
			"pgtype.Bool{Bool: data.IsActive, Valid: true}",
		},
		{
			"pgtype.Int2",
			"pgtype.Int2",
			"pgtype.Int2",
			"data.SmallAge",
			"pgtype.Int2{Int16: data.SmallAge, Valid: true}",
		},
		{
			"pgtype.Int4",
			"pgtype.Int4",
			"pgtype.Int4",
			"data.Age",
			"pgtype.Int4{Int32: data.Age, Valid: true}",
		},
		{
			"pgtype.Int8",
			"pgtype.Int8",
			"pgtype.Int8",
			"data.BigAge",
			"pgtype.Int8{Int64: data.BigAge, Valid: true}",
		},
		{
			"pgtype.Float4",
			"pgtype.Float4",
			"pgtype.Float4",
			"data.SmallPrice",
			"pgtype.Float4{Float32: data.SmallPrice, Valid: true}",
		},
		{
			"pgtype.Float8",
			"pgtype.Float8",
			"pgtype.Float8",
			"data.Price",
			"pgtype.Float8{Float64: data.Price, Valid: true}",
		},
		{"uuid direct", "uuid.UUID", "uuid.UUID", "data.ID", "data.ID"},
		{
			"pgtype.UUID",
			"pgtype.UUID",
			"uuid.UUID",
			"data.RefID",
			"pgtype.UUID{Bytes: data.RefID, Valid: true}",
		},
		{
			"pgtype.Timestamptz",
			"pgtype.Timestamptz",
			"pgtype.Timestamptz",
			"data.CreatedAt",
			"pgtype.Timestamptz{Time: data.CreatedAt, Valid: true}",
		},
		{
			"pgtype.Timestamp",
			"pgtype.Timestamp",
			"pgtype.Timestamp",
			"data.UpdatedAt",
			"pgtype.Timestamp{Time: data.UpdatedAt, Valid: true}",
		},
		{"pgtype.Numeric", "pgtype.Numeric", "pgtype.Numeric", "data.Amount", "data.Amount"},
		{
			"pgtype.JSONB",
			"pgtype.JSONB",
			"pgtype.JSONB",
			"data.Metadata",
			"data.Metadata",
		},
	}

	tm := NewTypeMapper("postgresql")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.GenerateConversionToDB(tt.sqlcType, tt.goType, tt.valueExpr)
			if result != tt.expectedCode {
				t.Errorf("GenerateConversionToDB(%s, %s, %s) = %s, want %s",
					tt.sqlcType, tt.goType, tt.valueExpr, result, tt.expectedCode)
			}
		})
	}
}

func TestGenerateZeroCheck(t *testing.T) {
	tests := []struct {
		name         string
		goType       string
		valueExpr    string
		expectedCode string
	}{
		{"pgtype.Text", "pgtype.Text", "data.Name", "data.Name.Valid"},
		{"pgtype.Timestamptz", "pgtype.Timestamptz", "data.CreatedAt", "data.CreatedAt.Valid"},
		{"pgtype.Bool", "pgtype.Bool", "data.IsActive", "data.IsActive.Valid"},
		{"pgtype.Int2", "pgtype.Int2", "data.SmallAge", "data.SmallAge.Valid"},
		{"pgtype.Int4", "pgtype.Int4", "data.Age", "data.Age.Valid"},
		{"pgtype.Int8", "pgtype.Int8", "data.Count", "data.Count.Valid"},
		{"pgtype.Float4", "pgtype.Float4", "data.SmallPrice", "data.SmallPrice.Valid"},
		{"pgtype.Float8", "pgtype.Float8", "data.Amount", "data.Amount.Valid"},
		{"pgtype.Numeric", "pgtype.Numeric", "data.Price", "data.Price.Valid"},
		{"pgtype.JSONB", "pgtype.JSONB", "data.Metadata", "data.Metadata.Valid"},
		{"uuid.UUID", "uuid.UUID", "data.ID", "data.ID != uuid.Nil"},
		{"interface{}", "interface{}", "data.Unknown", "true"},
	}

	tm := NewTypeMapper("postgresql")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.GenerateZeroCheck(tt.goType, tt.valueExpr)
			if result != tt.expectedCode {
				t.Errorf("GenerateZeroCheck(%s, %s) = %s, want %s",
					tt.goType, tt.valueExpr, result, tt.expectedCode)
			}
		})
	}
}

func TestTypeOverrides(t *testing.T) {
	tm := NewTypeMapper("postgresql")

	tm.Overrides = append(tm.Overrides, TypeOverride{
		DatabaseType: "text",
		GoType:       "CustomString",
		Package:      "myapp/types",
		Nullable:     false,
	})

	goType, sqlcType, pkg, err := tm.MapSQLTypeToGo("text", false)
	if err != nil {
		t.Fatalf("MapSQLTypeToGo with override failed: %v", err)
	}

	if goType != "CustomString" {
		t.Errorf("Override goType = %s, want CustomString", goType)
	}

	if pkg != "myapp/types" {
		t.Errorf("Override package = %s, want myapp/types", pkg)
	}

	if sqlcType != "" {
		t.Errorf("Override sqlcType = %s, want empty string", sqlcType)
	}
}

func TestSQLTypeNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"VARCHAR(255)", "varchar"},
		{"TEXT", "text"},
		{"DECIMAL(10,2)", "decimal"},
		{"TIMESTAMP WITH TIME ZONE", "timestamp with time zone"},
		{"int4", "integer"},
		{"int8", "bigint"},
		{"int2", "smallint"},
		{"float4", "real"},
		{"float8", "double precision"},
		{"bool", "boolean"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeSQLType(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeSQLType(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestUnknownTypes(t *testing.T) {
	tm := NewTypeMapper("postgresql")

	goType, sqlcType, pkg, err := tm.MapSQLTypeToGo("unknown_type", false)
	if err != nil {
		t.Fatalf("MapSQLTypeToGo with unknown type failed: %v", err)
	}

	if goType != "interface{}" {
		t.Errorf("Unknown type goType = %s, want interface{}", goType)
	}

	if sqlcType != "interface{}" {
		t.Errorf("Unknown type sqlcType = %s, want interface{}", sqlcType)
	}

	if pkg != "" {
		t.Errorf("Unknown type package = %s, want empty string", pkg)
	}
}

func TestComplexTypeConversions(t *testing.T) {
	tm := NewTypeMapper("postgresql")

	complexTests := []struct {
		name      string
		sqlcType  string
		goType    string
		valueExpr string
	}{
		{"pgtype.Numeric conversion", "pgtype.Numeric", "pgtype.Numeric", "data.Price"},
		{"pgtype.Text conversion", "pgtype.Text", "pgtype.Text", "data.Description"},
	}

	for _, tt := range complexTests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.GenerateConversionToDB(tt.sqlcType, tt.goType, tt.valueExpr)

			if result == "" {
				t.Errorf("GenerateConversionToDB returned empty string for %s", tt.name)
			}

			if tt.sqlcType == "pgtype.Text" {
				expected := "pgtype.Text{String: data.Description, Valid: true}"
				if result != expected {
					t.Errorf("Expected pgtype struct creation, got: %s, want: %s", result, expected)
				}
			} else {
				if result != tt.valueExpr {
					t.Errorf("Expected direct mapping, got: %s, want: %s", result, tt.valueExpr)
				}
			}
		})
	}
}
