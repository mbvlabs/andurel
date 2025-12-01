package files

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/mbvlabs/andurel/layout/versions"
	"github.com/mbvlabs/andurel/pkg/cache"
	"github.com/mbvlabs/andurel/pkg/constants"
)

// UnifiedManager provides centralized file operations with consistent error handling
type UnifiedManager struct {
	permissions Permissions
	cache       *cache.FileSystemCache
}

// Permissions defines file permission settings
type Permissions struct {
	FilePrivate   os.FileMode
	FilePublic    os.FileMode
	DirDefault    os.FileMode
	DirExecutable os.FileMode
}

// DefaultPermissions returns standard permission settings
func DefaultPermissions() Permissions {
	return Permissions{
		FilePrivate:   constants.FilePermissionPrivate,
		FilePublic:    constants.FilePermissionPublic,
		DirDefault:    constants.DirPermissionDefault,
		DirExecutable: 0o755,
	}
}

// NewUnifiedFileManager creates a new unified file manager
func NewUnifiedFileManager() *UnifiedManager {
	return &UnifiedManager{
		permissions: DefaultPermissions(),
		cache:       cache.NewFileSystemCache(5 * time.Minute),
	}
}

// WriteFile writes content to a file, creating directories as needed
func (fm *UnifiedManager) WriteFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := fm.EnsureDir(dir); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(content), fm.permissions.FilePrivate)
}

// WriteFileWithPermissions writes content to a file with specific permissions
func (fm *UnifiedManager) WriteFileWithPermissions(path, content string, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := fm.EnsureDir(dir); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(content), perm)
}

// ReadFile reads content from a file
func (fm *UnifiedManager) ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// FileExists checks if a file exists (with caching)
func (fm *UnifiedManager) FileExists(path string) bool {
	return cache.GetFileExists("file_exists:"+path, func() bool {
		_, err := os.Stat(path)
		return err == nil
	})
}

// EnsureDir creates a directory if it doesn't exist
func (fm *UnifiedManager) EnsureDir(path string) error {
	return os.MkdirAll(path, fm.permissions.DirDefault)
}

// EnsureDirWithPermissions creates a directory with specific permissions
func (fm *UnifiedManager) EnsureDirWithPermissions(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// ValidateFileNotExists returns an error if file already exists
func (fm *UnifiedManager) ValidateFileNotExists(path string) error {
	if fm.FileExists(path) {
		return &FileOperationError{
			Operation: "validate_not_exists",
			Path:      path,
			Err:       os.ErrExist,
		}
	}
	return nil
}

// ValidateFileExists returns an error if file doesn't exist
func (fm *UnifiedManager) ValidateFileExists(path string) error {
	if !fm.FileExists(path) {
		return &FileOperationError{
			Operation: "validate_exists",
			Path:      path,
			Err:       os.ErrNotExist,
		}
	}
	return nil
}

// Ensure all interface methods are implemented by UnifiedFileManager
var (
	_ Reader         = (*UnifiedManager)(nil)
	_ Writer         = (*UnifiedManager)(nil)
	_ Validator      = (*UnifiedManager)(nil)
	_ Formatter      = (*UnifiedManager)(nil)
	_ ProjectLocator = (*UnifiedManager)(nil)
	_ SQLCRunner     = (*UnifiedManager)(nil)
	_ Manager        = (*UnifiedManager)(nil)
)

// FormatGoFile formats a Go file using gofmt
func (fm *UnifiedManager) FormatGoFile(path string) error {
	cmd := exec.Command("gofmt", "-w", path)
	if err := cmd.Run(); err != nil {
		return &FileOperationError{
			Operation: "format_go",
			Path:      path,
			Err:       err,
		}
	}
	return nil
}

// FindGoModRoot finds the root directory containing go.mod (with caching)
func (fm *UnifiedManager) FindGoModRoot() (string, error) {
	return cache.GetDirectoryRoot("go_mod_root", func() (string, error) {
		dir, err := os.Getwd()
		if err != nil {
			return "", &FileOperationError{
				Operation: "find_gomod_root",
				Path:      ".",
				Err:       err,
			}
		}

		for {
			goModPath := filepath.Join(dir, "go.mod")
			if _, err := os.Stat(goModPath); err == nil {
				return dir, nil
			}

			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}

		return "", &FileOperationError{
			Operation: "find_gomod_root",
			Path:      ".",
			Err:       os.ErrNotExist,
		}
	})
}

// RunSQLCGenerate runs sqlc compile and generate commands
func (fm *UnifiedManager) RunSQLCGenerate() error {
	rootDir, err := fm.FindGoModRoot()
	if err != nil {
		return &FileOperationError{
			Operation: "sqlc_generate",
			Path:      ".",
			Err:       err,
		}
	}

	// Compile
	if err := fm.runSQLCCommand(rootDir, "compile"); err != nil {
		return err
	}

	// Generate
	if err := fm.runSQLCCommand(rootDir, "generate"); err != nil {
		return err
	}

	return nil
}

// runSQLCCommand runs a specific sqlc command
func (fm *UnifiedManager) runSQLCCommand(rootDir, command string) error {
	var cmd *exec.Cmd

	if os.Getenv("ANDUREL_SKIP_BUILD") == "true" {
		cmd = exec.Command("go", "run", "github.com/sqlc-dev/sqlc/cmd/sqlc@"+versions.Sqlc, "-f", "./database/sqlc.yaml", command)
	} else {
		sqlcBin := filepath.Join(rootDir, "bin", "sqlc")
		cmd = exec.Command(sqlcBin, "-f", "./database/sqlc.yaml", command)
	}

	cmd.Dir = rootDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &FileOperationError{
			Operation: "sqlc_" + command,
			Path:      rootDir,
			Err:       err,
			Output:    string(output),
		}
	}

	return nil
}

// GetPermissions returns the current file permissions
func (fm *UnifiedManager) GetPermissions() Permissions {
	return fm.permissions
}

// SetPermissions updates file permissions
func (fm *UnifiedManager) SetPermissions(permissions Permissions) {
	fm.permissions = permissions
}
