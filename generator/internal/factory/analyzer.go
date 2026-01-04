package factory

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/generator/models"
)

// FieldAnalyzer determines appropriate default values for factory fields
type FieldAnalyzer struct {
	databaseType string
}

func NewFieldAnalyzer(dbType string) *FieldAnalyzer {
	return &FieldAnalyzer{databaseType: dbType}
}

// FactoryFieldInfo contains metadata for generating a factory field
type FactoryFieldInfo struct {
	Name          string
	Type          string
	GoZero        string
	DefaultValue  string
	OptionName    string
	IsFK          bool
	IsTimestamp   bool
	IsID          bool
	IsAutoManaged bool
}

// AnalyzeField returns default value expression and metadata for a field
func (fa *FieldAnalyzer) AnalyzeField(field models.GeneratedField, tableName string) FactoryFieldInfo {
	info := FactoryFieldInfo{
		Name:          field.Name,
		Type:          field.Type,
		OptionName:    fmt.Sprintf("With%s%s", toCamelCase(tableName), field.Name),
		IsID:          field.Name == "ID",
		IsTimestamp:   field.Type == "time.Time" || strings.Contains(field.Type, "Time"),
		IsAutoManaged: field.Name == "ID" || field.Name == "CreatedAt" || field.Name == "UpdatedAt",
		IsFK:          strings.HasSuffix(field.Name, "ID") && field.Name != "ID",
	}

	// Determine default value
	info.DefaultValue = fa.determineDefault(field.Name, field.Type, field.SQLCType)
	info.GoZero = fa.getGoZero(field.Type)

	return info
}

func (fa *FieldAnalyzer) determineDefault(fieldName, goType, sqlcType string) string {
	// Handle by type first
	switch goType {
	case "string":
		return fa.stringDefault(fieldName)
	case "int32", "int64", "int":
		return fa.intDefault(fieldName)
	case "bool":
		return "false"
	case "time.Time":
		return "time.Time{}"
	case "uuid.UUID":
		return "uuid.UUID{}"
	case "[]byte":
		return "[]byte{}"
	}

	// Handle pgtype wrappers
	if strings.Contains(goType, "pgtype") {
		return fa.pgtypeDefault(goType)
	}

	// Default fallback
	return fmt.Sprintf("%s{}", goType)
}

func (fa *FieldAnalyzer) stringDefault(fieldName string) string {
	lower := strings.ToLower(fieldName)

	// Field name heuristics
	switch {
	case lower == "email":
		return "faker.Email()"
	case lower == "name" || strings.HasSuffix(lower, "name"):
		return "faker.Name()"
	case lower == "phone" || strings.Contains(lower, "phone"):
		return "faker.Phonenumber()"
	case lower == "url" || strings.Contains(lower, "url"):
		return "faker.URL()"
	case lower == "description" || strings.HasSuffix(lower, "description"):
		return "faker.Sentence()"
	case lower == "title" || strings.HasSuffix(lower, "title"):
		return "faker.Word()"
	case lower == "address" || strings.Contains(lower, "address"):
		return "faker.GetRealAddress().Address"
	case lower == "city":
		return "faker.GetRealAddress().City"
	case lower == "country":
		return "faker.GetRealAddress().Country"
	case lower == "zipcode" || lower == "postalcode":
		return "faker.GetRealAddress().PostalCode"
	case strings.Contains(lower, "color"):
		return "faker.GetRandomColor()"
	default:
		return "faker.Word()"
	}
}

func (fa *FieldAnalyzer) intDefault(fieldName string) string {
	lower := strings.ToLower(fieldName)

	switch {
	case strings.Contains(lower, "price") || strings.Contains(lower, "amount"):
		return "faker.RandomInt(100, 10000)" // Price in cents
	case strings.Contains(lower, "count") || strings.Contains(lower, "quantity"):
		return "faker.RandomInt(1, 100)"
	case strings.Contains(lower, "age"):
		return "faker.RandomInt(18, 80)"
	default:
		return "faker.RandomInt(1, 1000)"
	}
}

func (fa *FieldAnalyzer) pgtypeDefault(goType string) string {
	// Handle nullable pgtype fields - default to zero/null
	return fmt.Sprintf("%s{}", goType)
}

func (fa *FieldAnalyzer) getGoZero(goType string) string {
	switch goType {
	case "string":
		return `""`
	case "int", "int32", "int64", "float32", "float64":
		return "0"
	case "bool":
		return "false"
	case "time.Time":
		return "time.Time{}"
	case "uuid.UUID":
		return "uuid.UUID{}"
	case "[]byte":
		return "nil"
	default:
		if strings.HasPrefix(goType, "[]") {
			return "nil"
		}
		return fmt.Sprintf("%s{}", goType)
	}
}

func toCamelCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
