package cli

import "github.com/spf13/cobra"

func newSyncGroupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Refresh derived files from a source of truth",
		Long: `Refresh generated artifacts. This group does not create application-owned files.

  queries    domains/**/*.sql → domains/internal/queries
  routes     routes/*.go → resources/js/routes.ts (Inertia)
  payloads   handler payload/Bind structs → resources/js/types/payloads.ts
  email      authored email templates → inlined email renderers

andurel tool sync downloads pinned binaries. It is not this group.`,
		Example: `  andurel sync queries
  andurel sync routes --json
  andurel sync payloads --json
  andurel sync email`,
	}
	cmd.AddCommand(
		newGenerateQueriesCommand(),
		newGenerateRoutesCommand(),
		newGeneratePayloadsCommand(),
		newEmailCompileCommand(),
	)
	setStandardHelp(cmd)
	return cmd
}

func newInspectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Read-only project shape",
		Long: `List project metadata and source files without writing anything.

Use andurel sync to refresh derived files. Use andurel patterns to read the
reference shape before writing application-owned code yourself.`,
		Example: `  andurel inspect project --json
  andurel inspect routes --json
  andurel inspect domains --json
  andurel inspect handlers --json`,
	}
	cmd.AddCommand(
		newProjectInfoCommand(),
		newRoutesCommand(),
		newMigrationsCommand(),
		newDomainsCommand(),
		newHandlersCommand(),
		newJobsCommand(),
	)
	setStandardHelp(cmd)
	return cmd
}
