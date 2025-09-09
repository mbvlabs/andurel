// Package cli provides the command-line interface for the cli.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewRootCommand(version, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "andurel",
		Short:   "Andurel - The Go Web development framework",
		Long:    `Andurel is a comprehensive web development framework for Go,`,
		Version: fmt.Sprintf("%s (built: %s)", version, date),
	}

	rootCmd.AddCommand(newProjectCommand())
	rootCmd.AddCommand(newGenerateCommand())

	return rootCmd
}
