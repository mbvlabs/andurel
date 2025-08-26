package models

import (
	"fmt"
	"mbvlabs/andurel/generator/internal/catalog"
	"mbvlabs/andurel/generator/internal/ddl"
	"mbvlabs/andurel/generator/internal/migrations"
	"mbvlabs/andurel/generator/templates"
	"mbvlabs/andurel/generator/types"
	"os"
	"sort"
	"strings"
	"text/template"
)

type GeneratedField struct {
	Name                    string
	Type                    string
	Comment                 string
	Package                 string
	SQLCType                string
	ConversionFromDB        string
	ConversionToDB          string
	ConversionToDBForUpdate string
	ZeroCheck               string
}

type GeneratedModel struct {
	Name       string
	Package    string
	Fields     []GeneratedField
	Imports    []string
	TableName  string
	ModulePath string
}

type Config struct {
	TableName    string
	ResourceName string
	PackageName  string
	DatabaseType string
	ModulePath   string
	CustomTypes  []types.TypeOverride
}

type SQLData struct {
	ResourceName       string
	PluralName         string
	InsertColumns      string
	InsertPlaceholders string
	UpdateColumns      string
}

type Generator struct {
	typeMapper *types.TypeMapper
}

func NewGenerator(databaseType string) *Generator {
	return &Generator{
		typeMapper: types.NewTypeMapper(databaseType),
	}
}

func (g *Generator) Build(cat *catalog.Catalog, config Config) (*GeneratedModel, error) {
	table, err := cat.GetTable("", config.TableName)
	if err != nil {
		return nil, fmt.Errorf("table %s not found: %w", config.TableName, err)
	}

	g.typeMapper.Overrides = append(g.typeMapper.Overrides, config.CustomTypes...)

	model := &GeneratedModel{
		Name:       config.ResourceName,
		Package:    config.PackageName,
		TableName:  config.TableName,
		ModulePath: config.ModulePath,
		Fields:     make([]GeneratedField, 0, len(table.Columns)),
		Imports:    make([]string, 0),
	}

	importSet := make(map[string]bool)

	for _, col := range table.Columns {
		field, err := g.buildField(col)
		if err != nil {
			return nil, fmt.Errorf("failed to build field for column %s: %w", col.Name, err)
		}

		if field.Package != "" {
			importSet[field.Package] = true
		}

		g.addTypeImports(field.SQLCType, importSet)
		model.Fields = append(model.Fields, field)
	}

	for imp := range importSet {
		model.Imports = append(model.Imports, imp)
	}
	sort.Strings(model.Imports)

	return model, nil
}

func (g *Generator) buildField(col *catalog.Column) (GeneratedField, error) {
	goType, sqlcType, pkg, err := g.typeMapper.MapSQLTypeToGo(col.DataType, col.IsNullable)
	if err != nil {
		return GeneratedField{}, err
	}

	field := GeneratedField{
		Name:     types.FormatFieldName(col.Name),
		Type:     goType,
		SQLCType: sqlcType,
		Package:  pkg,
	}

	field.ConversionFromDB = g.typeMapper.GenerateConversionFromDB(
		field.Name,
		field.SQLCType,
		field.Type,
	)

	if col.Name == "created_at" || col.Name == "updated_at" {
		field.ConversionToDB = g.typeMapper.GenerateConversionToDB(
			field.SQLCType,
			field.Type,
			"resource."+field.Name,
		)
	} else {
		field.ConversionToDB = g.typeMapper.GenerateConversionToDB(field.SQLCType, field.Type, "data."+field.Name)
	}

	field.ConversionToDBForUpdate = g.typeMapper.GenerateConversionToDB(
		field.SQLCType,
		field.Type,
		"data."+field.Name,
	)
	field.ZeroCheck = g.typeMapper.GenerateZeroCheck(field.Type, "data."+field.Name)

	return field, nil
}

func (g *Generator) addTypeImports(sqlcType string, importSet map[string]bool) {
	switch sqlcType {
	case "sql.NullString", "sql.NullBool", "sql.NullInt32", "sql.NullInt64", "sql.NullFloat64":
		importSet["database/sql"] = true
	case "pgtype.Timestamptz", "pgtype.Timestamp", "pgtype.Numeric":
		importSet["github.com/jackc/pgx/v5/pgtype"] = true
	}
}

func (g *Generator) RenderModelFile(model *GeneratedModel, templateStr string) (string, error) {
	funcMap := template.FuncMap{
		"SQLCTypeName": func(tableName string) string {
			singular := strings.TrimSuffix(tableName, "s") // Simple singularization
			return types.FormatFieldName(singular)
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

func (g *Generator) GenerateModel(
	cat *catalog.Catalog,
	resourceName, pluralName string,
	modelPath, sqlPath string,
	modulePath string,
) error {
	table, err := cat.GetTable("", pluralName)
	if err != nil {
		return fmt.Errorf("table '%s' not found in catalog: %w", pluralName, err)
	}

	if err := g.GenerateSQLFile(resourceName, pluralName, table, sqlPath); err != nil {
		return fmt.Errorf("failed to generate SQL file: %w", err)
	}

	model, err := g.Build(cat, Config{
		TableName:    pluralName,
		ResourceName: resourceName,
		PackageName:  "models",
		DatabaseType: "postgresql",
		ModulePath:   modulePath,
	})
	if err != nil {
		return fmt.Errorf("failed to build model: %w", err)
	}

	templateContent, err := templates.Files.ReadFile("model.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read model template: %w", err)
	}

	modelContent, err := g.RenderModelFile(model, string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to render model file: %w", err)
	}

	if err := os.WriteFile(modelPath, []byte(modelContent), 0600); err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}

	return nil
}

func (g *Generator) GenerateSQLFile(
	resourceName string,
	pluralName string,
	table *catalog.Table,
	sqlPath string,
) error {
	templateContent, err := templates.Files.ReadFile("crud_operations.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read SQL template: %w", err)
	}

	data := g.prepareSQLData(resourceName, pluralName, table)

	t, err := template.New("sql").Parse(string(templateContent))
	if err != nil {
		return err
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return err
	}

	return os.WriteFile(sqlPath, []byte(buf.String()), 0600)
}

func (g *Generator) prepareSQLData(
	resourceName string,
	pluralName string,
	table *catalog.Table,
) SQLData {
	var insertColumns []string
	var insertPlaceholders []string
	var updateColumns []string

	placeholderIndex := 1

	for _, col := range table.Columns {
		insertColumns = append(insertColumns, col.Name)
		insertPlaceholders = append(
			insertPlaceholders,
			fmt.Sprintf("$%d", placeholderIndex),
		)
		placeholderIndex++
	}

	placeholderIndex = 2
	for _, col := range table.Columns {
		if col.Name != "id" && col.Name != "created_at" {
			updateColumns = append(
				updateColumns,
				fmt.Sprintf("%s=$%d", col.Name, placeholderIndex),
			)
			placeholderIndex++
		}
	}

	return SQLData{
		ResourceName:       resourceName,
		PluralName:         pluralName,
		InsertColumns:      strings.Join(insertColumns, ", "),
		InsertPlaceholders: strings.Join(insertPlaceholders, ", "),
		UpdateColumns:      strings.Join(updateColumns, ", "),
	}
}

func (g *Generator) buildCatalogFromTableMigrations(
	tableName string,
	migrationDirs []string,
) (*catalog.Catalog, error) {
	allMigrations, err := migrations.DiscoverMigrations(migrationDirs)
	if err != nil {
		return nil, fmt.Errorf("failed to discover migrations: %w", err)
	}

	relevantMigrations := g.filterMigrationsForTable(tableName, allMigrations)

	cat := catalog.NewCatalog("public")

	for _, migration := range relevantMigrations {
		for _, statement := range migration.Statements {
			if err := ddl.ApplyDDL(cat, statement, migration.FilePath); err != nil {
				return nil, fmt.Errorf("failed to apply DDL from %s: %w", migration.FilePath, err)
			}
		}
	}

	return cat, nil
}

func (g *Generator) filterMigrationsForTable(
	tableName string,
	allMigrations []migrations.Migration,
) []migrations.Migration {
	var relevantMigrations []migrations.Migration

	for _, migration := range allMigrations {
		isRelevant := false

		for _, statement := range migration.Statements {
			if g.statementAffectsTable(statement, tableName) {
				isRelevant = true
				break
			}
		}

		if isRelevant {
			relevantMigrations = append(relevantMigrations, migration)
		}
	}

	return relevantMigrations
}

func (g *Generator) statementAffectsTable(statement, tableName string) bool {
	stmt, err := ddl.ParseDDLStatement(statement, "")
	if err != nil {
		return false
	}

	if stmt == nil {
		return false
	}

	switch stmt.Type {
	case ddl.CreateTable, ddl.AlterTable, ddl.DropTable:
		return stmt.TableName == tableName
	default:
		return false
	}
}

func (g *Generator) GenerateModelFromMigrations(
	tableName, resourceName string,
	migrationDirs []string,
	modelPath, sqlPath string,
	modulePath string,
) error {
	cat, err := g.buildCatalogFromTableMigrations(tableName, migrationDirs)
	if err != nil {
		return fmt.Errorf("failed to build catalog from migrations: %w", err)
	}

	return g.GenerateModel(cat, resourceName, tableName, modelPath, sqlPath, modulePath)
}
