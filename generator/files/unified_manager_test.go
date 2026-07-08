package files

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/pkg/cache"
)

func TestUnifiedManagerFileOperations(t *testing.T) {
	manager := NewUnifiedFileManager()
	root := t.TempDir()

	privatePath := filepath.Join(root, "private", "note.txt")
	if err := manager.WriteFile(privatePath, "secret"); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if got, err := manager.ReadFile(privatePath); err != nil || got != "secret" {
		t.Fatalf("ReadFile = %q, %v", got, err)
	}
	if !manager.FileExists(privatePath) {
		t.Fatal("FileExists should return true for written file")
	}
	info, err := os.Stat(privatePath)
	if err != nil {
		t.Fatalf("stat private file: %v", err)
	}
	if got := info.Mode().Perm(); got != manager.GetPermissions().FilePrivate {
		t.Fatalf("private file mode = %v, want %v", got, manager.GetPermissions().FilePrivate)
	}

	publicPath := filepath.Join(root, "public.txt")
	if err := manager.WriteFileWithPermissions(publicPath, "public", 0o644); err != nil {
		t.Fatalf("WriteFileWithPermissions: %v", err)
	}
	info, err = os.Stat(publicPath)
	if err != nil {
		t.Fatalf("stat public file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("public file mode = %v, want 0644", got)
	}
}

func TestUnifiedManagerValidationAndPermissions(t *testing.T) {
	manager := NewUnifiedFileManager()
	root := t.TempDir()

	custom := manager.GetPermissions()
	custom.FilePrivate = 0o640
	custom.DirDefault = 0o750
	manager.SetPermissions(custom)
	if got := manager.GetPermissions(); got.FilePrivate != 0o640 || got.DirDefault != 0o750 {
		t.Fatalf("permissions not updated: %#v", got)
	}

	dir := filepath.Join(root, "nested")
	if err := manager.EnsureDirWithPermissions(dir, 0o755); err != nil {
		t.Fatalf("EnsureDirWithPermissions: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("dir mode = %v, want 0755", got)
	}

	path := filepath.Join(root, "existing.txt")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	if err := manager.ValidateFileExists(path); err != nil {
		t.Fatalf("ValidateFileExists existing file: %v", err)
	}
	err = manager.ValidateFileNotExists(path)
	if err == nil || !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected os.ErrExist, got %v", err)
	}

	missing := filepath.Join(root, "missing.txt")
	if err := manager.ValidateFileNotExists(missing); err != nil {
		t.Fatalf("ValidateFileNotExists missing file: %v", err)
	}
	err = manager.ValidateFileExists(missing)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestFileOperationError(t *testing.T) {
	cause := errors.New("boom")

	err := &FileOperationError{Operation: "write", Path: "models/user.go", Err: cause}
	if !errors.Is(err, cause) {
		t.Fatal("FileOperationError should unwrap cause")
	}
	if got := err.Error(); got != "file operation 'write' failed for path 'models/user.go': boom" {
		t.Fatalf("Error() = %q", got)
	}

	err.Output = "compiler output"
	if got := err.Error(); !strings.Contains(got, "compiler output") {
		t.Fatalf("Error() with output = %q", got)
	}
}

func TestFindGoModRoot(t *testing.T) {
	cache.ClearFileSystemCache()
	t.Cleanup(cache.ClearFileSystemCache)

	manager := NewUnifiedFileManager()
	root := t.TempDir()
	nested := filepath.Join(root, "cmd", "app")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir nested: %v", err)
	}

	found, err := manager.FindGoModRoot()
	if err != nil {
		t.Fatalf("FindGoModRoot: %v", err)
	}
	if found != root {
		t.Fatalf("FindGoModRoot = %q, want %q", found, root)
	}
}
