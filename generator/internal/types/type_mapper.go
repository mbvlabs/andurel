package types

import (
	"strings"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

// TypeOverride lets users map a SQL database type to a custom Go type.
// When the column is nullable, the resulting Go type will be wrapped
// according to the TypeMapper's NullType setting.
type TypeOverride struct {
	DatabaseType string
	GoType       string
	Package      string
}

type TypeMapper struct {
	DatabaseType string
	NullType     string // "pointer", "sql.Null", or "bun.Null"
	Overrides    []TypeOverride
}

func NewTypeMapper(databaseType string) *TypeMapper {
	return &TypeMapper{
		DatabaseType: databaseType,
		NullType:     "sql.Null",
		Overrides:    make([]TypeOverride, 0),
	}
}

// sqlNullTypeMap maps base Go types to their database/sql null equivalent.
var sqlNullTypeMap = map[string]string{
	"string":  "sql.NullString",
	"bool":    "sql.NullBool",
	"int16":   "sql.NullInt16",
	"int32":   "sql.NullInt32",
	"int64":   "sql.NullInt64",
	"float64": "sql.NullFloat64",
	"time.Time": "sql.NullTime",
}

// bunNullTypeMap maps base Go types to their bun null equivalent.
var bunNullTypeMap = map[string]string{
	"string":  "bun.NullString",
	"bool":    "bun.NullBool",
	"int32":   "bun.NullInt32",
	"int64":   "bun.NullInt64",
	"float64": "bun.NullFloat64",
	"time.Time": "bun.NullTime",
}

// MapSQLTypeToGo returns the Go type for a SQL column. Nullable columns are
// wrapped according to tm.NullType ("pointer" → *string, "sql.Null" →
// sql.NullString, "bun.Null" → bun.NullString). The second return value is
// the import path required for the type, or "" if it is a builtin.
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

	base, pkg := tm.basePostgresType(normalized)
	if base == "" {
		return "any", "", nil
	}

	return tm.wrapNullable(base, nullable), pkg, nil
}

// BuildBunTag returns the value of the `bun:"..."` struct tag for a column.
// Only emits attributes that affect query/marshaling behavior — column name,
// primary-key marker, and a `type:` hint where bun's default mapping would
// otherwise be wrong (notably uuid columns). DDL-only attributes
// (notnull/nullzero/default/unique/autoincrement) are intentionally omitted
// because andurel does not use bun for schema management.
func (tm *TypeMapper) BuildBunTag(col *catalog.Column) string {
	parts := []string{col.Name}

	if col.IsPrimaryKey {
		parts = append(parts, "pk")
	}

	normalized := normalizeSQLType(col.DataType)
	switch normalized {
	case "uuid":
		parts = append(parts, "type:uuid")
	case "jsonb":
		parts = append(parts, "type:jsonb")
	}

	if col.IsAutoIncrement {
		parts = append(parts, "autoincrement")
	}

	return strings.Join(parts, ",")
}

func (tm *TypeMapper) wrapNullable(goType string, nullable bool) string {
	if !nullable {
		return goType
	}
	if strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") {
		return goType
	}

	switch tm.NullType {
	case "sql.Null":
		if nt, ok := sqlNullTypeMap[goType]; ok {
			return nt
		}
	case "bun.Null":
		if nt, ok := bunNullTypeMap[goType]; ok {
			return nt
		}
	}
	// Default to pointer for types without a null type equivalent.
	return "*" + goType
}

func (tm *TypeMapper) basePostgresType(
	normalized string,
) (goType, packageName string) {
	switch normalized {
	case "uuid":
		return "uuid.UUID", "github.com/google/uuid"
	case "varchar", "text", "char",
		"xml", "tsvector", "tsquery",
		"inet", "cidr", "macaddr", "macaddr8",
		"point", "lseg", "box", "path", "polygon", "circle",
		"int4range", "int8range", "numrange",
		"tsrange", "tstzrange", "daterange",
		"money", "bit", "varbit",
		"interval":
		return "string", ""
	case "bytea":
		return "[]byte", ""
	case "boolean":
		return "bool", ""
	case "smallint":
		return "int16", ""
	case "integer":
		return "int32", ""
	case "bigint":
		return "int64", ""
	case "real":
		return "float32", ""
	case "double precision":
		return "float64", ""
	case "decimal", "numeric":
		return "float64", ""
	case "timestamp", "timestamp without time zone",
		"timestamptz", "timestamp with time zone",
		"date", "time", "timetz":
		return "time.Time", "time"
	case "json", "jsonb":
		return "[]byte", ""
	case "_integer":
		return "[]int32", ""
	case "_text":
		return "[]string", ""
	}

	return "", ""
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

func FormatFieldName(dbColumnName string) string {
	if dbColumnName == "id" {
		return "ID"
	}

	parts := strings.Split(dbColumnName, "_")

	var builder strings.Builder
	builder.Grow(len(dbColumnName))

	for _, part := range parts {
		if len(part) > 0 && part == "id" {
			builder.WriteString(strings.ToUpper(part))
		}

		if len(part) > 0 && part != "id" {
			builder.WriteString(strings.ToUpper(part[:1]))
			builder.WriteString(strings.ToLower(part[1:]))
		}
	}

	return builder.String()
}

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

func (tm *TypeMapper) GetDatabaseType() string {
	return tm.DatabaseType
}
