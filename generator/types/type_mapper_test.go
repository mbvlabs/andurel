package types

import (
	"strings"
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
		// String types
		{"varchar", "varchar", "string", "string", ""},
		{"text", "text", "string", "string", ""},
		{"char", "char", "string", "string", ""},
		
		// UUID
		{"uuid", "uuid", "uuid.UUID", "uuid.UUID", "github.com/google/uuid"},
		
		// Boolean
		{"boolean", "boolean", "bool", "bool", ""},
		{"bool", "bool", "bool", "bool", ""},
		
		// Integer types
		{"integer", "integer", "int32", "int32", ""},
		{"int", "int", "int32", "int32", ""},
		{"int4", "int4", "int32", "int32", ""},
		{"serial", "serial", "int32", "int32", ""},
		{"bigint", "bigint", "int64", "int64", ""},
		{"int8", "int8", "int64", "int64", ""},
		{"bigserial", "bigserial", "int64", "int64", ""},
		{"smallint", "smallint", "int16", "int16", ""},
		{"int2", "int2", "int16", "int16", ""},
		{"smallserial", "smallserial", "int16", "int16", ""},
		
		// Float types
		{"real", "real", "float32", "float32", ""},
		{"float4", "float4", "float32", "float32", ""},
		{"double precision", "double precision", "float64", "float64", ""},
		{"float8", "float8", "float64", "float64", ""},
		{"decimal", "decimal", "float64", "float64", ""},
		{"numeric", "numeric", "float64", "float64", ""},
		
		// Time types (non-nullable should still use pgtype for proper handling)
		{"timestamp", "timestamp", "time.Time", "pgtype.Timestamp", "time"},
		{"timestamp without time zone", "timestamp without time zone", "time.Time", "pgtype.Timestamp", "time"},
		{"timestamptz", "timestamptz", "time.Time", "pgtype.Timestamptz", "time"},
		{"timestamp with time zone", "timestamp with time zone", "time.Time", "pgtype.Timestamptz", "time"},
		{"date", "date", "time.Time", "pgtype.Timestamptz", "time"},
		{"time", "time", "time.Time", "pgtype.Timestamptz", "time"},
		
		// Binary and JSON
		{"bytea", "bytea", "[]byte", "[]byte", ""},
		{"jsonb", "jsonb", "interface{}", "interface{}", ""},
		{"json", "json", "interface{}", "interface{}", ""},
	}

	tm := NewTypeMapper("postgresql")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goType, sqlcType, pkg, err := tm.MapSQLTypeToGo(tt.sqlType, false)
			if err != nil {
				t.Fatalf("MapSQLTypeToGo(%s, false) error = %v", tt.sqlType, err)
			}

			if goType != tt.expectedGo {
				t.Errorf("MapSQLTypeToGo(%s, false) goType = %s, want %s", tt.sqlType, goType, tt.expectedGo)
			}

			if sqlcType != tt.expectedSQLC {
				t.Errorf("MapSQLTypeToGo(%s, false) sqlcType = %s, want %s", tt.sqlType, sqlcType, tt.expectedSQLC)
			}

			if pkg != tt.expectedPkg {
				t.Errorf("MapSQLTypeToGo(%s, false) package = %s, want %s", tt.sqlType, pkg, tt.expectedPkg)
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
		// String types -> sql.NullString
		{"varchar nullable", "varchar", "string", "sql.NullString", ""},
		{"text nullable", "text", "string", "sql.NullString", ""},
		{"char nullable", "char", "string", "sql.NullString", ""},
		
		// Boolean -> sql.NullBool
		{"boolean nullable", "boolean", "bool", "sql.NullBool", ""},
		{"bool nullable", "bool", "bool", "sql.NullBool", ""},
		
		// Integer types -> sql.NullInt32/NullInt64
		{"integer nullable", "integer", "int32", "sql.NullInt32", ""},
		{"int nullable", "int", "int32", "sql.NullInt32", ""},
		{"int4 nullable", "int4", "int32", "sql.NullInt32", ""},
		{"serial nullable", "serial", "int32", "sql.NullInt32", ""},
		{"bigint nullable", "bigint", "int64", "sql.NullInt64", ""},
		{"int8 nullable", "int8", "int64", "sql.NullInt64", ""},
		{"bigserial nullable", "bigserial", "int64", "sql.NullInt64", ""},
		
		// Float types -> sql.NullFloat64 or pgtype.Numeric
		{"real nullable", "real", "float32", "sql.NullFloat64", ""},
		{"float4 nullable", "float4", "float32", "sql.NullFloat64", ""},
		{"double precision nullable", "double precision", "float64", "sql.NullFloat64", ""},
		{"float8 nullable", "float8", "float64", "sql.NullFloat64", ""},
		{"decimal nullable", "decimal", "float64", "pgtype.Numeric", ""},
		{"numeric nullable", "numeric", "float64", "pgtype.Numeric", ""},
		
		// Time types -> pgtype variants
		{"timestamp nullable", "timestamp", "time.Time", "pgtype.Timestamp", "time"},
		{"timestamp without time zone nullable", "timestamp without time zone", "time.Time", "pgtype.Timestamp", "time"},
		{"timestamptz nullable", "timestamptz", "time.Time", "pgtype.Timestamptz", "time"},
		{"timestamp with time zone nullable", "timestamp with time zone", "time.Time", "pgtype.Timestamptz", "time"},
		{"date nullable", "date", "time.Time", "pgtype.Timestamptz", "time"},
		{"time nullable", "time", "time.Time", "pgtype.Timestamptz", "time"},
		
		// UUID should remain unchanged when nullable
		{"uuid nullable", "uuid", "uuid.UUID", "uuid.UUID", "github.com/google/uuid"},
		
		// Binary and JSON should remain unchanged when nullable  
		{"bytea nullable", "bytea", "[]byte", "[]byte", ""},
		{"jsonb nullable", "jsonb", "interface{}", "interface{}", ""},
		{"json nullable", "json", "interface{}", "interface{}", ""},
	}

	tm := NewTypeMapper("postgresql")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goType, sqlcType, pkg, err := tm.MapSQLTypeToGo(tt.sqlType, true)
			if err != nil {
				t.Fatalf("MapSQLTypeToGo(%s, true) error = %v", tt.sqlType, err)
			}

			if goType != tt.expectedGo {
				t.Errorf("MapSQLTypeToGo(%s, true) goType = %s, want %s", tt.sqlType, goType, tt.expectedGo)
			}

			if sqlcType != tt.expectedSQLC {
				t.Errorf("MapSQLTypeToGo(%s, true) sqlcType = %s, want %s", tt.sqlType, sqlcType, tt.expectedSQLC)
			}

			if pkg != tt.expectedPkg {
				t.Errorf("MapSQLTypeToGo(%s, true) package = %s, want %s", tt.sqlType, pkg, tt.expectedPkg)
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
		// Direct mappings (no conversion needed)
		{"string direct", "Name", "string", "string", "row.Name"},
		{"bool direct", "IsActive", "bool", "bool", "row.IsActive"},
		{"int16 direct", "SmallAge", "int16", "int16", "row.SmallAge"},
		{"int32 direct", "Age", "int32", "int32", "row.Age"},
		{"int64 direct", "BigAge", "int64", "int64", "row.BigAge"},
		{"float32 direct", "SmallPrice", "float32", "float32", "row.SmallPrice"},
		{"float64 direct", "Price", "float64", "float64", "row.Price"},
		{"uuid direct", "ID", "uuid.UUID", "uuid.UUID", "row.ID"},
		
		// sql.Null types
		{"sql.NullString", "Description", "sql.NullString", "string", "row.Description.String"},
		{"sql.NullBool", "IsActive", "sql.NullBool", "bool", "row.IsActive.Bool"},
		{"sql.NullInt32 for int16", "SmallAge", "sql.NullInt32", "int16", "row.SmallAge.Int32"},
		{"sql.NullInt32", "Age", "sql.NullInt32", "int32", "row.Age.Int32"},
		{"sql.NullInt64", "Count", "sql.NullInt64", "int64", "row.Count.Int64"},
		{"sql.NullFloat64 for float32", "SmallPrice", "sql.NullFloat64", "float32", "row.SmallPrice.Float64"},
		{"sql.NullFloat64", "Price", "sql.NullFloat64", "float64", "row.Price.Float64"},
		
		// pgtype types
		{"pgtype.Timestamptz", "CreatedAt", "pgtype.Timestamptz", "time.Time", "row.CreatedAt.Time"},
		{"pgtype.Timestamp", "UpdatedAt", "pgtype.Timestamp", "time.Time", "row.UpdatedAt.Time"},
		{"pgtype.Numeric", "Amount", "pgtype.Numeric", "float64", "func() float64 { if row.Amount.Valid { f, _ := row.Amount.Float64Value(); return f.Float64 }; return 0 }()"},
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
		// Direct mappings (no conversion needed)
		{"string direct", "string", "string", "data.Name", "data.Name"},
		{"bool direct", "bool", "bool", "data.IsActive", "data.IsActive"},
		{"int16 direct", "int16", "int16", "data.SmallAge", "data.SmallAge"},
		{"int32 direct", "int32", "int32", "data.Age", "data.Age"},
		{"int64 direct", "int64", "int64", "data.BigAge", "data.BigAge"},
		{"float32 direct", "float32", "float32", "data.SmallPrice", "data.SmallPrice"},
		{"float64 direct", "float64", "float64", "data.Price", "data.Price"},
		{"uuid direct", "uuid.UUID", "uuid.UUID", "data.ID", "data.ID"},
		
		// sql.Null types
		{"sql.NullString", "sql.NullString", "string", "data.Description", "sql.NullString{String: data.Description, Valid: data.Description != \"\"}"},
		{"sql.NullBool", "sql.NullBool", "bool", "data.IsActive", "sql.NullBool{Bool: data.IsActive, Valid: true}"},
		{"sql.NullInt32 for int16", "sql.NullInt32", "int16", "data.SmallAge", "sql.NullInt32{Int32: data.SmallAge, Valid: true}"},
		{"sql.NullInt32", "sql.NullInt32", "int32", "data.Age", "sql.NullInt32{Int32: data.Age, Valid: true}"},
		{"sql.NullInt64", "sql.NullInt64", "int64", "data.Count", "sql.NullInt64{Int64: data.Count, Valid: true}"},
		{"sql.NullFloat64 for float32", "sql.NullFloat64", "float32", "data.SmallPrice", "sql.NullFloat64{Float64: data.SmallPrice, Valid: true}"},
		{"sql.NullFloat64", "sql.NullFloat64", "float64", "data.Price", "sql.NullFloat64{Float64: data.Price, Valid: true}"},
		
		// pgtype types
		{"pgtype.Timestamptz", "pgtype.Timestamptz", "time.Time", "data.CreatedAt", "pgtype.Timestamptz{Time: data.CreatedAt, Valid: true}"},
		{"pgtype.Timestamp", "pgtype.Timestamp", "time.Time", "data.UpdatedAt", "pgtype.Timestamp{Time: data.UpdatedAt, Valid: true}"},
		{"pgtype.Numeric", "pgtype.Numeric", "float64", "data.Amount", "func() pgtype.Numeric { var n pgtype.Numeric; _ = n.Scan(data.Amount); return n }()"},
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
		{"string", "string", "data.Name", "data.Name != \"\""},
		{"time.Time", "time.Time", "data.CreatedAt", "!data.CreatedAt.IsZero()"},
		{"bool", "bool", "data.IsActive", "true"},
		{"int16", "int16", "data.SmallAge", "data.SmallAge != 0"},
		{"int32", "int32", "data.Age", "data.Age != 0"},
		{"int64", "int64", "data.Count", "data.Count != 0"},
		{"float32", "float32", "data.SmallPrice", "data.SmallPrice != 0"},
		{"float64", "float64", "data.Amount", "data.Amount != 0"},
		{"uuid.UUID", "uuid.UUID", "data.ID", "data.ID != uuid.Nil"},
		{"[]byte", "[]byte", "data.Data", "len(data.Data) > 0"},
		{"interface{}", "interface{}", "data.Metadata", "true"},
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
	
	// Add custom override
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

	// Test that complex conversions generate valid Go code
	complexTests := []struct {
		name      string
		sqlcType  string
		goType    string
		valueExpr string
	}{
		{"pgtype.Numeric conversion", "pgtype.Numeric", "float64", "data.Price"},
		{"sql.NullString with validation", "sql.NullString", "string", "data.Description"},
	}

	for _, tt := range complexTests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.GenerateConversionToDB(tt.sqlcType, tt.goType, tt.valueExpr)
			
			// Should not be empty
			if result == "" {
				t.Errorf("GenerateConversionToDB returned empty string for %s", tt.name)
			}
			
			// Should contain the original value expression
			if !strings.Contains(result, tt.valueExpr) {
				t.Errorf("Generated conversion does not contain original value expression: %s", result)
			}
			
			// Should contain appropriate type construction
			if tt.sqlcType == "pgtype.Numeric" && !strings.Contains(result, "pgtype.Numeric") {
				t.Errorf("pgtype.Numeric conversion should contain type name: %s", result)
			}
			
			if strings.HasPrefix(tt.sqlcType, "sql.Null") && !strings.Contains(result, tt.sqlcType) {
				t.Errorf("sql.Null conversion should contain type name: %s", result)
			}
		})
	}
}