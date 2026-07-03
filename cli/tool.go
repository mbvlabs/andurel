package cli

import (
	"github.com/spf13/cobra"
)

func newToolCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tool",
		Aliases: []string{"t"},
		Short:   "Manage project tools and binaries",
		Long: `Manage CLI tools and binaries used by your Andurel project.

Tools are defined in andurel.lock and downloaded to bin/. Use the
subcommands below to sync, configure, or run tools.`,
		Example: `  andurel tool sync
  andurel tool set-version templ 0.3.977
  andurel tool dblab
  andurel tool mailpit`,
	}

	cmd.AddCommand(newSyncCommand())
	cmd.AddCommand(newSetVersionCommand())
	cmd.AddCommand(newDblabCommand())
	cmd.AddCommand(newMailpitCommand())

	return cmd
}
