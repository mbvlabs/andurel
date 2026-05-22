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
	relevantNames := collectRelevantNames(migrationsList, tableName)

	for _, migration := range migrationsList {
		for _, stmt := range migration.Statements {
			if isRelevantForTable(stmt, relevantNames) {
				if err := ddl.ApplyDDL(cat, stmt, migration.FilePath, databaseType); err != nil {
					return nil, fmt.Errorf(
						"failed to apply DDL from %s: %w",
						migration.FilePath,
						err,
					)
				}
			}
		}
	}

	if _, err := cat.GetTable(cat.DefaultSchema, tableName); err != nil {
		return nil, fmt.Errorf(
			"table '%s' not found in any migration. Create a migration for this table or use --table-name to specify a different table name",
			tableName,
		)
	}

	return cat, nil
}

func collectRelevantNames(
	migrationsList []migrations.Migration,
	targetTable string,
) map[string]bool {
	targetLower := strings.ToLower(targetTable)
	names := map[string]bool{targetLower: true}

	renameRe := regexp.MustCompile(
		`(?i)alter\s+table\s+(?:if\s+exists\s+)?(?:\w+\.)?(\w+)\s+.*rename\s+to\s+(\w+)`,
	)

	for i := 0; i < 100; i++ {
		changed := false
		for _, migration := range migrationsList {
			for _, stmt := range migration.Statements {
				matches := renameRe.FindStringSubmatch(stmt)
				if len(matches) > 2 {
					srcName := strings.ToLower(matches[1])
					dstName := strings.ToLower(matches[2])
					if names[dstName] && !names[srcName] {
						names[srcName] = true
						changed = true
					}
				}
			}
		}
		if !changed {
			break
		}
	}

	return names
}

func isRelevantForTable(stmt string, relevantNames map[string]bool) bool {
	stmtLower := strings.ToLower(stmt)

	var tableName string

	switch {
	case strings.Contains(stmtLower, "create table"):
		re := regexp.MustCompile(
			`(?i)create\s+table(?:\s+if\s+not\s+exists)?\s+(?:\w+\.)?(\w+)`,
		)
		matches := re.FindStringSubmatch(stmt)
		if len(matches) > 1 {
			tableName = strings.ToLower(matches[1])
		}
	case strings.Contains(stmtLower, "alter table"):
		re := regexp.MustCompile(
			`(?i)alter\s+table\s+(?:if\s+exists\s+)?(?:\w+\.)?(\w+)`,
		)
		matches := re.FindStringSubmatch(stmt)
		if len(matches) > 1 {
			tableName = strings.ToLower(matches[1])
		}
	case strings.Contains(stmtLower, "drop table"):
		re := regexp.MustCompile(
			`(?i)drop\s+table(?:\s+if\s+exists)?\s+(?:\w+\.)?(\w+)`,
		)
		matches := re.FindStringSubmatch(stmt)
		if len(matches) > 1 {
			tableName = strings.ToLower(matches[1])
		}
	}

	return tableName != "" && relevantNames[tableName]
}
