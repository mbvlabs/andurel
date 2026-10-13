package cli

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/cli/patterns"
	"github.com/spf13/cobra"
)

func newPatternsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patterns",
		Short: "Read the reference patterns for every layer",
		Long: `Print the read-only reference patterns that describe the complete shape of
each primitive: the files to write, the placement rules, and a definition-of-done
checklist.

andurel patterns never writes application code. Read a pattern with
andurel patterns show <kind> and write the files yourself. andurel new produces
a complete reference application that doubles as the executable pattern book.

Pattern ids: domain, service, handler, route, job, email, migration,
test-domain, test-handler, test-integration, composition.`,
		Example: `  andurel patterns
  andurel patterns list --json
  andurel patterns show domain
  andurel patterns show handler --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPatternsList(cmd)
		},
	}
	cmd.AddCommand(newPatternsListCommand(), newPatternsShowCommand())
	setAgentMetadata(
		cmd,
		"reference",
		"Read-only pattern library. Lists pattern ids; use andurel patterns show <kind> to read one.",
	)
	setStandardHelp(cmd)
	return cmd
}

func newPatternsListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the available pattern ids",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPatternsList(cmd)
		},
	}
	setAgentMetadata(
		cmd,
		"reference",
		"Lists pattern ids and titles. Pair with andurel patterns show <kind>.",
	)
	setStandardHelp(cmd)
	return cmd
}

func newPatternsShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <kind>",
		Short: "Print one reference pattern",
		Long: `Print one pattern, including its reference files, rules, and checklist.

In human mode the raw Markdown is printed. In --json/--agent mode the parsed
pattern (id, title, summary, files, rules, checklist) is emitted.`,
		Example: `  andurel patterns show domain
  andurel patterns show handler --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPatternsShow(cmd, args[0])
		},
	}
	setAgentMetadata(
		cmd,
		"reference",
		"Prints one pattern's reference code, rules, and checklist. Use --json for the parsed form.",
	)
	setStandardHelp(cmd)
	return cmd
}

func runPatternsList(cmd *cobra.Command) error {
	if err := patterns.Err(); err != nil {
		return output.WrapError(
			output.CodeConfigError,
			fmt.Errorf("load embedded patterns: %w", err),
			output.ExitConfig,
			"The Andurel binary is missing its embedded pattern assets; reinstall it.",
		)
	}

	all := patterns.All()
	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if opts.Mode == output.ModeHuman {
		if opts.Quiet {
			return nil
		}
		var b strings.Builder
		for _, pattern := range all {
			fmt.Fprintf(&b, "%s\t%s\n", pattern.ID, pattern.Title)
		}
		if _, err := fmt.Fprint(cmd.OutOrStdout(), b.String()); err != nil {
			return err
		}
		return nil
	}

	return output.OK(cmd, all, fmt.Sprintf("Listed %d patterns", len(all)))
}

func runPatternsShow(cmd *cobra.Command, id string) error {
	pattern, ok := patterns.Get(id)
	if !ok {
		return output.NewError(
			output.CodeUsage,
			fmt.Sprintf("unknown pattern %q", id),
			output.ExitUsage,
			"Run andurel patterns to list the available pattern ids.",
		)
	}

	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if opts.Mode == output.ModeHuman {
		if opts.Quiet {
			return nil
		}
		_, err := fmt.Fprint(cmd.OutOrStdout(), pattern.Markdown())
		return err
	}

	return output.OK(cmd, pattern, "Pattern "+pattern.ID)
}
