package types

import (
	"testing"
)

func TestSQLiteGenerateConversionFromDB(t *testing.T) {
	tm := NewTypeMapper("sqlite")
	
	tests := []struct {
		name        string
		fieldName   string
		sqlcType    string
		goType      string
		expected    string
	}{
		// SQLite sql.Null* types
		{"NullString conversion", "Name", "sql.NullString", "string", "row.Name.String"},
		{"NullInt64 conversion", "Age", "sql.NullInt64", "int64", "row.Age.Int64"},
		{"NullFloat64 conversion", "Price", "sql.NullFloat64", "float64", "row.Price.Float64"},
		{"NullBool conversion", "Active", "sql.NullBool", "bool", "row.Active.Bool"},
		{"NullTime conversion", "CreatedAt", "sql.NullTime", "time.Time", "row.CreatedAt.Time"},
		
		// Non-null SQLite types should return as-is
		{"string direct", "Title", "string", "string", "row.Title"},
		{"int64 direct", "Count", "int64", "int64", "row.Count"},
		{"float64 direct", "Rate", "float64", "float64", "row.Rate"},
		{"bool direct", "Enabled", "bool", "bool", "row.Enabled"},
		{"time direct", "UpdatedAt", "time.Time", "time.Time", "row.UpdatedAt"},
		{"bytes direct", "Data", "[]byte", "[]byte", "row.Data"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.GenerateConversionFromDB(tt.fieldName, tt.sqlcType, tt.goType)
			if result != tt.expected {
				t.Errorf("GenerateConversionFromDB(%s, %s, %s) = %s, want %s", 
					tt.fieldName, tt.sqlcType, tt.goType, result, tt.expected)
			}
		})
	}
}

func TestSQLiteGenerateConversionToDB(t *testing.T) {
	tm := NewTypeMapper("sqlite")
	
	tests := []struct {
		name      string
		sqlcType  string
		goType    string
		valueExpr string
		expected  string
	}{
		// SQLite sql.Null* types
		{"NullString to DB", "sql.NullString", "string", "entity.Name", "sql.NullString{String: entity.Name, Valid: true}"},
		{"NullInt64 to DB", "sql.NullInt64", "int64", "entity.Age", "sql.NullInt64{Int64: entity.Age, Valid: true}"},
		{"NullFloat64 to DB", "sql.NullFloat64", "float64", "entity.Price", "sql.NullFloat64{Float64: entity.Price, Valid: true}"},
		{"NullBool to DB", "sql.NullBool", "bool", "entity.Active", "sql.NullBool{Bool: entity.Active, Valid: true}"},
		{"NullTime to DB", "sql.NullTime", "time.Time", "entity.CreatedAt", "sql.NullTime{Time: entity.CreatedAt, Valid: true}"},
		
		// Non-null SQLite types should return as-is
		{"string direct", "string", "string", "entity.Title", "entity.Title"},
		{"int64 direct", "int64", "int64", "entity.Count", "entity.Count"},
		{"float64 direct", "float64", "float64", "entity.Rate", "entity.Rate"},
		{"bool direct", "bool", "bool", "entity.Enabled", "entity.Enabled"},
		{"time direct", "time.Time", "time.Time", "entity.UpdatedAt", "entity.UpdatedAt"},
		{"bytes direct", "[]byte", "[]byte", "entity.Data", "entity.Data"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.GenerateConversionToDB(tt.sqlcType, tt.goType, tt.valueExpr)
			if result != tt.expected {
				t.Errorf("GenerateConversionToDB(%s, %s, %s) = %s, want %s", 
					tt.sqlcType, tt.goType, tt.valueExpr, result, tt.expected)
			}
		})
	}
}

func TestSQLiteGenerateZeroCheck(t *testing.T) {
	tm := NewTypeMapper("sqlite")
	
	tests := []struct {
		name      string
		goType    string
		valueExpr string
		expected  string
	}{
		// SQLite sql.Null* types should check Valid field
		{"NullString zero check", "sql.NullString", "entity.Name", "entity.Name.Valid"},
		{"NullInt64 zero check", "sql.NullInt64", "entity.Age", "entity.Age.Valid"},
		{"NullFloat64 zero check", "sql.NullFloat64", "entity.Price", "entity.Price.Valid"},
		{"NullBool zero check", "sql.NullBool", "entity.Active", "entity.Active.Valid"},
		{"NullTime zero check", "sql.NullTime", "entity.CreatedAt", "entity.CreatedAt.Valid"},
		
		// Non-null SQLite types should return true
		{"string zero check", "string", "entity.Title", "true"},
		{"int64 zero check", "int64", "entity.Count", "true"},
		{"float64 zero check", "float64", "entity.Rate", "true"},
		{"bool zero check", "bool", "entity.Enabled", "true"},
		{"time zero check", "time.Time", "entity.UpdatedAt", "true"},
		{"bytes zero check", "[]byte", "entity.Data", "true"},
		
		// UUID should have special handling
		{"UUID zero check", "uuid.UUID", "entity.ID", "entity.ID != uuid.Nil"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.GenerateZeroCheck(tt.goType, tt.valueExpr)
			if result != tt.expected {
				t.Errorf("GenerateZeroCheck(%s, %s) = %s, want %s", 
					tt.goType, tt.valueExpr, result, tt.expected)
			}
		})
	}
}

func TestSQLiteConversionsIntegration(t *testing.T) {
	tm := NewTypeMapper("sqlite")
	
	// Test a complete workflow for a nullable SQLite field
	goType, sqlcType, pkg, err := tm.MapSQLTypeToGo("text", true)
	if err != nil {
		t.Fatalf("MapSQLTypeToGo failed: %v", err)
	}
	
	// Verify mapping
	if goType != "string" || sqlcType != "sql.NullString" || pkg != "database/sql" {
		t.Errorf("MapSQLTypeToGo(text, true) = (%s, %s, %s), want (string, sql.NullString, database/sql)", 
			goType, sqlcType, pkg)
	}
	
	// Test conversions using the mapped types
	fromDB := tm.GenerateConversionFromDB("Title", sqlcType, goType)
	expected := "row.Title.String"
	if fromDB != expected {
		t.Errorf("GenerateConversionFromDB with mapped types = %s, want %s", fromDB, expected)
	}
	
	toDB := tm.GenerateConversionToDB(sqlcType, goType, "entity.Title")
	expected = "sql.NullString{String: entity.Title, Valid: true}"
	if toDB != expected {
		t.Errorf("GenerateConversionToDB with mapped types = %s, want %s", toDB, expected)
	}
	
	zeroCheck := tm.GenerateZeroCheck(sqlcType, "entity.Title")
	expected = "entity.Title.Valid"
	if zeroCheck != expected {
		t.Errorf("GenerateZeroCheck with mapped types = %s, want %s", zeroCheck, expected)
	}
}