package types

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

// PostgreSQLTypeMapper implements DatabaseTypeMapper for PostgreSQL
type PostgreSQLTypeMapper struct {
	databaseType string
}

// NewPostgreSQLTypeMapper creates a new PostgreSQL type mapper
func NewPostgreSQLTypeMapper() *PostgreSQLTypeMapper {
	return &PostgreSQLTypeMapper{
		databaseType: "postgresql",
	}
}

// MapToGoType maps a PostgreSQL column to a Go type
func (pgm *PostgreSQLTypeMapper) MapToGoType(column *catalog.Column) (TypeMapping, error) {
	normalizedType := normalizeSQLType(column.DataType)
	nullable := column.IsNullable

	// Handle UUID special case
	if normalizedType == "uuid" {
		return TypeMapping{
			GoType:      "uuid.UUID",
			SQLCType:    "uuid.UUID",
			PackageName: "github.com/google/uuid",
		}, nil
	}

	goType, sqlcType, packageName := pgm.getPostgreSQLType(normalizedType, nullable)

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

// MapToSQLType maps a Go type to a PostgreSQL type
func (pgm *PostgreSQLTypeMapper) MapToSQLType(goType string, nullable bool) (string, error) {
	// This is a simplified mapping - in practice this would be more complex
	switch goType {
	case "string":
		if nullable {
			return "pgtype.Text", nil
		}
		return "string", nil
	case "int32":
		if nullable {
			return "pgtype.Int4", nil
		}
		return "int32", nil
	case "int64":
		if nullable {
			return "pgtype.Int8", nil
		}
		return "int64", nil
	case "bool":
		if nullable {
			return "pgtype.Bool", nil
		}
		return "bool", nil
	case "float32":
		if nullable {
			return "pgtype.Float4", nil
		}
		return "float32", nil
	case "float64":
		if nullable {
			return "pgtype.Float8", nil
		}
		return "float64", nil
	case "time.Time":
		if nullable {
			return "pgtype.Timestamptz", nil
		}
		return "pgtype.Timestamptz", nil
	case "[]byte":
		if nullable {
			return "pgtype.Bytea", nil
		}
		return "[]byte", nil
	default:
		return "", ErrUnsupportedType
	}
}

// GenerateConversionFromDB generates conversion code from PostgreSQL to Go type
func (pgm *PostgreSQLTypeMapper) GenerateConversionFromDB(
	fieldName, sqlcType, goType string,
) string {
	if strings.HasPrefix(sqlcType, "pgtype.") {
		switch sqlcType {
		case "pgtype.Text":
			return fmt.Sprintf("row.%s.String", fieldName)
		case "pgtype.Int4":
			return fmt.Sprintf("row.%s.Int32", fieldName)
		case "pgtype.Int8":
			return fmt.Sprintf("row.%s.Int64", fieldName)
		case "pgtype.Int2":
			return fmt.Sprintf("row.%s.Int16", fieldName)
		case "pgtype.Float4":
			return fmt.Sprintf("row.%s.Float32", fieldName)
		case "pgtype.Float8":
			return fmt.Sprintf("row.%s.Float64", fieldName)
		case "pgtype.Bool":
			return fmt.Sprintf("row.%s.Bool", fieldName)
		case "pgtype.Timestamptz", "pgtype.Timestamp":
			return fmt.Sprintf("row.%s.Time", fieldName)
		case "pgtype.Date":
			return fmt.Sprintf("row.%s.Time", fieldName)
		case "pgtype.Time":
			return fmt.Sprintf("row.%s.Time", fieldName)
		case "pgtype.Timetz":
			return fmt.Sprintf("row.%s.Time", fieldName)
		case "pgtype.Interval":
			return fmt.Sprintf("row.%s.Microseconds", fieldName)
		case "pgtype.JSONB", "pgtype.JSON":
			return fmt.Sprintf("row.%s.Bytes", fieldName)
		case "pgtype.Inet", "pgtype.CIDR", "pgtype.Macaddr", "pgtype.Macaddr8":
			return fmt.Sprintf("row.%s.IPNet.String()", fieldName)
		case "pgtype.Point",
			"pgtype.Lseg",
			"pgtype.Box",
			"pgtype.Path",
			"pgtype.Polygon",
			"pgtype.Circle":
			return fmt.Sprintf("string(row.%s.Bytes)", fieldName)
		case "pgtype.Int4range",
			"pgtype.Int8range",
			"pgtype.Numrange",
			"pgtype.Tsrange",
			"pgtype.Tstzrange",
			"pgtype.Daterange":
			return fmt.Sprintf("string(row.%s.Bytes)", fieldName)
		case "pgtype.Money":
			return fmt.Sprintf("row.%s.String", fieldName)
		case "pgtype.Bit", "pgtype.Varbit":
			return fmt.Sprintf("string(row.%s.Bytes)", fieldName)
		case "pgtype.Array[int32]":
			return fmt.Sprintf("row.%s.Elements", fieldName)
		case "pgtype.Array[string]":
			return fmt.Sprintf("row.%s.Elements", fieldName)
		default:
			return fmt.Sprintf("row.%s", fieldName)
		}
	}

	return fmt.Sprintf("row.%s", fieldName)
}

// GenerateConversionToDB generates conversion code from Go to PostgreSQL type
func (pgm *PostgreSQLTypeMapper) GenerateConversionToDB(sqlcType, goType, valueExpr string) string {
	if strings.HasPrefix(sqlcType, "pgtype.") {
		switch sqlcType {
		case "pgtype.Text":
			return fmt.Sprintf("pgtype.Text{String: %s, Valid: true}", valueExpr)
		case "pgtype.Int4":
			return fmt.Sprintf("pgtype.Int4{Int32: %s, Valid: true}", valueExpr)
		case "pgtype.Int8":
			return fmt.Sprintf("pgtype.Int8{Int64: %s, Valid: true}", valueExpr)
		case "pgtype.Int2":
			return fmt.Sprintf("pgtype.Int2{Int16: %s, Valid: true}", valueExpr)
		case "pgtype.Float4":
			return fmt.Sprintf("pgtype.Float4{Float32: %s, Valid: true}", valueExpr)
		case "pgtype.Float8":
			return fmt.Sprintf("pgtype.Float8{Float64: %s, Valid: true}", valueExpr)
		case "pgtype.Bool":
			return fmt.Sprintf("pgtype.Bool{Bool: %s, Valid: true}", valueExpr)
		case "pgtype.Timestamptz":
			return fmt.Sprintf("pgtype.Timestamptz{Time: %s, Valid: true}", valueExpr)
		case "pgtype.Timestamp":
			return fmt.Sprintf("pgtype.Timestamp{Time: %s, Valid: true}", valueExpr)
		case "pgtype.Date":
			return fmt.Sprintf("pgtype.Date{Time: %s, Valid: true}", valueExpr)
		case "pgtype.Time":
			return fmt.Sprintf("pgtype.Time{Time: %s, Valid: true}", valueExpr)
		case "pgtype.Timetz":
			return fmt.Sprintf("pgtype.Timetz{Time: %s, Valid: true}", valueExpr)
		case "pgtype.Interval":
			return fmt.Sprintf("pgtype.Interval{Microseconds: %s, Valid: true}", valueExpr)
		case "pgtype.JSONB":
			return fmt.Sprintf("pgtype.JSONB{Bytes: %s, Valid: true}", valueExpr)
		case "pgtype.JSON":
			return fmt.Sprintf("pgtype.JSON{Bytes: %s, Valid: true}", valueExpr)
		case "pgtype.Inet", "pgtype.CIDR", "pgtype.Macaddr", "pgtype.Macaddr8":
			return fmt.Sprintf("pgtype.Inet{IPNet: %s, Valid: true}", valueExpr)
		case "pgtype.Money":
			return fmt.Sprintf("pgtype.Money{String: %s, Valid: true}", valueExpr)
		case "pgtype.Array[int32]":
			return fmt.Sprintf("pgtype.Array[int32]{Elements: %s, Valid: true}", valueExpr)
		case "pgtype.Array[string]":
			return fmt.Sprintf("pgtype.Array[string]{Elements: %s, Valid: true}", valueExpr)
		default:
			return valueExpr
		}
	}

	return valueExpr
}

// GenerateZeroCheck generates zero-value check code for a Go type
func (pgm *PostgreSQLTypeMapper) GenerateZeroCheck(goType, valueExpr string) string {
	switch goType {
	case "uuid.UUID":
		return fmt.Sprintf("%s != uuid.Nil", valueExpr)
	default:
		if strings.HasPrefix(goType, "pgtype.") {
			return fmt.Sprintf("%s.Valid", valueExpr)
		}
		return "true"
	}
}

// GetDatabaseType returns the database type
func (pgm *PostgreSQLTypeMapper) GetDatabaseType() string {
	return pgm.databaseType
}

// getPostgreSQLType returns the PostgreSQL-specific type mapping
func (pgm *PostgreSQLTypeMapper) getPostgreSQLType(
	normalizedType string,
	nullable bool,
) (goType, sqlcType, packageName string) {
	switch normalizedType {
	case "varchar", "text", "char":
		if nullable {
			return "string", "pgtype.Text", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "string", ""
	case "bytea":
		if nullable {
			return "[]byte", "pgtype.Bytea", "github.com/jackc/pgx/v5/pgtype"
		}
		return "[]byte", "[]byte", ""
	case "boolean", "bool":
		if nullable {
			return "bool", "pgtype.Bool", "github.com/jackc/pgx/v5/pgtype"
		}
		return "bool", "bool", ""
	case "integer", "int", "int4", "serial":
		if nullable {
			return "int32", "pgtype.Int4", "github.com/jackc/pgx/v5/pgtype"
		}
		return "int32", "int32", ""
	case "bigint", "int8", "bigserial":
		if nullable {
			return "int64", "pgtype.Int8", "github.com/jackc/pgx/v5/pgtype"
		}
		return "int64", "int64", ""
	case "smallint", "int2", "smallserial":
		if nullable {
			return "int16", "pgtype.Int2", "github.com/jackc/pgx/v5/pgtype"
		}
		return "int16", "int16", ""
	case "real", "float4":
		if nullable {
			return "float32", "pgtype.Float4", "github.com/jackc/pgx/v5/pgtype"
		}
		return "float32", "float32", ""
	case "double precision", "float8":
		if nullable {
			return "float64", "pgtype.Float8", "github.com/jackc/pgx/v5/pgtype"
		}
		return "float64", "float64", ""
	case "decimal", "numeric":
		if nullable {
			return "float64", "pgtype.Numeric", "github.com/jackc/pgx/v5/pgtype"
		}
		return "float64", "pgtype.Numeric", "github.com/jackc/pgx/v5/pgtype"
	case "timestamp", "timestamp without time zone":
		if nullable {
			return "time.Time", "pgtype.Timestamp", "github.com/jackc/pgx/v5/pgtype"
		}
		return "time.Time", "pgtype.Timestamp", "github.com/jackc/pgx/v5/pgtype"
	case "timestamptz", "timestamp with time zone":
		if nullable {
			return "time.Time", "pgtype.Timestamptz", "github.com/jackc/pgx/v5/pgtype"
		}
		return "time.Time", "pgtype.Timestamptz", "github.com/jackc/pgx/v5/pgtype"
	case "date":
		if nullable {
			return "time.Time", "pgtype.Date", "github.com/jackc/pgx/v5/pgtype"
		}
		return "time.Time", "pgtype.Date", "github.com/jackc/pgx/v5/pgtype"
	case "time":
		if nullable {
			return "time.Time", "pgtype.Time", "github.com/jackc/pgx/v5/pgtype"
		}
		return "time.Time", "pgtype.Time", "github.com/jackc/pgx/v5/pgtype"
	case "timetz":
		if nullable {
			return "time.Time", "pgtype.Timetz", "github.com/jackc/pgx/v5/pgtype"
		}
		return "time.Time", "pgtype.Timetz", "github.com/jackc/pgx/v5/pgtype"
	case "interval":
		if nullable {
			return "string", "pgtype.Interval", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Interval", "github.com/jackc/pgx/v5/pgtype"
	case "jsonb":
		if nullable {
			return "[]byte", "pgtype.JSONB", "github.com/jackc/pgx/v5/pgtype"
		}
		return "[]byte", "pgtype.JSONB", "github.com/jackc/pgx/v5/pgtype"
	case "json":
		if nullable {
			return "[]byte", "pgtype.JSON", "github.com/jackc/pgx/v5/pgtype"
		}
		return "[]byte", "pgtype.JSON", "github.com/jackc/pgx/v5/pgtype"
	case "inet":
		if nullable {
			return "string", "pgtype.Inet", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Inet", "github.com/jackc/pgx/v5/pgtype"
	case "cidr":
		if nullable {
			return "string", "pgtype.CIDR", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.CIDR", "github.com/jackc/pgx/v5/pgtype"
	case "macaddr":
		if nullable {
			return "string", "pgtype.Macaddr", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Macaddr", "github.com/jackc/pgx/v5/pgtype"
	case "macaddr8":
		if nullable {
			return "string", "pgtype.Macaddr8", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Macaddr8", "github.com/jackc/pgx/v5/pgtype"
	case "point", "lseg", "box", "path", "polygon", "circle":
		if nullable {
			return "string", "pgtype." + normalizedType, "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype." + normalizedType, "github.com/jackc/pgx/v5/pgtype"
	case "int4range", "int8range", "numrange", "tsrange", "tstzrange", "daterange":
		if nullable {
			return "string", "pgtype." + normalizedType, "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype." + normalizedType, "github.com/jackc/pgx/v5/pgtype"
	case "money":
		if nullable {
			return "string", "pgtype.Money", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Money", "github.com/jackc/pgx/v5/pgtype"
	case "bit", "varbit":
		if nullable {
			return "string", "pgtype." + normalizedType, "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype." + normalizedType, "github.com/jackc/pgx/v5/pgtype"
	case "xml":
		if nullable {
			return "string", "pgtype.Text", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "string", ""
	case "tsvector", "tsquery":
		if nullable {
			return "string", "pgtype.Text", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "string", ""
	case "_integer":
		if nullable {
			return "[]int32", "pgtype.Array[int32]", "github.com/jackc/pgx/v5/pgtype"
		}
		return "[]int32", "pgtype.Array[int32]", "github.com/jackc/pgx/v5/pgtype"
	case "_text":
		if nullable {
			return "[]string", "pgtype.Array[string]", "github.com/jackc/pgx/v5/pgtype"
		}
		return "[]string", "pgtype.Array[string]", "github.com/jackc/pgx/v5/pgtype"
	default:
		return "", "", ""
	}
}
