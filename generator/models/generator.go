// Package models generates model source files from database schema metadata.
package models

import (
	"fmt"
	"go/format"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/jinzhu/inflection"
	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"github.com/mbvlabs/andurel/generator/internal/migrations"
	"github.com/mbvlabs/andurel/generator/internal/types"
	"github.com/mbvlabs/andurel/generator/internal/validation"
	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/errors"
	"github.com/mbvlabs/andurel/pkg/naming"
)

// GeneratedField describes one model field derived from a database column.
type GeneratedField struct {
	Name          string
	Type          string
	Comment       string
	Package       string
	BunTag        string // Full bun struct tag (e.g., `bun:"id,pk,type:uuid"`)
	IsForeignKey  bool
	IsNullable    bool
	IsPrimaryKey  bool
	AllowedValues []string
}

// GeneratedModel contains the template data for a generated model file.
type GeneratedModel struct {
	Name                string
	PluralName          string // The pluralized form of the supplied resource name for function names
	Package             string
	Fields              []GeneratedField
	StandardImports     []string
	ExternalImports     []string
	Imports             []string
	TableName           string
	TableNameOverride   string
	TableNameOverridden bool
	ModulePath          string
	DatabaseType        string
	IDType              string // "uuid.UUID", "int32", "int64", "string"
	IDGoType            string // Same as IDType (for template clarity)
	IsAutoIncrementID   bool   // True for serial/bigserial
	IDFieldName         string // SQL column name of PK (e.g., "id", "user_id")
	IDGoFieldName       string // Go struct field name of PK (e.g., "ID", "UserID")
	HasPrimaryKey       bool   // Whether the table has any primary key
	EntityName          string // ServerEntity (resource name + "Entity")
	NamespaceVar        string // Server (exported, package-scope)
	NamespaceType       string // server (unexported receiver type)
	ReceiverName        string // s (for the namespace methods)
	HasCreatedAt        bool
	HasUpdatedAt        bool
}

// Config controls model generation for a database table.
type Config struct {
	TableName         string
	ResourceName      string
	PackageName       string
	DatabaseType      string
	ModulePath        string
	NullType          string
	CustomTypes       []types.TypeOverride
	PrimaryKeyColumn  string // Override PK column name (empty = auto-detect)
	GenerateWithoutPK bool   // Force generation without PK handling
}

// BunModelConfig holds configuration for bun model generation
type BunModelConfig struct {
	ResourceName  string
	TableName     string
	ModulePath    string
	PackageName   string
	UseSoftDelete bool // If true, use deleted_at for soft deletes
}

// Generator builds model template data and writes model files.
type Generator struct {
	typeMapper   *types.TypeMapper
	databaseType string
}

// NewGenerator creates a new generator.
func NewGenerator(databaseType string) *Generator {
	return &Generator{
		typeMapper:   types.NewTypeMapper(databaseType),
		databaseType: databaseType,
	}
}

// BuildCatalogFromMigrations builds a catalog from migration files
func (g *Generator) BuildCatalogFromMigrations(tableName string, migrationDirs []string) (*catalog.Catalog, error) {
	allMigrations, err := migrations.DiscoverMigrations(migrationDirs)
	if err != nil {
		return nil, fmt.Errorf("failed to discover migrations: %w", err)
	}

	cat := catalog.NewCatalog("public")

	for _, migration := range allMigrations {
		for _, statement := range migration.Statements {
			if err := ddl.ApplyDDL(cat, statement, migration.FilePath, g.databaseType); err != nil {
				return nil, fmt.Errorf("failed to apply DDL from %s: %w", migration.FilePath, err)
			}
		}
	}

	return cat, nil
}

// Build converts catalog metadata and config into a generated model.
func (g *Generator) Build(cat *catalog.Catalog, config Config) (*GeneratedModel, error) {
	table, err := cat.GetTable("", config.TableName)
	if err != nil {
		return nil, errors.NewDatabaseError("get table", config.TableName, err)
	}

	g.typeMapper.Overrides = append(g.typeMapper.Overrides, config.CustomTypes...)
	if config.NullType != "" {
		g.typeMapper.NullType = config.NullType
	}

	entityName := config.ResourceName + "Entity"
	namespaceVar := config.ResourceName
	namespaceType := naming.ToLowerCamelCaseFromAny(config.ResourceName)
	receiverName := naming.ToReceiverName(config.ResourceName)

	model := &GeneratedModel{
		Name:            config.ResourceName,
		PluralName:      inflection.Plural(config.ResourceName),
		EntityName:      entityName,
		NamespaceVar:    namespaceVar,
		NamespaceType:   namespaceType,
		ReceiverName:    receiverName,
		Package:         config.PackageName,
		TableName:       config.TableName,
		ModulePath:      config.ModulePath,
		DatabaseType:    g.typeMapper.GetDatabaseType(),
		Fields:          make([]GeneratedField, 0, len(table.Columns)),
		StandardImports: make([]string, 0),
		ExternalImports: make([]string, 0),
		Imports:         make([]string, 0),
		IDFieldName:     config.PrimaryKeyColumn,
		IDGoFieldName:   "",
		HasPrimaryKey:   false,
	}

	importSet := make(map[string]bool)
	importSet["context"] = true
	importSet["errors"] = true
	importSet["time"] = true
	importSet["github.com/uptrace/bun"] = true
	if config.ModulePath != "" {
		importSet[config.ModulePath+"/internal/storage"] = true
		importSet[config.ModulePath+"/internal/validation"] = true
	}

	for _, col := range table.Columns {
		field, err := g.buildField(col)
		if err != nil {
			return nil, errors.NewGeneratorError("build field", col.Name, err)
		}
		if enum, enumErr := cat.GetEnum(table.Schema, col.DataType); enumErr == nil {
			field.Type = "string"
			field.Package = ""
			field.AllowedValues = append([]string(nil), enum.Values...)
		}

		if field.Package != "" {
			importSet[field.Package] = true
		}

		typeImports := g.addModelTypeImports(field.Type)
		for imp := range typeImports {
			importSet[imp] = true
		}

		model.Fields = append(model.Fields, field)

		if col.Name == "created_at" {
			model.HasCreatedAt = true
		}
		if col.Name == "updated_at" {
			model.HasUpdatedAt = true
		}
	}

	// Three-pass PK detection:
	// 1. Use config override if provided
	// 2. Look for column named "id" that is primary key
	// 3. Fall back to any column with IsPrimaryKey flag
	if config.PrimaryKeyColumn != "" {
		col := findColumn(table, config.PrimaryKeyColumn)
		if col != nil {
			setModelPK(model, col)
			model.IDFieldName = col.Name
			model.IDGoFieldName = types.FormatFieldName(col.Name)
			model.HasPrimaryKey = true
		}
	} else if !config.GenerateWithoutPK {
		for _, col := range table.Columns {
			if col.Name == "id" && col.IsPrimaryKey {
				setModelPK(model, col)
				model.IDFieldName = col.Name
				model.IDGoFieldName = types.FormatFieldName(col.Name)
				model.HasPrimaryKey = true
				break
			}
		}
		if !model.HasPrimaryKey {
			for _, col := range table.Columns {
				if col.IsPrimaryKey {
					setModelPK(model, col)
					model.IDFieldName = col.Name
					model.IDGoFieldName = types.FormatFieldName(col.Name)
					model.HasPrimaryKey = true
					break
				}
			}
		}
	}

	if model.HasPrimaryKey && model.IDType == "uuid.UUID" {
		importSet["github.com/google/uuid"] = true
	}

	stdImports, extImports := groupAndSortImports(importSet)
	model.StandardImports = stdImports
	model.ExternalImports = extImports
	model.Imports = append(
		append(make([]string, 0, len(stdImports)+len(extImports)), stdImports...),
		extImports...)

	return model, nil
}

func groupAndSortImports(importSet map[string]bool) (stdImports []string, extImports []string) {
	for imp := range importSet {
		if strings.Contains(imp, ".") {
			extImports = append(extImports, imp)
		} else {
			stdImports = append(stdImports, imp)
		}
	}
	sort.Strings(stdImports)
	sort.Strings(extImports)
	return stdImports, extImports
}

func setModelPK(model *GeneratedModel, col *catalog.Column) {
	pkType, _ := validation.ClassifyPrimaryKeyType(col.DataType)
	model.IDType = validation.GoType(pkType)
	model.IDGoType = model.IDType
	model.IsAutoIncrementID = validation.IsAutoIncrement(col.DataType)
}

func findColumn(table *catalog.Table, name string) *catalog.Column {
	for _, col := range table.Columns {
		if col.Name == name {
			return col
		}
	}
	return nil
}

func (g *Generator) buildField(col *catalog.Column) (GeneratedField, error) {
	goType, pkg, err := g.typeMapper.MapSQLTypeToGo(col.DataType, col.IsNullable)
	if err != nil {
		return GeneratedField{}, err
	}

	bunTag := g.typeMapper.BuildBunTag(col)

	field := GeneratedField{
		Name:          types.FormatFieldName(col.Name),
		Type:          goType,
		Package:       pkg,
		BunTag:        bunTag,
		IsForeignKey:  col.ForeignKey != nil,
		IsNullable:    col.IsNullable,
		IsPrimaryKey:  col.IsPrimaryKey,
		AllowedValues: append([]string(nil), col.AllowedValues...),
	}

	return field, nil
}

func (g *Generator) addModelTypeImports(goType string) map[string]bool {
	importSet := map[string]bool{}
	if strings.Contains(goType, "time.Time") {
		importSet["time"] = true
	}
	if strings.Contains(goType, "uuid.UUID") {
		importSet["github.com/google/uuid"] = true
	}
	if strings.HasPrefix(goType, "sql.Null") {
		importSet["database/sql"] = true
	}
	if strings.Contains(goType, "json.RawMessage") {
		importSet["encoding/json"] = true
	}
	return importSet
}

// GenerateModelFile renders model template data into Go source.
func (g *Generator) GenerateModelFile(model *GeneratedModel, templateStr string) (string, error) {
	funcMap := template.FuncMap{
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"Plural": inflection.Plural,
		"columnName": func(bunTag string) string {
			if before, _, ok := strings.Cut(bunTag, ","); ok {
				return before
			}
			return bunTag
		},
	}

	tmpl, err := template.New("model").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, model); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// GenerateModel renders and writes a model file for a resource.
func (g *Generator) GenerateModel(
	cat *catalog.Catalog,
	resourceName string,
	pluralName string,
	modelPath string,
	modulePath string,
	tableNameOverride string,
	nullType string,
	primaryKeyColumn string,
	generateWithoutPK bool,
) error {
	tableName := pluralName
	if tableNameOverride != "" {
		tableName = tableNameOverride
	}

	model, err := g.Build(cat, Config{
		TableName:         tableName,
		ResourceName:      resourceName,
		PackageName:       "models",
		DatabaseType:      g.typeMapper.GetDatabaseType(),
		ModulePath:        modulePath,
		NullType:          nullType,
		PrimaryKeyColumn:  primaryKeyColumn,
		GenerateWithoutPK: generateWithoutPK,
	})
	if err != nil {
		return fmt.Errorf("failed to build model: %w", err)
	}

	model.TableNameOverride = tableNameOverride
	model.TableNameOverridden = tableNameOverride != ""
	// When table name is overridden, don't pluralize the resource name for function names
	if tableNameOverride != "" {
		model.PluralName = resourceName
	} else {
		model.PluralName = inflection.Plural(resourceName)
	}

	templateContent, err := templates.Files.ReadFile("model.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read model template: %w", err)
	}

	modelContent, err := g.GenerateModelFile(model, string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to render model file: %w", err)
	}

	if err := os.WriteFile(modelPath, []byte(modelContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}

	if err := files.FormatGoFile(modelPath); err != nil {
		return fmt.Errorf("failed to format model file: %w", err)
	}

	return nil
}

// GeneratedFactory represents a factory for a model
type GeneratedFactory struct {
	ModelName         string
	EntityName        string // ServerEntity (resource name + "Entity")
	NamespaceVar      string // Server (exported, package-scope)
	Package           string
	Fields            []FactoryField
	ModulePath        string
	StandardImports   []string
	ExternalImports   []string
	HasCreateFunction bool
	HasForeignKeys    bool           // True if there are 1+ FKs
	ForeignKeyFields  []FactoryField // All FK fields
	IDType            string         // "uuid.UUID", "int32", "int64", "string"
	IDGoFieldName     string         // Go field name for the primary key (e.g., "ID")
	IsAutoIncrementID bool           // True for serial/bigserial
	HasCreatedAt      bool
	HasUpdatedAt      bool
}

// FactoryField represents a field in a factory
type FactoryField struct {
	Name          string
	ArgumentName  string
	Type          string
	GoZero        string
	DefaultValue  string
	OptionName    string
	IsFK          bool
	IsTimestamp   bool
	IsID          bool
	IsAutoManaged bool
}

// BuildFactory generates factory metadata from a model
func (g *Generator) BuildFactory(cat *catalog.Catalog, config Config, genModel *GeneratedModel) (*GeneratedFactory, error) {
	// Import the factory analyzer
	factoryFields := make([]FactoryField, 0, len(genModel.Fields))

	for _, field := range genModel.Fields {
		fieldInfo := g.analyzeFactoryField(field, genModel.Name)
		factoryFields = append(factoryFields, fieldInfo)
	}

	// Check for foreign keys
	var fkFields []FactoryField
	for _, field := range factoryFields {
		if field.IsFK {
			fkFields = append(fkFields, field)
		}
	}

	// Collect imports - context and fmt are already in the template
	standardImports := []string{}
	externalImports := []string{}
	for _, field := range factoryFields {
		if strings.Contains(field.DefaultValue, "faker.") {
			externalImports = append(externalImports, "github.com/go-faker/faker/v4")
			break
		}
	}

	// Only add uuid import if ID type uses UUID
	if genModel.IDType == "uuid.UUID" || genModel.IDType == "" {
		externalImports = append(externalImports, "github.com/google/uuid")
	}

	// Add time import if needed
	for _, field := range factoryFields {
		if field.IsTimestamp {
			standardImports = append(standardImports, "time")
			break
		}
	}
	for _, field := range genModel.Fields {
		switch {
		case strings.Contains(field.Type, "sql.Null"):
			standardImports = append(standardImports, "database/sql")
		case strings.Contains(field.Type, "bun.Null"):
			externalImports = append(externalImports, "github.com/uptrace/bun")
		}
		if field.Package == "" {
			continue
		}
		if strings.Contains(strings.Split(field.Package, "/")[0], ".") {
			externalImports = append(externalImports, field.Package)
		} else {
			standardImports = append(standardImports, field.Package)
		}
	}
	sort.Strings(standardImports)
	standardImports = slices.Compact(standardImports)
	sort.Strings(externalImports)
	externalImports = slices.Compact(externalImports)

	// Default IDGoFieldName if not set
	idGoFieldName := genModel.IDGoFieldName
	if idGoFieldName == "" {
		idGoFieldName = "ID"
	}

	return &GeneratedFactory{
		ModelName:         genModel.Name,
		EntityName:        genModel.EntityName,
		NamespaceVar:      genModel.NamespaceVar,
		Package:           "factories",
		Fields:            factoryFields,
		ModulePath:        config.ModulePath,
		StandardImports:   standardImports,
		ExternalImports:   externalImports,
		HasCreateFunction: true, // Assume Create function exists
		HasForeignKeys:    len(fkFields) > 0,
		ForeignKeyFields:  fkFields,
		IDType:            genModel.IDType,
		IDGoFieldName:     idGoFieldName,
		IsAutoIncrementID: genModel.IsAutoIncrementID,
		HasCreatedAt:      genModel.HasCreatedAt,
		HasUpdatedAt:      genModel.HasUpdatedAt,
	}, nil
}

// analyzeFactoryField analyzes a field and returns factory metadata
func (g *Generator) analyzeFactoryField(field GeneratedField, modelName string) FactoryField {
	info := FactoryField{
		Name:          field.Name,
		ArgumentName:  naming.ToLowerCamelCase(field.Name),
		Type:          field.Type,
		OptionName:    fmt.Sprintf("With%s%s", modelName, field.Name),
		IsID:          field.IsPrimaryKey,
		IsTimestamp:   field.Type == "time.Time" || strings.Contains(field.Type, "Time"),
		IsAutoManaged: field.IsPrimaryKey || field.Name == "CreatedAt" || field.Name == "UpdatedAt",
		IsFK:          field.IsForeignKey,
	}

	// Determine default value
	info.DefaultValue = g.determineFactoryDefaultForField(field)
	info.GoZero = g.getFactoryGoZero(field.Type)

	return info
}

func (g *Generator) determineFactoryDefaultForField(field GeneratedField) string {
	if len(field.AllowedValues) > 0 {
		return strconv.Quote(field.AllowedValues[0])
	}
	return g.determineFactoryDefault(field.Name, field.Type)
}

func (g *Generator) determineFactoryDefault(fieldName, goType string) string {
	if strings.HasPrefix(goType, "*") {
		return "nil"
	}
	if strings.HasPrefix(goType, "sql.Null") || strings.HasPrefix(goType, "bun.Null") {
		return fmt.Sprintf("%s{}", goType)
	}

	// Handle by type first
	switch goType {
	case "string":
		return g.stringFactoryDefault(fieldName)
	case "int32", "int":
		return "randomInt(1, 1000, 100)"
	case "int64":
		return "randomInt64(1, 1000, 100)"
	case "int16":
		return "randomInt16(1, 1000, 100)"
	case "bool":
		return "randomBool()"
	case "time.Time":
		return "time.Time{}"
	case "uuid.UUID":
		return "uuid.UUID{}"
	case "json.RawMessage":
		return `json.RawMessage("{}")`
	case "[]byte":
		return "[]byte{}"
	}

	// Default fallback
	return fmt.Sprintf("*new(%s)", goType)
}

func (g *Generator) stringFactoryDefault(fieldName string) string {
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
	case lower == "cidr" || strings.Contains(lower, "cidr"):
		return `"10.0.0.0/24"`
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

func (g *Generator) intFactoryDefault(fieldName string) string {
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

func (g *Generator) getFactoryGoZero(goType string) string {
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
	case "json.RawMessage":
		return "nil"
	case "[]byte":
		return "nil"
	// sql.Null and bun.Null zero values use their empty struct literal
	case "sql.NullString", "sql.NullBool", "sql.NullInt16", "sql.NullInt32",
		"sql.NullInt64", "sql.NullFloat64", "sql.NullTime":
		return fmt.Sprintf("%s{}", goType)
	case "bun.NullString", "bun.NullBool", "bun.NullInt32", "bun.NullInt64",
		"bun.NullFloat64", "bun.NullTime":
		return fmt.Sprintf("%s{}", goType)
	default:
		if strings.HasPrefix(goType, "[]") {
			return "nil"
		}
		return fmt.Sprintf("%s{}", goType)
	}
}

// GenerateFactoryFile renders a factory file from a template
func (g *Generator) GenerateFactoryFile(factory *GeneratedFactory, templateStr string) (string, error) {
	funcMap := template.FuncMap{
		"toLower": func(s string) string {
			return strings.ToLower(s)
		},
		"plural": inflection.Plural,
		"lowerCamel": func(s string) string {
			return naming.ToLowerCamelCase(s)
		},
	}

	tmpl, err := template.New("factory").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse factory template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, factory); err != nil {
		return "", fmt.Errorf("failed to execute factory template: %w", err)
	}

	return buf.String(), nil
}

// WriteFactoryFile writes a factory file to disk
func (g *Generator) WriteFactoryFile(factory *GeneratedFactory, outputDir string) error {
	factoryContent, err := g.PlanFactorySource(factory)
	if err != nil {
		return err
	}

	// Determine output path using snake_case for consistency with model files
	fileName := fmt.Sprintf("%s.go", naming.ToSnakeCase(factory.ModelName))
	outputPath := fmt.Sprintf("%s/models/factories/%s", outputDir, fileName)

	// Ensure directory exists
	if err := os.MkdirAll(fmt.Sprintf("%s/models/factories", outputDir), 0755); err != nil {
		return fmt.Errorf("failed to create factories directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(outputPath, []byte(factoryContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write factory file: %w", err)
	}

	return nil
}

// PlanFactorySource renders formatted factory source without writing files.
func (g *Generator) PlanFactorySource(factory *GeneratedFactory) (string, error) {
	templateContent, err := templates.Files.ReadFile("factory.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to read factory template: %w", err)
	}
	factoryContent, err := g.GenerateFactoryFile(factory, string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to render factory file: %w", err)
	}
	formatted, err := format.Source([]byte(factoryContent))
	if err != nil {
		return "", fmt.Errorf("failed to format factory source: %w", err)
	}
	return string(formatted), nil
}
