package cli

import (
	"fmt"
	"os"
	"path/filepath"
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
		Example: `  andurel patterns list
  andurel patterns list --json
  andurel patterns show domain
  andurel patterns show handler --json`,
	}
	cmd.AddCommand(newPatternsListCommand(), newPatternsShowCommand())
	setAgentMetadata(
		cmd,
		"reference",
		"Group for the read-only pattern library. Use patterns list for ids and patterns show <kind> to read one.",
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
pattern (id, title, summary, files, rules, checklist) is emitted.

Pass --entity (and optionally --table) with the domain pattern to render the
ordered file manifest for one entity: the read-only replacement for the old
generate-model convention. The manifest lists the files to create, in order;
the agent writes every file.

When domains/internal/queries/models.go declares <Entity>Row (after the
migration is applied and andurel sync queries ran), the manifest also carries
file contents rendered from that row: the entity struct with andurel tags and
the row's Go types, a Validate() stub, the receiver with New<Plural> and InTx,
and a CRUD .sql skeleton. Otherwise it carries a migration skeleton.`,
		Example: `  andurel patterns show domain
  andurel patterns show handler --json
  andurel patterns show domain --entity User --table users
  andurel patterns show domain --entity User --table users --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPatternsShow(cmd, args[0])
		},
	}
	cmd.Flags().String("entity", "", "Render the file manifest for one entity (e.g. User)")
	cmd.Flags().String("table", "", "Table the entity maps to (defaults to the lowercased entity name, pluralized)")
	setAgentMetadata(
		cmd,
		"reference",
		"Prints one pattern's reference code, rules, and checklist. Use --json for the parsed form. Use --entity/--table with domain for the per-entity file manifest.",
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

	entity, _ := cmd.Flags().GetString("entity")
	table, _ := cmd.Flags().GetString("table")
	if (entity != "" || table != "") && pattern.ID != "domain" {
		return output.NewError(
			output.CodeUsage,
			fmt.Sprintf("--entity and --table only apply to the domain pattern, not %q", pattern.ID),
			output.ExitUsage,
			"Run andurel patterns show domain --entity <Name> [--table <table>] for the per-entity manifest.",
		)
	}
	if table != "" && entity == "" {
		return output.NewError(
			output.CodeUsage,
			"--table requires --entity",
			output.ExitUsage,
			"Run andurel patterns show domain --entity <Name> --table <table>.",
		)
	}
	var manifest *patterns.Manifest
	if entity != "" {
		made, err := patterns.ManifestForProject(entity, table, readPatternProject())
		if err != nil {
			return output.WrapError(
				output.CodeConfigError,
				err,
				output.ExitConfig,
				"Run andurel sync queries to regenerate "+patterns.ModelsPath+".",
			)
		}
		manifest = &made
		pattern.Manifest = manifest
	}

	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if opts.Mode == output.ModeHuman {
		if opts.Quiet {
			return nil
		}
		if _, err := fmt.Fprint(cmd.OutOrStdout(), pattern.Markdown()); err != nil {
			return err
		}
		if manifest != nil {
			if _, err := fmt.Fprint(cmd.OutOrStdout(), renderManifestMarkdown(*manifest)); err != nil {
				return err
			}
		}
		return nil
	}

	return output.OK(cmd, pattern, "Pattern "+pattern.ID)
}

// readPatternProject reads the project context the domain manifest renders
// from. Outside a project, or without generated queries, it is empty and the
// manifest falls back to the file list plus a migration skeleton.
func readPatternProject() patterns.Project {
	rootDir, err := findGoModRoot()
	if err != nil {
		return patterns.Project{}
	}
	module, _, _ := readGoModMetadata(rootDir)
	models, err := os.ReadFile(filepath.Join(rootDir, filepath.FromSlash(patterns.ModelsPath)))
	if err != nil {
		models = nil
	}
	return patterns.Project{Module: module, Models: models}
}

// renderManifestMarkdown renders the per-entity file manifest as Markdown.
func renderManifestMarkdown(manifest patterns.Manifest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n## Manifest for %s (`%s`)\n\n", manifest.Entity, manifest.Table)
	if manifest.RowFound {
		fmt.Fprintf(&b, "Rendered from `queries.%s` in `%s`.\n\n", manifest.Row, patterns.ModelsPath)
	} else {
		fmt.Fprintf(&b, "`queries.%s` is not in `%s` yet: write the migration, apply it, add the .sql, run andurel sync queries, then run this again for the rendered entity.\n\n", manifest.Row, patterns.ModelsPath)
	}
	for i, file := range manifest.Files {
		fmt.Fprintf(&b, "%d. %s `%s` — %s\n", i+1, file.Action, file.Path, file.Summary)
	}
	for _, file := range manifest.Files {
		if file.Content == "" {
			continue
		}
		fmt.Fprintf(&b, "\n### %s\n\n```%s\n%s```\n", file.Path, file.Language, file.Content)
	}
	b.WriteString("\nNext:\n\n")
	for _, next := range manifest.Next {
		fmt.Fprintf(&b, "- %s\n", next)
	}
	return b.String()
}
