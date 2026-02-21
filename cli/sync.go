package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/mbvlabs/andurel/pkg/naming"
	"github.com/spf13/cobra"
)

// getToolVersionForSync gets the version of a tool binary.
// Uses the same logic as doctor.go's getToolVersion.
func getToolVersionForSync(name string) (string, error) {
	binPath := filepath.Join("bin", naming.BinaryName(name))
	return versionFromCommand(binPath, name)
}

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

	// Ensure bin directory exists
	binDir := filepath.Join(projectRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	fmt.Println("Syncing tools from andurel.lock...")

	// Track expected tools for cleanup
	expectedTools := make(map[string]bool)
	for name := range lock.Tools {
		expectedTools[naming.BinaryName(name)] = true
	}

	for name, tool := range lock.Tools {
		if err := syncSingleTool(projectRoot, name, tool, goos, goarch); err != nil {
			if errors.Is(err, cmds.ErrFailedToGetRleaseURL) {
				fmt.Printf(
					"failed to find release for %s %s on %s/%s \n",
					name,
					tool.Version,
					goos,
					goarch,
				)
				continue
			}

			return err
		}
	}

	// Cleanup: remove binaries not in lock file
	entries, err := os.ReadDir(binDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if !expectedTools[entry.Name()] {
				fmt.Printf("✗ Removing %s (not in andurel.lock)\n", entry.Name())
				if err := os.Remove(filepath.Join(binDir, entry.Name())); err != nil {
					return fmt.Errorf("failed to remove %s: %w", entry.Name(), err)
				}
			}
		}
	}

	fmt.Println("\nAll tools synced successfully!")
	return nil
}

func syncSingleTool(projectRoot, name string, tool *layout.Tool, goos, goarch string) error {
	binPath := filepath.Join(projectRoot, "bin", naming.BinaryName(name))

	if _, err := os.Stat(binPath); err == nil {
		actualVersion, verr := getToolVersionForSync(name)
		if verr == nil && versionsMatch(tool.Version, actualVersion) {
			fmt.Printf("✓ %s (%s) - up to date\n", name, tool.Version)
			return nil
		}

		if verr != nil {
			fmt.Printf("⟳ %s: version unknown, re-downloading %s\n", name, tool.Version)
		} else {
			fmt.Printf("⟳ %s: updating %s → %s\n", name, actualVersion, tool.Version)
		}

		if err := os.Remove(binPath); err != nil {
			return fmt.Errorf("failed to remove outdated %s: %w", name, err)
		}
	}

	fmt.Printf("⬇ Downloading %s %s for %s/%s...\n", name, tool.Version, goos, goarch)
	if err := downloadFromLockTool(name, tool, goos, goarch, binPath); err != nil {
		return fmt.Errorf("failed to download %s: %w", name, err)
	}
	fmt.Printf("✓ %s (%s) - downloaded successfully\n", name, tool.Version)

	return nil
}

func downloadFromLockTool(name string, tool *layout.Tool, goos, goarch, binPath string) error {
	if tool == nil {
		return fmt.Errorf("missing tool configuration")
	}

	if tool.Download != nil && tool.Download.URLTemplate != "" {
		archive := tool.Download.Archive
		if archive == "" {
			archive = "binary"
		}

		return cmds.DownloadFromURLTemplate(
			name,
			tool.Version,
			tool.Download.URLTemplate,
			archive,
			tool.Download.BinaryName,
			goos,
			goarch,
			binPath,
		)
	}

	if tool.Source != "" {
		return cmds.DownloadGoTool(name, tool.Source, tool.Version, goos, goarch, binPath)
	}

	if name == "tailwindcli" {
		return cmds.DownloadTailwindCLI(tool.Version, goos, goarch, binPath)
	}

	return fmt.Errorf("tool has no download metadata")
}
