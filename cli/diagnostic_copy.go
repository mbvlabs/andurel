package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var makeDiagnosticTempDir = os.MkdirTemp
var removeDiagnosticTempDir = os.RemoveAll

func withDiagnosticProjectCopy(rootDir string, run func(tempRoot string) error) (err error) {
	tempParent, err := makeDiagnosticTempDir("", "andurel-diagnostic-*")
	if err != nil {
		return fmt.Errorf("create diagnostic temporary directory: %w", err)
	}
	defer func() {
		if cleanupErr := removeDiagnosticTempDir(tempParent); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("remove diagnostic temporary directory: %w", cleanupErr))
		}
	}()

	tempRoot := filepath.Join(tempParent, filepath.Base(filepath.Clean(rootDir)))
	if err := copyDiagnosticProject(rootDir, tempRoot); err != nil {
		return fmt.Errorf("copy project for diagnostics: %w", err)
	}
	return run(tempRoot)
}

func copyDiagnosticProject(src, dst string) error {
	return copyDiagnosticEntry(src, dst, make(map[string]struct{}))
}

func copyDiagnosticEntry(src, dst string, active map[string]struct{}) error {
	original := src
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", original, err)
	}
	if isDiagnosticExcluded(info.Name()) {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, resolveErr := filepath.EvalSymlinks(src)
		if resolveErr != nil {
			return fmt.Errorf("resolve symlink %s: %w", original, resolveErr)
		}
		src = resolved
		if isDiagnosticExcludedPath(src) {
			return nil
		}
		info, err = os.Stat(src)
		if err != nil {
			return fmt.Errorf("inspect resolved symlink %s: %w", original, err)
		}
	}

	switch {
	case info.Mode().IsRegular():
		if err := copyFile(src, dst, info.Mode()); err != nil {
			return fmt.Errorf("copy file %s: %w", original, err)
		}
		if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
			return fmt.Errorf("preserve file permissions for %s: %w", original, err)
		}
		return nil
	case info.IsDir():
		canonical, err := filepath.EvalSymlinks(src)
		if err != nil {
			return fmt.Errorf("resolve directory %s: %w", original, err)
		}
		canonical, err = filepath.Abs(canonical)
		if err != nil {
			return fmt.Errorf("resolve absolute directory %s: %w", original, err)
		}
		if _, exists := active[canonical]; exists {
			return fmt.Errorf("diagnostic copy cycle at %s", original)
		}
		active[canonical] = struct{}{}
		defer delete(active, canonical)
		if err := os.MkdirAll(dst, 0o700); err != nil {
			return fmt.Errorf("create directory for %s: %w", original, err)
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return fmt.Errorf("read directory %s: %w", original, err)
		}
		for _, entry := range entries {
			if isDiagnosticExcluded(entry.Name()) {
				continue
			}
			if err := copyDiagnosticEntry(
				filepath.Join(src, entry.Name()),
				filepath.Join(dst, entry.Name()),
				active,
			); err != nil {
				return err
			}
		}
		if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
			return fmt.Errorf("preserve directory permissions for %s: %w", original, err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported special file %s with mode %s", original, info.Mode())
	}
}

func isDiagnosticExcluded(name string) bool {
	return name == ".git" || name == "node_modules" || name == ".andurel-cache"
}

func isDiagnosticExcludedPath(path string) bool {
	current := filepath.Clean(path)
	for {
		if isDiagnosticExcluded(filepath.Base(current)) {
			return true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}
