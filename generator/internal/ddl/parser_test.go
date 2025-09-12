package ddl

import (
	"testing"
)

func TestValidatePrimaryKeyDatatype(t *testing.T) {
	testCases := []struct {
		name         string
		dataType     string
		databaseType string
		expectError  bool
	}{
		// PostgreSQL valid cases
		{"postgresql_uuid_valid", "UUID", "postgresql", false},
		{"postgresql_uuid_lowercase", "uuid", "postgresql", false},

		// PostgreSQL invalid cases
		{"postgresql_text_invalid", "TEXT", "postgresql", true},
		{"postgresql_integer_invalid", "INTEGER", "postgresql", true},
		{"postgresql_varchar_invalid", "VARCHAR", "postgresql", true},

		// SQLite valid cases
		{"sqlite_text_valid", "TEXT", "sqlite", false},
		{"sqlite_text_lowercase", "text", "sqlite", false},

		// SQLite invalid cases
		{"sqlite_uuid_invalid", "UUID", "sqlite", true},
		{"sqlite_integer_invalid", "INTEGER", "sqlite", true},
		{"sqlite_varchar_invalid", "VARCHAR", "sqlite", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePrimaryKeyDatatype(tc.dataType, tc.databaseType, "test.sql", "id")
			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidatePrimaryKeyDatatype_ErrorMessages(t *testing.T) {
	testCases := []struct {
		name           string
		dataType       string
		databaseType   string
		columnName     string
		migrationFile  string
		expectedSubstr string
	}{
		{
			name:           "postgresql_text_error_message",
			dataType:       "TEXT",
			databaseType:   "postgresql",
			columnName:     "id",
			migrationFile:  "/path/to/001_create_users.sql",
			expectedSubstr: "PostgreSQL primary keys must use 'uuid'",
		},
		{
			name:           "sqlite_uuid_error_message",
			dataType:       "UUID",
			databaseType:   "sqlite",
			columnName:     "user_id",
			migrationFile:  "/path/to/002_create_posts.sql",
			expectedSubstr: "SQLite primary keys must use 'text'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePrimaryKeyDatatype(tc.dataType, tc.databaseType, tc.migrationFile, tc.columnName)
			if err == nil {
				t.Fatal("Expected error but got none")
			}

			errorMsg := err.Error()
			if !containsString(errorMsg, tc.expectedSubstr) {
				t.Errorf("Expected error message to contain '%s', but got: %s", tc.expectedSubstr, errorMsg)
			}

			// Check that the error message contains the column name
			if !containsString(errorMsg, tc.columnName) {
				t.Errorf("Expected error message to contain column name '%s', but got: %s", tc.columnName, errorMsg)
			}

			// Check that the error message contains the migration file basename
			if !containsString(errorMsg, "001_create_users.sql") && !containsString(errorMsg, "002_create_posts.sql") {
				t.Errorf("Expected error message to contain migration file name, but got: %s", errorMsg)
			}
		})
	}
}

func TestValidatePrimaryKeyDatatype_UnsupportedDatabase(t *testing.T) {
	// For unsupported database types, validation should pass (no error)
	err := validatePrimaryKeyDatatype("INTEGER", "mysql", "test.sql", "id")
	if err != nil {
		t.Errorf("Expected no error for unsupported database type, but got: %v", err)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestParseColumnDefinitions_PrimaryKeyValidation(t *testing.T) {
	testCases := []struct {
		name         string
		columnDefs   string
		databaseType string
		expectError  bool
		errorSubstr  string
	}{
		{
			name:         "postgresql_valid_uuid_primary_key",
			columnDefs:   "id UUID PRIMARY KEY, name TEXT NOT NULL",
			databaseType: "postgresql",
			expectError:  false,
		},
		{
			name:         "postgresql_invalid_text_primary_key",
			columnDefs:   "id TEXT PRIMARY KEY, name TEXT NOT NULL",
			databaseType: "postgresql",
			expectError:  true,
			errorSubstr:  "PostgreSQL primary keys must use 'uuid'",
		},
		{
			name:         "sqlite_valid_text_primary_key",
			columnDefs:   "id TEXT PRIMARY KEY, name TEXT NOT NULL",
			databaseType: "sqlite",
			expectError:  false,
		},
		{
			name:         "sqlite_invalid_uuid_primary_key",
			columnDefs:   "id UUID PRIMARY KEY, name TEXT NOT NULL",
			databaseType: "sqlite",
			expectError:  true,
			errorSubstr:  "SQLite primary keys must use 'text'",
		},
		{
			name:         "postgresql_separate_primary_key_constraint_valid",
			columnDefs:   "id UUID NOT NULL, name TEXT, PRIMARY KEY (id)",
			databaseType: "postgresql",
			expectError:  false,
		},
		{
			name:         "postgresql_separate_primary_key_constraint_invalid",
			columnDefs:   "id INTEGER NOT NULL, name TEXT, PRIMARY KEY (id)",
			databaseType: "postgresql",
			expectError:  true,
			errorSubstr:  "PostgreSQL primary keys must use 'uuid'",
		},
		{
			name:         "sqlite_separate_primary_key_constraint_valid",
			columnDefs:   "id TEXT NOT NULL, name TEXT, PRIMARY KEY (id)",
			databaseType: "sqlite",
			expectError:  false,
		},
		{
			name:         "sqlite_separate_primary_key_constraint_invalid",
			columnDefs:   "id INTEGER NOT NULL, name TEXT, PRIMARY KEY (id)",
			databaseType: "sqlite",
			expectError:  true,
			errorSubstr:  "SQLite primary keys must use 'text'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			columns, err := parseColumnDefinitions(tc.columnDefs, "test.sql", tc.databaseType)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tc.errorSubstr != "" && !containsSubstring(err.Error(), tc.errorSubstr) {
					t.Errorf("Expected error to contain '%s', but got: %s", tc.errorSubstr, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error but got: %v", err)
				}

				// Verify that we got the expected columns
				if len(columns) < 2 {
					t.Errorf("Expected at least 2 columns, got %d", len(columns))
				}

				// Find the primary key column and verify it's marked correctly
				var foundPK bool
				for _, col := range columns {
					if col.IsPrimaryKey {
						foundPK = true
						if col.Name != "id" {
							t.Errorf("Expected primary key column to be 'id', got '%s'", col.Name)
						}
					}
				}

				if !foundPK {
					t.Error("Expected to find a primary key column but didn't")
				}
			}
		})
	}
}

func TestParseCreateTable_PrimaryKeyValidation(t *testing.T) {
	testCases := []struct {
		name         string
		sql          string
		databaseType string
		expectError  bool
		errorSubstr  string
	}{
		{
			name:         "postgresql_valid_create_table",
			sql:          "CREATE TABLE users (id UUID PRIMARY KEY, email TEXT NOT NULL)",
			databaseType: "postgresql",
			expectError:  false,
		},
		{
			name:         "postgresql_invalid_create_table",
			sql:          "CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL)",
			databaseType: "postgresql",
			expectError:  true,
			errorSubstr:  "PostgreSQL primary keys must use 'uuid'",
		},
		{
			name:         "sqlite_valid_create_table",
			sql:          "CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL)",
			databaseType: "sqlite",
			expectError:  false,
		},
		{
			name:         "sqlite_invalid_create_table",
			sql:          "CREATE TABLE users (id UUID PRIMARY KEY, email TEXT NOT NULL)",
			databaseType: "sqlite",
			expectError:  true,
			errorSubstr:  "SQLite primary keys must use 'text'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stmt, err := parseCreateTable(tc.sql, "test.sql", tc.databaseType)

			if tc.expectError {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tc.errorSubstr != "" && !containsSubstring(err.Error(), tc.errorSubstr) {
					t.Errorf("Expected error to contain '%s', but got: %s", tc.errorSubstr, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Expected no error but got: %v", err)
				}

				if stmt == nil {
					t.Fatal("Expected statement but got nil")
				}

				if stmt.Type != CreateTable {
					t.Errorf("Expected CREATE TABLE statement type, got %v", stmt.Type)
				}
			}
		})
	}
}
