package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newViewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "View management commands",
		Long: `Manage view templates and Templ code generation.`,
		Example: `  andurel view generate
  andurel view format`,
	}

	setStandardHelp(cmd,
		helpCommand{
			Use:         "view generate",
			Description: "generates Go code from Templ templates",
		},
		helpCommand{
			Use:         "view format",
			Description: "formats Templ templates in views and email directories",
		},
	)

	cmd.AddCommand(
		newTemplGenerateCommand(),
		newTemplFormatCommand(),
	)

	return cmd
}

func newTemplGenerateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Aliases: []string{"compile"},
		Short: "Generate Go code from Templ templates",
		Long:  "Run templ generate to produce Go code from .templ files in views/ and email/.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTempl("generate")
		},
	}
}

func newTemplFormatCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "format",
		Short: "Format Templ templates in views and email directories",
		Long:  "Run templ fmt on all .templ files in views/ and email/ directories.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, dir := range []string{"views", "email"} {
				if err := runTempl("fmt", dir); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func runTempl(args ...string) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	templBin := filepath.Join(rootDir, "bin", "templ")
	if _, err := os.Stat(templBin); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"templ binary not found at %s\nRun 'andurel tool sync' to download it",
				templBin,
			)
		}
		return err
	}

	cmd := exec.Command(templBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir

	return cmd.Run()
}
