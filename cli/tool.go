package cli

import (
	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

func newToolCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tool",
		Aliases: []string{"tools", "t"},
		Short:   "Manage project tools and binaries",
		Long: `Manage CLI tools and binaries used by your Andurel project.

Tools are defined in andurel.lock and downloaded to bin/. Use the
subcommands below to sync, configure, or run tools.`,
		Example: `  andurel tool sync
  andurel tools --json
  andurel tool set-version templ 0.3.977
  andurel tool dblab
  andurel tool mailpit`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			lock, err := layout.ReadLockFile(rootDir)
			if err != nil {
				return err
			}
			return output.OK(cmd, toolInfos(rootDir, lock), "Listed tools")
		},
	}
	setAgentMetadata(cmd, "introspection", "Read-only by default; use subcommands for tool mutations.")

	cmd.AddCommand(newSyncCommand())
	cmd.AddCommand(newSetVersionCommand())
	cmd.AddCommand(newDblabCommand())
	cmd.AddCommand(newMailpitCommand())
	cmd.AddCommand(newToolListCommand())

	return cmd
}

func newToolListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "status"},
		Short:   "List project tool status",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			lock, err := layout.ReadLockFile(rootDir)
			if err != nil {
				return err
			}
			return output.OK(cmd, toolInfos(rootDir, lock), "Listed tools")
		},
	}
}
