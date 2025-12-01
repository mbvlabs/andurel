package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/spf13/cobra"
)

func newSyncCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Download and validate binaries specified in andurel.lock",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := findGoModRoot()
			if err != nil {
				return fmt.Errorf("not in an andurel project directory: %w", err)
			}

			return syncBinaries(projectRoot)
		},
	}
}

func syncBinaries(projectRoot string) error {
	lockPath := filepath.Join(projectRoot, "andurel.lock")
	if _, err := os.Stat(lockPath); err != nil {
		return fmt.Errorf("andurel.lock not found. Are you in an andurel project?")
	}

	lock, err := layout.ReadLockFile(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to read lock file: %w", err)
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	fmt.Println("Syncing tools from andurel.lock...")

	for name, tool := range lock.Tools {
		binPath := filepath.Join(projectRoot, "bin", name)

		if _, err := os.Stat(binPath); err == nil {
			if tool.Source == "built" {
				fmt.Printf("✓ %s - already built\n", name)
			} else {
				fmt.Printf("✓ %s (%s) - already present\n", name, tool.Version)
			}
			continue
		}

		switch tool.Source {
		case "go":
			fmt.Printf("⬇ Downloading %s %s for %s/%s...\n", name, tool.Version, goos, goarch)
			if err := cmds.DownloadGoTool(name, tool.Module, tool.Version, goos, goarch, binPath); err != nil {
				return fmt.Errorf("failed to download %s: %w", name, err)
			}
			fmt.Printf("✓ %s (%s) - downloaded successfully\n", name, tool.Version)

		case "binary":
			fmt.Printf("⬇ Downloading %s %s for %s/%s...\n", name, tool.Version, goos, goarch)
			if name == "tailwindcli" {
				if err := cmds.DownloadTailwindCLI(tool.Version, goos, goarch, binPath); err != nil {
					return fmt.Errorf("failed to download %s: %w", name, err)
				}
			} else {
				return fmt.Errorf("unknown binary tool: %s", name)
			}
			fmt.Printf("✓ %s (%s) - downloaded successfully\n", name, tool.Version)

		case "built":
			fmt.Printf("🔨 Building %s from %s...\n", name, tool.Path)
			if name == "run" {
				if err := cmds.RunGoRunBin(projectRoot); err != nil {
					return fmt.Errorf("failed to build %s: %w", name, err)
				}
			} else {
				return fmt.Errorf("unknown built binary: %s", name)
			}
			fmt.Printf("✓ %s - built successfully\n", name)

		default:
			return fmt.Errorf("unknown tool source: %s for %s", tool.Source, name)
		}
	}

	fmt.Println("\nAll tools synced successfully!")
	return nil
}
