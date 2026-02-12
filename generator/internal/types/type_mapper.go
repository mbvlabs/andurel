package types

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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

	switch databaseType {
	case "postgresql":
		tm.initPostgreSQLMappings()
	}

	return tm
}

func (tm *TypeMapper) initPostgreSQLMappings() {
	tm.TypeMap["uuid"] = "uuid.UUID"
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

	if normalizedType == "uuid" {
		if nullable && tm.DatabaseType == "postgresql" {
			return "uuid.UUID", "pgtype.UUID", "github.com/jackc/pgx/v5/pgtype", nil
		}
		return "uuid.UUID", "uuid.UUID", "github.com/google/uuid", nil
	}

	goType, sqlcType, packageName = tm.getPostgreSQLType(normalizedType, nullable)

	if goType == "" {
		return "interface{}", "interface{}", "", nil
	}

	return goType, sqlcType, packageName, nil
}

func (tm *TypeMapper) GenerateConversionFromDB(fieldName, sqlcType, goType string) string {
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
		case "pgtype.UUID":
			return fmt.Sprintf("uuid.UUID(row.%s.Bytes)", fieldName)
		case "pgtype.JSONB", "pgtype.JSON":
			// pgtype.JSONB and pgtype.JSON are type aliases for []byte in pgx v5
			return fmt.Sprintf("row.%s", fieldName)
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
		default:
			return fmt.Sprintf("row.%s", fieldName)
		}
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

func (tm *TypeMapper) getPostgreSQLType(
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
	case "point":
		if nullable {
			return "string", "pgtype.Point", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Point", "github.com/jackc/pgx/v5/pgtype"
	case "lseg":
		if nullable {
			return "string", "pgtype.Lseg", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Lseg", "github.com/jackc/pgx/v5/pgtype"
	case "box":
		if nullable {
			return "string", "pgtype.Box", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Box", "github.com/jackc/pgx/v5/pgtype"
	case "path":
		if nullable {
			return "string", "pgtype.Path", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Path", "github.com/jackc/pgx/v5/pgtype"
	case "polygon":
		if nullable {
			return "string", "pgtype.Polygon", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Polygon", "github.com/jackc/pgx/v5/pgtype"
	case "circle":
		if nullable {
			return "string", "pgtype.Circle", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Circle", "github.com/jackc/pgx/v5/pgtype"
	case "int4range":
		if nullable {
			return "string", "pgtype.Int4range", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Int4range", "github.com/jackc/pgx/v5/pgtype"
	case "int8range":
		if nullable {
			return "string", "pgtype.Int8range", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Int8range", "github.com/jackc/pgx/v5/pgtype"
	case "numrange":
		if nullable {
			return "string", "pgtype.Numrange", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Numrange", "github.com/jackc/pgx/v5/pgtype"
	case "tsrange":
		if nullable {
			return "string", "pgtype.Tsrange", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Tsrange", "github.com/jackc/pgx/v5/pgtype"
	case "tstzrange":
		if nullable {
			return "string", "pgtype.Tstzrange", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Tstzrange", "github.com/jackc/pgx/v5/pgtype"
	case "daterange":
		if nullable {
			return "string", "pgtype.Daterange", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Daterange", "github.com/jackc/pgx/v5/pgtype"
	case "money":
		if nullable {
			return "string", "pgtype.Money", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Money", "github.com/jackc/pgx/v5/pgtype"
	case "bit":
		if nullable {
			return "string", "pgtype.Bit", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Bit", "github.com/jackc/pgx/v5/pgtype"
	case "varbit":
		if nullable {
			return "string", "pgtype.Varbit", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "pgtype.Varbit", "github.com/jackc/pgx/v5/pgtype"
	case "xml":
		if nullable {
			return "string", "pgtype.Text", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "string", ""
	case "tsvector":
		if nullable {
			return "string", "pgtype.Text", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "string", ""
	case "tsquery":
		if nullable {
			return "string", "pgtype.Text", "github.com/jackc/pgx/v5/pgtype"
		}
		return "string", "string", ""
	case "_integer":
		// sqlc generates []int32 directly for integer[] columns, no pgtype wrapper
		return "[]int32", "[]int32", ""
	case "_text":
		// sqlc generates []string directly for text[] columns, no pgtype wrapper
		return "[]string", "[]string", ""
	default:
		return "", "", ""
	}
}

func (tm *TypeMapper) GenerateConversionToDB(
	sqlcType string,
	goType string,
	valueExpr string,
) string {
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
		case "pgtype.JSONB", "pgtype.JSON":
			// JSONB and JSON types accept []byte directly without wrapping
			return valueExpr
		case "pgtype.UUID":
			return fmt.Sprintf("pgtype.UUID{Bytes: %s, Valid: true}", valueExpr)
		case "pgtype.Inet", "pgtype.CIDR", "pgtype.Macaddr", "pgtype.Macaddr8":
			return fmt.Sprintf("pgtype.Inet{IPNet: %s, Valid: true}", valueExpr)
		case "pgtype.Money":
			return fmt.Sprintf("pgtype.Money{String: %s, Valid: true}", valueExpr)
		default:
			return valueExpr
		}
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

func (tm *TypeMapper) GenerateZeroCheck(
	goType string,
	valueExpr string,
) string {
	switch goType {
	case "uuid.UUID":
		return fmt.Sprintf("%s != uuid.Nil", valueExpr)
	case "int16", "int32", "int64", "int":
		return fmt.Sprintf("%s != 0", valueExpr)
	case "string":
		return fmt.Sprintf("%s != \"\"", valueExpr)
	default:
		if strings.HasPrefix(goType, "pgtype.") {
			return fmt.Sprintf("%s.Valid", valueExpr)
		}
		if strings.HasPrefix(goType, "sql.Null") {
			return fmt.Sprintf("%s.Valid", valueExpr)
		}
		return "true"
	}
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
	case "time with time zone":
		return "timetz"
	case "character varying", "varying character":
		return "varchar"
	case "character":
		return "char"
	case "integer[]":
		return "_integer"
	case "integer[][]":
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

func ValidateSQLCConfig(projectPath string) error {
	basePath := filepath.Join(projectPath, "internal", "storage", "andurel_sqlc_config.yaml")
	if _, err := os.Stat(basePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat sqlc config: %w", err)
	}

	userPath := filepath.Join(projectPath, "database", "sqlc.yaml")
	if _, err := os.Stat(userPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("missing %s", userPath)
		}
		return fmt.Errorf("failed to stat sqlc config: %w", err)
	}

	baseMap, err := readYAMLAsMap(basePath)
	if err != nil {
		return fmt.Errorf("failed to read base sqlc config: %w", err)
	}
	userMap, err := readYAMLAsMap(userPath)
	if err != nil {
		return fmt.Errorf("failed to read user sqlc config: %w", err)
	}
	if len(userMap) == 0 {
		return fmt.Errorf("database/sqlc.yaml cannot be empty")
	}
	if err := validateSQLCSubset(baseMap, userMap, basePath, userPath, ""); err != nil {
		return err
	}

	return nil
}

func readYAMLAsMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return map[string]any{}, nil
	}

	result := map[string]any{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return map[string]any{}, nil
	}
	return result, nil
}

func validateSQLCSubset(base, user any, basePath, userPath, fieldPath string) error {
	switch baseTyped := base.(type) {
	case map[string]any:
		userTyped, ok := user.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be a map", renderFieldPath(fieldPath))
		}
		for key, baseValue := range baseTyped {
			userValue, ok := userTyped[key]
			childPath := joinFieldPath(fieldPath, key)
			if !ok {
				return fmt.Errorf("missing required key %q in database/sqlc.yaml", childPath)
			}
			if err := validateSQLCSubset(baseValue, userValue, basePath, userPath, childPath); err != nil {
				return err
			}
		}
		return nil
	case []any:
		userTyped, ok := user.([]any)
		if !ok {
			return fmt.Errorf("%s must be a list", renderFieldPath(fieldPath))
		}
		for _, baseValue := range baseTyped {
			matched := false
			for _, userValue := range userTyped {
				if err := validateSQLCSubset(baseValue, userValue, basePath, userPath, fieldPath); err == nil {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Errorf(
					"database/sqlc.yaml is missing a required entry under %s defined in internal/storage/andurel_sqlc_config.yaml",
					renderFieldPath(fieldPath),
				)
			}
		}
		return nil
	default:
		if valuesEqualForField(base, user, basePath, userPath, fieldPath) {
			return nil
		}
		return fmt.Errorf("required value mismatch at %s: expected %v, got %v", renderFieldPath(fieldPath), base, user)
	}
}

func valuesEqualForField(base, user any, basePath, userPath, fieldPath string) bool {
	baseStr, baseIsString := base.(string)
	userStr, userIsString := user.(string)
	if baseIsString && userIsString && isPathField(fieldPath) {
		return resolveConfigPath(baseStr, basePath) == resolveConfigPath(userStr, userPath)
	}
	return fmt.Sprint(base) == fmt.Sprint(user)
}

func isPathField(fieldPath string) bool {
	return strings.HasSuffix(fieldPath, ".schema") ||
		strings.HasSuffix(fieldPath, ".queries") ||
		strings.HasSuffix(fieldPath, ".out")
}

func resolveConfigPath(value, configPath string) string {
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(configPath), value))
}

func joinFieldPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func renderFieldPath(fieldPath string) string {
	if fieldPath == "" {
		return "root"
	}
	return fieldPath
}
