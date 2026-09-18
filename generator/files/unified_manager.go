package files

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mbvlabs/andurel/internal/cache"
	"github.com/mbvlabs/andurel/internal/constants"
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
	_ ProjectLocator = (*UnifiedManager)(nil)
	_ Manager        = (*UnifiedManager)(nil)
)

// FormatGoFile formats a Go file using goimports and go fmt.
// Tool binaries are resolved via LookPath (and ANDUREL_TOOL_BIN when set), so
// CI and golden harnesses can put pinned goimports on PATH / in that bin dir.
//
// goimports runs with GOWORK=off and cwd set to the file's directory so an
// ambient go.work (e.g. this repo's workspace) cannot change import resolution
// for generated project files.
func FormatGoFile(path string) error {
	goimportsPath, err := resolveTool("goimports")
	if err != nil {
		return &FileOperationError{
			Operation: "goimports",
			Path:      path,
			Err:       err,
		}
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return &FileOperationError{
			Operation: "goimports",
			Path:      path,
			Err:       err,
		}
	}
	fileDir := filepath.Dir(absPath)

	cmd := exec.Command(goimportsPath, "-w", absPath)
	cmd.Dir = fileDir
	cmd.Env = toolEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return &FileOperationError{
			Operation: "goimports",
			Path:      path,
			Err:       err,
			Output:    string(out),
		}
	}

	cmd = exec.Command("go", "fmt", absPath)
	cmd.Dir = fileDir
	cmd.Env = toolEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return &FileOperationError{
			Operation: "go_fmt",
			Path:      path,
			Err:       err,
			Output:    string(out),
		}
	}

	return nil
}

// toolEnv is the process environment for formatter subprocesses: inherit the
// current env but force GOWORK=off so workspace mode cannot rewrite imports.
func toolEnv() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, "GOWORK=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, "GOWORK=off")
}

// resolveTool finds a formatter/codegen binary. ANDUREL_TOOL_BIN, when set,
// is checked first so golden/CI installs of pinned versions win over PATH drift.
func resolveTool(name string) (string, error) {
	if dir := os.Getenv("ANDUREL_TOOL_BIN"); dir != "" {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found in ANDUREL_TOOL_BIN or PATH: %w", name, err)
	}
	return path, nil
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

// GetPermissions returns the current file permissions
func (fm *UnifiedManager) GetPermissions() Permissions {
	return fm.permissions
}

// SetPermissions updates file permissions
func (fm *UnifiedManager) SetPermissions(permissions Permissions) {
	fm.permissions = permissions
}
