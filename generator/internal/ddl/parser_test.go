package ddl

import (
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/generator/internal/validation"
)

func TestStripComments(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no comments",
			input:    "SELECT * FROM users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "single line comment at end",
			input:    "SELECT * FROM users -- get all users",
			expected: "SELECT * FROM users ",
		},
		{
			name:     "single line comment on own line",
			input:    "-- comment\nSELECT * FROM users",
			expected: "\nSELECT * FROM users",
		},
		{
			name:     "block comment",
			input:    "SELECT /* all columns */ * FROM users",
			expected: "SELECT   * FROM users",
		},
		{
			name:     "multiline block comment",
			input:    "SELECT * FROM users /* this\nis\na\ncomment */",
			expected: "SELECT * FROM users  ",
		},
		{
			name:     "comment in single-quoted string preserved",
			input:    "SELECT '-- not a comment' FROM users",
			expected: "SELECT '-- not a comment' FROM users",
		},
		{
			name:     "comment in double-quoted identifier preserved",
			input:    `SELECT "-- not a comment" FROM users`,
			expected: `SELECT "-- not a comment" FROM users`,
		},
		{
			name:     "block comment in string preserved",
			input:    "SELECT '/* not a comment */' FROM users",
			expected: "SELECT '/* not a comment */' FROM users",
		},
		{
			name:     "escaped single quote in string",
			input:    "SELECT 'it''s -- not a comment' FROM users",
			expected: "SELECT 'it''s -- not a comment' FROM users",
		},
		{
			name:     "inline comment after column",
			input:    "CREATE TABLE users (\n  id UUID PRIMARY KEY, -- primary key\n  name TEXT NOT NULL -- user name\n)",
			expected: "CREATE TABLE users (\n  id UUID PRIMARY KEY, \n  name TEXT NOT NULL \n)",
		},
		{
			name:     "multiple comment types",
			input:    "/* header */ CREATE TABLE users ( -- inline\n  id UUID /* type */ PRIMARY KEY\n)",
			expected: "  CREATE TABLE users ( \n  id UUID   PRIMARY KEY\n)",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only comment",
			input:    "-- just a comment",
			expected: "",
		},
		{
			name:     "only block comment",
			input:    "/* just a comment */",
			expected: " ",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := StripComments(tc.input)
			if result != tc.expected {
				t.Errorf("StripComments(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestStripComments_CreateTableWithComments(t *testing.T) {
	input := `CREATE TABLE users (
		id UUID PRIMARY KEY, -- unique identifier
		email TEXT NOT NULL, /* user email address */
		name TEXT -- user's display name
	)`

	result := StripComments(input)

	// Should not contain any comment markers
	if strings.Contains(result, "--") {
		t.Errorf("Result still contains single-line comment marker: %s", result)
	}
	if strings.Contains(result, "/*") || strings.Contains(result, "*/") {
		t.Errorf("Result still contains block comment markers: %s", result)
	}

	// Should still contain the actual SQL structure
	if !strings.Contains(result, "CREATE TABLE users") {
		t.Errorf("Result missing CREATE TABLE: %s", result)
	}
	if !strings.Contains(result, "id UUID PRIMARY KEY") {
		t.Errorf("Result missing id column: %s", result)
	}
	if !strings.Contains(result, "email TEXT NOT NULL") {
		t.Errorf("Result missing email column: %s", result)
	}
	if !strings.Contains(result, "name TEXT") {
		t.Errorf("Result missing name column: %s", result)
	}
}

func TestDDLParser_ParseWithComments(t *testing.T) {
	parser := NewDDLParser()

	testCases := []struct {
		name          string
		sql           string
		expectedTable string
		expectedCols  int
	}{
		{
			name: "create table with inline comments",
			sql: `CREATE TABLE users (
				id UUID PRIMARY KEY, -- primary key column
				email TEXT NOT NULL, -- user email
				name TEXT -- display name
			)`,
			expectedTable: "users",
			expectedCols:  3,
		},
		{
			name: "create table with block comments",
			sql: `/* Users table for storing user information */
			CREATE TABLE users (
				id UUID PRIMARY KEY,
				/* Email must be unique */
				email TEXT NOT NULL,
				name TEXT
			)`,
			expectedTable: "users",
			expectedCols:  3,
		},
		{
			name: "create table with mixed comments",
			sql: `-- Users table
			CREATE TABLE users (
				id UUID PRIMARY KEY, -- pk
				email TEXT NOT NULL /* unique email */,
				name TEXT -- optional
			)`,
			expectedTable: "users",
			expectedCols:  3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stmt, err := parser.Parse(tc.sql, "test.sql", "postgresql")
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			createStmt, ok := stmt.(*CreateTableStatement)
			if !ok {
				t.Fatalf("Expected CreateTableStatement, got %T", stmt)
			}

			if createStmt.TableName != tc.expectedTable {
				t.Errorf("Expected table name %q, got %q", tc.expectedTable, createStmt.TableName)
			}

			if len(createStmt.Columns) != tc.expectedCols {
				t.Errorf("Expected %d columns, got %d", tc.expectedCols, len(createStmt.Columns))
			}
		})
	}
}

func TestDDLParser_ParseColumnNameStartingWithCheck(t *testing.T) {
	parser := NewDDLParser()

	sql := `CREATE TABLE server_operational_status (
		id SERIAL PRIMARY KEY,
		checked_at TIMESTAMP WITH TIME ZONE NOT NULL,
		status TEXT NOT NULL
	)`

	stmt, err := parser.Parse(sql, "test.sql", "postgresql")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	createStmt, ok := stmt.(*CreateTableStatement)
	if !ok {
		t.Fatalf("Expected CreateTableStatement, got %T", stmt)
	}

	if len(createStmt.Columns) != 3 {
		t.Fatalf("Expected 3 columns, got %d", len(createStmt.Columns))
	}

	columnNames := map[string]bool{}
	for _, col := range createStmt.Columns {
		columnNames[col.Name] = true
	}

	if !columnNames["checked_at"] {
		t.Fatalf("Expected checked_at column to be parsed, got columns: %v", columnNames)
	}
}

func TestValidatePrimaryKeyDatatype(t *testing.T) {
	testCases := []struct {
		name         string
		dataType     string
		databaseType string
		expectError  bool
	}{
		{"postgresql_uuid_valid", "UUID", "postgresql", false},
		{"postgresql_uuid_lowercase", "uuid", "postgresql", false},
		{"postgresql_text_valid", "TEXT", "postgresql", false},
		{"postgresql_varchar_valid", "VARCHAR", "postgresql", false},
		{"postgresql_integer_valid", "INTEGER", "postgresql", false},
		{"postgresql_serial_valid", "serial", "postgresql", false},
		{"postgresql_bigserial_valid", "bigserial", "postgresql", false},
		{"postgresql_bigint_valid", "bigint", "postgresql", false},
		{"postgresql_bytea_invalid", "BYTEA", "postgresql", true},
		{"postgresql_jsonb_invalid", "JSONB", "postgresql", true},
		{"postgresql_boolean_invalid", "BOOLEAN", "postgresql", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.ValidatePrimaryKeyDatatype(tc.dataType, tc.databaseType, "test.sql", "id")
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
			name:           "postgresql_bytea_error_message",
			dataType:       "BYTEA",
			databaseType:   "postgresql",
			columnName:     "id",
			migrationFile:  "/path/to/001_create_users.sql",
			expectedSubstr: "unsupported primary key type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validation.ValidatePrimaryKeyDatatype(
				tc.dataType,
				tc.databaseType,
				tc.migrationFile,
				tc.columnName,
			)
			if err == nil {
				t.Fatal("Expected error but got none")
			}

			errorMsg := err.Error()
			if !containsString(errorMsg, tc.expectedSubstr) {
				t.Errorf(
					"Expected error message to contain '%s', but got: %s",
					tc.expectedSubstr,
					errorMsg,
				)
			}
		})
	}
}

func TestValidatePrimaryKeyDatatype_UnsupportedType(t *testing.T) {
	// For unsupported primary key types, validation should return an error
	err := validation.ValidatePrimaryKeyDatatype("BYTEA", "postgresql", "test.sql", "id")
	if err == nil {
		t.Error("Expected an error for unsupported primary key type, but got none")
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
			name:         "postgresql_valid_text_primary_key",
			columnDefs:   "id TEXT PRIMARY KEY, name TEXT NOT NULL",
			databaseType: "postgresql",
			expectError:  false,
		},
		{
			name:         "postgresql_valid_serial_primary_key",
			columnDefs:   "id SERIAL PRIMARY KEY, name TEXT NOT NULL",
			databaseType: "postgresql",
			expectError:  false,
		},
		{
			name:         "postgresql_separate_primary_key_constraint_valid",
			columnDefs:   "id UUID NOT NULL, name TEXT, PRIMARY KEY (id)",
			databaseType: "postgresql",
			expectError:  false,
		},
		{
			name:         "postgresql_separate_primary_key_constraint_integer_valid",
			columnDefs:   "id INTEGER NOT NULL, name TEXT, PRIMARY KEY (id)",
			databaseType: "postgresql",
			expectError:  false,
		},
		{
			name:         "postgresql_invalid_bytea_primary_key",
			columnDefs:   "id BYTEA PRIMARY KEY, name TEXT NOT NULL",
			databaseType: "postgresql",
			expectError:  true,
			errorSubstr:  "unsupported primary key type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewCreateTableParser()
			columns, err := parser.parseColumnDefinitions(tc.columnDefs, "test.sql", tc.databaseType)

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
			name:         "postgresql_valid_create_table_uuid",
			sql:          "CREATE TABLE users (id UUID PRIMARY KEY, email TEXT NOT NULL)",
			databaseType: "postgresql",
			expectError:  false,
		},
		{
			name:         "postgresql_valid_create_table_text",
			sql:          "CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL)",
			databaseType: "postgresql",
			expectError:  false,
		},
		{
			name:         "postgresql_valid_create_table_serial",
			sql:          "CREATE TABLE users (id SERIAL PRIMARY KEY, email TEXT NOT NULL)",
			databaseType: "postgresql",
			expectError:  false,
		},
		{
			name:         "postgresql_valid_create_table_bigserial",
			sql:          "CREATE TABLE users (id BIGSERIAL PRIMARY KEY, email TEXT NOT NULL)",
			databaseType: "postgresql",
			expectError:  false,
		},
		{
			name:         "postgresql_invalid_create_table_bytea",
			sql:          "CREATE TABLE users (id BYTEA PRIMARY KEY, email TEXT NOT NULL)",
			databaseType: "postgresql",
			expectError:  true,
			errorSubstr:  "unsupported primary key type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := NewCreateTableParser()
			stmt, err := parser.Parse(tc.sql, "test.sql", tc.databaseType)

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
					t.Fatalf("Expected no error but got: %v", err)
				}

				if stmt == nil {
					t.Fatal("Expected statement but got nil")
				}

				if stmt.GetType() != CreateTable {
					t.Errorf("Expected CREATE TABLE statement type, got %v", stmt.GetType())
				}
			}
		})
	}
}
