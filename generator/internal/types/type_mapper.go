package types

import (
	"strings"

	"github.com/mbvlabs/andurel/internal/naming"
)

// TypeOverride lets users map a SQL database type to a custom Go type.
// When the column is nullable, the resulting Go type will be wrapped
// according to the TypeMapper's NullType setting.
type TypeOverride struct {
	DatabaseType string
	GoType       string
	Package      string
}

// TypeMapper represents type mapper.
type TypeMapper struct {
	DatabaseType string
	NullType     string // "pointer" or "pgtype.Null"
	Overrides    []TypeOverride
}

// NewTypeMapper creates a new type mapper.
func NewTypeMapper(databaseType string) *TypeMapper {
	return &TypeMapper{
		DatabaseType: databaseType,
		NullType:     "pgtype.Null",
		Overrides:    make([]TypeOverride, 0),
	}
}

// sqlNullTypeMap maps base Go types to their database/sql null equivalent.
var sqlNullTypeMap = map[string]string{
	"string":    "sql.NullString",
	"bool":      "sql.NullBool",
	"int16":     "sql.NullInt16",
	"int32":     "sql.NullInt32",
	"int64":     "sql.NullInt64",
	"float64":   "sql.NullFloat64",
	"time.Time": "sql.NullTime",
}

// MapSQLTypeToGo returns the Go type for a SQL column, matching the types
// narsilc generates for pgx/v5 so model structs can be assigned to query
// parameters directly. The second return value is the import path required
// for the type, or "" if it is a builtin.
func (tm *TypeMapper) MapSQLTypeToGo(
	sqlType string,
	nullable bool,
) (goType, packageName string, err error) {
	normalized := normalizeSQLType(sqlType)

	for _, override := range tm.Overrides {
		if override.DatabaseType == normalized {
			return tm.wrapNullable(override.GoType, nullable), override.Package, nil
		}
	}

	goType, pkg := tm.postgresType(normalized, nullable)
	return goType, pkg, nil
}

func (tm *TypeMapper) wrapNullable(goType string, nullable bool) string {
	if !nullable {
		return goType
	}
	if strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") {
		return goType
	}

	switch tm.NullType {
	case "pgtype.Null":
		// Overrides fall through to pointers; postgresType emits pgtype for
		// nullable columns on the default mapping path.
	case "sql.Null":
		if nt, ok := sqlNullTypeMap[goType]; ok {
			return nt
		}
	}
	// Default to pointer for types without a null type equivalent.
	return "*" + goType
}

func (tm *TypeMapper) postgresType(
	normalized string,
	nullable bool,
) (goType, packageName string) {
	pgtypePkg := "github.com/jackc/pgx/v5/pgtype"

	switch normalized {
	case "uuid":
		return "pgtype.UUID", pgtypePkg
	case "timestamptz", "timestamp with time zone":
		return "pgtype.Timestamptz", pgtypePkg
	case "timestamp", "timestamp without time zone":
		return "pgtype.Timestamp", pgtypePkg
	case "date":
		return "pgtype.Date", pgtypePkg
	case "time", "time without time zone":
		return "pgtype.Time", pgtypePkg
	case "varchar", "text", "char",
		"xml", "tsvector", "tsquery", "name":
		if nullable {
			return "pgtype.Text", pgtypePkg
		}
		return "string", ""
	case "boolean":
		if nullable {
			return "pgtype.Bool", pgtypePkg
		}
		return "bool", ""
	case "smallint":
		if nullable {
			return "pgtype.Int2", pgtypePkg
		}
		return "int16", ""
	case "integer":
		if nullable {
			return "pgtype.Int4", pgtypePkg
		}
		return "int32", ""
	case "bigint":
		if nullable {
			return "pgtype.Int8", pgtypePkg
		}
		return "int64", ""
	case "real":
		if nullable {
			return "pgtype.Float4", pgtypePkg
		}
		return "float32", ""
	case "double precision":
		if nullable {
			return "pgtype.Float8", pgtypePkg
		}
		return "float64", ""
	case "decimal", "numeric", "money":
		return "pgtype.Numeric", pgtypePkg
	case "bytea", "json", "jsonb":
		return "[]byte", ""
	case "inet":
		// match narsilc pgx/v5. Other network types stay on string.
		if nullable {
			return "*netip.Addr", "net/netip"
		}
		return "netip.Addr", "net/netip"
	case "_integer":
		return "[]int32", ""
	case "_text":
		return "[]string", ""
	}

	// Types without a pgx/v5-narsilc mapping keep a string fallback so
	// generated models remain compilable for exotic columns.
	if nullable {
		return "*string", ""
	}
	return "string", ""
}

func normalizeSQLType(sqlType string) string {
	normalizedType := strings.ToLower(sqlType)

	if idx := strings.Index(normalizedType, "("); idx != -1 {
		normalizedType = normalizedType[:idx]
	}

	if idx := strings.Index(normalizedType, ";"); idx != -1 {
		normalizedType = normalizedType[:idx]
	}

	switch normalizedType {
	case "int4", "serial":
		return "integer"
	case "int8", "bigserial":
		return "bigint"
	case "int2", "smallserial":
		return "smallint"
	case "float4":
		return "real"
	case "float8":
		return "double precision"
	case "bool":
		return "boolean"
	case "time with time zone":
		return "timetz"
	case "character varying", "varying character":
		return "varchar"
	case "character":
		return "char"
	case "integer[]", "integer[][]":
		return "_integer"
	case "text[]":
		return "_text"
	case "native character", "nchar":
		return "char"
	case "nvarchar":
		return "varchar"
	case "unsigned big int":
		return "bigint"
	}

	return normalizedType
}

// FormatFieldName formats field name.
func FormatFieldName(dbColumnName string) string {
	// The conventional primary-key column is the only structural exception.
	// All other schema vocabulary is converted mechanically.
	if dbColumnName == "id" {
		return "ID"
	}
	return naming.ToPascalCase(dbColumnName)
}

// FormatDisplayName formats display name.
func FormatDisplayName(dbColumnName string) string {
	parts := strings.Split(dbColumnName, "_")

	var builder strings.Builder
	builder.Grow(len(dbColumnName) + len(parts) - 1)

	for i, part := range parts {
		if len(part) > 0 {
			if i > 0 {
				builder.WriteString(" ")
			}
			builder.WriteString(strings.ToUpper(part[:1]))
			builder.WriteString(strings.ToLower(part[1:]))
		}
	}
	return builder.String()
}

// FormatCamelCase formats camel case.
func FormatCamelCase(dbColumnName string) string {
	parts := strings.Split(dbColumnName, "_")
	if len(parts) == 0 {
		return dbColumnName
	}

	var builder strings.Builder
	builder.Grow(len(dbColumnName))

	builder.WriteString(strings.ToLower(parts[0]))
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			builder.WriteString(strings.ToUpper(parts[i][:1]))
			builder.WriteString(strings.ToLower(parts[i][1:]))
		}
	}
	return builder.String()
}

// GetDatabaseType returns database type.
func (tm *TypeMapper) GetDatabaseType() string {
	return tm.DatabaseType
}
