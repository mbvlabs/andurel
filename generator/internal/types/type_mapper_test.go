package types

import (
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

func TestMapSQLTypeToGo_NonNullableTypes(t *testing.T) {
	tests := []struct {
		name       string
		sqlType    string
		expectedGo string
		expectedPkg string
	}{
		{"varchar", "varchar", "string", ""},
		{"text", "text", "string", ""},
		{"char", "char", "string", ""},

		{"uuid", "uuid", "uuid.UUID", "github.com/google/uuid"},

		{"boolean", "boolean", "bool", ""},
		{"bool", "bool", "bool", ""},

		{"integer", "integer", "int32", ""},
		{"int4", "int4", "int32", ""},
		{"serial", "serial", "int32", ""},
		{"bigint", "bigint", "int64", ""},
		{"int8", "int8", "int64", ""},
		{"bigserial", "bigserial", "int64", ""},
		{"smallint", "smallint", "int16", ""},
		{"int2", "int2", "int16", ""},
		{"smallserial", "smallserial", "int16", ""},

		{"real", "real", "float32", ""},
		{"float4", "float4", "float32", ""},
		{"double precision", "double precision", "float64", ""},
		{"float8", "float8", "float64", ""},
		{"decimal", "decimal", "float64", ""},
		{"numeric", "numeric", "float64", ""},

		{"timestamp", "timestamp", "time.Time", "time"},
		{"timestamp without time zone", "timestamp without time zone", "time.Time", "time"},
		{"timestamptz", "timestamptz", "time.Time", "time"},
		{"timestamp with time zone", "timestamp with time zone", "time.Time", "time"},
		{"date", "date", "time.Time", "time"},
		{"time", "time", "time.Time", "time"},

		{"bytea", "bytea", "[]byte", ""},
		{"jsonb", "jsonb", "[]byte", ""},
		{"json", "json", "[]byte", ""},
	}

	tm := NewTypeMapper("postgresql")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goType, pkg, err := tm.MapSQLTypeToGo(tt.sqlType, false)
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
		name       string
		sqlType    string
		expectedGo string
		expectedPkg string
	}{
		{"varchar nullable", "varchar", "*string", ""},
		{"text nullable", "text", "*string", ""},
		{"boolean nullable", "boolean", "*bool", ""},
		{"integer nullable", "integer", "*int32", ""},
		{"bigint nullable", "bigint", "*int64", ""},
		{"decimal nullable", "decimal", "*float64", ""},
		{"numeric nullable", "numeric", "*float64", ""},
		{"timestamp nullable", "timestamp", "*time.Time", "time"},
		{"timestamptz nullable", "timestamptz", "*time.Time", "time"},
		{"uuid nullable", "uuid", "*uuid.UUID", "github.com/google/uuid"},
	}

	tm := NewTypeMapper("postgresql")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goType, pkg, err := tm.MapSQLTypeToGo(tt.sqlType, true)
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

func TestBuildBunTag(t *testing.T) {
	tm := NewTypeMapper("postgresql")

	tests := []struct {
		name     string
		col      *catalog.Column
		expected string
	}{
		{
			name: "primary key uuid column",
			col: &catalog.Column{
				Name:         "id",
				DataType:     "uuid",
				IsPrimaryKey: true,
			},
			expected: "id,pk,type:uuid",
		},
		{
			name: "nullable column",
			col: &catalog.Column{
				Name:        "email",
				DataType:    "varchar",
				IsNullable:  true,
			},
			expected: "email",
		},
		{
			name: "notnull unique column",
			col: &catalog.Column{
				Name:     "email",
				DataType: "varchar",
				IsUnique: true,
			},
			expected: "email",
		},
		{
			name: "created_at timestamp",
			col: &catalog.Column{
				Name:     "created_at",
				DataType: "timestamp",
			},
			expected: "created_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.BuildBunTag(tt.col)
			if result != tt.expected {
				t.Errorf("BuildBunTag() = %s, want %s", result, tt.expected)
			}
		})
	}
}
