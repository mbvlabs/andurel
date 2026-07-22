package types

import (
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

func TestMapSQLTypeToGo_NonNullableTypes(t *testing.T) {
	tests := []struct {
		name        string
		sqlType     string
		expectedGo  string
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
		{"jsonb", "jsonb", "json.RawMessage", "encoding/json"},
		{"json", "json", "json.RawMessage", "encoding/json"},
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

func TestFormatFieldNameUsesMechanicalSchemaCasing(t *testing.T) {
	tests := map[string]string{
		"id":                       "ID",
		"url":                      "Url",
		"cidr":                     "Cidr",
		"server_ssh_credential_id": "ServerSshCredentialId",
		"wireguard_peer_id":        "WireguardPeerId",
	}
	for input, want := range tests {
		if got := FormatFieldName(input); got != want {
			t.Fatalf("FormatFieldName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMapSQLTypeToGo_NullableTypes_Pointer(t *testing.T) {
	tests := []struct {
		name        string
		sqlType     string
		expectedGo  string
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
	tm.NullType = "pointer"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goType, pkg, err := tm.MapSQLTypeToGo(tt.sqlType, true)
			if err != nil {
				t.Fatalf("MapSQLTypeToGo(%s, true) error = %v", tt.sqlType, err)
			}
			if goType != tt.expectedGo {
				t.Errorf("MapSQLTypeToGo(%s, true) goType = %s, want %s", tt.sqlType, goType, tt.expectedGo)
			}
			if pkg != tt.expectedPkg {
				t.Errorf("MapSQLTypeToGo(%s, true) package = %s, want %s", tt.sqlType, pkg, tt.expectedPkg)
			}
		})
	}
}

func TestMapSQLTypeToGo_NullableTypes_SqlNull(t *testing.T) {
	tests := []struct {
		name        string
		sqlType     string
		expectedGo  string
		expectedPkg string
	}{
		{"varchar nullable", "varchar", "sql.NullString", ""},
		{"text nullable", "text", "sql.NullString", ""},
		{"boolean nullable", "boolean", "sql.NullBool", ""},
		{"smallint nullable", "smallint", "sql.NullInt16", ""},
		{"integer nullable", "integer", "sql.NullInt32", ""},
		{"bigint nullable", "bigint", "sql.NullInt64", ""},
		{"decimal nullable", "decimal", "sql.NullFloat64", ""},
		{"numeric nullable", "numeric", "sql.NullFloat64", ""},
		{"timestamp nullable", "timestamp", "sql.NullTime", "time"},
		{"timestamptz nullable", "timestamptz", "sql.NullTime", "time"},
		{"uuid nullable", "uuid", "*uuid.UUID", "github.com/google/uuid"},
	}

	tm := NewTypeMapper("postgresql")
	tm.NullType = "sql.Null"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goType, pkg, err := tm.MapSQLTypeToGo(tt.sqlType, true)
			if err != nil {
				t.Fatalf("MapSQLTypeToGo(%s, true) error = %v", tt.sqlType, err)
			}
			if goType != tt.expectedGo {
				t.Errorf("MapSQLTypeToGo(%s, true) goType = %s, want %s", tt.sqlType, goType, tt.expectedGo)
			}
			if pkg != tt.expectedPkg {
				t.Errorf("MapSQLTypeToGo(%s, true) package = %s, want %s", tt.sqlType, pkg, tt.expectedPkg)
			}
		})
	}
}

func TestMapSQLTypeToGo_NullableTypes_BunNull(t *testing.T) {
	tests := []struct {
		name        string
		sqlType     string
		expectedGo  string
		expectedPkg string
	}{
		{"varchar nullable", "varchar", "bun.NullString", ""},
		{"text nullable", "text", "bun.NullString", ""},
		{"boolean nullable", "boolean", "bun.NullBool", ""},
		{"integer nullable", "integer", "bun.NullInt32", ""},
		{"bigint nullable", "bigint", "bun.NullInt64", ""},
		{"decimal nullable", "decimal", "bun.NullFloat64", ""},
		{"numeric nullable", "numeric", "bun.NullFloat64", ""},
		{"timestamp nullable", "timestamp", "bun.NullTime", "time"},
		{"timestamptz nullable", "timestamptz", "bun.NullTime", "time"},
		{"uuid nullable", "uuid", "*uuid.UUID", "github.com/google/uuid"},
	}

	tm := NewTypeMapper("postgresql")
	tm.NullType = "bun.Null"

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goType, pkg, err := tm.MapSQLTypeToGo(tt.sqlType, true)
			if err != nil {
				t.Fatalf("MapSQLTypeToGo(%s, true) error = %v", tt.sqlType, err)
			}
			if goType != tt.expectedGo {
				t.Errorf("MapSQLTypeToGo(%s, true) goType = %s, want %s", tt.sqlType, goType, tt.expectedGo)
			}
			if pkg != tt.expectedPkg {
				t.Errorf("MapSQLTypeToGo(%s, true) package = %s, want %s", tt.sqlType, pkg, tt.expectedPkg)
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
				Name:       "email",
				DataType:   "varchar",
				IsNullable: true,
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
		{
			name: "text array column",
			col: &catalog.Column{
				Name:     "tags",
				DataType: "text[]",
			},
			expected: "tags,array",
		},
		{
			name: "integer array column",
			col: &catalog.Column{
				Name:     "scores",
				DataType: "integer[]",
			},
			expected: "scores,array",
		},
		{
			name: "array column with primary key",
			col: &catalog.Column{
				Name:         "id",
				DataType:     "uuid",
				IsPrimaryKey: true,
			},
			expected: "id,pk,type:uuid",
		},
		{
			name: "non-array jsonb column",
			col: &catalog.Column{
				Name:     "settings",
				DataType: "jsonb",
			},
			expected: "settings,type:jsonb",
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
