package cli

import (
	"fmt"
	"strings"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/generator"
	"github.com/spf13/cobra"
)

type factorySyncReport struct {
	Results []*generator.FactorySyncResult `json:"results"`
}

func newGenerateFactoryCommand() *cobra.Command {
	var check bool
	var sync bool
	var diff bool

	cmd := &cobra.Command{
		Use:   "factory NAME",
		Short: "Generate or sync one model factory",
		Long: `Generate or sync a model factory from the current model Entity.

The sync rewrites Andurel generated regions and preserves custom helpers outside
those regions.`,
		Example: `  andurel generate factory User
  andurel generate factory User --check
  andurel generate factory User --sync
  andurel generate factory User --check --diff`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := generator.FactorySyncOptions{Check: check, Sync: sync, Diff: diff}
			if !opts.Check && !opts.Sync {
				opts.Sync = true
			}
			return runFactorySyncCommand(cmd, []string{args[0]}, opts)
		},
	}
	setAgentMetadata(cmd, "generation", "Syncs one factory from the model Entity. Use --check --json in CI and --sync to write changes.")

	cmd.Flags().BoolVar(&check, "check", false, "Report factory drift without writing")
	cmd.Flags().BoolVar(&sync, "sync", false, "Create or update generated factory regions")
	cmd.Flags().BoolVar(&diff, "diff", false, "Include proposed factory changes")

	return cmd
}

func newGenerateFactoriesCommand() *cobra.Command {
	var check bool
	var sync bool
	var diff bool

	cmd := &cobra.Command{
		Use:   "factories",
		Short: "Check or sync all model factories",
		Long: `Check or sync every model factory from the current model Entity files.

The plural command requires --check or --sync to avoid accidental repo-wide writes.`,
		Example: `  andurel generate factories --check
  andurel generate factories --check --diff
  andurel generate factories --sync`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !check && !sync {
				return output.NewError(output.CodeUsage, "generate factories requires --check or --sync", output.ExitUsage, "Run andurel generate factories --check or andurel generate factories --sync.")
			}
			opts := generator.FactorySyncOptions{Check: check, Sync: sync, Diff: diff}
			return runFactorySyncCommand(cmd, nil, opts)
		},
	}
	setAgentMetadata(cmd, "generation", "Bulk factory drift command. Use --check --json for CI and --sync to write changes.")

	cmd.Flags().BoolVar(&check, "check", false, "Report factory drift without writing")
	cmd.Flags().BoolVar(&sync, "sync", false, "Create or update generated factory regions")
	cmd.Flags().BoolVar(&diff, "diff", false, "Include proposed factory changes")

	return cmd
}

func runFactorySyncCommand(cmd *cobra.Command, names []string, opts generator.FactorySyncOptions) error {
	if err := chdirToProjectRoot(); err != nil {
		return err
	}

	gen, err := newGenerator()
	if err != nil {
		return err
	}

	var results []*generator.FactorySyncResult
	if len(names) == 0 {
		results, err = gen.SyncFactories(opts)
	} else {
		for _, name := range names {
			result, syncErr := gen.SyncFactory(name, opts)
			if syncErr != nil {
				return syncErr
			}
			results = append(results, result)
		}
	}
	if err != nil {
		return err
	}

	if opts.Check && len(driftedFactories(results)) > 0 {
		if renderErr := renderFactorySyncResults(cmd, results); renderErr != nil {
			return renderErr
		}
		return output.NewError(output.CodeGenerationFailed, fmt.Sprintf("%d factories are stale", len(driftedFactories(results))), output.ExitGeneration, "Run andurel generate factories --sync or andurel generate factory NAME --sync.")
	}

	return renderFactorySyncResults(cmd, results)
}

func renderFactorySyncResults(cmd *cobra.Command, results []*generator.FactorySyncResult) error {
	outOpts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if output.UsesStructuredOutput(outOpts) {
		return output.OK(cmd, factorySyncReport{Results: results}, factorySyncSummary(results))
	}
	printFactorySyncResults(results)
	return nil
}

func driftedFactories(results []*generator.FactorySyncResult) []*generator.FactorySyncResult {
	var drifted []*generator.FactorySyncResult
	for _, result := range results {
		if result.HasDrift() {
			drifted = append(drifted, result)
		}
	}
	return drifted
}

func factorySyncSummary(results []*generator.FactorySyncResult) string {
	drifted := len(driftedFactories(results))
	written := 0
	for _, result := range results {
		if result.Written {
			written++
		}
	}
	if written > 0 {
		return fmt.Sprintf("Synced %d factories", written)
	}
	if drifted > 0 {
		return fmt.Sprintf("Found %d stale factories", drifted)
	}
	return "Factories are up to date"
}

func printFactorySyncResults(results []*generator.FactorySyncResult) {
	for _, result := range results {
		switch {
		case result.Written:
			fmt.Printf("Synced %s\n", result.Path)
		case result.Missing:
			fmt.Printf("Missing %s\n", result.Path)
		case result.Stale:
			fmt.Printf("Stale %s\n", result.Path)
		default:
			fmt.Printf("Up to date %s\n", result.Path)
		}
		if strings.TrimSpace(result.Diff) != "" {
			fmt.Println()
			printColoredDiff(result.Diff)
		}
	}
}
