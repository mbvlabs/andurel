package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mbvlabs/andurel/generator/internal/catalog"
	"github.com/mbvlabs/andurel/generator/internal/ddl"
	"github.com/mbvlabs/andurel/generator/internal/migrations"
)

type MigrationManager struct{}

func NewMigrationManager() *MigrationManager {
	return &MigrationManager{}
}

func (mm *MigrationManager) BuildCatalogFromMigrations(
	tableName string,
	config *UnifiedConfig,
) (*catalog.Catalog, error) {
	databaseType := config.Database.Type
	migrationsList, err := migrations.DiscoverMigrations(config.Database.MigrationDirs)
	if err != nil {
		return nil, fmt.Errorf("failed to discover migrations: %w", err)
	}

	cat := catalog.NewCatalog("public")
	foundTable := false

	for _, migration := range migrationsList {
		for _, stmt := range migration.Statements {
			if isRelevantForTable(stmt, tableName) {
				if err := ddl.ApplyDDL(cat, stmt, migration.FilePath, databaseType); err != nil {
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
			"no migration found for table '%s'. Create a migration for this table or use --table-name to specify a different table name",
			tableName,
		)
	}

	return cat, nil
}

func isRelevantForTable(stmt, targetTable string) bool {
	stmtLower := strings.ToLower(stmt)
	targetLower := strings.ToLower(targetTable)

	if strings.Contains(stmtLower, "create table") &&
		strings.Contains(stmtLower, targetLower) {
		createTableRegex, err := regexp.Compile(
			`(?i)create\s+table(?:\s+if\s+not\s+exists)?\s+(?:\w+\.)?(\w+)`,
		)
		if err != nil {
			return false
		}
		matches := createTableRegex.FindStringSubmatch(stmt)
		if len(matches) > 1 && strings.ToLower(matches[1]) == targetLower {
			return true
		}
	}

	if strings.Contains(stmtLower, "alter table") &&
		strings.Contains(stmtLower, targetLower) {
		alterTableRegex, err := regexp.Compile(
			`(?i)alter\s+table\s+(?:if\s+exists\s+)?(?:\w+\.)?(\w+)`,
		)
		if err != nil {
			return false
		}
		matches := alterTableRegex.FindStringSubmatch(stmt)
		if len(matches) > 1 && strings.ToLower(matches[1]) == targetLower {
			return true
		}
	}

	if strings.Contains(stmtLower, "drop table") &&
		strings.Contains(stmtLower, targetLower) {
		dropTableRegex, err := regexp.Compile(
			`(?i)drop\s+table(?:\s+if\s+exists)?\s+(?:\w+\.)?(\w+)`,
		)
		if err != nil {
			return false
		}
		matches := dropTableRegex.FindStringSubmatch(stmt)
		if len(matches) > 1 && strings.ToLower(matches[1]) == targetLower {
			return true
		}
	}

	return false
}
