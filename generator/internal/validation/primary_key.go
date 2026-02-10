package validation

import (
	"fmt"
	"strings"
)

// PKType represents the type category of a primary key
type PKType string

const (
	PKTypeUUID   PKType = "uuid"
	PKTypeInt32  PKType = "int32"
	PKTypeInt64  PKType = "int64"
	PKTypeString PKType = "string"
)

// ClassifyPrimaryKeyType determines the PKType for a given SQL data type
func ClassifyPrimaryKeyType(dataType string) (PKType, error) {
	switch strings.ToLower(dataType) {
	case "uuid":
		return PKTypeUUID, nil
	case "serial", "int", "int4", "integer":
		return PKTypeInt32, nil
	case "bigserial", "bigint", "int8":
		return PKTypeInt64, nil
	case "text", "varchar", "character varying":
		return PKTypeString, nil
	default:
		return "", fmt.Errorf("unsupported primary key type: %s", dataType)
	}
}

// IsAutoIncrement returns true if the data type is auto-incrementing (serial/bigserial)
func IsAutoIncrement(dataType string) bool {
	normalized := strings.ToLower(dataType)
	return normalized == "serial" || normalized == "bigserial"
}

// GoType converts a PKType to its corresponding Go type string
func GoType(pkType PKType) string {
	switch pkType {
	case PKTypeUUID:
		return "uuid.UUID"
	case PKTypeInt32:
		return "int32"
	case PKTypeInt64:
		return "int64"
	case PKTypeString:
		return "string"
	default:
		return "uuid.UUID"
	}
}

// ValidatePrimaryKeyDatatype validates primary key data types
func ValidatePrimaryKeyDatatype(dataType, databaseType, migrationFile, columnName string) error {
	_, err := ClassifyPrimaryKeyType(dataType)
	return err
}
