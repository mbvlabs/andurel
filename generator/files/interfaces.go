package files

import "os"

// FileReader handles file reading operations
type FileReader interface {
	ReadFile(path string) (string, error)
	FileExists(path string) bool
}

// FileWriter handles file writing operations
type FileWriter interface {
	WriteFile(path, content string) error
	EnsureDir(path string) error
}

// FileValidator handles file validation operations
type FileValidator interface {
	ValidateFileNotExists(path string) error
	ValidateFileExists(path string) error
}

// FileFormatter handles file formatting operations
type FileFormatter interface {
	FormatGoFile(path string) error
}

// ProjectLocator handles project-related file operations
type ProjectLocator interface {
	FindGoModRoot() (string, error)
}

// SQLCRunner handles SQLC operations
type SQLCRunner interface {
	RunSQLCGenerate() error
}

// FileManager combines all file-related interfaces
type FileManager interface {
	FileReader
	FileWriter
	FileValidator
	FileFormatter
	ProjectLocator
	SQLCRunner
}

// EnhancedFileManager extends FileManager with additional methods
type EnhancedFileManager interface {
	FileManager
	WriteFileWithPermissions(path, content string, perm os.FileMode) error
	EnsureDirWithPermissions(path string, perm os.FileMode) error
	GetPermissions() FilePermissions
	SetPermissions(permissions FilePermissions)
}

// Ensure UnifiedFileManager implements the interfaces
var (
	_ FileReader          = (*UnifiedFileManager)(nil)
	_ FileWriter          = (*UnifiedFileManager)(nil)
	_ FileValidator       = (*UnifiedFileManager)(nil)
	_ FileFormatter       = (*UnifiedFileManager)(nil)
	_ ProjectLocator      = (*UnifiedFileManager)(nil)
	_ SQLCRunner          = (*UnifiedFileManager)(nil)
	_ FileManager         = (*UnifiedFileManager)(nil)
	_ EnhancedFileManager = (*UnifiedFileManager)(nil)
)
