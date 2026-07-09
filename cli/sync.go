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

func getToolVersionForSync(name string, vc *layout.VersionCheck) (string, error) {
	binPath := filepath.Join("bin", name)
	return versionFromCommand(binPath, vc, name)
}

func newSyncCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "sync",
		Aliases: []string{"s"},
		Short:   "Download and validate binaries specified in andurel.lock",
		Long: `Download all tools listed in andurel.lock to bin/. Already-downloaded
tools at the correct version are skipped.

Managed tools include templ, goose, mailpit, usql, dblab, shadowfax,
and tailwindcli. Versions are pinned in andurel.lock.`,
		Example: `  andurel tool sync`,
		Args:    cobra.ExactArgs(0),
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
		if err := syncSingleToolFunc(projectRoot, name, tool, goos, goarch); err != nil {
			return fmt.Errorf("tool synchronization incomplete: %w", err)
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
	binPath := filepath.Join(projectRoot, "bin", name)

	if _, err := os.Stat(binPath); err == nil {
		actualVersion, verr := getToolVersionForSync(name, tool.VersionCheck)
		if verr == nil && versionsMatch(tool.Version, actualVersion) {
			fmt.Printf("✓ %s (%s) - up to date\n", name, tool.Version)
			return nil
		}

		if verr != nil {
			fmt.Printf("⟳ %s: version unknown, re-downloading %s\n", name, tool.Version)
		} else {
			fmt.Printf("⟳ %s: updating %s → %s\n", name, actualVersion, tool.Version)
		}

	}

	fmt.Printf("⬇ Downloading %s %s for %s/%s...\n", name, tool.Version, goos, goarch)
	candidatePath, err := prepareToolCandidate(projectRoot, name, tool, goos, goarch)
	if err != nil {
		return err
	}
	defer os.Remove(candidatePath)
	if err := os.Rename(candidatePath, binPath); err != nil {
		return fmt.Errorf("failed to atomically replace %s: %w", name, err)
	}
	fmt.Printf("✓ %s (%s) - downloaded successfully\n", name, tool.Version)

	return nil
}

func prepareToolCandidate(projectRoot, name string, tool *layout.Tool, goos, goarch string) (string, error) {
	binPath := filepath.Join(projectRoot, "bin", name)
	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		return "", fmt.Errorf("failed to create bin directory: %w", err)
	}
	candidate, err := os.CreateTemp(filepath.Dir(binPath), ".andurel-candidate-*")
	if err != nil {
		return "", fmt.Errorf("failed to create candidate path for %s: %w", name, err)
	}
	candidatePath := candidate.Name()
	if err := candidate.Close(); err != nil {
		_ = os.Remove(candidatePath)
		return "", fmt.Errorf("failed to close candidate path for %s: %w", name, err)
	}
	if err := os.Remove(candidatePath); err != nil {
		return "", fmt.Errorf("failed to prepare candidate path for %s: %w", name, err)
	}

	if err := downloadFromLockToolFunc(name, tool, goos, goarch, candidatePath); err != nil {
		_ = os.Remove(candidatePath)
		return "", fmt.Errorf("failed to download %s: %w", name, err)
	}
	actualVersion, err := versionFromExecutable(candidatePath, tool.VersionCheck, name)
	if err != nil {
		_ = os.Remove(candidatePath)
		return "", fmt.Errorf("failed to verify downloaded %s: %w", name, err)
	}
	if !versionsMatch(tool.Version, actualVersion) {
		_ = os.Remove(candidatePath)
		return "", fmt.Errorf("downloaded %s version %s does not match expected %s", name, actualVersion, tool.Version)
	}
	return candidatePath, nil
}

func downloadFromLockTool(name string, tool *layout.Tool, goos, goarch, binPath string) error {
	if tool == nil {
		return fmt.Errorf("missing tool configuration")
	}

	if tool.Download != nil && tool.Download.URLTemplate != "" {
		platform := goos + "/" + goarch
		switch platform {
		case "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64":
		default:
			return fmt.Errorf("unsupported platform %s", platform)
		}
		digest, ok := tool.Download.SHA256[platform]
		if !ok || digest == "" {
			return fmt.Errorf("missing SHA-256 digest for %s", platform)
		}
		archive := tool.Download.Archive
		if archive == "" {
			archive = "binary"
		}

		return cmds.DownloadVerifiedFromURLTemplate(
			name,
			tool.Version,
			tool.Download.URLTemplate,
			archive,
			tool.Download.BinaryName,
			goos,
			goarch,
			binPath,
			digest,
		)
	}

	return fmt.Errorf("tool has no verified download metadata")
}
