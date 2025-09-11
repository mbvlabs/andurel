package types

import (
	"errors"
	"fmt"
	"log"
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

	if databaseType == "postgresql" {
		tm.initPostgreSQLMappings()
	} else if databaseType == "sqlite" {
		tm.initSQLiteMappings()
	}

	return tm
}

func (tm *TypeMapper) initPostgreSQLMappings() {
	tm.TypeMap["uuid"] = "uuid.UUID"
}

func (tm *TypeMapper) initSQLiteMappings() {
	// SQLite has a more flexible type system based on type affinity
	// We'll handle the main type affinities here
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
		return "uuid.UUID", "uuid.UUID", "github.com/google/uuid", nil
	}

	if tm.DatabaseType == "postgresql" {
		goType, sqlcType, packageName = tm.getPostgreSQLType(normalizedType, nullable)
	} else if tm.DatabaseType == "sqlite" {
		goType, sqlcType, packageName = tm.getSQLiteType(normalizedType, nullable)
	} else {
		goType, sqlcType, packageName = tm.getPostgreSQLType(normalizedType, nullable)
	}
	
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
	
	// Handle SQLite sql.Null* types
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

func (tm *TypeMapper) getSQLiteType(
	normalizedType string,
	nullable bool,
) (goType, sqlcType, packageName string) {
	// SQLite uses type affinity system with 5 storage classes:
	// TEXT, NUMERIC, INTEGER, REAL, BLOB
	
	switch normalizedType {
	// TEXT affinity - strings
	case "varchar", "text", "char", "clob", "character", "varying character", "nchar", "native character", "nvarchar":
		if nullable {
			return "string", "sql.NullString", "database/sql"
		}
		return "string", "string", ""
		
	// INTEGER affinity - integers
	case "int", "integer", "tinyint", "smallint", "mediumint", "bigint", "unsigned big int", "int2", "int8":
		if nullable {
			return "int64", "sql.NullInt64", "database/sql"
		}
		return "int64", "int64", ""
		
	// REAL affinity - floating point
	case "real", "double", "double precision", "float":
		if nullable {
			return "float64", "sql.NullFloat64", "database/sql"
		}
		return "float64", "float64", ""
		
	// NUMERIC affinity - can store any type, but we'll map common cases
	case "numeric", "decimal", "boolean", "date", "datetime":
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
		// Default numeric to float64
		if nullable {
			return "float64", "sql.NullFloat64", "database/sql"
		}
		return "float64", "float64", ""
		
	// BLOB affinity - binary data
	case "blob":
		return "[]byte", "[]byte", ""
		
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
	
	// Handle SQLite sql.Null* types
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
	// PostgreSQL-specific normalizations
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
	
	// SQLite-specific normalizations
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
		if len(part) > 0 {
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

// DatabaseType returns the database type for this type mapper
func (tm *TypeMapper) GetDatabaseType() string {
	return tm.DatabaseType
}

type SQLCConfig struct {
	Version string `yaml:"version"`
	SQL     []struct {
		Schema  string `yaml:"schema"`
		Queries string `yaml:"queries"`
		Engine  string `yaml:"engine"`
		Gen     struct {
			Go struct {
				Package                   string `yaml:"package"`
				Out                       string `yaml:"out"`
				OutputDBFileName          string `yaml:"output_db_file_name"`
				OutputModelsFileName      string `yaml:"output_models_file_name"`
				EmitMethodsWithDBArgument bool   `yaml:"emit_methods_with_db_argument"`
				SQLPackage                string `yaml:"sql_package"`
				Overrides                 []struct {
					DBType string `yaml:"db_type"`
					GoType string `yaml:"go_type"`
				} `yaml:"overrides"`
			} `yaml:"go"`
		} `yaml:"gen"`
	} `yaml:"sql"`
}

func ValidateSQLCConfig(projectPath string) error {
	sqlcPath := filepath.Join(projectPath, "database", "sqlc.yaml")

	if _, err := os.Stat(sqlcPath); os.IsNotExist(err) {
		log.Printf("Warning: sqlc.yaml not found at %s", sqlcPath)
		return nil
	}

	data, err := os.ReadFile(sqlcPath)
	if err != nil {
		return fmt.Errorf("failed to read sqlc.yaml: %w", err)
	}

	var config SQLCConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse sqlc.yaml: %w", err)
	}

	if len(config.SQL) == 0 {
		return fmt.Errorf("no SQL configurations found in sqlc.yaml")
	}

	sqlConfig := config.SQL[0]
	overrides := sqlConfig.Gen.Go.Overrides

	var invalidOverrides []string
	for _, override := range overrides {
		if override.DBType != "uuid" {
			invalidOverrides = append(
				invalidOverrides,
				fmt.Sprintf("%s -> %s", override.DBType, override.GoType),
			)
		} else if override.GoType != "github.com/google/uuid.UUID" {
			invalidOverrides = append(invalidOverrides, fmt.Sprintf("uuid should map to 'github.com/google/uuid.UUID', not '%s'", override.GoType))
		}
	}

	if len(invalidOverrides) > 0 {
		log.Printf("WARNING: Invalid type overrides found in %s:", sqlcPath)
		log.Printf("The generator only supports pgtype types (except for uuid).")
		log.Printf("Please remove the following overrides:")
		for _, override := range invalidOverrides {
			log.Printf("  - %s", override)
		}
		log.Printf(
			"Only UUID override is supported: db_type: 'uuid' -> go_type: 'github.com/google/uuid.UUID'",
		)

		return errors.New("invalid type overrides in sqlc.yaml")
	}

	return nil
}
