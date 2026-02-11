package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

type managedTool struct {
	Name        string
	Module      string
	Description string
}

var managedTools = []managedTool{
	{Name: "templ", Module: "github.com/a-h/templ/cmd/templ", Description: "Templ templating engine"},
	{Name: "sqlc", Module: "github.com/sqlc-dev/sqlc/cmd/sqlc", Description: "SQL compiler"},
	{Name: "goose", Module: "github.com/pressly/goose/v3/cmd/goose", Description: "Database migrations"},
	{Name: "mailpit", Module: "github.com/axllent/mailpit", Description: "Email testing"},
	{Name: "usql", Module: "github.com/xo/usql", Description: "Universal SQL CLI"},
	{Name: "dblab", Module: "github.com/danvergara/dblab", Description: "Database UI"},
	{Name: "shadowfax", Module: "github.com/mbvlabs/shadowfax", Description: "Shadowfax dev server"},
	{Name: "tailwindcli", Description: "Tailwind CSS CLI"},
}

var managedToolByName = func() map[string]managedTool {
	m := make(map[string]managedTool, len(managedTools))
	for _, tool := range managedTools {
		m[tool.Name] = tool
	}
	return m
}()

func newSetVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-version <tool> <version>",
		Short: "Set a specific version for a tool",
		Long: `Set the version of a tool and update it.

The tool entry in andurel.lock controls where binaries are downloaded from.
The version should be specified WITHOUT the "v" prefix.

Examples:
  andurel tool set-version templ 0.3.977
  andurel tool set-version sqlc 1.30.0
  andurel tool set-version tailwindcli 4.1.18
  andurel tool set-version shadowfax 0.1.3

This updates andurel.lock and syncs the tool binary to bin/.`,
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
	if version == "" {
		return fmt.Errorf("version cannot be empty\n\nExample: andurel tool set-version %s 1.0.0", toolName)
	}

	if strings.HasPrefix(version, "v") {
		version = version[1:]
	}
	versionWithV := "v" + version

	lockPath := filepath.Join(projectRoot, "andurel.lock")
	if _, err := os.Stat(lockPath); err != nil {
		return fmt.Errorf("andurel.lock not found. Are you in an andurel project?")
	}

	lock, err := layout.ReadLockFile(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to read lock file: %w", err)
	}

	tool, exists := lock.Tools[toolName]
	if !exists {
		managed, ok := managedToolByName[toolName]
		if !ok {
			return unknownToolError(toolName)
		}

		if managed.Module != "" {
			tool = layout.NewGoTool(toolName, extractModulePath(managed.Module), versionWithV)
		} else {
			tool = layout.NewBinaryTool(toolName, versionWithV)
		}
		lock.AddTool(toolName, tool)
	} else {
		tool.Version = versionWithV
		if tool.Download == nil {
			if spec, ok := layout.GetDefaultToolDownload(toolName); ok {
				tool.Download = spec
			}
		}
		lock.AddTool(toolName, tool)
	}

	if err := lock.WriteLockFile(projectRoot); err != nil {
		return fmt.Errorf("failed to update lock file: %w", err)
	}

	fmt.Printf("Setting %s to version %s...\n", toolName, versionWithV)

	binDir := filepath.Join(projectRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	if err := syncSingleTool(projectRoot, toolName, lock.Tools[toolName], runtime.GOOS, runtime.GOARCH); err != nil {
		return err
	}

	fmt.Printf("\n✓ Successfully updated %s to %s\n", toolName, versionWithV)
	return nil
}

func unknownToolError(toolName string) error {
	allNames := make([]string, 0, len(managedTools))
	for _, tool := range managedTools {
		allNames = append(allNames, tool.Name)
	}
	sort.Strings(allNames)

	return fmt.Errorf(
		"unknown tool: %s\n\nSupported tools:\n  %s\n\nRun 'andurel tool set-version --help' for more information",
		toolName,
		strings.Join(allNames, ", "),
	)
}

func extractModulePath(module string) string {
	parts := strings.Split(module, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], "/")
	}
	return module
}
