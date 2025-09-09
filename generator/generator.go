// Package generator provides functionality to generate Go models, controllers, and views
package generator

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mbvlabs/andurel/generator/controllers"
	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/config"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"github.com/mbvlabs/andurel/generator/internal/migrations"
	"github.com/mbvlabs/andurel/generator/internal/types"
	"github.com/mbvlabs/andurel/generator/models"
	"github.com/mbvlabs/andurel/generator/views"

	"github.com/jinzhu/inflection"
)

type Generator struct {
	modulePath          string
	fileManager         *files.Manager
	modelGenerator      *models.Generator
	controllerGenerator *controllers.Generator
	viewGenerator       *views.Generator
}

func New() (Generator, error) {
	g := Generator{
		fileManager:         files.NewManager(),
		modelGenerator:      models.NewGenerator("postgresql"),
		controllerGenerator: controllers.NewGenerator("postgresql"),
		viewGenerator:       views.NewGenerator("postgresql"),
	}

	modulePath, err := g.getCurrentModulePath()
	if err != nil {
		return Generator{}, fmt.Errorf("failed to get module path: %w", err)
	}
	g.modulePath = modulePath

	return g, nil
}

func (g *Generator) getCurrentModulePath() (string, error) {
	rootDir, err := g.fileManager.FindGoModRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find go.mod: %w", err)
	}

	goModPath := filepath.Join(rootDir, "go.mod")
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
	rootDir, err := g.fileManager.FindGoModRoot()
	if err != nil {
		return fmt.Errorf("failed to find go.mod root: %w", err)
	}
	if err := types.ValidateSQLCConfig(rootDir); err != nil {
		return fmt.Errorf("SQLC configuration validation failed: %w", err)
	}

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

func (g *Generator) GenerateController(resourceName, tableName string) error {
	modelPath := filepath.Join("models", strings.ToLower(resourceName)+".go")
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate model first",
			modelPath,
		)
	}

	cat, err := g.buildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := g.controllerGenerator.GenerateController(cat, resourceName, controllers.ResourceController, g.modulePath); err != nil {
		return fmt.Errorf("failed to generate controller: %w", err)
	}

	if err := g.viewGenerator.GenerateView(cat, resourceName, g.modulePath); err != nil {
		return fmt.Errorf("failed to generate view: %w", err)
	}

	fmt.Printf("Successfully generated resource controller and view for %s\n", resourceName)
	return nil
}

// func (g *Generator) GenerateController(resourceName string) error {
// 	emptycat := catalog.NewCatalog("public")
//
// 	if err := g.controllerGenerator.GenerateController(emptycat, resourceName, controllers.NormalController, g.modulePath); err != nil {
// 		return fmt.Errorf("failed to generate controller: %w", err)
// 	}
//
// 	fmt.Printf("Successfully generated controller for %s\n", resourceName)
// 	return nil
// }

func (g *Generator) GenerateView(resourceName, tableName string) error {
	modelPath := filepath.Join("models", strings.ToLower(resourceName)+".go")
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"model file %s does not exist. Generate model first",
			modelPath,
		)
	}

	cat, err := g.buildCatalogFromMigrations(tableName)
	if err != nil {
		return err
	}

	if err := g.viewGenerator.GenerateView(cat, resourceName, g.modulePath); err != nil {
		return fmt.Errorf("failed to generate view: %w", err)
	}

	fmt.Printf("Successfully generated resource view for %s\n", resourceName)

	return nil
}

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
