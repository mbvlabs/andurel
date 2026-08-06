package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newGenerateViewsCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "view",
		Aliases: []string{"v"},
		Short:   "Generate Go code from Templ templates",
		Long: `Run templ generate to produce Go code from .templ files.

Scans views/ and email/ directories for .templ files and produces
corresponding _templ.go files. Email templates are then compiled through
Andurel so Tailwind utilities become email-compatible inline styles.`,
		Example: `  andurel generate view`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runTemplFunc("generate", "-path", "./views"); err != nil {
				return err
			}
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			if !hasEmailCompilerInputs(rootDir) {
				return nil
			}
			if err := compileEmailProject(cmd.Context(), rootDir); err != nil {
				return fmt.Errorf("compile generated email views: %w", err)
			}
			return nil
		},
	}
}
