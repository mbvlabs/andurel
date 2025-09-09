package types

import (
	"fmt"
	"strings"
)

type TypeOverride struct {
	DatabaseType string
	GoType       string
	Package      string
	Nullable     bool
}

type TypeMapper struct {
	DatabaseType string
	TypeMap      map[string]string
	Overrides    []TypeOverride
}

func NewTypeMapper(databaseType string) *TypeMapper {
	tm := &TypeMapper{
		DatabaseType: databaseType,
		TypeMap:      make(map[string]string),
		Overrides:    make([]TypeOverride, 0),
	}

	if databaseType == "postgresql" {
		tm.initPostgreSQLMappings()
	}

	return tm
}

func (tm *TypeMapper) initPostgreSQLMappings() {
	tm.TypeMap["uuid"] = "uuid.UUID"
	tm.TypeMap["varchar"] = "string"
	tm.TypeMap["text"] = "string"
	tm.TypeMap["char"] = "string"
	tm.TypeMap["bytea"] = "[]byte"
	tm.TypeMap["bool"] = "bool"
	tm.TypeMap["boolean"] = "bool"
	tm.TypeMap["timestamp"] = "time.Time"
	tm.TypeMap["timestamptz"] = "time.Time"
	tm.TypeMap["timestamp with time zone"] = "time.Time"
	tm.TypeMap["timestamp without time zone"] = "time.Time"
	tm.TypeMap["date"] = "time.Time"
	tm.TypeMap["time"] = "time.Time"
	tm.TypeMap["jsonb"] = "interface{}"
	tm.TypeMap["json"] = "interface{}"
	tm.TypeMap["int"] = "int32"
	tm.TypeMap["integer"] = "int32"
	tm.TypeMap["int4"] = "int32"
	tm.TypeMap["serial"] = "int32"
	tm.TypeMap["bigint"] = "int64"
	tm.TypeMap["int8"] = "int64"
	tm.TypeMap["bigserial"] = "int64"
	tm.TypeMap["smallint"] = "int16"
	tm.TypeMap["int2"] = "int16"
	tm.TypeMap["smallserial"] = "int16"
	tm.TypeMap["decimal"] = "float64"
	tm.TypeMap["numeric"] = "float64"
	tm.TypeMap["real"] = "float32"
	tm.TypeMap["float4"] = "float32"
	tm.TypeMap["double precision"] = "float64"
	tm.TypeMap["float8"] = "float64"
}

func (tm *TypeMapper) MapSQLTypeToGo(
	sqlType string,
	nullable bool,
) (goType, sqlcType, packageName string, err error) {
	normalizedType := normalizeSQLType(sqlType)

	for _, override := range tm.Overrides {
		if override.DatabaseType == normalizedType &&
			override.Nullable == nullable {
			return override.GoType, "", override.Package, nil
		}
	}

	baseGoType, exists := tm.TypeMap[normalizedType]
	if !exists {
		return "interface{}", "interface{}", "", nil
	}

	var pkg string
	switch baseGoType {
	case "uuid.UUID":
		pkg = "github.com/google/uuid"
	case "time.Time":
		pkg = "time"
	}

	if nullable {
		sqlcType, goType = tm.mapNullableType(normalizedType, baseGoType)
	} else {
		goType = baseGoType
		sqlcType = tm.getSQLCType(normalizedType, baseGoType)
	}

	return goType, sqlcType, pkg, nil
}

func (tm *TypeMapper) getSQLCType(sqlType, baseGoType string) string {
	switch baseGoType {
	case "time.Time":
		switch sqlType {
		case "timestamp", "timestamp without time zone":
			return "pgtype.Timestamp"
		case "timestamptz", "timestamp with time zone":
			return "pgtype.Timestamptz"
		default:
			return "pgtype.Timestamptz"
		}
	default:
		return baseGoType
	}
}

func (tm *TypeMapper) mapNullableType(
	sqlType, baseGoType string,
) (sqlcType, goType string) {
	switch baseGoType {
	case "time.Time":
		switch sqlType {
		case "timestamp", "timestamp without time zone":
			return "pgtype.Timestamp", "time.Time"
		case "timestamptz", "timestamp with time zone":
			return "pgtype.Timestamptz", "time.Time"
		default:
			return "pgtype.Timestamptz", "time.Time"
		}
	case "string":
		return "sql.NullString", "string"
	case "bool":
		return "sql.NullBool", "bool"
	case "int16":
		return "sql.NullInt32", "int16"
	case "int32":
		return "sql.NullInt32", "int32"
	case "int64":
		return "sql.NullInt64", "int64"
	case "float32":
		return "sql.NullFloat64", "float32"
	case "float64":
		if sqlType == "decimal" || sqlType == "numeric" {
			return "pgtype.Numeric", "float64"
		}
		return "sql.NullFloat64", "float64"
	case "[]byte":
		return "[]byte", "[]byte"
	case "uuid.UUID":
		return "uuid.UUID", "uuid.UUID"
	default:
		return "interface{}", baseGoType
	}
}

func (tm *TypeMapper) GenerateConversionFromDB(fieldName, sqlcType, goType string) string {
	if sqlcType == goType {
		return fmt.Sprintf("row.%s", fieldName)
	}

	switch sqlcType {
	case "pgtype.Timestamptz", "pgtype.Timestamp":
		return fmt.Sprintf("row.%s.Time", fieldName)
	case "sql.NullString":
		return fmt.Sprintf("row.%s.String", fieldName)
	case "sql.NullBool":
		return fmt.Sprintf("row.%s.Bool", fieldName)
	case "sql.NullInt32":
		return fmt.Sprintf("row.%s.Int32", fieldName)
	case "sql.NullInt64":
		return fmt.Sprintf("row.%s.Int64", fieldName)
	case "sql.NullFloat64":
		return fmt.Sprintf("row.%s.Float64", fieldName)
	case "pgtype.Numeric":
		return fmt.Sprintf(
			"func() float64 { if row.%s.Valid { f, _ := row.%s.Float64Value(); return f.Float64 }; return 0 }()",
			fieldName,
			fieldName,
		)
	default:
		return fmt.Sprintf("row.%s", fieldName)
	}
}

func (tm *TypeMapper) GenerateConversionToDB(
	sqlcType, goType string,
	valueExpr string,
) string {
	if sqlcType == goType {
		return valueExpr
	}

	switch sqlcType {
	case "pgtype.Timestamptz":
		return fmt.Sprintf(
			"pgtype.Timestamptz{Time: %s, Valid: true}",
			valueExpr,
		)
	case "pgtype.Timestamp":
		return fmt.Sprintf(
			"pgtype.Timestamp{Time: %s, Valid: true}",
			valueExpr,
		)
	case "sql.NullString":
		return fmt.Sprintf(
			"sql.NullString{String: %s, Valid: %s != \"\"}",
			valueExpr,
			valueExpr,
		)
	case "sql.NullBool":
		return fmt.Sprintf("sql.NullBool{Bool: %s, Valid: true}", valueExpr)
	case "sql.NullInt32":
		return fmt.Sprintf("sql.NullInt32{Int32: %s, Valid: true}", valueExpr)
	case "sql.NullInt64":
		return fmt.Sprintf("sql.NullInt64{Int64: %s, Valid: true}", valueExpr)
	case "sql.NullFloat64":
		return fmt.Sprintf(
			"sql.NullFloat64{Float64: %s, Valid: true}",
			valueExpr,
		)
	case "pgtype.Numeric":
		return fmt.Sprintf(
			"func() pgtype.Numeric { var n pgtype.Numeric; _ = n.Scan(%s); return n }()",
			valueExpr,
		)
	default:
		return valueExpr
	}
}

func (tm *TypeMapper) GenerateZeroCheck(
	goType string,
	valueExpr string,
) string {
	switch goType {
	case "string":
		return fmt.Sprintf("%s != \"\"", valueExpr)
	case "time.Time":
		return fmt.Sprintf("!%s.IsZero()", valueExpr)
	case "bool":
		return "true"
	case "int16", "int32", "int64", "float32", "float64":
		return fmt.Sprintf("%s != 0", valueExpr)
	case "uuid.UUID":
		return fmt.Sprintf("%s != uuid.Nil", valueExpr)
	case "[]byte":
		return fmt.Sprintf("len(%s) > 0", valueExpr)
	default:
		return "true"
	}
}

func normalizeSQLType(sqlType string) string {
	normalizedType := strings.ToLower(sqlType)

	if idx := strings.Index(normalizedType, "("); idx != -1 {
		normalizedType = normalizedType[:idx]
	}

	switch normalizedType {
	case "int4":
		return "integer"
	case "int8":
		return "bigint"
	case "int2":
		return "smallint"
	case "float4":
		return "real"
	case "float8":
		return "double precision"
	case "bool":
		return "boolean"
	}

	return normalizedType
}

func FormatFieldName(dbColumnName string) string {
	if dbColumnName == "id" {
		return "ID"
	}

	parts := strings.Split(dbColumnName, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}

	return strings.Join(parts, "")
}

func FormatDisplayName(dbColumnName string) string {
	parts := strings.Split(dbColumnName, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}

func FormatCamelCase(dbColumnName string) string {
	parts := strings.Split(dbColumnName, "_")
	if len(parts) == 0 {
		return dbColumnName
	}

	result := strings.ToLower(parts[0])
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			result += strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
		}
	}
	return result
}