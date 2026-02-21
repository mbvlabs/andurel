package internal

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mbvlabs/andurel/pkg/naming"
)

type Project struct {
	Dir          string
	Name         string
	T            *testing.T
	BinaryPath   string
	Database     string
	CSS          string
	SharedBinDir string
}

func NewProject(t *testing.T, andurelBinary, sharedBinDir string) *Project {
	t.Helper()

	tmpDir := t.TempDir()
	projectName := "testapp"

	return &Project{
		Dir:          filepath.Join(tmpDir, projectName),
		Name:         projectName,
		T:            t,
		BinaryPath:   naming.BinaryName(andurelBinary),
		Database:     "",
		SharedBinDir: sharedBinDir,
	}
}

func NewProjectWithDatabase(t *testing.T, andurelBinary, sharedBinDir, database string) *Project {
	t.Helper()

	tmpDir := t.TempDir()
	projectName := "testapp"

	return &Project{
		Dir:          filepath.Join(tmpDir, projectName),
		Name:         projectName,
		T:            t,
		BinaryPath:   naming.BinaryName(andurelBinary),
		Database:     database,
		SharedBinDir: sharedBinDir,
	}
}

func (p *Project) Scaffold(args ...string) error {
	p.T.Helper()

	env := []string{
		"ANDUREL_TEST_MODE=true",
		"ANDUREL_SKIP_TAILWIND=true",
	}

	baseArgs := []string{"new", p.Name}
	allArgs := append(baseArgs, args...)

	if err := RunCLI(p.T, p.BinaryPath, filepath.Dir(p.Dir), env, allArgs...); err != nil {
		return err
	}

	// Copy shared tools to the project's bin directory
	return p.setupToolBinaries()
}

// setupToolBinaries copies the pre-downloaded tools from the shared bin directory
// to the project's bin directory
func (p *Project) setupToolBinaries() error {
	p.T.Helper()

	if p.SharedBinDir == "" {
		return nil
	}

	projectBinDir := filepath.Join(p.Dir, "bin")
	if err := os.MkdirAll(projectBinDir, 0o755); err != nil {
		return err
	}

	// Copy each tool binary
	entries, err := os.ReadDir(p.SharedBinDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		srcPath := filepath.Join(p.SharedBinDir, entry.Name())
		dstPath := filepath.Join(projectBinDir, entry.Name())

		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

// copyFile copies a file from src to dst, preserving permissions
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}

	defer func() error {
		if err := dstFile.Close(); err != nil {
			return err
		}

		return nil
	}()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (p *Project) Generate(args ...string) error {
	p.T.Helper()

	env := []string{
		"ANDUREL_TEST_MODE=true",
	}

	return RunCLI(p.T, p.BinaryPath, p.Dir, env, args...)
}

// GenerateExpectError runs a generate command that is expected to fail, suppressing failure logs.
func (p *Project) GenerateExpectError(args ...string) error {
	p.T.Helper()

	env := []string{
		"ANDUREL_TEST_MODE=true",
	}

	return RunCommandExpectError(p.T, p.BinaryPath, p.Dir, env, args...)
}

func (p *Project) GoVet() error {
	p.T.Helper()

	return RunCommand(p.T, "go", p.Dir, nil, "vet", "./...")
}

func (p *Project) GoBuild(target string) error {
	p.T.Helper()

	return RunCommand(p.T, "go", p.Dir, nil, "build", target)
}

func (p *Project) FileExists(path string) bool {
	p.T.Helper()

	fullPath := filepath.Join(p.Dir, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

func (p *Project) DirExists(path string) bool {
	p.T.Helper()

	fullPath := filepath.Join(p.Dir, path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}
