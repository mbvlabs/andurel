package models

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"github.com/mbvlabs/andurel/generator/internal/migrations"
	"github.com/mbvlabs/andurel/generator/internal/types"
	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/errors"
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
		return nil, errors.NewDatabaseError("get table", config.TableName, err)
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
			return nil, errors.NewGeneratorError("build field", col.Name, err)
		}

		if field.Package != "" {
			importSet[field.Package] = true
		}

		typeImports := g.addTypeImports(field.SQLCType, field.Type)
		for imp := range typeImports {
			importSet[imp] = true
		}
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
		field.ConversionToDB = ""
	} else {
		field.ConversionToDB = g.typeMapper.GenerateConversionToDB(field.SQLCType, field.Type, "data."+field.Name)
	}

	if col.Name == "updated_at" {
		field.ConversionToDBForUpdate = ""
	} else {
		field.ConversionToDBForUpdate = g.typeMapper.GenerateConversionToDB(
			field.SQLCType,
			field.Type,
			"data."+field.Name,
		)
	}

	field.ZeroCheck = g.typeMapper.GenerateZeroCheck(field.Type, "data."+field.Name)

	return field, nil
}

func (g *Generator) addTypeImports(sqlcType, goType string) map[string]bool {
	importSet := map[string]bool{}
	if strings.Contains(sqlcType, "pgtype.") {
		importSet["github.com/jackc/pgx/v5/pgtype"] = true
	}
	if strings.Contains(goType, "time.Time") {
		importSet["time"] = true
	}

	return importSet
}

func (g *Generator) GenerateModelFile(model *GeneratedModel, templateStr string) (string, error) {
	funcMap := template.FuncMap{
		"SQLCTypeName": func(tableName string) string {
			singular := strings.TrimSuffix(tableName, "s") // Simple singularization
			return types.FormatFieldName(singular)
		},
		"lower": func(s string) string {
			return strings.ToLower(s)
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

	modelContent, err := g.GenerateModelFile(model, string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to render model file: %w", err)
	}

	if err := os.WriteFile(modelPath, []byte(modelContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}

	if err := g.formatGoFile(modelPath); err != nil {
		return fmt.Errorf("failed to format model file: %w", err)
	}

	return nil
}

func (g *Generator) GenerateSQLFile(
	resourceName string,
	pluralName string,
	table *catalog.Table,
	sqlPath string,
) error {
	data := g.prepareSQLData(resourceName, pluralName, table)

	tmpl, err := templates.GetCachedTemplate("crud_operations.tmpl", template.FuncMap{})
	if err != nil {
		return errors.NewTemplateError("crud_operations.tmpl", "get template", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return errors.NewTemplateError("crud_operations.tmpl", "execute template", err)
	}

	return os.WriteFile(sqlPath, []byte(buf.String()), constants.FilePermissionPrivate)
}

func (g *Generator) GenerateSQLContent(
	resourceName string,
	pluralName string,
	table *catalog.Table,
) (string, error) {
	data := g.prepareSQLData(resourceName, pluralName, table)

	tmpl, err := templates.GetCachedTemplate("crud_operations.tmpl", template.FuncMap{})
	if err != nil {
		return "", errors.NewTemplateError("crud_operations.tmpl", "get template", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", errors.NewTemplateError("crud_operations.tmpl", "execute template", err)
	}

	return buf.String(), nil
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

		if col.Name == "created_at" || col.Name == "updated_at" {
			insertPlaceholders = append(insertPlaceholders, "now()")
		} else {
			insertPlaceholders = append(
				insertPlaceholders,
				fmt.Sprintf("$%d", placeholderIndex),
			)
			placeholderIndex++
		}
	}

	placeholderIndex = 2
	for _, col := range table.Columns {
		if col.Name != "id" && col.Name != "created_at" {
			if col.Name == "updated_at" {
				updateColumns = append(updateColumns, "updated_at=now()")
			} else {
				updateColumns = append(
					updateColumns,
					fmt.Sprintf("%s=$%d", col.Name, placeholderIndex),
				)
				placeholderIndex++
			}
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

func (g *Generator) formatGoFile(filePath string) error {
	cmd := exec.Command("go", "fmt", filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run go fmt on %s: %w", filePath, err)
	}
	return nil
}

func (g *Generator) RefreshModel(
	cat *catalog.Catalog,
	resourceName, pluralName string,
	modelPath, sqlPath string,
	modulePath string,
) error {
	// First, completely replace the SQL file with updated CRUD operations
	if err := g.refreshSQLFile(resourceName, pluralName, cat, sqlPath); err != nil {
		return fmt.Errorf("failed to refresh SQL file: %w", err)
	}

	// Then, selectively refresh the model file
	if err := g.refreshModelFile(cat, resourceName, pluralName, modelPath, modulePath); err != nil {
		return fmt.Errorf("failed to refresh model file: %w", err)
	}

	return nil
}

func (g *Generator) refreshSQLFile(
	resourceName string,
	pluralName string,
	cat *catalog.Catalog,
	sqlPath string,
) error {
	table, err := cat.GetTable("", pluralName)
	if err != nil {
		return fmt.Errorf("table '%s' not found in catalog: %w", pluralName, err)
	}

	// Generate new SQL content completely
	newSQLContent, err := g.GenerateSQLContent(resourceName, pluralName, table)
	if err != nil {
		return fmt.Errorf("failed to generate SQL content: %w", err)
	}

	// Write the new SQL file (completely replace)
	if err := os.WriteFile(sqlPath, []byte(newSQLContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write SQL file: %w", err)
	}

	return nil
}

func (g *Generator) refreshModelFile(
	cat *catalog.Catalog,
	resourceName, pluralName string,
	modelPath, modulePath string,
) error {
	// Read the existing file content
	existingContent, err := os.ReadFile(modelPath)
	if err != nil {
		return fmt.Errorf("failed to read existing model file: %w", err)
	}

	// Build the new model data
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

	// Generate the new complete model content to extract new generated parts
	templateContent, err := templates.Files.ReadFile("model.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read model template: %w", err)
	}

	newModelContent, err := g.GenerateModelFile(model, string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to render model file: %w", err)
	}

	// Replace known generated parts with updated versions
	updatedContent, err := g.replaceGeneratedParts(string(existingContent), newModelContent, resourceName)
	if err != nil {
		return fmt.Errorf("failed to replace generated parts: %w", err)
	}

	// Write the updated content
	if err := os.WriteFile(modelPath, []byte(updatedContent), constants.FilePermissionPrivate); err != nil {
		return fmt.Errorf("failed to write updated model file: %w", err)
	}

	// Always run go fmt to ensure proper formatting
	if err := g.formatGoFile(modelPath); err != nil {
		return fmt.Errorf("failed to format model file: %w", err)
	}

	return nil
}

func (g *Generator) replaceGeneratedParts(existingContent, newContent, resourceName string) (string, error) {
	// Extract all generated parts from the new content
	newParts := g.extractGeneratedParts(newContent, resourceName)
	
	updatedContent := existingContent
	
	// Replace each generated part in the existing content
	for signature, newContent := range newParts {
		updatedContent = g.replacePartBySignature(updatedContent, signature, newContent)
	}
	
	return updatedContent, nil
}

func (g *Generator) extractGeneratedParts(content, resourceName string) map[string]string {
	parts := make(map[string]string)
	lines := strings.Split(content, "\n")
	
	// Define the generated function/type signatures we're looking for
	signatures := []string{
		fmt.Sprintf("type %s struct", resourceName),
		fmt.Sprintf("type Create%sPayload struct", resourceName),
		fmt.Sprintf("type Update%sPayload struct", resourceName),
		fmt.Sprintf("type Paginated%ss struct", resourceName),
		fmt.Sprintf("func Find%s(", resourceName),
		fmt.Sprintf("func Create%s(", resourceName),
		fmt.Sprintf("func Update%s(", resourceName),
		fmt.Sprintf("func Destroy%s(", resourceName),
		fmt.Sprintf("func All%ss(", resourceName),
		fmt.Sprintf("func Paginate%ss(", resourceName),
		fmt.Sprintf("func rowTo%s(", resourceName),
	}
	
	for _, signature := range signatures {
		part := g.extractPartBySignature(lines, signature)
		if part != "" {
			parts[signature] = part
		}
	}
	
	return parts
}

func (g *Generator) extractPartBySignature(lines []string, signature string) string {
	var result []string
	inBlock := false
	braceCount := 0
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Check if this line starts our target block
		if strings.HasPrefix(trimmed, signature) {
			inBlock = true
			result = []string{line}
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")
			
			// If the opening brace closes on the same line, we're done
			if braceCount == 0 {
				return strings.Join(result, "\n")
			}
			continue
		}
		
		if inBlock {
			result = append(result, line)
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")
			
			// When braces are balanced, we've reached the end
			if braceCount == 0 {
				return strings.Join(result, "\n")
			}
		}
	}
	
	return ""
}

func (g *Generator) replacePartBySignature(content, signature, newPart string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inBlock := false
	braceCount := 0
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Check if this line starts our target block
		if strings.HasPrefix(trimmed, signature) {
			inBlock = true
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")
			
			// If the opening brace closes on the same line, replace just this line
			if braceCount == 0 {
				result = append(result, newPart)
				inBlock = false
				continue
			}
			continue
		}
		
		if inBlock {
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")
			
			// When braces are balanced, we've reached the end - replace the whole block
			if braceCount == 0 {
				result = append(result, newPart)
				inBlock = false
				continue
			}
			// Skip lines that are part of the block being replaced
			continue
		}
		
		// Keep all other lines
		result = append(result, line)
	}
	
	return strings.Join(result, "\n")
}
