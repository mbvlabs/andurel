// Package generator provides functionality to generate Go models, controllers, and views
package generator

import (
	"bufio"
	"fmt"
	"mbvlabs/andurel/generator/controllers"
	"mbvlabs/andurel/generator/files"
	"mbvlabs/andurel/generator/internal/catalog"
	"mbvlabs/andurel/generator/internal/config"
	"mbvlabs/andurel/generator/internal/ddl"
	"mbvlabs/andurel/generator/internal/migrations"
	"mbvlabs/andurel/generator/models"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jinzhu/inflection"
)

type Generator struct {
	modulePath        string
	fileManager       *files.Manager
	modelGenerator    *models.Generator
	controllerGenerator *controllers.Generator
}

func New() (Generator, error) {
	modulePath, err := getCurrentModulePath()
	if err != nil {
		return Generator{}, fmt.Errorf("failed to get module path: %w", err)
	}

	return Generator{
		modulePath:        modulePath,
		fileManager:       files.NewManager(),
		modelGenerator:    models.NewGenerator("postgresql"),
		controllerGenerator: controllers.NewGenerator("postgresql"),
	}, nil
}

func getCurrentModulePath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			file, err := os.Open(goModPath)
			if err != nil {
				return "", fmt.Errorf("failed to open go.mod: %w", err)
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "module ") {
					return strings.Fields(line)[1], nil
				}
			}

			return "", fmt.Errorf("module declaration not found in go.mod")
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("go.mod not found in current directory or any parent directory")
}

func (g *Generator) buildCatalogFromMigrations(tableName string) (*catalog.Catalog, error) {
	cfg := config.NewDefaultConfig()
	migrationsList, err := migrations.DiscoverMigrations(cfg.MigrationDirs)
	if err != nil {
		return nil, fmt.Errorf("failed to discover migrations: %w", err)
	}

	cat := catalog.NewCatalog("public")
	foundTable := false

	for _, migration := range migrationsList {
		for _, stmt := range migration.Statements {
			if isRelevantForTable(stmt, tableName) {
				if err := ddl.ApplyDDL(cat, stmt, migration.FilePath); err != nil {
					return nil, fmt.Errorf(
						"failed to apply DDL from %s: %w",
						migration.FilePath,
						err,
					)
				}
				foundTable = true
			}
		}
	}

	if !foundTable {
		return nil, fmt.Errorf(
			"no migration found for table '%s'. Please create a migration first using: just create-migration create_%s_table",
			tableName,
			tableName,
		)
	}

	return cat, nil
}

func (g *Generator) GenerateModel(resourceName, tableName string) error {
	pluralName := inflection.Plural(strings.ToLower(resourceName))
	modelPath := filepath.Join("models", strings.ToLower(resourceName)+".go")
	sqlPath := filepath.Join("database/queries", pluralName+".sql")

	if err := g.fileManager.ValidateFileNotExists(modelPath); err != nil {
		return err
	}
	if err := g.fileManager.ValidateFileNotExists(sqlPath); err != nil {
		return err
	}

	cat, err := g.buildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := g.modelGenerator.GenerateModel(cat, resourceName, pluralName, modelPath, sqlPath, g.modulePath); err != nil {
		return fmt.Errorf("failed to generate model: %w", err)
	}

	if err := g.fileManager.RunSQLCGenerate(); err != nil {
		return fmt.Errorf("failed to run sqlc generate: %w", err)
	}

	fmt.Printf(
		"Successfully generated complete model for %s with database functions\n",
		resourceName,
	)
	return nil
}

func (g *Generator) GenerateResourceController(resourceName, tableName string) error {
	// Check if model exists
	modelPath := filepath.Join("models", strings.ToLower(resourceName)+".go")
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate model first",
			modelPath,
		)
	}

	// Build catalog from migrations
	cat, err := g.buildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	// Generate controller
	if err := g.controllerGenerator.GenerateController(cat, resourceName, controllers.ResourceController, g.modulePath); err != nil {
		return fmt.Errorf("failed to generate controller: %w", err)
	}

	fmt.Printf("Successfully generated resource controller for %s\n", resourceName)
	return nil
}

func (g *Generator) GenerateController(resourceName string) error {
	// For normal controllers, we don't need a table, just generate empty controller
	emptycat := catalog.NewCatalog("public")
	
	if err := g.controllerGenerator.GenerateController(emptycat, resourceName, controllers.NormalController, g.modulePath); err != nil {
		return fmt.Errorf("failed to generate controller: %w", err)
	}

	fmt.Printf("Successfully generated controller for %s\n", resourceName)
	return nil
}
//
//	func (g *Generator) GenerateView(resourceName string) error {
//		pluralName := inflection.Plural(strings.ToLower(resourceName))
//		viewPath := filepath.Join("views", pluralName+"_resource.templ")
//
//		if _, err := os.Stat(viewPath); err == nil {
//			return fmt.Errorf("file %s already exists", viewPath)
//		}
//
//		content, err := g.generateViewContent(resourceName, pluralName)
//		if err != nil {
//			return fmt.Errorf("failed to generate content: %w", err)
//		}
//
//		if err := os.WriteFile(viewPath, []byte(content), 0600); err != nil {
//			return fmt.Errorf("failed to write file: %w", err)
//		}
//
//		if err := runCompileTemplates(); err != nil {
//			return fmt.Errorf("failed to compile templates: %w", err)
//		}
//
//		fmt.Printf("Successfully generated view for %s\n", resourceName)
//		return nil
//	}

func isRelevantForTable(stmt, targetTable string) bool {
	stmtLower := strings.ToLower(stmt)
	targetLower := strings.ToLower(targetTable)

	if strings.Contains(stmtLower, "create table") &&
		strings.Contains(stmtLower, targetLower) {
		createTableRegex := regexp.MustCompile(
			`(?i)create\s+table(?:\s+if\s+not\s+exists)?\s+(?:\w+\.)?(\w+)`,
		)
		matches := createTableRegex.FindStringSubmatch(stmt)
		if len(matches) > 1 && strings.ToLower(matches[1]) == targetLower {
			return true
		}
	}

	if strings.Contains(stmtLower, "alter table") &&
		strings.Contains(stmtLower, targetLower) {
		alterTableRegex := regexp.MustCompile(
			`(?i)alter\s+table\s+(?:if\s+exists\s+)?(?:\w+\.)?(\w+)`,
		)
		matches := alterTableRegex.FindStringSubmatch(stmt)
		if len(matches) > 1 && strings.ToLower(matches[1]) == targetLower {
			return true
		}
	}

	if strings.Contains(stmtLower, "drop table") &&
		strings.Contains(stmtLower, targetLower) {
		dropTableRegex := regexp.MustCompile(
			`(?i)drop\s+table(?:\s+if\s+exists)?\s+(?:\w+\.)?(\w+)`,
		)
		matches := dropTableRegex.FindStringSubmatch(stmt)
		if len(matches) > 1 && strings.ToLower(matches[1]) == targetLower {
			return true
		}
	}

	return false
}

//	type ViewField struct {
//		Name            string
//		GoType          string
//		GoFormType      string
//		DisplayName     string
//		IsTimestamp     bool
//		InputType       string
//		StringConverter string
//		DBName          string
//		CamelCase       string
//		IsSystemField   bool
//	}
//
//	type TemplateData struct {
//		ResourceName string
//		PluralName   string
//		Fields       []ViewField
//		ModulePath   string
//	}
//
//	func (g *Generator) generateViewContent(resourceName, pluralName string) (string, error) {
//		cfg := config.NewDefaultConfig()
//		cfg.TableName = pluralName
//
//		migrationsList, err := migrations.DiscoverMigrations(cfg.MigrationDirs)
//		if err != nil {
//			return "", fmt.Errorf("failed to discover migrations: %w", err)
//		}
//
//		if len(migrationsList) == 0 {
//			return "", fmt.Errorf(
//				"no migration files found in %v",
//				cfg.MigrationDirs,
//			)
//		}
//
//		cat := catalog.NewCatalog("public")
//
//		for _, migration := range migrationsList {
//			for _, stmt := range migration.Statements {
//				if isRelevantForTable(stmt, pluralName) {
//					if err := ddl.ApplyDDL(cat, stmt, migration.FilePath); err != nil {
//						return "", fmt.Errorf(
//							"failed to apply DDL from %s: %w",
//							migration.FilePath,
//							err,
//						)
//					}
//				}
//			}
//		}
//
//		table, err := cat.GetTable("", pluralName)
//		if err != nil {
//			return "", fmt.Errorf(
//				"table '%s' not found in migrations. Check that you have a CREATE TABLE %s statement in your migrations",
//				pluralName,
//				pluralName,
//			)
//		}
//
//		typeMapper := generator.NewTypeMapper("postgresql")
//
//		fields := []ViewField{}
//		for _, col := range table.Columns {
//			if col.Name == "id" {
//				continue
//			}
//
//			field := ViewField{
//				Name:          formatFieldName(col.Name),
//				DisplayName:   formatDisplayName(col.Name),
//				DBName:        col.Name,
//				CamelCase:     formatCamelCase(col.Name),
//				IsSystemField: col.Name == "created_at" || col.Name == "updated_at",
//			}
//
//			goType, _, _, err := typeMapper.MapSQLTypeToGo(
//				col.DataType,
//				col.IsNullable,
//			)
//			if err != nil {
//				goType = "string"
//			}
//
//			field.GoType = goType
//
//			switch goType {
//			case "time.Time":
//				field.IsTimestamp = true
//				field.InputType = "date"
//				field.StringConverter = "%s.String()"
//			case "string":
//				field.InputType = "text"
//				field.StringConverter = ""
//			case "int16":
//				field.InputType = "number"
//				field.StringConverter = "fmt.Sprintf(\"%d\", %s)"
//			case "int32":
//				field.InputType = "number"
//				field.StringConverter = "fmt.Sprintf(\"%d\", %s)"
//			case "int64":
//				field.InputType = "number"
//				field.StringConverter = "fmt.Sprintf(\"%d\", %s)"
//			case "float32":
//				field.InputType = "number"
//				field.StringConverter = "fmt.Sprintf(\"%f\", %s)"
//			case "float64":
//				field.InputType = "number"
//				field.StringConverter = "fmt.Sprintf(\"%f\", %s)"
//			case "bool":
//				field.InputType = "checkbox"
//				field.StringConverter = "fmt.Sprintf(\"%t\", %s)"
//			case "uuid.UUID":
//				field.InputType = "text"
//				field.StringConverter = "%s.String()"
//			case "[]byte":
//				field.InputType = "text"
//				field.StringConverter = "string(%s)"
//			default:
//				field.InputType = "text"
//				field.StringConverter = ""
//			}
//
//			fields = append(fields, field)
//		}
//
//		tmplContent, err := os.ReadFile("generator/templates/resource_view.tmpl")
//		if err != nil {
//			return "", fmt.Errorf("failed to read template file: %w", err)
//		}
//
//		funcMap := template.FuncMap{
//			"ToLower": strings.ToLower,
//			"StringDisplay": func(field ViewField, resourceName string) string {
//				if field.StringConverter == "" {
//					return fmt.Sprintf(
//						"{ %s.%s }",
//						strings.ToLower(resourceName),
//						field.Name,
//					)
//				}
//				actualFieldRef := strings.ToLower(resourceName) + "." + field.Name
//				converter := strings.ReplaceAll(
//					field.StringConverter,
//					"%s",
//					actualFieldRef,
//				)
//				return fmt.Sprintf("{ %s }", converter)
//			},
//			"StringTableDisplay": func(field ViewField, resourceName string) string {
//				if field.StringConverter == "" {
//					return fmt.Sprintf(
//						"{ %s.%s }",
//						strings.ToLower(resourceName),
//						field.Name,
//					)
//				}
//				actualFieldRef := strings.ToLower(resourceName) + "." + field.Name
//				converter := strings.ReplaceAll(
//					field.StringConverter,
//					"%s",
//					actualFieldRef,
//				)
//				return fmt.Sprintf("{ %s }", converter)
//			},
//			"StringValue": func(field ViewField, resourceName string) string {
//				if field.StringConverter == "" {
//					return fmt.Sprintf(
//						"%s.%s",
//						strings.ToLower(resourceName),
//						field.Name,
//					)
//				}
//				actualFieldRef := strings.ToLower(resourceName) + "." + field.Name
//				return strings.ReplaceAll(
//					field.StringConverter,
//					"%s",
//					actualFieldRef,
//				)
//			},
//		}
//
//		tmpl, err := template.New("resource_view").
//			Funcs(funcMap).
//			Parse(string(tmplContent))
//		if err != nil {
//			return "", fmt.Errorf("failed to parse template: %w", err)
//		}
//
//		data := TemplateData{
//			ResourceName: resourceName,
//			PluralName:   pluralName,
//			Fields:       fields,
//			ModulePath:   g.modulePath,
//		}
//
//		var buf strings.Builder
//		if err := tmpl.Execute(&buf, data); err != nil {
//			return "", fmt.Errorf("failed to execute template: %w", err)
//		}
//
//		return buf.String(), nil
//	}
//
//	func (g *Generator) generateControllerFile(resourceName, pluralName, controllerPath string) error {
//		cfg := config.NewDefaultConfig()
//		cfg.TableName = pluralName
//
//		migrationsList, err := migrations.DiscoverMigrations(cfg.MigrationDirs)
//		if err != nil {
//			return fmt.Errorf("failed to discover migrations: %w", err)
//		}
//
//		if len(migrationsList) == 0 {
//			return fmt.Errorf("no migration files found in %v", cfg.MigrationDirs)
//		}
//
//		cat := catalog.NewCatalog("public")
//
//		for _, migration := range migrationsList {
//			for _, stmt := range migration.Statements {
//				if isRelevantForTable(stmt, pluralName) {
//					if err := ddl.ApplyDDL(cat, stmt, migration.FilePath); err != nil {
//						return fmt.Errorf(
//							"failed to apply DDL from %s: %w",
//							migration.FilePath,
//							err,
//						)
//					}
//				}
//			}
//		}
//
//		table, err := cat.GetTable("", pluralName)
//		if err != nil {
//			return fmt.Errorf(
//				"table '%s' not found in migrations. Check that you have a CREATE TABLE %s statement in your migrations",
//				pluralName,
//				pluralName,
//			)
//		}
//
//		typeMapper := generator.NewTypeMapper("postgresql")
//
//		fields := []ViewField{}
//		for _, col := range table.Columns {
//			if col.Name == "id" {
//				continue
//			}
//
//			field := ViewField{
//				Name:          formatFieldName(col.Name),
//				DisplayName:   formatDisplayName(col.Name),
//				DBName:        col.Name,
//				CamelCase:     formatCamelCase(col.Name),
//				IsSystemField: col.Name == "created_at" || col.Name == "updated_at",
//			}
//
//			goType, _, _, err := typeMapper.MapSQLTypeToGo(
//				col.DataType,
//				col.IsNullable,
//			)
//			if err != nil {
//				goType = "string"
//			}
//
//			field.GoType = goType
//
//			switch goType {
//			case "time.Time":
//				field.GoFormType = "time.Time"
//				field.IsTimestamp = true
//			case "int16":
//				field.GoFormType = "int16"
//			case "int32":
//				field.GoFormType = "int32"
//			case "int64":
//				field.GoFormType = "int64"
//			case "float32":
//				field.GoFormType = "float32"
//			case "float64":
//				field.GoFormType = "float64"
//			case "bool":
//				field.GoFormType = "bool"
//			default:
//				field.GoFormType = "string"
//			}
//
//			fields = append(fields, field)
//		}
//
//		templateContent, err := os.ReadFile("generator/templates/controller.tmpl")
//		if err != nil {
//			return fmt.Errorf("failed to read controller template: %w", err)
//		}
//
//		funcMap := template.FuncMap{
//			"ToLower": strings.ToLower,
//		}
//
//		tmpl, err := template.New("controller").
//			Funcs(funcMap).
//			Parse(string(templateContent))
//		if err != nil {
//			return fmt.Errorf("failed to parse controller template: %w", err)
//		}
//
//		data := TemplateData{
//			ResourceName: resourceName,
//			PluralName:   pluralName,
//			Fields:       fields,
//			ModulePath:   g.modulePath,
//		}
//
//		var buf strings.Builder
//		if err := tmpl.Execute(&buf, data); err != nil {
//			return fmt.Errorf("failed to execute controller template: %w", err)
//		}
//
//		if err := os.WriteFile(controllerPath, []byte(buf.String()), 0600); err != nil {
//			return fmt.Errorf("failed to write controller file: %w", err)
//		}
//
//		return nil
//	}
//
//	func (g *Generator) generateRoutesFile(resourceName, pluralName, routesPath string) error {
//		templateContent, err := os.ReadFile("generator/templates/route.tmpl")
//		if err != nil {
//			return fmt.Errorf("failed to read routes template: %w", err)
//		}
//
//		funcMap := template.FuncMap{
//			"ToLower": strings.ToLower,
//		}
//
//		tmpl, err := template.New("routes").
//			Funcs(funcMap).
//			Parse(string(templateContent))
//		if err != nil {
//			return fmt.Errorf("failed to parse routes template: %w", err)
//		}
//
//		data := TemplateData{
//			ResourceName: resourceName,
//			PluralName:   pluralName,
//			ModulePath:   g.modulePath,
//		}
//
//		var buf strings.Builder
//		if err := tmpl.Execute(&buf, data); err != nil {
//			return fmt.Errorf("failed to execute routes template: %w", err)
//		}
//
//		if err := os.WriteFile(routesPath, []byte(buf.String()), 0600); err != nil {
//			return fmt.Errorf("failed to write routes file: %w", err)
//		}
//
//		return nil
//	}
//
//	func formatFieldName(dbName string) string {
//		parts := strings.Split(dbName, "_")
//		for i, part := range parts {
//			parts[i] = cases.Title(language.English).String(part)
//		}
//		return strings.Join(parts, "")
//	}
//
//	func formatDisplayName(dbName string) string {
//		parts := strings.Split(dbName, "_")
//		for i, part := range parts {
//			parts[i] = cases.Title(language.English).String(part)
//		}
//		return strings.Join(parts, " ")
//	}
//
//	func formatCamelCase(dbName string) string {
//		parts := strings.Split(dbName, "_")
//		if len(parts) == 0 {
//			return dbName
//		}
//
//		result := parts[0]
//		for i := 1; i < len(parts); i++ {
//			result += cases.Title(language.English).String(parts[i])
//		}
//		return result
//	}

// func runCompileTemplates() error {
// 	cmd := exec.CommandContext(context.Background(), "just", "compile-templates")
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return fmt.Errorf(
// 			"failed to run 'just compile-templates': %w\nOutput: %s",
// 			err,
// 			output,
// 		)
// 	}
// 	fmt.Println("Generated view templates with templ")
// 	return nil
// }
//
// func registerController(resourceName, pluralName string) error {
// 	controllerFilePath := "controllers/controller.go"
//
// 	content, err := os.ReadFile(controllerFilePath)
// 	if err != nil {
// 		return fmt.Errorf("failed to read controller.go: %w", err)
// 	}
//
// 	contentStr := string(content)
//
// 	structField := fmt.Sprintf("\t%ss %ss", resourceName, resourceName)
// 	if !strings.Contains(contentStr, structField) {
// 		lines := strings.Split(contentStr, "\n")
// 		for i, line := range lines {
// 			if strings.Contains(line, "type Controllers struct {") {
// 				for j := i + 1; j < len(lines); j++ {
// 					if strings.TrimSpace(lines[j]) == "}" {
// 						lines = append(
// 							lines[:j],
// 							append([]string{structField}, lines[j:]...)...)
// 						break
// 					}
// 				}
// 				break
// 			}
// 		}
// 		contentStr = strings.Join(lines, "\n")
// 	}
//
// 	newFunctionCall := fmt.Sprintf(
// 		"\t%s := new%ss(db)",
// 		strings.ToLower(pluralName),
// 		resourceName,
// 	)
//
// 	if !strings.Contains(contentStr, newFunctionCall) {
// 		assetsPattern := "\tassets := newAssets()"
// 		assetsReplacement := assetsPattern + "\n" + newFunctionCall
// 		contentStr = strings.Replace(
// 			contentStr,
// 			assetsPattern,
// 			assetsReplacement,
// 			1,
// 		)
// 	}
//
// 	returnField := fmt.Sprintf("\t\t%s,", strings.ToLower(pluralName))
//
// 	if !strings.Contains(contentStr, returnField) {
// 		lines := strings.Split(contentStr, "\n")
// 		for i, line := range lines {
// 			if strings.Contains(line, "return Controllers{") {
// 				for j := i + 1; j < len(lines); j++ {
// 					if strings.TrimSpace(lines[j]) == "}" {
// 						lines = append(
// 							lines[:j],
// 							append([]string{returnField}, lines[j:]...)...)
// 						break
// 					}
// 				}
// 				break
// 			}
// 		}
// 		contentStr = strings.Join(lines, "\n")
// 	}
//
// 	return os.WriteFile(controllerFilePath, []byte(contentStr), 0600)
// }
//
// func registerRoutes(resourceName string) error {
// 	routesFilePath := "router/routes/routes.go"
//
// 	content, err := os.ReadFile(routesFilePath)
// 	if err != nil {
// 		return fmt.Errorf("failed to read routes.go: %w", err)
// 	}
//
// 	contentStr := string(content)
//
// 	routesToAdd := []string{
// 		fmt.Sprintf("\t\t%sIndex,", resourceName),
// 		fmt.Sprintf("\t\t%sShow.Route,", resourceName),
// 		fmt.Sprintf("\t\t%sNew,", resourceName),
// 		fmt.Sprintf("\t\t%sCreate,", resourceName),
// 		fmt.Sprintf("\t\t%sEdit.Route,", resourceName),
// 		fmt.Sprintf("\t\t%sUpdate.Route,", resourceName),
// 		fmt.Sprintf("\t\t%sDestroy.Route,", resourceName),
// 	}
//
// 	alreadyExists := false
// 	for _, route := range routesToAdd {
// 		if strings.Contains(contentStr, strings.TrimSpace(route)) {
// 			alreadyExists = true
// 			break
// 		}
// 	}
//
// 	if !alreadyExists {
// 		lines := strings.Split(contentStr, "\n")
//
// 		for i, line := range lines {
// 			if strings.Contains(line, "r = append(") {
// 				for j := i + 1; j < len(lines); j++ {
// 					if strings.TrimSpace(lines[j]) == ")" {
// 						newLines := make([]string, 0, len(lines)+len(routesToAdd))
// 						newLines = append(newLines, lines[:j]...)
// 						newLines = append(newLines, routesToAdd...)
// 						newLines = append(newLines, lines[j:]...)
// 						contentStr = strings.Join(newLines, "\n")
// 						break
// 					}
// 				}
// 				break
// 			}
// 		}
// 	}
//
// 	return os.WriteFile(routesFilePath, []byte(contentStr), 0600)
// }
