package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/cmds"
	"github.com/spf13/cobra"
)

func newLockCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Manage binary versions in andurel.lock",
	}

	cmd.AddCommand(newLockSetVersionCommand())

	return cmd
}

func newLockSetVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-version <binary> <version>",
		Short: "Set a specific version for a binary",
		Long: `Set the version of a binary in andurel.lock and download it.

Supported binaries:
  tailwindcli  - Tailwind CSS CLI
  mailhog      - MailHog email testing tool
  usql         - Universal SQL CLI

The version should be specified WITHOUT the "v" prefix.

Examples:
  andurel lock set-version tailwindcli 4.1.17
  andurel lock set-version mailhog 1.0.1
  andurel lock set-version usql 0.19.26

This command will:
  1. Update the version in andurel.lock
  2. Remove the existing binary from bin/
  3. Download the specified version
  4. Calculate and store the checksum

To see available versions, check the GitHub releases:
  - tailwindcli: https://github.com/tailwindlabs/tailwindcss/releases
  - mailhog:     https://github.com/mailhog/MailHog/releases
  - usql:        https://github.com/xo/usql/releases`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			binaryName := args[0]
			version := args[1]

			projectRoot, err := findGoModRoot()
			if err != nil {
				return fmt.Errorf("not in an andurel project directory: %w", err)
			}

			return setVersion(projectRoot, binaryName, version)
		},
	}
}

func setVersion(projectRoot, binaryName, version string) error {
	validBinaries := map[string]bool{
		"tailwindcli": true,
		"mailhog":     true,
		"usql":        true,
	}

	if !validBinaries[binaryName] {
		return fmt.Errorf("unknown binary: %s\n\nSupported binaries:\n  - tailwindcli\n  - mailhog\n  - usql\n\nRun 'andurel lock set-version --help' for more information", binaryName)
	}

	if version == "" {
		return fmt.Errorf("version cannot be empty\n\nExample: andurel lock set-version %s 4.1.17", binaryName)
	}

	if len(version) > 0 && version[0] == 'v' {
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

	fmt.Printf("Setting %s to version %s...\n", binaryName, version)

	binPath := filepath.Join(projectRoot, "bin", binaryName)
	if _, err := os.Stat(binPath); err == nil {
		fmt.Printf("  - Removing old binary...\n")
		if err := os.Remove(binPath); err != nil {
			return fmt.Errorf("failed to remove old binary: %w", err)
		}
	}

	fmt.Printf("  - Downloading %s %s...\n", binaryName, version)

	switch binaryName {
	case "tailwindcli":
		if err := cmds.SetupTailwindWithVersion(projectRoot, versionWithV); err != nil {
			return fmt.Errorf("failed to download %s: %w", binaryName, err)
		}
	case "mailhog":
		if err := cmds.SetupMailHogWithVersion(projectRoot, versionWithV); err != nil {
			return fmt.Errorf("failed to download %s: %w", binaryName, err)
		}
	case "usql":
		if err := cmds.SetupUsqlWithVersion(projectRoot, versionWithV); err != nil {
			return fmt.Errorf("failed to download %s: %w", binaryName, err)
		}
	}

	fmt.Printf("  - Calculating checksum...\n")
	checksum, err := layout.CalculateBinaryChecksum(binPath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	var url string
	switch binaryName {
	case "tailwindcli":
		url = layout.GetTailwindDownloadURL(versionWithV)
	case "mailhog":
		url = layout.GetMailHogDownloadURL(versionWithV)
	case "usql":
		url = layout.GetUsqlDownloadURL(versionWithV)
	}

	lock.Binaries[binaryName] = &layout.Binary{
		Version:  versionWithV,
		URL:      url,
		Checksum: checksum,
	}

	if err := lock.WriteLockFile(projectRoot); err != nil {
		return fmt.Errorf("failed to update lock file: %w", err)
	}

	fmt.Printf("\n✓ Successfully updated %s to %s\n", binaryName, version)
	return nil
}
