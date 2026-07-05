package cli

import (
	"github.com/spf13/cobra"
)

func newGenerateViewsCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "view",
		Aliases: []string{"v"},
		Short:   "Generate Go code from Templ templates",
		Long: `Run templ generate to produce Go code from .templ files.

Scans views/ and email/ directories for .templ files and produces
corresponding _templ.go files. Run this after editing any .templ file
or as part of your build pipeline.`,
		Example: `  andurel generate view`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTemplFunc("generate")
		},
	}
}
