package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/emailcompiler"
	"github.com/mbvlabs/andurel/v2/layout"
	"github.com/mbvlabs/andurel/v2/layout/versions"
	"github.com/spf13/cobra"
)

// emailCompileOutput is the structured --json shape for sync email.
type emailCompileOutput struct {
	GeneratedFiles []string `json:"generated_files"`
	ChangedFiles   []string `json:"changed_files"`
}

func newEmailCompileCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "email",
		Short: "Compile email templates into inline-styled Go",
		Long: `Compile every email/*.templ template into its email/*_templ.go renderer.

The compiler parses the templates, resolves Tailwind utilities from
css/email.css with their var() chains inlined, and writes the finished Go
renderers. Authored .templ files and the CSS inputs are never changed.

Run this after editing email templates or the email styles. Never run plain
` + "`templ generate`" + ` on email templates: it rewrites email/*_templ.go with
class attributes instead of the inlined styles this command produces.

Use --json for the generated and changed file lists.`,
		Example: `  andurel sync email
  andurel sync email --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			opts, err := output.ParseOptions(cmd)
			if err != nil {
				return err
			}
			if output.SuppressesHumanOutput(opts) {
				var report emailcompiler.Report
				if err := runWithOptionalStdoutSilence(true, func() error {
					var compileErr error
					report, compileErr = compileEmailProjectReport(cmd.Context(), rootDir)
					return compileErr
				}); err != nil {
					return err
				}
				payload := emailCompileOutput{
					GeneratedFiles: report.Written,
					ChangedFiles:   report.Changed,
				}
				return output.OK(cmd, payload, emailCompileSummary(payload))
			}
			report, err := compileEmailProjectReport(cmd.Context(), rootDir)
			if err != nil {
				return err
			}
			printEmailCompileReport(report)
			return nil
		},
	}
}

func emailCompileSummary(report emailCompileOutput) string {
	summary := fmt.Sprintf("Compiled %d email templates", len(report.GeneratedFiles))
	if len(report.ChangedFiles) > 0 {
		summary += fmt.Sprintf(" (%d changed)", len(report.ChangedFiles))
	}
	return summary
}

func printEmailCompileReport(report emailcompiler.Report) {
	for _, file := range report.Changed {
		fmt.Printf("  updated %s\n", file)
	}
	fmt.Println("Email templates compiled.")
}

func compileEmailProject(ctx context.Context, rootDir string) error {
	report, err := compileEmailProjectReport(ctx, rootDir)
	if err != nil {
		return err
	}
	printEmailCompileReport(report)
	return nil
}

func compileEmailProjectReport(ctx context.Context, rootDir string) (emailcompiler.Report, error) {
	fmt.Println("Compiling Tailwind email templates...")
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return emailcompiler.Report{}, fmt.Errorf("failed to read andurel.toml: %w", err)
	}

	tailwindTool, ok := lock.Tools["tailwindcli"]
	if !ok {
		tailwindTool = layout.NewBinaryTool("tailwindcli", versions.TailwindCLI)
	}
	if err := syncSingleToolFunc(
		rootDir,
		"tailwindcli",
		tailwindTool,
		runtime.GOOS,
		runtime.GOARCH,
	); err != nil {
		return emailcompiler.Report{}, fmt.Errorf("failed to sync tailwind CLI: %w", err)
	}

	return compileEmailWithTailwindReport(
		ctx,
		rootDir,
		filepath.Join(rootDir, "bin", "tailwindcli"),
	)
}

func compileEmailWithTailwind(ctx context.Context, rootDir, tailwindPath string) error {
	fmt.Println("Compiling Tailwind email templates...")
	report, err := compileEmailWithTailwindReport(ctx, rootDir, tailwindPath)
	if err != nil {
		return err
	}
	printEmailCompileReport(report)
	return nil
}

func compileEmailWithTailwindReport(
	ctx context.Context,
	rootDir, tailwindPath string,
) (emailcompiler.Report, error) {
	report, err := emailcompiler.CompileReport(ctx, emailcompiler.Config{
		ProjectRoot:  rootDir,
		TailwindPath: tailwindPath,
	})
	if err != nil {
		return emailcompiler.Report{}, fmt.Errorf("email compilation failed: %w", err)
	}
	return report, nil
}

func hasEmailCompilerInputs(rootDir string) bool {
	for _, path := range []string{
		filepath.Join(rootDir, "email"),
		filepath.Join(rootDir, "css", "email.css"),
	} {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

func hasEmailTemplates(rootDir string) bool {
	found := false
	_ = filepath.WalkDir(filepath.Join(rootDir, "email"), func(
		path string,
		entry fs.DirEntry,
		err error,
	) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == ".templ" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
