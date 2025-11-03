package controllers

import (
	"fmt"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/types"
)

type ControllerType int

const (
	ResourceController ControllerType = iota
	ResourceControllerNoViews
	NormalController
)

type GeneratedField struct {
	Name          string
	GoType        string
	GoFormType    string
	DBName        string
	IsSystemField bool
}

type GeneratedController struct {
	ResourceName string
	PluralName   string
	Package      string
	Fields       []GeneratedField
	ModulePath   string
	Type         ControllerType
	DatabaseType string
}

type Config struct {
	ResourceName   string
	PluralName     string
	PackageName    string
	ModulePath     string
	ControllerType ControllerType
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

func (g *Generator) Build(cat *catalog.Catalog, config Config) (*GeneratedController, error) {
	controller := &GeneratedController{
		ResourceName: config.ResourceName,
		PluralName:   config.PluralName,
		Package:      config.PackageName,
		ModulePath:   config.ModulePath,
		Type:         config.ControllerType,
		DatabaseType: g.typeMapper.GetDatabaseType(),
		Fields:       make([]GeneratedField, 0),
	}

	if config.ControllerType == ResourceController ||
		config.ControllerType == ResourceControllerNoViews {
		table, err := cat.GetTable("", config.PluralName)
		if err != nil {
			return nil, fmt.Errorf("table %s not found: %w", config.PluralName, err)
		}

		for _, col := range table.Columns {
			field, err := g.buildField(col)
			if err != nil {
				return nil, fmt.Errorf("failed to build field for column %s: %w", col.Name, err)
			}
			controller.Fields = append(controller.Fields, field)
		}
	}

	return controller, nil
}

func (g *Generator) buildField(col *catalog.Column) (GeneratedField, error) {
	var goType string
	var err error

	// Special handling for ID fields in SQLite - always use uuid.UUID
	if col.Name == "id" && g.typeMapper.GetDatabaseType() == "sqlite" {
		goType = "uuid.UUID"
	} else {
		goType, _, _, err = g.typeMapper.MapSQLTypeToGo(col.DataType, col.IsNullable)
		if err != nil {
			return GeneratedField{}, err
		}
	}

	field := GeneratedField{
		Name:          types.FormatFieldName(col.Name),
		GoType:        goType,
		DBName:        col.Name,
		IsSystemField: col.Name == "created_at" || col.Name == "updated_at" || col.Name == "id",
	}

	switch goType {
	case "time.Time":
		field.GoFormType = "time.Time"
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
	default:
		field.GoFormType = "string"
	}

	return field, nil
}
