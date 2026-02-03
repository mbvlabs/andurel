package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/spf13/cobra"
)

// getToolVersionForSync gets the version of a tool binary.
// Uses the same logic as doctor.go's getToolVersion.
func getToolVersionForSync(name string) (string, error) {
	binPath := filepath.Join("bin", name)
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
		expectedTools[name] = true
	}

	for name, tool := range lock.Tools {
		binPath := filepath.Join(projectRoot, "bin", name)

		if _, err := os.Stat(binPath); err == nil {
			// Binary exists - check version
			actualVersion, verr := getToolVersionForSync(name)
			if verr == nil && versionsMatch(tool.Version, actualVersion) {
				fmt.Printf("✓ %s (%s) - up to date\n", name, tool.Version)
				continue
			}
			// Version mismatch or couldn't determine version - re-download
			if verr != nil {
				fmt.Printf("⟳ %s: version unknown, re-downloading %s\n", name, tool.Version)
			} else {
				fmt.Printf("⟳ %s: updating %s → %s\n", name, actualVersion, tool.Version)
			}
			if err := os.Remove(binPath); err != nil {
				return fmt.Errorf("failed to remove outdated %s: %w", name, err)
			}
		}

		switch tool.Source {
		case "go":
			fmt.Printf("⬇ Downloading %s %s for %s/%s...\n", name, tool.Version, goos, goarch)
			if err := cmds.DownloadGoTool(name, tool.Module, tool.Version, goos, goarch, binPath); err != nil {
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

		default:
			fmt.Printf("unknown tool source - tool: %s source: %s \n", name, tool.Source)
			continue
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
