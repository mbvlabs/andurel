// Package cmds holds commands being used for scaffolding
package cmds

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/mbvlabs/andurel/v2/internal/testseed"
	"github.com/mbvlabs/andurel/v2/layout/versions"
)

func resolveProjectTool(name, projectDir string) string {
	if dir := os.Getenv("ANDUREL_TOOL_BIN"); dir != "" {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	candidate := filepath.Join(projectDir, "bin", name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return ""
}

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
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, out)
	}
	return nil
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
// It prefers ANDUREL_TOOL_BIN, then bin/narsilc, then PATH, then falls back to
// go run github.com/mbvlabs/narsilc/cmd/narsilc@<version> so scaffold can
// emit models/internal/queries. Outside golden builds, go mod tidy runs before
// go fmt so new query packages resolve. Golden builds skip tidy here — slim
// fixtures lack a full require graph, and layout.Scaffold already tidies when
// FullScaffold() is on.
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

	var cmd *exec.Cmd
	if narsilcBin := resolveProjectTool("narsilc", absTargetDir); narsilcBin != "" {
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

	// Golden CLI (PR or full) skips tidy here. ANDUREL_GOLDEN_FULL=1 would
	// otherwise re-enable tidy for every generate/sync fixture and break on
	// incomplete go.mod graphs. Full `andurel new` still tidies in Scaffold.
	if !testseed.Enabled() {
		if err := RunGoModTidy(absTargetDir); err != nil {
			return fmt.Errorf("go mod tidy after narsilc generate: %w", err)
		}
	}

	return RunGoFmtPath(absTargetDir, "./models/internal/queries/...")
}

// RunTemplGenerate runs templ generate.
func RunTemplGenerate(targetDir string) error {
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	var cmd *exec.Cmd
	if templBin := resolveProjectTool("templ", absTargetDir); templBin != "" {
		cmd = newCommand(templBin, "generate", "-path", ".")
	} else {
		cmd = newCommand(
			"go",
			"run",
			"github.com/a-h/templ/cmd/templ@"+versions.Templ,
			"generate",
			"-path",
			".",
		)
	}
	cmd.Dir = absTargetDir
	return cmd.Run()
}

// RunTemplFmt runs templ fmt.
func RunTemplFmt(targetDir string) error {
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	var cmd *exec.Cmd
	if templBin := resolveProjectTool("templ", absTargetDir); templBin != "" {
		cmd = newCommand(templBin, "fmt", "views")
	} else {
		cmd = newCommand(
			"go",
			"run",
			"github.com/a-h/templ/cmd/templ@"+versions.Templ,
			"fmt",
			"views",
		)
	}
	cmd.Dir = absTargetDir
	return cmd.Run()
}

// RunGooseFix runs goose fix.
func RunGooseFix(targetDir string) error {
	absTargetDir, err := absolutePath(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	var cmd *exec.Cmd
	if gooseBin := resolveProjectTool("goose", absTargetDir); gooseBin != "" {
		cmd = newCommand(gooseBin, "-dir", "migrations", "fix")
	} else {
		cmd = newCommand(
			"go",
			"run",
			"github.com/pressly/goose/v3/cmd/goose@"+versions.Goose,
			"-dir",
			"migrations",
			"fix",
		)
	}
	cmd.Dir = absTargetDir
	return cmd.Run()
}
