package catalog

import (
	"testing"
)

func TestColumn_ValidatePrimaryKeyDatatype(t *testing.T) {
	testCases := []struct {
		name          string
		column        *Column
		databaseType  string
		migrationFile string
		expectError   bool
		errorSubstr   string
	}{
		{
			name: "non_primary_key_column",
			column: &Column{
				Name:         "email",
				DataType:     "text",
				IsPrimaryKey: false,
			},
			databaseType:  "postgresql",
			migrationFile: "test.sql",
			expectError:   false,
		},
		{
			name: "postgresql_valid_primary_key",
			column: &Column{
				Name:         "id",
				DataType:     "uuid",
				IsPrimaryKey: true,
			},
			databaseType:  "postgresql",
			migrationFile: "test.sql",
			expectError:   false,
		},
		{
			name: "postgresql_invalid_primary_key",
			column: &Column{
				Name:         "id",
				DataType:     "text",
				IsPrimaryKey: true,
			},
			databaseType:  "postgresql",
			migrationFile: "test.sql",
			expectError:   true,
			errorSubstr:   "PostgreSQL primary keys must use 'uuid'",
		},
		{
			name: "sqlite_valid_primary_key",
			column: &Column{
				Name:         "id",
				DataType:     "text",
				IsPrimaryKey: true,
			},
			databaseType:  "sqlite",
			migrationFile: "test.sql",
			expectError:   false,
		},
		{
			name: "sqlite_invalid_primary_key",
			column: &Column{
				Name:         "id",
				DataType:     "uuid",
				IsPrimaryKey: true,
			},
			databaseType:  "sqlite",
			migrationFile: "test.sql",
			expectError:   true,
			errorSubstr:   "SQLite primary keys must use 'text'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.column.ValidatePrimaryKeyDatatype(tc.databaseType, tc.migrationFile)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tc.errorSubstr != "" && !containsSubstring(err.Error(), tc.errorSubstr) {
					t.Errorf(
						"Expected error to contain '%s', but got: %s",
						tc.errorSubstr,
						err.Error(),
					)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestColumn_SetPrimaryKey(t *testing.T) {
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

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
