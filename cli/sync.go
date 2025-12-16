package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

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

	fmt.Println("Syncing binaries from andurel.lock...")

	for name, binary := range lock.Binaries {
		binPath := filepath.Join(projectRoot, "bin", name)

		if _, err := os.Stat(binPath); err == nil {
			if binary.Type == "built" {
				fmt.Printf("✓ %s - already built\n", name)
				continue
			}
			if binary.Checksum != "" {
				if err := layout.ValidateBinaryChecksum(binPath, binary.Checksum); err == nil {
					fmt.Printf("✓ %s (%s) - already present and valid\n", name, binary.Version)
					continue
				}
				fmt.Printf("⚠ %s - checksum mismatch, re-downloading...\n", name)
				os.Remove(binPath)
			} else {
				fmt.Printf("✓ %s (%s) - already present (no checksum to validate)\n", name, binary.Version)
				continue
			}
		}

		if binary.Type == "built" {
			fmt.Printf("🔨 Building %s from %s...\n", name, binary.Source)

			switch name {
			case "run":
				if err := cmds.RunGoRunBin(projectRoot); err != nil {
					return fmt.Errorf("failed to build %s: %w", name, err)
				}
			case "migration":
				if err := cmds.RunGoMigrationBin(projectRoot); err != nil {
					return fmt.Errorf("failed to build %s: %w", name, err)
				}
			case "console":
				if err := cmds.RunConsoleBin(projectRoot); err != nil {
					return fmt.Errorf("failed to build %s: %w", name, err)
				}
			default:
				return fmt.Errorf("unknown built binary: %s", name)
			}

			fmt.Printf("✓ %s - built successfully\n", name)
			continue
		}

		fmt.Printf("⬇ Downloading %s %s...\n", name, binary.Version)

		downloadErr := retryDownload(name, func() error {
			switch name {
			case "tailwindcli":
				return cmds.SetupTailwindWithVersion(projectRoot, binary.Version, 30*time.Second)
			case "mailpit":
				return cmds.SetupMailpitWithVersion(projectRoot, binary.Version, 30*time.Second)
			case "dblab":
				return cmds.SetupDblabWithVersion(projectRoot, binary.Version, 30*time.Second)
			default:
				return fmt.Errorf("unknown binary: %s", name)
			}
		})

		if downloadErr != nil {
			return fmt.Errorf("failed to download %s: %w", name, downloadErr)
		}

		if binary.Checksum != "" {
			if err := layout.ValidateBinaryChecksum(binPath, binary.Checksum); err != nil {
				return fmt.Errorf("checksum validation failed for %s: %w", name, err)
			}
			fmt.Printf("✓ %s (%s) - downloaded and verified\n", name, binary.Version)
		} else {
			checksum, err := layout.CalculateBinaryChecksum(binPath)
			if err != nil {
				return fmt.Errorf("failed to calculate checksum for %s: %w", name, err)
			}
			binary.Checksum = checksum
			fmt.Printf("✓ %s (%s) - downloaded and checksum calculated\n", name, binary.Version)
		}
	}

	if err := lock.WriteLockFile(projectRoot); err != nil {
		return fmt.Errorf("failed to update lock file: %w", err)
	}

	fmt.Println("\nAll binaries synced successfully!")
	return nil
}

func retryDownload(name string, downloadFn func() error) error {
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("  Retry %d/%d...\n", attempt-1, maxRetries-1)
			time.Sleep(time.Second * 2)
		}

		err := downloadFn()
		if err == nil {
			return nil
		}

		lastErr = err
		if attempt < maxRetries {
			fmt.Printf("  Download failed: %v\n", err)
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}
