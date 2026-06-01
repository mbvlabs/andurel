package cli

import (
	"github.com/spf13/cobra"
)

func newGenerateViewsCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "views",
		Aliases: []string{"view"},
		Short:   "Generate Go code from Templ templates",
		Long: `Run templ generate to produce Go code from .templ files in views/ and email/.`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTempl("generate")
		},
	}
}
