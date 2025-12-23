package files

import "os"

// Reader handles file reading operations
type Reader interface {
	ReadFile(path string) (string, error)
	FileExists(path string) bool
}

// Writer handles file writing operations
type Writer interface {
	WriteFile(path, content string) error
	EnsureDir(path string) error
}

// Validator handles file validation operations
type Validator interface {
	ValidateFileNotExists(path string) error
	ValidateFileExists(path string) error
}

// ProjectLocator handles project-related file operations
type ProjectLocator interface {
	FindGoModRoot() (string, error)
}

// SQLCRunner handles SQLC operations
type SQLCRunner interface {
	RunSQLCGenerate() error
}

// Manager combines all file-related interfaces
type Manager interface {
	Reader
	Writer
	Validator
	ProjectLocator
	SQLCRunner
}

// EnhancedFileManager extends FileManager with additional methods
type EnhancedFileManager interface {
	Manager
	WriteFileWithPermissions(path, content string, perm os.FileMode) error
	EnsureDirWithPermissions(path string, perm os.FileMode) error
	GetPermissions() Permissions
	SetPermissions(permissions Permissions)
}

// Ensure UnifiedFileManager implements the interfaces
var (
	_ Reader              = (*UnifiedManager)(nil)
	_ Writer              = (*UnifiedManager)(nil)
	_ Validator           = (*UnifiedManager)(nil)
	_ ProjectLocator      = (*UnifiedManager)(nil)
	_ SQLCRunner          = (*UnifiedManager)(nil)
	_ Manager             = (*UnifiedManager)(nil)
	_ EnhancedFileManager = (*UnifiedManager)(nil)
)
