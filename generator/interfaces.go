package generator

import "github.com/mbvlabs/andurel/generator/internal/catalog"

type CodeGenerator interface {
	Generate(cat *catalog.Catalog, resourceName, modulePath string) error
	ValidateInputs(resourceName string) error
}

type FileManager interface {
	WriteFile(path, content string) error
	EnsureDir(path string) error
	FormatGoFile(path string) error
	ValidateFileNotExists(path string) error
	FindGoModRoot() (string, error)
	RunBobGenerate() error
}

type ProjectManagerInterface interface {
	GetModulePath() (string, error)
	ValidateBobConfig(rootDir string) error
}

type MigrationManagerInterface interface {
	BuildCatalogFromMigrations(tableName string) (*catalog.Catalog, error)
}
