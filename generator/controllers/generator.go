package controllers

import (
	"fmt"
	"strings"

	"github.com/jinzhu/inflection"
	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/types"
	"github.com/mbvlabs/andurel/generator/internal/validation"
	"github.com/mbvlabs/andurel/pkg/naming"
)

type ControllerType int

const (
	ResourceController ControllerType = iota
	NormalController
)

type GeneratedField struct {
	Name          string
	GoType        string
	GoFormType    string
	DBName        string
	CamelCase     string
	IsSystemField bool
	IsPointer     bool
}

type GeneratedController struct {
	ResourceName            string
	ModelName               string
	PluralName              string
	ModelPluralName         string
	PluralResourceName      string // The pluralized form of ResourceName (respects --table-name override)
	ModelPluralResourceName string
	ReceiverName            string // Short receiver name for methods (e.g., "sf" for StudentFeedback)
	Namespace               string // "admin" (empty if no namespace)
	NamespacePascal         string // "Admin" (PascalCase for prefixing)
	Package                 string
	Fields                  []GeneratedField
	ModulePath              string
	Type                    ControllerType
	DatabaseType            string
	TableNameOverridden     bool
	IDType                  string // "uuid.UUID", "int32", "int64", "string"
	IsAutoIncrementID       bool   // True for serial/bigserial
	IDGoFieldName           string // Go struct field name of PK (e.g., "ID", "UserID")
	HasPrimaryKey           bool   // Whether the table has any primary key
	Actions                 []string
	IsAPI                   bool // Generate JSON API controller under controllers/api
}

type Config struct {
	ResourceName             string
	ModelName                string
	PluralName               string
	ModelPluralName          string
	TableName                string
	ModelTableName           string
	Namespace                string
	PackageName              string
	ModulePath               string
	ControllerType           ControllerType
	TableNameOverridden      bool
	ModelTableNameOverridden bool
	PrimaryKeyColumn         string // Override PK column name (empty = auto-detect)
	Actions                  []string
	IsAPI                    bool // Controller is JSON API
}

type Generator struct {
	typeMapper  *types.TypeMapper
	fileManager files.Manager
}

func NewGenerator(databaseType string) *Generator {
	return &Generator{
		typeMapper:  types.NewTypeMapper(databaseType),
		fileManager: files.NewUnifiedFileManager(),
	}
}

func (g *Generator) SetNullType(nullType string) {
	g.typeMapper.NullType = nullType
}

func (g *Generator) Build(cat *catalog.Catalog, config Config) (*GeneratedController, error) {
	modelName := config.ModelName
	if modelName == "" {
		modelName = config.ResourceName
	}
	modelPluralName := config.ModelPluralName
	if modelPluralName == "" {
		modelPluralName = config.PluralName
	}
	// Compute PluralResourceName: use resource name as-is when table name is overridden,
	// otherwise use standard pluralization
	pluralResourceName := inflection.Plural(config.ResourceName)
	if config.TableNameOverridden {
		pluralResourceName = config.ResourceName
	}
	modelPluralResourceName := inflection.Plural(modelName)
	if config.ModelTableNameOverridden {
		modelPluralResourceName = modelName
	}

	controller := &GeneratedController{
		ResourceName:            config.ResourceName,
		ModelName:               modelName,
		PluralName:              config.PluralName,
		ModelPluralName:         modelPluralName,
		PluralResourceName:      pluralResourceName,
		ModelPluralResourceName: modelPluralResourceName,
		ReceiverName:            naming.ToReceiverName(config.ResourceName),
		Namespace:               config.Namespace,
		NamespacePascal:         naming.ToPascalCase(config.Namespace),
		Package:                 config.PackageName,
		ModulePath:              config.ModulePath,
		Type:                    config.ControllerType,
		DatabaseType:            g.typeMapper.GetDatabaseType(),
		TableNameOverridden:     config.TableNameOverridden,
		Fields:                  make([]GeneratedField, 0),
		IDType:                  "uuid.UUID", // Default to UUID
		Actions:                 config.Actions,
		IsAPI:                   config.IsAPI,
	}

	if config.ControllerType == ResourceController {
		tableName := config.TableName
		if config.ModelTableName != "" {
			tableName = config.ModelTableName
		}
		if tableName == "" {
			tableName = modelPluralName
		}
		table, err := cat.GetTable("", tableName)
		if err != nil {
			return nil, fmt.Errorf("table %s not found: %w", tableName, err)
		}

		for _, col := range table.Columns {
			field, err := g.buildField(col)
			if err != nil {
				return nil, fmt.Errorf("failed to build field for column %s: %w", col.Name, err)
			}
			controller.Fields = append(controller.Fields, field)
		}

		// Three-pass PK detection:
		// 1. Use config override if provided
		// 2. Look for column named "id" that is primary key
		// 3. Fall back to any column with IsPrimaryKey flag
		if config.PrimaryKeyColumn != "" {
			for _, col := range table.Columns {
				if col.Name == config.PrimaryKeyColumn {
					setControllerPK(controller, col)
					controller.IDGoFieldName = types.FormatFieldName(col.Name)
					controller.HasPrimaryKey = true
					break
				}
			}
		} else {
			for _, col := range table.Columns {
				if col.Name == "id" && col.IsPrimaryKey {
					setControllerPK(controller, col)
					controller.IDGoFieldName = types.FormatFieldName(col.Name)
					controller.HasPrimaryKey = true
					break
				}
			}
			if !controller.HasPrimaryKey {
				for _, col := range table.Columns {
					if col.IsPrimaryKey {
						setControllerPK(controller, col)
						controller.IDGoFieldName = types.FormatFieldName(col.Name)
						controller.HasPrimaryKey = true
						break
					}
				}
			}
		}
	}

	return controller, nil
}

func setControllerPK(controller *GeneratedController, col *catalog.Column) {
	pkType, _ := validation.ClassifyPrimaryKeyType(col.DataType)
	controller.IDType = validation.GoType(pkType)
	controller.IsAutoIncrementID = validation.IsAutoIncrement(col.DataType)
}

// isNullableType returns true if the given type is a pointer or a null-wrapper type.
func isNullableType(goType string) bool {
	if strings.HasPrefix(goType, "*") {
		return true
	}
	switch goType {
	case "sql.NullString", "sql.NullBool", "sql.NullInt16", "sql.NullInt32",
		"sql.NullInt64", "sql.NullFloat64", "sql.NullTime",
		"bun.NullString", "bun.NullBool", "bun.NullInt32", "bun.NullInt64",
		"bun.NullFloat64", "bun.NullTime":
		return true
	}
	return false
}

// resolveControllerBaseType strips null-type wrappers and pointer prefixes.
func resolveControllerBaseType(goType string) string {
	switch goType {
	case "sql.NullString", "bun.NullString":
		return "string"
	case "sql.NullBool", "bun.NullBool":
		return "bool"
	case "sql.NullInt16":
		return "int16"
	case "sql.NullInt32", "bun.NullInt32":
		return "int32"
	case "sql.NullInt64", "bun.NullInt64":
		return "int64"
	case "sql.NullFloat64", "bun.NullFloat64":
		return "float64"
	case "sql.NullTime", "bun.NullTime":
		return "time.Time"
	}
	return strings.TrimPrefix(goType, "*")
}

func (g *Generator) buildField(col *catalog.Column) (GeneratedField, error) {
	var goType string
	var err error

	goType, _, err = g.typeMapper.MapSQLTypeToGo(col.DataType, col.IsNullable)
	if err != nil {
		return GeneratedField{}, err
	}

	baseGoType := resolveControllerBaseType(goType)

	field := GeneratedField{
		Name:          types.FormatFieldName(col.Name),
		GoType:        goType,
		DBName:        col.Name,
		CamelCase:     types.FormatCamelCase(col.Name),
		IsSystemField: col.Name == "created_at" || col.Name == "updated_at" || col.IsPrimaryKey,
		IsPointer:     isNullableType(goType),
	}

	switch baseGoType {
	case "time.Time":
		field.GoFormType = "time.Time"
	case "uuid.UUID":
		field.GoFormType = "string"
	case "int16":
		field.GoFormType = "int16"
	case "int32":
		field.GoFormType = "int32"
	case "int64":
		field.GoFormType = "int64"
	case "float32":
		field.GoFormType = "float32"
	case "float64":
		field.GoFormType = "float64"
	case "bool":
		field.GoFormType = "bool"
	case "[]string":
		field.GoFormType = "[]string"
	case "[]int32":
		field.GoFormType = "[]int32"
	default:
		if strings.HasPrefix(goType, "sql.Null") || strings.HasPrefix(goType, "bun.Null") {
			field.GoFormType = "string"
		} else if isNullableType(goType) {
			field.GoFormType = goType
		} else {
			field.GoFormType = "string"
		}
	}

	return field, nil
}
