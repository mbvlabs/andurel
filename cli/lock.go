package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/spf13/cobra"
)

var goTools = map[string]string{
	"templ":   "github.com/a-h/templ/cmd/templ",
	"sqlc":    "github.com/sqlc-dev/sqlc/cmd/sqlc",
	"goose":   "github.com/pressly/goose/v3/cmd/goose",
	"air":     "github.com/air-verse/air",
	"mailpit": "github.com/axllent/mailpit",
	"usql":    "github.com/xo/usql",
}

var binaryTools = map[string]bool{
	"tailwindcli": true,
}

func newSetVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-version <tool> <version>",
		Short: "Set a specific version for a tool",
		Long: `Set the version of a tool and update it.

Go tools (downloaded from GitHub releases):
  templ        - Templ templating engine
  sqlc         - SQL compiler
  goose        - Database migrations
  air          - Live reload
  mailpit      - Email testing
  usql         - Universal SQL CLI

Binary tools (downloaded from GitHub):
  tailwindcli  - Tailwind CSS CLI

The version should be specified WITHOUT the "v" prefix.

Examples:
  andurel tool set-version templ 0.3.950
  andurel tool set-version sqlc 1.28.0
  andurel tool set-version tailwindcli 4.1.17

This updates andurel.lock and downloads the tool binary to bin/.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			toolName := args[0]
			version := args[1]

			projectRoot, err := findGoModRoot()
			if err != nil {
				return fmt.Errorf("not in an andurel project directory: %w", err)
			}

			return setVersion(projectRoot, toolName, version)
		},
	}
}

func setVersion(projectRoot, toolName, version string) error {
	_, isGoTool := goTools[toolName]
	_, isBinaryTool := binaryTools[toolName]

	if !isGoTool && !isBinaryTool {
		return fmt.Errorf("unknown tool: %s\n\nSupported Go tools:\n  templ, sqlc, goose, air, mailpit, usql\n\nSupported binary tools:\n  tailwindcli\n\nRun 'andurel tool set-version --help' for more information", toolName)
	}

	if version == "" {
		return fmt.Errorf("version cannot be empty\n\nExample: andurel tool set-version %s 1.0.0", toolName)
	}

	if len(version) > 0 && version[0] == 'v' {
		version = version[1:]
	}

	versionWithV := "v" + version

	if isGoTool {
		return setGoToolVersion(projectRoot, toolName, versionWithV)
	}

	return setBinaryToolVersion(projectRoot, toolName, versionWithV)
}

func setGoToolVersion(projectRoot, toolName, version string) error {
	modulePath := goTools[toolName]
	moduleRepo := extractModulePath(modulePath)

	// Read and update lock file
	lockPath := filepath.Join(projectRoot, "andurel.lock")
	if _, err := os.Stat(lockPath); err != nil {
		return fmt.Errorf("andurel.lock not found. Are you in an andurel project?")
	}

	lock, err := layout.ReadLockFile(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to read lock file: %w", err)
	}

	lock.AddTool(toolName, layout.NewGoTool(moduleRepo, version))

	if err := lock.WriteLockFile(projectRoot); err != nil {
		return fmt.Errorf("failed to update lock file: %w", err)
	}

	fmt.Printf("Setting %s to version %s...\n", toolName, version)

	// Remove old binary if it exists
	binPath := filepath.Join(projectRoot, "bin", toolName)
	if _, err := os.Stat(binPath); err == nil {
		fmt.Printf("  - Removing old binary...\n")
		if err := os.Remove(binPath); err != nil {
			return fmt.Errorf("failed to remove old binary: %w", err)
		}
	}

	// Download new binary
	fmt.Printf("  - Downloading %s %s...\n", toolName, version)

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if err := cmds.DownloadGoTool(toolName, moduleRepo, version, goos, goarch, binPath); err != nil {
		return fmt.Errorf("failed to download %s: %w", toolName, err)
	}

	fmt.Printf("\n✓ Successfully updated %s to %s\n", toolName, version)
	return nil
}

func extractModulePath(module string) string {
	parts := strings.Split(module, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], "/")
	}
	return module
}

func setBinaryToolVersion(projectRoot, toolName, version string) error {
	lockPath := filepath.Join(projectRoot, "andurel.lock")
	if _, err := os.Stat(lockPath); err != nil {
		return fmt.Errorf("andurel.lock not found. Are you in an andurel project?")
	}

	lock, err := layout.ReadLockFile(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to read lock file: %w", err)
	}

	fmt.Printf("Setting %s to version %s...\n", toolName, version)

	binPath := filepath.Join(projectRoot, "bin", toolName)
	if _, err := os.Stat(binPath); err == nil {
		fmt.Printf("  - Removing old binary...\n")
		if err := os.Remove(binPath); err != nil {
			return fmt.Errorf("failed to remove old binary: %w", err)
		}
	}

	fmt.Printf("  - Downloading %s %s...\n", toolName, version)

	if toolName == "tailwindcli" {
		if err := cmds.DownloadTailwindCLI(version, runtime.GOOS, runtime.GOARCH, binPath); err != nil {
			return fmt.Errorf("failed to download %s: %w", toolName, err)
		}
	} else {
		return fmt.Errorf("unknown binary tool: %s", toolName)
	}

	lock.AddTool(toolName, layout.NewBinaryTool(version))

	if err := lock.WriteLockFile(projectRoot); err != nil {
		return fmt.Errorf("failed to update lock file: %w", err)
	}

	fmt.Printf("\n✓ Successfully updated %s to %s\n", toolName, version)
	return nil
}
