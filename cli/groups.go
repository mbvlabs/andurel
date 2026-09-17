package cli

import "github.com/spf13/cobra"

func newSyncGroupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Refresh derived files from a source of truth",
		Long: `Refresh generated artifacts. This group does not create application-owned files.

  views      .templ → *_templ.go (and email compile when inputs exist)
  queries    models/queries/*.sql → models/internal/queries
  routes     router/routes/*.go → resources/js/routes.ts (Inertia)
  payloads   controller payload/Bind structs → resources/js/types/payloads.ts
  email      authored email templates → inlined email renderers
  factory    model Entity → one models/factories file
  factories  model Entities → every Andurel-owned factory declaration

andurel tool sync downloads pinned binaries. It is not this group.`,
		Example: `  andurel sync views
  andurel sync queries
  andurel sync routes --json
  andurel sync payloads --json
  andurel sync email
  andurel sync factory User --check
  andurel sync factories --sync`,
	}
	cmd.AddCommand(
		newGenerateViewsCommand(),
		newGenerateQueriesCommand(),
		newGenerateRoutesCommand(),
		newGeneratePayloadsCommand(),
		newEmailCompileCommand(),
		newGenerateFactoryCommand(),
		newGenerateFactoriesCommand(),
	)
	setStandardHelp(cmd)
	return cmd
}

func newInspectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Read-only project shape",
		Long: `List project metadata and source files without writing anything.

Use andurel sync to refresh derived files. Use andurel generate to create application-owned code.`,
		Example: `  andurel inspect project --json
  andurel inspect routes --json
  andurel inspect models --json`,
	}
	cmd.AddCommand(
		newProjectInfoCommand(),
		newRoutesCommand(),
		newModelsCommand(),
		newMigrationsCommand(),
		newControllersCommand(),
		newViewsCommand(),
		newJobsCommand(),
	)
	setStandardHelp(cmd)
	return cmd
}
