// Package cmds holds commands being used for scaffolding
package cmds

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/layout/versions"
	"github.com/mbvlabs/andurel/pkg/storage"
)

var (
	absolutePath = filepath.Abs
	newCommand   = exec.Command
)

// RunGoModTidy runs go mod tidy.
func RunGoModTidy(targetDir string) error {
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmd := newCommand("go", "mod", "tidy")
	cmd.Dir = absTargetDir

	return cmd.Run()
}

// RunGoFmt runs go fmt.
func RunGoFmt(targetDir string) error {
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmd := newCommand("go", "fmt", "./...")
	cmd.Dir = absTargetDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go fmt failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// RunGoFmtPath runs go fmt path.
func RunGoFmtPath(targetDir, path string) error {
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	cmd := newCommand("go", "fmt", path)
	cmd.Dir = absTargetDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go fmt failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// RunGolines runs golines.
func RunGolines(targetDir string) error {
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := newCommand("golines", "-w", "-m", "100", ".")
	cmd.Dir = absTargetDir
	return cmd.Run()
}

// RunSQLCGenerate runs sqlc generate when models/queries contains SQL files.
func RunSQLCGenerate(targetDir string) error {
	return runSQLCGenerate(targetDir, false)
}

// RunSQLCGenerateOptional runs sqlc generate when queries and bin/sqlc exist.
// Missing queries or binary are ignored so scaffold flows stay no-op safe.
func RunSQLCGenerateOptional(targetDir string) error {
	return runSQLCGenerate(targetDir, true)
}

func runSQLCGenerate(targetDir string, skipMissingBinary bool) error {
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	hasQueries, err := storage.HasSQLCQueryFiles(absTargetDir)
	if err != nil {
		return fmt.Errorf("check sqlc queries: %w", err)
	}
	if !hasQueries {
		return nil
	}

	sqlcBin := filepath.Join(absTargetDir, "bin", "sqlc")
	if _, err := os.Stat(sqlcBin); err != nil {
		if skipMissingBinary {
			return nil
		}
		return fmt.Errorf("sqlc binary not found at %s: run 'andurel tool sync'", sqlcBin)
	}

	cmd := newCommand(sqlcBin, "generate")
	cmd.Dir = absTargetDir
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return fmt.Errorf("sqlc generate failed: %w\nOutput: %s", runErr, string(output))
	}

	return RunGoFmtPath(absTargetDir, "./models/internal/queries/...")
}

// RunTemplGenerate runs templ generate.
func RunTemplGenerate(targetDir string) error {
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := newCommand(
		"go",
		"run",
		"github.com/a-h/templ/cmd/templ@"+versions.Templ,
		"generate",
		"-path",
		".",
	)
	cmd.Dir = absTargetDir
	return cmd.Run()
}

// RunTemplFmt runs templ fmt.
func RunTemplFmt(targetDir string) error {
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := newCommand(
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
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	cmd := newCommand(
		"go",
		"run",
		"github.com/pressly/goose/v3/cmd/goose@"+versions.Goose,
		"-dir",
		"migrations",
		"fix",
	)
	cmd.Dir = absTargetDir
	return cmd.Run()
}
