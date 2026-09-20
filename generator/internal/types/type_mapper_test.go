package types

import (
	"testing"
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

		{"uuid", "uuid", "uuid.UUID", "uuid"},

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
		{"decimal", "decimal", "pgtype.Numeric", "github.com/jackc/pgx/v5/pgtype"},
		{"numeric", "numeric", "pgtype.Numeric", "github.com/jackc/pgx/v5/pgtype"},

		{"timestamp", "timestamp", "pgtype.Timestamp", "github.com/jackc/pgx/v5/pgtype"},
		{"timestamp without time zone", "timestamp without time zone", "pgtype.Timestamp", "github.com/jackc/pgx/v5/pgtype"},
		{"timestamptz", "timestamptz", "pgtype.Timestamptz", "github.com/jackc/pgx/v5/pgtype"},
		{"timestamp with time zone", "timestamp with time zone", "pgtype.Timestamptz", "github.com/jackc/pgx/v5/pgtype"},
		{"date", "date", "pgtype.Date", "github.com/jackc/pgx/v5/pgtype"},
		{"time", "time", "pgtype.Time", "github.com/jackc/pgx/v5/pgtype"},

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

func TestMapSQLTypeToGo_NullableTypes(t *testing.T) {
	pgtypePkg := "github.com/jackc/pgx/v5/pgtype"
	tests := []struct {
		name        string
		sqlType     string
		expectedGo  string
		expectedPkg string
	}{
		{"varchar nullable", "varchar", "pgtype.Text", pgtypePkg},
		{"text nullable", "text", "pgtype.Text", pgtypePkg},
		{"boolean nullable", "boolean", "pgtype.Bool", pgtypePkg},
		{"smallint nullable", "smallint", "pgtype.Int2", pgtypePkg},
		{"integer nullable", "integer", "pgtype.Int4", pgtypePkg},
		{"bigint nullable", "bigint", "pgtype.Int8", pgtypePkg},
		{"decimal nullable", "decimal", "pgtype.Numeric", pgtypePkg},
		{"numeric nullable", "numeric", "pgtype.Numeric", pgtypePkg},
		{"timestamp nullable", "timestamp", "pgtype.Timestamp", pgtypePkg},
		{"timestamptz nullable", "timestamptz", "pgtype.Timestamptz", pgtypePkg},
		{"uuid nullable", "uuid", "*uuid.UUID", "uuid"},
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
