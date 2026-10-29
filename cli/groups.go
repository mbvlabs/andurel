package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

func newSyncGroupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Refresh derived files from a source of truth",
		Long: `Refresh generated artifacts. This group does not create application-owned files.

  queries    domains/**/*.sql → domains/internal/queries
  routes     routes/*.go → resources/js/routes.ts (Inertia)
  payloads   handler page props/request payloads → resources/js/types/payloads.ts
  email      authored email templates → inlined email renderers

andurel tool sync downloads pinned binaries. It is not this group.`,
		Example: `  andurel sync queries
  andurel sync routes --json
  andurel sync payloads --json
  andurel sync payloads --strict
  andurel sync email --json`,
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
		Long: `List project metadata, config keys, and source files without writing anything.

Use andurel sync to refresh derived files. Use andurel patterns to read the
reference shape before writing application-owned code yourself.`,
		Example: `  andurel inspect project --json
  andurel inspect config --json
  andurel inspect routes --json
  andurel inspect pages --json
  andurel inspect process --json
  andurel inspect urls --json
  andurel inspect domains --json
  andurel inspect handlers --json`,
	}
	cmd.AddCommand(
		newProjectInfoCommand(),
		newConfigInspectionCommand(),
		newRoutesCommand(),
		newPagesCommand(),
		newProcessCommand(),
		newURLsCommand(),
		newMigrationsCommand(),
		newDomainsCommand(),
		newHandlersCommand(),
		newJobsCommand(),
		newUpgradeNotesCommand(),
	)
	setStandardHelp(cmd)
	return cmd
}

// configureGroupCommandBehavior makes an unknown positional argument to a
// group a usage error. Cobra's default for a non-runnable group is to print
// help and exit 0, which hides typos and missing commands from humans and
// agents. Bare group invocations keep printing help successfully.
func configureGroupCommandBehavior(root *cobra.Command) {
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		children := availableSubcommands(cmd)
		if cmd != root && len(children) > 0 && !cmd.Runnable() {
			cmd.Args = cobra.ArbitraryArgs
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				if len(args) == 0 {
					return cmd.Help()
				}
				return unknownSubcommandError(cmd, args[0])
			}
		}
		for _, child := range children {
			walk(child)
		}
	}
	walk(root)
}

// noArgsOrUnknownSubcommand keeps a runnable leaf-with-children (inspect
// project) strict: extra positionals are typed unknown-subcommand usage
// errors rather than Cobra's unclassified arg error.
func noArgsOrUnknownSubcommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return unknownSubcommandError(cmd, args[0])
}

func unknownSubcommandError(cmd *cobra.Command, name string) error {
	suggestions := cmd.SuggestionsFor(name)
	available := make([]string, 0, len(availableSubcommands(cmd)))
	for _, child := range availableSubcommands(cmd) {
		available = append(available, child.Name())
	}
	sort.Strings(available)

	hint := fmt.Sprintf(
		"Run %q to see available subcommands: %s.",
		cmd.CommandPath()+" --help",
		strings.Join(available, ", "),
	)
	if len(suggestions) > 0 {
		hint = fmt.Sprintf("Did you mean %q? %s", suggestions[0], hint)
	}
	return output.NewError(
		output.CodeUsage,
		fmt.Sprintf("unknown subcommand %q for %q", name, cmd.CommandPath()),
		output.ExitUsage,
		hint,
	)
}
