package cli

import (
	"fmt"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

func newExtensionCommand() *cobra.Command {
	var showAvailable bool
	extensionCmd := &cobra.Command{
		Use:     "extension",
		Aliases: []string{"extensions", "ext", "e"},
		Short:   "Manage project extensions",
		Long: `Add and list extensions applied to the current Andurel project.

Extensions add optional features like Docker or email integration. Adding an
extension generates its code files and updates framework-managed files.`,
		Example: `  andurel extension add docker
  andurel extension list`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionList(cmd, showAvailable)
		},
	}
	setAgentMetadata(extensionCmd, "introspection", "Read-only by default; use extension add for mutations.")

	extensionCmd.AddCommand(
		newExtensionAddCommand(),
		newExtensionListCommand(),
	)
	extensionCmd.Flags().BoolVar(&showAvailable, "available", false, "List available built-in extensions")

	return extensionCmd
}

func newExtensionAddCommand() *cobra.Command {
	var dryRun bool
	var diff bool
	cmd := &cobra.Command{
		Use:     "add [extension-name]",
		Aliases: []string{"a"},
		Short:   "Add an extension to the project",
		Long: `Add an extension to an existing project.

This generates the extension's code files, updates framework-managed files
(config.go, .env.example, main.go, etc.), and records the extension in
andurel.lock.

⚠️  Commit or create a branch before running this command, as it modifies
files in place.`,
		Example: "  andurel extension add docker",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			extensionName := args[0]

			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			return runMutation(cmd, mutationOptions{
				Action:   "extension add",
				Resource: extensionName,
				RootDir:  rootDir,
				DryRun:   dryRun,
				Diff:     diff,
				CommandsRun: []string{
					"goose fix",
					"templ generate",
					"go mod tidy",
				},
				Breadcrumbs: []output.Breadcrumb{
					{Command: "andurel doctor", Description: "Verify project health after applying the extension"},
				},
				Run: func(rootDir string) error {
					applied, err := layout.ApplyExtension(rootDir, extensionName)
					if err != nil {
						return err
					}
					for _, name := range applied {
						fmt.Printf("Extension '%s' added to project\n", name)
					}
					return nil
				},
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview file changes without applying the extension")
	cmd.Flags().BoolVar(&diff, "diff", false, "Include a text diff preview in structured output")
	return cmd
}

func newExtensionListCommand() *cobra.Command {
	var showAvailable bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all extensions applied to the project",
		Long:    "Show every extension registered in andurel.lock with the date it was applied.",
		Example: "  andurel extension list",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionList(cmd, showAvailable)
		},
	}
	cmd.Flags().BoolVar(&showAvailable, "available", false, "List available built-in extensions")
	return cmd
}

func runExtensionList(cmd *cobra.Command, showAvailable bool) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return fmt.Errorf("failed to read lock file: %w", err)
	}

	infos := extensionInfos(lock)
	if showAvailable {
		applied := map[string]bool{}
		for _, info := range infos {
			applied[info.Name] = true
		}
		available, err := layout.AvailableExtensionNames()
		if err != nil {
			return err
		}
		for _, name := range available {
			if applied[name] {
				continue
			}
			infos = append(infos, extensionInfo{Name: name, Available: true})
		}
	}

	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if opts.Mode == output.ModeJSON || opts.Mode == output.ModeAgent || opts.Mode == output.ModeMarkdown || opts.Quiet {
		return output.OK(cmd, infos, "Listed extensions")
	}

	if len(infos) == 0 {
		fmt.Println("No extensions applied to this project")
		return nil
	}

	fmt.Println("Extensions:")
	for _, info := range infos {
		if info.AppliedAt != "" {
			fmt.Printf("  - %s (applied: %s)\n", info.Name, info.AppliedAt)
		} else {
			fmt.Printf("  - %s\n", info.Name)
		}
	}

	return nil
}
