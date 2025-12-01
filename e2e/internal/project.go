package internal

import (
	"os"
	"path/filepath"
	"testing"
)

type Project struct {
	Dir        string
	Name       string
	T          *testing.T
	BinaryPath string
	Database   string
}

func NewProject(t *testing.T, andurelBinary string) *Project {
	t.Helper()

	tmpDir := t.TempDir()
	projectName := "testapp"

	return &Project{
		Dir:        filepath.Join(tmpDir, projectName),
		Name:       projectName,
		T:          t,
		BinaryPath: andurelBinary,
		Database:   "",
	}
}

func NewProjectWithDatabase(t *testing.T, andurelBinary, database string) *Project {
	t.Helper()

	tmpDir := t.TempDir()
	projectName := "testapp"

	return &Project{
		Dir:        filepath.Join(tmpDir, projectName),
		Name:       projectName,
		T:          t,
		BinaryPath: andurelBinary,
		Database:   database,
	}
}

func (p *Project) Scaffold(args ...string) error {
	p.T.Helper()

	env := []string{
		"ANDUREL_TEST_MODE=true",
		"ANDUREL_SKIP_TAILWIND=true",
		"ANDUREL_SKIP_BUILD=true",
	}

	baseArgs := []string{"new", p.Name}
	allArgs := append(baseArgs, args...)

	return RunCLI(p.T, p.BinaryPath, filepath.Dir(p.Dir), env, allArgs...)
}

func (p *Project) Generate(args ...string) error {
	p.T.Helper()

	env := []string{
		"ANDUREL_TEST_MODE=true",
	}

	return RunCLI(p.T, p.BinaryPath, p.Dir, env, args...)
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
