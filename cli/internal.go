package cli

import (
	"github.com/spf13/cobra"

	"github.com/mbvlabs/andurel/internal/sqlcgen"
)

// newInternalCommand groups subcommands that are not part of the public CLI.
// They exist for plugin/process integrations that need a stable entrypoint
// when the dedicated binary (e.g. andurel-sqlc-gen) isn't installed
// separately.
func newInternalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "internal",
		Short:  "Internal subcommands used by andurel itself",
		Hidden: true,
	}
	cmd.AddCommand(newInternalSqlcGenCommand())
	return cmd
}

// newInternalSqlcGenCommand exposes the sqlc process plugin via the andurel
// binary. sqlc invokes the plugin with the gRPC method path as os.Args[1],
// so this subcommand is only useful when sqlc is configured to call
// `andurel internal sqlc-gen` and pass the method as an additional arg —
// in practice users should configure `cmd: andurel-sqlc-gen` instead. We
// keep this as a fallback for environments that ship only the andurel binary.
func newInternalSqlcGenCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "sqlc-gen",
		Short:  "Run the andurel sqlc process plugin (reads stdin, writes stdout)",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			sqlcgen.Run()
		},
	}
}
