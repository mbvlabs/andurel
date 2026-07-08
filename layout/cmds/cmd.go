// Package cmds holds commands being used for scaffolding
package cmds

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/layout/versions"
)

// RunGoModTidy runs go mod tidy.
func RunGoModTidy(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = absTargetDir

	return cmd.Run()
}

// RunGoFmt runs go fmt.
func RunGoFmt(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Dir = absTargetDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go fmt failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// RunGoFmtPath runs go fmt path.
func RunGoFmtPath(targetDir, path string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmd := exec.Command("go", "fmt", path)
	cmd.Dir = absTargetDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go fmt failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// RunGolines runs golines.
func RunGolines(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := exec.Command("golines", "-w", "-m", "100", ".")
	cmd.Dir = absTargetDir
	return cmd.Run()
}

// RunTemplGenerate runs templ generate.
func RunTemplGenerate(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := exec.Command(
		"go",
		"run",
		"github.com/a-h/templ/cmd/templ@"+versions.Templ,
		"generate",
		"./views",
	)
	cmd.Dir = absTargetDir
	return cmd.Run()
}

// RunTemplFmt runs templ fmt.
func RunTemplFmt(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := exec.Command(
		"go",
		"run",
		"github.com/a-h/templ/cmd/templ@"+versions.Templ,
		"fmt",
		"views",
	)
	cmd.Dir = absTargetDir
	return cmd.Run()
}

// RunGooseFix runs goose fix.
func RunGooseFix(targetDir string) error {
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := exec.Command(
		"go",
		"run",
		"github.com/pressly/goose/v3/cmd/goose@"+versions.Goose,
		"-dir",
		"database/migrations",
		"fix",
	)
	cmd.Dir = absTargetDir
	return cmd.Run()
}
