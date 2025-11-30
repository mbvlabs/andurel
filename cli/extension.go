package cli

import (
	"fmt"
	"time"

	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

func newExtensionCommand() *cobra.Command {
	extensionCmd := &cobra.Command{
		Use:     "extension",
		Aliases: []string{"ext", "e"},
		Short:   "Manage project extensions",
		Long:    "Add and list extensions applied to the current andurel project.",
	}

	extensionCmd.AddCommand(
		newExtensionAddCommand(),
		newExtensionListCommand(),
	)

	return extensionCmd
}

func newExtensionAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add [extension-name]",
		Short: "Add an extension to the project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			extensionName := args[0]

			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			lock, err := layout.ReadLockFile(rootDir)
			if err != nil {
				return fmt.Errorf("failed to read lock file: %w", err)
			}

			if _, exists := lock.Extensions[extensionName]; exists {
				return fmt.Errorf("extension '%s' is already applied to this project", extensionName)
			}

			lock.AddExtension(extensionName, time.Now().Format(time.RFC3339))

			if err := lock.WriteLockFile(rootDir); err != nil {
				return fmt.Errorf("failed to write lock file: %w", err)
			}

			fmt.Printf("Extension '%s' added to andurel.lock\n", extensionName)
			return nil
		},
	}
}

func newExtensionListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all extensions applied to the project",
		Args:    cobra.NoArgs,
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
