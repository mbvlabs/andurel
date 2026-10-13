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
new SQL migration for goose. Application code is not generated: read the
Andurel patterns with andurel patterns and write the code yourself.

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
