package cli

import (
	"fmt"
	"os"
	"path/filepath"

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
			if err := layout.ValidateBinaryChecksum(binPath, binary.Checksum); err == nil {
				fmt.Printf("✓ %s (%s) - already present and valid\n", name, binary.Version)
				continue
			}
			fmt.Printf("⚠ %s - checksum mismatch, re-downloading...\n", name)
			os.Remove(binPath)
		}

		fmt.Printf("⬇ Downloading %s %s...\n", name, binary.Version)

		switch name {
		case "tailwindcli":
			if err := cmds.SetupTailwindWithVersion(projectRoot, binary.Version); err != nil {
				return fmt.Errorf("failed to download %s: %w", name, err)
			}
		case "mailhog":
			if err := cmds.SetupMailHogWithVersion(projectRoot, binary.Version); err != nil {
				return fmt.Errorf("failed to download %s: %w", name, err)
			}
		case "usql":
			if err := cmds.SetupUsqlWithVersion(projectRoot, binary.Version); err != nil {
				return fmt.Errorf("failed to download %s: %w", name, err)
			}
		default:
			return fmt.Errorf("unknown binary: %s", name)
		}

		if err := layout.ValidateBinaryChecksum(binPath, binary.Checksum); err != nil {
			return fmt.Errorf("checksum validation failed for %s: %w", name, err)
		}

		fmt.Printf("✓ %s (%s) - downloaded and verified\n", name, binary.Version)
	}

	fmt.Println("\nAll binaries synced successfully!")
	return nil
}
