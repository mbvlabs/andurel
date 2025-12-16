package validation

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePrimaryKeyDatatype validates primary key data types
func ValidatePrimaryKeyDatatype(dataType, databaseType, migrationFile, columnName string) error {
	normalizedDataType := strings.ToLower(dataType)

	if normalizedDataType != "uuid" {
		return fmt.Errorf(`Primary key validation failed in migration '%s':
Column '%s' has datatype '%s' but primary keys must use 'uuid'.

To fix this, change:
  %s %s PRIMARY KEY
to:
  %s UUID PRIMARY KEY`,
			filepath.Base(migrationFile), columnName, dataType,
			columnName, dataType, columnName)
	}

	return nil
}
