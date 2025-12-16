package validation

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePrimaryKeyDatatype validates primary key data types based on database
func ValidatePrimaryKeyDatatype(dataType, databaseType, migrationFile, columnName string) error {
	normalizedDataType := strings.ToLower(dataType)

	switch databaseType {
	case "postgresql":
		if normalizedDataType != "uuid" {
			return fmt.Errorf(`Primary key validation failed in migration '%s':
Column '%s' has datatype '%s' but PostgreSQL primary keys must use 'uuid'.

To fix this, change:
  %s %s PRIMARY KEY
to:
  %s UUID PRIMARY KEY`,
				filepath.Base(migrationFile), columnName, dataType,
				columnName, dataType, columnName)
		}
	case "sqlite":
		if normalizedDataType != "text" {
			return fmt.Errorf(`Primary key validation failed in migration '%s':
Column '%s' has datatype '%s' but SQLite primary keys must use 'text'.

To fix this, change:
  %s %s PRIMARY KEY
to:
  %s TEXT PRIMARY KEY`,
				filepath.Base(migrationFile), columnName, dataType,
				columnName, dataType, columnName)
		}
	default:
		return fmt.Errorf("unsupported database type for primary key validation: %s", databaseType)
	}

	return nil
}
