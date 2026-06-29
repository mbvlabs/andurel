package cli

import (
	"fmt"

	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

func newExtensionCommand() *cobra.Command {
	extensionCmd := &cobra.Command{
		Use:   "extension",
		Aliases: []string{"ext", "e"},
		Short: "Manage project extensions",
		Long: `Add and list extensions applied to the current Andurel project.

Extensions add optional features like Docker or email integration. Adding an
extension generates its code files and updates framework-managed files.`,
		Example: `  andurel extension add docker
  andurel extension list`,
	}

	extensionCmd.AddCommand(
		newExtensionAddCommand(),
		newExtensionListCommand(),
	)

	return extensionCmd
}

func newExtensionAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "add [extension-name]",
		Aliases: []string{"a"},
		Short: "Add an extension to the project",
		Long: `Add an extension to an existing project.

This generates the extension's code files, updates framework-managed files
(config.go, .env.example, main.go, etc.), and records the extension in
andurel.lock.

⚠️  Commit or create a branch before running this command, as it modifies
files in place.`,
		Example: "  andurel extension add docker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			extensionName := args[0]

			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			applied, err := layout.ApplyExtension(rootDir, extensionName)
			if err != nil {
				return err
			}

			for _, name := range applied {
				fmt.Printf("Extension '%s' added to project\n", name)
			}

			return nil
		},
	}
}

func newExtensionListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Aliases: []string{"ls"},
		Short: "List all extensions applied to the project",
		Long:  "Show every extension registered in andurel.lock with the date it was applied.",
		Example: "  andurel extension list",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			lock, err := layout.ReadLockFile(rootDir)
			if err != nil {
				return fmt.Errorf("failed to read lock file: %w", err)
			}

			if len(lock.Extensions) == 0 {
				fmt.Println("No extensions applied to this project")
				return nil
			}

			fmt.Println("Extensions:")
			for name, ext := range lock.Extensions {
				fmt.Printf("  - %s (applied: %s)\n", name, ext.AppliedAt)
			}

			return nil
		},
	}
}
