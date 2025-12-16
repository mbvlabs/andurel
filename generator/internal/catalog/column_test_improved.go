package catalog

import (
	"testing"

	testsuite "github.com/mbvlabs/andurel/pkg/testing"
)

// testError is a simple error for testing purposes
type testError struct{}

func (e *testError) Error() string {
	return "test error"
}

// TestColumnSuite provides organized tests for Column functionality
func TestColumnSuite(t *testing.T) {
	suite := testsuite.NewTestSuite()

	// Unit Tests
	suite.AddUnitTest(
		"ValidatePrimaryKeyDatatype",
		"Tests primary key datatype validation",
		testValidatePrimaryKeyDatatype,
	)
	suite.AddUnitTest("SetPrimaryKey", "Tests setting column as primary key", testSetPrimaryKey)
	suite.AddUnitTest("SetNotNull", "Tests setting column as not null", testSetNotNull)
	suite.AddUnitTest("SetUnique", "Tests setting column as unique", testSetUnique)
	suite.AddUnitTest("SetDefault", "Tests setting column default value", testSetDefault)
	suite.AddUnitTest("SetLength", "Tests setting column length", testSetLength)
	suite.AddUnitTest(
		"SetPrecisionScale",
		"Tests setting column precision and scale",
		testSetPrecisionScale,
	)
	suite.AddUnitTest("SetArray", "Tests setting column as array", testSetArray)
	suite.AddUnitTest("Clone", "Tests column cloning functionality", testClone)

	// Run all unit tests
	suite.RunUnitTests(t)
}

// testValidatePrimaryKeyDatatype tests primary key datatype validation
func testValidatePrimaryKeyDatatype(t *testing.T) {
	tests := []testsuite.TestData{
		{
			Name: "non_primary_key_column",
			Input: struct {
				column        *Column
				databaseType  string
				migrationFile string
			}{
				column: &Column{
					Name:         "email",
					DataType:     "text",
					IsPrimaryKey: false,
				},
				databaseType:  "postgresql",
				migrationFile: "test.sql",
			},
			Error: nil,
		},
		{
			Name: "postgresql_valid_primary_key",
			Input: struct {
				column        *Column
				databaseType  string
				migrationFile string
			}{
				column: &Column{
					Name:         "id",
					DataType:     "uuid",
					IsPrimaryKey: true,
				},
				databaseType:  "postgresql",
				migrationFile: "test.sql",
			},
			Error: nil,
		},
		{
			Name: "postgresql_invalid_primary_key",
			Input: struct {
				column        *Column
				databaseType  string
				migrationFile string
			}{
				column: &Column{
					Name:         "id",
					DataType:     "text",
					IsPrimaryKey: true,
				},
				databaseType:  "postgresql",
				migrationFile: "test.sql",
			},
			Error: &testError{}, // We'll check for error existence
		},
	}

	tableTest := testsuite.NewTableDrivenTest(
		"ValidatePrimaryKeyDatatype",
		tests,
		func(t *testing.T, test testsuite.TestData) {
			input := test.Input.(struct {
				column        *Column
				databaseType  string
				migrationFile string
			})

			err := input.column.ValidatePrimaryKeyDatatype(input.databaseType, input.migrationFile)

			if test.Error != nil {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				// Additional error message validation could be added here
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		},
	)

	tableTest.Run(t)
}

// testSetPrimaryKey tests setting column as primary key
func testSetPrimaryKey(t *testing.T) {
	col := NewColumn("id", "uuid")
	col.IsNullable = true // should be set to false when setting as primary key

	result := col.SetPrimaryKey()

	if result != col {
		t.Error("SetPrimaryKey should return the same column instance")
	}

	if !col.IsPrimaryKey {
		t.Error("Column should be marked as primary key")
	}

	if col.IsNullable {
		t.Error("Primary key column should not be nullable")
	}
}

// testSetNotNull tests setting column as not null
func testSetNotNull(t *testing.T) {
	col := NewColumn("name", "text")
	col.IsNullable = true

	result := col.SetNotNull()

	if result != col {
		t.Error("SetNotNull should return the same column instance")
	}

	if col.IsNullable {
		t.Error("Column should not be nullable")
	}
}

// testSetUnique tests setting column as unique
func testSetUnique(t *testing.T) {
	col := NewColumn("email", "text")
	col.IsUnique = false

	result := col.SetUnique()

	if result != col {
		t.Error("SetUnique should return the same column instance")
	}

	if !col.IsUnique {
		t.Error("Column should be marked as unique")
	}
}

// testSetDefault tests setting column default value
func testSetDefault(t *testing.T) {
	col := NewColumn("status", "text")
	defaultValue := "active"

	result := col.SetDefault(defaultValue)

	if result != col {
		t.Error("SetDefault should return the same column instance")
	}

	if col.DefaultVal == nil {
		t.Error("Column should have a default value")
	}

	if *col.DefaultVal != defaultValue {
		t.Errorf("Expected default value '%s', got '%s'", defaultValue, *col.DefaultVal)
	}
}

// testSetLength tests setting column length
func testSetLength(t *testing.T) {
	col := NewColumn("name", "varchar")
	length := int32(255)

	result := col.SetLength(length)

	if result != col {
		t.Error("SetLength should return the same column instance")
	}

	if col.Length == nil {
		t.Error("Column should have a length")
	}

	if *col.Length != length {
		t.Errorf("Expected length %d, got %d", length, *col.Length)
	}
}

// testSetPrecisionScale tests setting column precision and scale
func testSetPrecisionScale(t *testing.T) {
	col := NewColumn("price", "decimal")
	precision := int32(10)
	scale := int32(2)

	result := col.SetPrecisionScale(precision, scale)

	if result != col {
		t.Error("SetPrecisionScale should return the same column instance")
	}

	if col.Precision == nil {
		t.Error("Column should have precision")
	}

	if *col.Precision != precision {
		t.Errorf("Expected precision %d, got %d", precision, *col.Precision)
	}

	if col.Scale == nil {
		t.Error("Column should have scale")
	}

	if *col.Scale != scale {
		t.Errorf("Expected scale %d, got %d", scale, *col.Scale)
	}
}

// testSetArray tests setting column as array
func testSetArray(t *testing.T) {
	col := NewColumn("tags", "text")
	col.IsArray = false

	result := col.SetArray()

	if result != col {
		t.Error("SetArray should return the same column instance")
	}

	if !col.IsArray {
		t.Error("Column should be marked as array")
	}
}

// testClone tests column cloning functionality
func testClone(t *testing.T) {
	original := NewColumn("email", "varchar")
	original.IsNullable = false
	original.IsArray = true
	original.IsPrimaryKey = true
	original.IsUnique = true
	length := int32(255)
	original.Length = &length
	defaultValue := "test@example.com"
	original.DefaultVal = &defaultValue

	clone := original.Clone()

	// Verify it's a different object
	if clone == original {
		t.Error("Clone should return a different object")
	}

	// Verify all fields are copied correctly
	if clone.Name != original.Name {
		t.Errorf("Expected name '%s', got '%s'", original.Name, clone.Name)
	}

	if clone.DataType != original.DataType {
		t.Errorf("Expected datatype '%s', got '%s'", original.DataType, clone.DataType)
	}

	if clone.IsNullable != original.IsNullable {
		t.Errorf("Expected nullable %v, got %v", original.IsNullable, clone.IsNullable)
	}

	if clone.IsArray != original.IsArray {
		t.Errorf("Expected array %v, got %v", original.IsArray, clone.IsArray)
	}

	if clone.IsPrimaryKey != original.IsPrimaryKey {
		t.Errorf("Expected primary key %v, got %v", original.IsPrimaryKey, clone.IsPrimaryKey)
	}

	if clone.IsUnique != original.IsUnique {
		t.Errorf("Expected unique %v, got %v", original.IsUnique, clone.IsUnique)
	}

	if clone.Length == nil || *clone.Length != *original.Length {
		t.Errorf("Expected length %v, got %v", original.Length, clone.Length)
	}

	if clone.DefaultVal == nil || *clone.DefaultVal != *original.DefaultVal {
		t.Errorf("Expected default value %v, got %v", original.DefaultVal, clone.DefaultVal)
	}

	// Verify modifying clone doesn't affect original
	clone.Name = "modified"
	if original.Name == "modified" {
		t.Error("Modifying clone should not affect original")
	}
}
