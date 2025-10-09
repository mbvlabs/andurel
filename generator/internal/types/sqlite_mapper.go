package types

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

// SQLiteTypeMapper implements DatabaseTypeMapper for SQLite
type SQLiteTypeMapper struct {
	databaseType string
}

// NewSQLiteTypeMapper creates a new SQLite type mapper
func NewSQLiteTypeMapper() *SQLiteTypeMapper {
	return &SQLiteTypeMapper{
		databaseType: "sqlite",
	}
}

// MapToGoType maps a SQLite column to a Go type
func (stm *SQLiteTypeMapper) MapToGoType(column *catalog.Column) (TypeMapping, error) {
	normalizedType := normalizeSQLType(column.DataType)
	nullable := column.IsNullable

	goType, sqlcType, packageName := stm.getSQLiteType(normalizedType, nullable)

	if goType == "" {
		return TypeMapping{
			GoType:      "interface{}",
			SQLCType:    "interface{}",
			PackageName: "",
		}, nil
	}

	return TypeMapping{
		GoType:      goType,
		SQLCType:    sqlcType,
		PackageName: packageName,
	}, nil
}

// MapToSQLType maps a Go type to a SQLite type
func (stm *SQLiteTypeMapper) MapToSQLType(goType string, nullable bool) (string, error) {
	switch goType {
	case "string":
		if nullable {
			return "sql.NullString", nil
		}
		return "string", nil
	case "int64":
		if nullable {
			return "sql.NullInt64", nil
		}
		return "int64", nil
	case "float64":
		if nullable {
			return "sql.NullFloat64", nil
		}
		return "float64", nil
	case "bool":
		if nullable {
			return "sql.NullBool", nil
		}
		return "bool", nil
	case "time.Time":
		if nullable {
			return "sql.NullTime", nil
		}
		return "time.Time", nil
	case "[]byte":
		return "[]byte", nil
	default:
		return "", ErrUnsupportedType
	}
}

// GenerateConversionFromDB generates conversion code from SQLite to Go type
func (stm *SQLiteTypeMapper) GenerateConversionFromDB(fieldName, sqlcType, goType string) string {
	// Special case: UUID from string for SQLite ID fields
	if goType == "uuid.UUID" && sqlcType == "string" {
		return fmt.Sprintf("uuid.Parse(row.%s)", fieldName)
	}

	if strings.HasPrefix(sqlcType, "sql.Null") {
		switch sqlcType {
		case "sql.NullString":
			return fmt.Sprintf("row.%s.String", fieldName)
		case "sql.NullInt64":
			return fmt.Sprintf("row.%s.Int64", fieldName)
		case "sql.NullFloat64":
			return fmt.Sprintf("row.%s.Float64", fieldName)
		case "sql.NullBool":
			return fmt.Sprintf("row.%s.Bool", fieldName)
		case "sql.NullTime":
			return fmt.Sprintf("row.%s.Time", fieldName)
		default:
			return fmt.Sprintf("row.%s", fieldName)
		}
	}

	return fmt.Sprintf("row.%s", fieldName)
}

// GenerateConversionToDB generates conversion code from Go to SQLite type
func (stm *SQLiteTypeMapper) GenerateConversionToDB(sqlcType, goType, valueExpr string) string {
	// Special case: UUID to string for SQLite ID fields
	if goType == "uuid.UUID" && sqlcType == "string" {
		return fmt.Sprintf("%s.String()", valueExpr)
	}

	if strings.HasPrefix(sqlcType, "sql.Null") {
		switch sqlcType {
		case "sql.NullString":
			return fmt.Sprintf("sql.NullString{String: %s, Valid: true}", valueExpr)
		case "sql.NullInt64":
			return fmt.Sprintf("sql.NullInt64{Int64: %s, Valid: true}", valueExpr)
		case "sql.NullFloat64":
			return fmt.Sprintf("sql.NullFloat64{Float64: %s, Valid: true}", valueExpr)
		case "sql.NullBool":
			return fmt.Sprintf("sql.NullBool{Bool: %s, Valid: true}", valueExpr)
		case "sql.NullTime":
			return fmt.Sprintf("sql.NullTime{Time: %s, Valid: true}", valueExpr)
		default:
			return valueExpr
		}
	}

	return valueExpr
}

// GenerateZeroCheck generates zero-value check code for a Go type
func (stm *SQLiteTypeMapper) GenerateZeroCheck(goType, valueExpr string) string {
	switch goType {
	case "uuid.UUID":
		return fmt.Sprintf("%s != uuid.Nil", valueExpr)
	default:
		if strings.HasPrefix(goType, "sql.Null") {
			return fmt.Sprintf("%s.Valid", valueExpr)
		}
		return "true"
	}
}

// GetDatabaseType returns the database type
func (stm *SQLiteTypeMapper) GetDatabaseType() string {
	return stm.databaseType
}

// getSQLiteType returns SQLite-specific type mapping
func (stm *SQLiteTypeMapper) getSQLiteType(
	normalizedType string,
	nullable bool,
) (goType, sqlcType, packageName string) {
	switch normalizedType {
	case "varchar",
		"text",
		"char",
		"clob",
		"character",
		"varying character",
		"nchar",
		"native character",
		"nvarchar":
		if nullable {
			return "string", "sql.NullString", "database/sql"
		}
		return "string", "string", ""

	case "int",
		"integer",
		"tinyint",
		"smallint",
		"mediumint",
		"bigint",
		"unsigned big int",
		"int2",
		"int8":
		if nullable {
			return "int64", "sql.NullInt64", "database/sql"
		}
		return "int64", "int64", ""

	case "real", "double", "double precision", "float":
		if nullable {
			return "float64", "sql.NullFloat64", "database/sql"
		}
		return "float64", "float64", ""

	case "numeric", "decimal", "boolean", "date", "datetime", "timestamp", "time":
		if normalizedType == "boolean" {
			if nullable {
				return "bool", "sql.NullBool", "database/sql"
			}
			return "bool", "bool", ""
		}
		if normalizedType == "date" || normalizedType == "datetime" {
			if nullable {
				return "time.Time", "sql.NullTime", "database/sql"
			}
			return "time.Time", "time.Time", ""
		}
		if normalizedType == "timestamp" || normalizedType == "time" {
			if nullable {
				return "time.Time", "sql.NullTime", "database/sql"
			}
			return "time.Time", "time.Time", ""
		}
		if nullable {
			return "float64", "sql.NullFloat64", "database/sql"
		}
		return "float64", "float64", ""

	case "blob":
		return "[]byte", "[]byte", ""

	default:
		return "", "", ""
	}
}
