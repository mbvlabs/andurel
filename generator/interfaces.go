package generator

import (
	"github.com/mbvlabs/andurel/generator/internal/catalog"
)

type CodeGenerator interface {
	Generate(cat *catalog.Catalog, resourceName, modulePath string) error
	ValidateInputs(resourceName string) error
}

type ProjectManagerInterface interface {
	GetModulePath() (string, error)
	ValidateSQLCConfig(rootDir string) error
}

type MigrationManagerInterface interface {
	BuildCatalogFromMigrations(tableName string) (*catalog.Catalog, error)
}
