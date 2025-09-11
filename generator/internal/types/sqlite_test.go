package types

import (
	"testing"
)

func TestSQLiteTypeMapper_NonNullableTypes(t *testing.T) {
	tests := []struct {
		name         string
		sqlType      string
		expectedGo   string
		expectedSQLC string
		expectedPkg  string
	}{
		{"text", "text", "string", "string", ""},
		{"varchar", "varchar", "string", "string", ""},
		{"char", "char", "string", "string", ""},
		{"clob", "clob", "string", "string", ""},

		{"integer", "integer", "int64", "int64", ""},
		{"int", "int", "int64", "int64", ""},
		{"tinyint", "tinyint", "int64", "int64", ""},
		{"smallint", "smallint", "int64", "int64", ""},
		{"bigint", "bigint", "int64", "int64", ""},

		{"real", "real", "float64", "float64", ""},
		{"double", "double", "float64", "float64", ""},
		{"float", "float", "float64", "float64", ""},
		{"double precision", "double precision", "float64", "float64", ""},

		{"boolean", "boolean", "bool", "bool", ""},
		{"date", "date", "time.Time", "time.Time", ""},
		{"datetime", "datetime", "time.Time", "time.Time", ""},
		{"numeric", "numeric", "float64", "float64", ""},
		{"decimal", "decimal", "float64", "float64", ""},

		{"blob", "blob", "[]byte", "[]byte", ""},
	}

	tm := NewTypeMapper("sqlite")

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

func TestSQLiteTypeMapper_NullableTypes(t *testing.T) {
	tests := []struct {
		name         string
		sqlType      string
		expectedGo   string
		expectedSQLC string
		expectedPkg  string
	}{
		{"text nullable", "text", "string", "sql.NullString", "database/sql"},
		{"varchar nullable", "varchar", "string", "sql.NullString", "database/sql"},

		{"integer nullable", "integer", "int64", "sql.NullInt64", "database/sql"},
		{"bigint nullable", "bigint", "int64", "sql.NullInt64", "database/sql"},

		{"real nullable", "real", "float64", "sql.NullFloat64", "database/sql"},
		{"double nullable", "double", "float64", "sql.NullFloat64", "database/sql"},

		{"boolean nullable", "boolean", "bool", "sql.NullBool", "database/sql"},
		{"date nullable", "date", "time.Time", "sql.NullTime", "database/sql"},
		{"datetime nullable", "datetime", "time.Time", "sql.NullTime", "database/sql"},
		{"numeric nullable", "numeric", "float64", "sql.NullFloat64", "database/sql"},

		{"blob", "blob", "[]byte", "[]byte", ""},
	}

	tm := NewTypeMapper("sqlite")

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

func TestSQLiteTypeNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"VARCHAR(255)", "varchar"},
		{"TEXT", "text"},
		{"INTEGER", "integer"},
		{"REAL", "real"},
		{"BLOB", "blob"},
		{"BOOLEAN", "boolean"},
		{"DATETIME", "datetime"},
		{"NATIVE CHARACTER", "char"},
		{"NVARCHAR", "varchar"},
		{"UNSIGNED BIG INT", "bigint"},
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

func TestSQLiteUnknownTypes(t *testing.T) {
	tm := NewTypeMapper("sqlite")

	goType, sqlcType, pkg, err := tm.MapSQLTypeToGo("unknown_sqlite_type", false)
	if err != nil {
		t.Fatalf("MapSQLTypeToGo with unknown SQLite type failed: %v", err)
	}

	if goType != "interface{}" {
		t.Errorf("Unknown SQLite type goType = %s, want interface{}", goType)
	}

	if sqlcType != "interface{}" {
		t.Errorf("Unknown SQLite type sqlcType = %s, want interface{}", sqlcType)
	}

	if pkg != "" {
		t.Errorf("Unknown SQLite type package = %s, want empty string", pkg)
	}
}
