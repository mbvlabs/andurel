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

// RunNarsilcGenerate runs narsilc generate when models/queries contains SQL files.
// If bin/narsilc is not installed yet, it falls back to
// go run github.com/mbvlabs/narsilc/cmd/narsilc@<version> so scaffold can
// emit models/internal/queries. go mod tidy runs before go fmt because
// rewriting go.mod from the scaffold template leaves the module untidy.
func RunNarsilcGenerate(targetDir string) error {
	return runNarsilcGenerate(targetDir)
}

// RunNarsilcGenerateOptional runs narsilc generate when annotated query files exist.
func RunNarsilcGenerateOptional(targetDir string) error {
	return runNarsilcGenerate(targetDir)
}

func runNarsilcGenerate(targetDir string) error {
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	hasQueries, err := storage.HasQueryFiles(absTargetDir)
	if err != nil {
		return fmt.Errorf("check narsilc queries: %w", err)
	}
	if !hasQueries {
		return nil
	}

	narsilcBin := filepath.Join(absTargetDir, "bin", "narsilc")
	var cmd *exec.Cmd
	if _, err := os.Stat(narsilcBin); err == nil {
		cmd = newCommand(narsilcBin, "generate")
	} else {
		cmd = newCommand(
			"go",
			"run",
			"github.com/mbvlabs/narsilc/cmd/narsilc@"+versions.NarsilcModule,
			"generate",
		)
	}
	cmd.Dir = absTargetDir
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return fmt.Errorf("narsilc generate failed: %w\nOutput: %s", runErr, string(output))
	}

	if err := RunGoModTidy(absTargetDir); err != nil {
		return fmt.Errorf("go mod tidy after narsilc generate: %w", err)
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
