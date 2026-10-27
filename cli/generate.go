package cli

import (
	"github.com/spf13/cobra"
)

func newGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Create application-owned files",
		Long: `Create application-owned files that have no derived source.

Today the only generator is andurel generate migration, which authorises a
new SQL migration for goose. Domains, services, handlers, routes, jobs, and
emails are written by hand: read andurel patterns list, then
andurel patterns show <kind> for the shape, placement rules, and checklist.

Refresh derived files with andurel sync. Inspect existing files with
andurel inspect.`,
		Example: `  andurel generate migration create_posts`,
	}
	cmd.AddCommand(
		newGenerateMigrationCommand(),
	)
	setStandardHelp(cmd)
	return cmd
}
