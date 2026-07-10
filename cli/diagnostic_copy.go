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
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == ".andurel-cache") {
			return filepath.SkipDir
		}

		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		}
		return copyFile(path, target, info.Mode())
	})
}
