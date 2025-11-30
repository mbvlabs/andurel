package cli

import (
	"github.com/spf13/cobra"
)

func newToolCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Manage project tools and binaries",
	}

	cmd.AddCommand(newSyncCommand())
	cmd.AddCommand(newSetVersionCommand())

	return cmd
}
