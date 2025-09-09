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
		{"varchar", "varchar", "pgtype.Text", "pgtype.Text", "github.com/jackc/pgx/v5/pgtype"},
		{"text", "text", "pgtype.Text", "pgtype.Text", "github.com/jackc/pgx/v5/pgtype"},
		{"char", "char", "pgtype.Text", "pgtype.Text", "github.com/jackc/pgx/v5/pgtype"},

		{"uuid", "uuid", "uuid.UUID", "uuid.UUID", "github.com/google/uuid"},

		{"boolean", "boolean", "pgtype.Bool", "pgtype.Bool", "github.com/jackc/pgx/v5/pgtype"},
		{"bool", "bool", "pgtype.Bool", "pgtype.Bool", "github.com/jackc/pgx/v5/pgtype"},

		{"integer", "integer", "pgtype.Int4", "pgtype.Int4", "github.com/jackc/pgx/v5/pgtype"},
		{"int", "int", "pgtype.Int4", "pgtype.Int4", "github.com/jackc/pgx/v5/pgtype"},
		{"int4", "int4", "pgtype.Int4", "pgtype.Int4", "github.com/jackc/pgx/v5/pgtype"},
		{"serial", "serial", "pgtype.Int4", "pgtype.Int4", "github.com/jackc/pgx/v5/pgtype"},
		{"bigint", "bigint", "pgtype.Int8", "pgtype.Int8", "github.com/jackc/pgx/v5/pgtype"},
		{"int8", "int8", "pgtype.Int8", "pgtype.Int8", "github.com/jackc/pgx/v5/pgtype"},
		{"bigserial", "bigserial", "pgtype.Int8", "pgtype.Int8", "github.com/jackc/pgx/v5/pgtype"},
		{"smallint", "smallint", "pgtype.Int2", "pgtype.Int2", "github.com/jackc/pgx/v5/pgtype"},
		{"int2", "int2", "pgtype.Int2", "pgtype.Int2", "github.com/jackc/pgx/v5/pgtype"},
		{
			"smallserial",
			"smallserial",
			"pgtype.Int2",
			"pgtype.Int2",
			"github.com/jackc/pgx/v5/pgtype",
		},

		{"real", "real", "pgtype.Float4", "pgtype.Float4", "github.com/jackc/pgx/v5/pgtype"},
		{"float4", "float4", "pgtype.Float4", "pgtype.Float4", "github.com/jackc/pgx/v5/pgtype"},
		{
			"double precision",
			"double precision",
			"pgtype.Float8",
			"pgtype.Float8",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{"float8", "float8", "pgtype.Float8", "pgtype.Float8", "github.com/jackc/pgx/v5/pgtype"},
		{
			"decimal",
			"decimal",
			"pgtype.Numeric",
			"pgtype.Numeric",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"numeric",
			"numeric",
			"pgtype.Numeric",
			"pgtype.Numeric",
			"github.com/jackc/pgx/v5/pgtype",
		},

		{
			"timestamp",
			"timestamp",
			"pgtype.Timestamp",
			"pgtype.Timestamp",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"timestamp without time zone",
			"timestamp without time zone",
			"pgtype.Timestamp",
			"pgtype.Timestamp",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"timestamptz",
			"timestamptz",
			"pgtype.Timestamptz",
			"pgtype.Timestamptz",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"timestamp with time zone",
			"timestamp with time zone",
			"pgtype.Timestamptz",
			"pgtype.Timestamptz",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{"date", "date", "pgtype.Date", "pgtype.Date", "github.com/jackc/pgx/v5/pgtype"},
		{"time", "time", "pgtype.Time", "pgtype.Time", "github.com/jackc/pgx/v5/pgtype"},

		{"bytea", "bytea", "pgtype.Bytea", "pgtype.Bytea", "github.com/jackc/pgx/v5/pgtype"},
		{"jsonb", "jsonb", "pgtype.JSONB", "pgtype.JSONB", "github.com/jackc/pgx/v5/pgtype"},
		{"json", "json", "pgtype.JSON", "pgtype.JSON", "github.com/jackc/pgx/v5/pgtype"},
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
			"pgtype.Text",
			"pgtype.Text",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{"text nullable", "text", "pgtype.Text", "pgtype.Text", "github.com/jackc/pgx/v5/pgtype"},

		{
			"boolean nullable",
			"boolean",
			"pgtype.Bool",
			"pgtype.Bool",
			"github.com/jackc/pgx/v5/pgtype",
		},

		{
			"integer nullable",
			"integer",
			"pgtype.Int4",
			"pgtype.Int4",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"bigint nullable",
			"bigint",
			"pgtype.Int8",
			"pgtype.Int8",
			"github.com/jackc/pgx/v5/pgtype",
		},

		{
			"decimal nullable",
			"decimal",
			"pgtype.Numeric",
			"pgtype.Numeric",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"numeric nullable",
			"numeric",
			"pgtype.Numeric",
			"pgtype.Numeric",
			"github.com/jackc/pgx/v5/pgtype",
		},

		{
			"timestamp nullable",
			"timestamp",
			"pgtype.Timestamp",
			"pgtype.Timestamp",
			"github.com/jackc/pgx/v5/pgtype",
		},
		{
			"timestamptz nullable",
			"timestamptz",
			"pgtype.Timestamptz",
			"pgtype.Timestamptz",
			"github.com/jackc/pgx/v5/pgtype",
		},

		{"uuid nullable", "uuid", "uuid.UUID", "uuid.UUID", "github.com/google/uuid"},
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
		{"pgtype.Text", "Name", "pgtype.Text", "pgtype.Text", "row.Name"},
		{"pgtype.Bool", "IsActive", "pgtype.Bool", "pgtype.Bool", "row.IsActive"},
		{"pgtype.Int2", "SmallAge", "pgtype.Int2", "pgtype.Int2", "row.SmallAge"},
		{"pgtype.Int4", "Age", "pgtype.Int4", "pgtype.Int4", "row.Age"},
		{"pgtype.Int8", "BigAge", "pgtype.Int8", "pgtype.Int8", "row.BigAge"},
		{"pgtype.Float4", "SmallPrice", "pgtype.Float4", "pgtype.Float4", "row.SmallPrice"},
		{"pgtype.Float8", "Price", "pgtype.Float8", "pgtype.Float8", "row.Price"},
		{"uuid direct", "ID", "uuid.UUID", "uuid.UUID", "row.ID"},
		{
			"pgtype.Timestamptz",
			"CreatedAt",
			"pgtype.Timestamptz",
			"pgtype.Timestamptz",
			"row.CreatedAt",
		},
		{"pgtype.Timestamp", "UpdatedAt", "pgtype.Timestamp", "pgtype.Timestamp", "row.UpdatedAt"},
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
		{"pgtype.Text", "pgtype.Text", "pgtype.Text", "data.Name", "data.Name"},
		{"pgtype.Bool", "pgtype.Bool", "pgtype.Bool", "data.IsActive", "data.IsActive"},
		{"pgtype.Int2", "pgtype.Int2", "pgtype.Int2", "data.SmallAge", "data.SmallAge"},
		{"pgtype.Int4", "pgtype.Int4", "pgtype.Int4", "data.Age", "data.Age"},
		{"pgtype.Int8", "pgtype.Int8", "pgtype.Int8", "data.BigAge", "data.BigAge"},
		{"pgtype.Float4", "pgtype.Float4", "pgtype.Float4", "data.SmallPrice", "data.SmallPrice"},
		{"pgtype.Float8", "pgtype.Float8", "pgtype.Float8", "data.Price", "data.Price"},
		{"uuid direct", "uuid.UUID", "uuid.UUID", "data.ID", "data.ID"},
		{
			"pgtype.Timestamptz",
			"pgtype.Timestamptz",
			"pgtype.Timestamptz",
			"data.CreatedAt",
			"data.CreatedAt",
		},
		{
			"pgtype.Timestamp",
			"pgtype.Timestamp",
			"pgtype.Timestamp",
			"data.UpdatedAt",
			"data.UpdatedAt",
		},
		{"pgtype.Numeric", "pgtype.Numeric", "pgtype.Numeric", "data.Amount", "data.Amount"},
		{"pgtype.JSONB", "pgtype.JSONB", "pgtype.JSONB", "data.Metadata", "data.Metadata"},
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

			if result != tt.valueExpr {
				t.Errorf("Expected direct mapping, got: %s, want: %s", result, tt.valueExpr)
			}
		})
	}
}
