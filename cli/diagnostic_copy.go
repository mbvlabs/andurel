package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var makeDiagnosticTempDir = os.MkdirTemp
var removeDiagnosticTempDir = os.RemoveAll

// diagnosticCopyOptions selects an optional narrowed copy mode. The zero value
// preserves the full-project copy used by diagnostics that run arbitrary
// tooling, such as go mod tidy.
type diagnosticCopyOptions struct {
	selectBinTools bool
	binTools       []string
}

func withDiagnosticProjectCopy(rootDir string, run func(tempRoot string) error) (err error) {
	return withDiagnosticProjectCopyOptions(rootDir, diagnosticCopyOptions{}, run)
}

// withDiagnosticProjectCopyTools copies the project but restricts the
// top-level bin directory to the named tool binaries. Generators only need
// their own binaries; copying every installed tool (some hundreds of
// megabytes) dominates a warm verify run. An empty tool list omits bin
// entirely. The original project is still never used as the generator working
// directory, and copied tools are independent regular files, not links.
func withDiagnosticProjectCopyTools(
	rootDir string,
	tools []string,
	run func(tempRoot string) error,
) (err error) {
	return withDiagnosticProjectCopyOptions(rootDir, diagnosticCopyOptions{
		selectBinTools: true,
		binTools:       append([]string(nil), tools...),
	}, run)
}

func withDiagnosticProjectCopyOptions(
	rootDir string,
	opts diagnosticCopyOptions,
	run func(tempRoot string) error,
) (err error) {
	tempParent, err := makeDiagnosticTempDir("", "andurel-diagnostic-*")
	if err != nil {
		return fmt.Errorf("create diagnostic temporary directory: %w", err)
	}
	defer func() {
		if cleanupErr := removeDiagnosticTempDir(tempParent); cleanupErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf("remove diagnostic temporary directory: %w", cleanupErr),
			)
		}
	}()

	tempRoot := filepath.Join(tempParent, filepath.Base(filepath.Clean(rootDir)))
	if err := copyDiagnosticProjectWithOptions(rootDir, tempRoot, opts); err != nil {
		return fmt.Errorf("copy project for diagnostics: %w", err)
	}
	return run(tempRoot)
}

func copyDiagnosticProject(src, dst string) error {
	return copyDiagnosticProjectWithOptions(src, dst, diagnosticCopyOptions{})
}

func copyDiagnosticProjectWithOptions(src, dst string, opts diagnosticCopyOptions) error {
	return copyDiagnosticEntry(src, dst, "", opts, make(map[string]struct{}))
}

func copyDiagnosticEntry(
	src, dst, rel string,
	opts diagnosticCopyOptions,
	active map[string]struct{},
) error {
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
			childRel := entry.Name()
			if rel != "" {
				childRel = filepath.Join(rel, entry.Name())
			}
			if opts.selectBinTools && childRel == "bin" {
				if err := copyDiagnosticSelectedBin(
					filepath.Join(src, entry.Name()),
					filepath.Join(dst, entry.Name()),
					opts.binTools,
					opts,
					active,
				); err != nil {
					return err
				}
				continue
			}
			if err := copyDiagnosticEntry(
				filepath.Join(src, entry.Name()),
				filepath.Join(dst, entry.Name()),
				childRel,
				opts,
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

// copyDiagnosticSelectedBin copies only the named immediate children of the
// project's top-level bin directory. It mirrors the symlink dereferencing,
// exclusions, cycle detection and permission preservation of the full copy.
// When none of the requested tools exist, bin is omitted rather than created
// empty.
func copyDiagnosticSelectedBin(
	src, dst string,
	tools []string,
	opts diagnosticCopyOptions,
	active map[string]struct{},
) error {
	if len(tools) == 0 {
		return nil
	}

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
	if !info.IsDir() {
		return fmt.Errorf("diagnostic bin path %s is not a directory", original)
	}

	wanted := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		wanted[tool] = struct{}{}
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read directory %s: %w", original, err)
	}
	selected := make([]os.DirEntry, 0, len(tools))
	for _, entry := range entries {
		if _, ok := wanted[entry.Name()]; !ok {
			continue
		}
		if isDiagnosticExcluded(entry.Name()) {
			continue
		}
		selected = append(selected, entry)
	}
	if len(selected) == 0 {
		return nil
	}

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
	for _, entry := range selected {
		if err := copyDiagnosticEntry(
			filepath.Join(src, entry.Name()),
			filepath.Join(dst, entry.Name()),
			filepath.Join("bin", entry.Name()),
			opts,
			active,
		); err != nil {
			return err
		}
	}
	if err := os.Chmod(dst, info.Mode().Perm()); err != nil {
		return fmt.Errorf("preserve directory permissions for %s: %w", original, err)
	}
	return nil
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
