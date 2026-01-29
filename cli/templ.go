package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newViewsCommand() *cobra.Command {
	viewsCmd := &cobra.Command{
		Use:     "views",
		Aliases: []string{"v"},
		Short:   "View template helpers",
		Long:    "Manage Templ code generation for the current project.",
	}

	viewsCmd.AddCommand(
		newViewsGenerateCommand(),
		newViewsFormatCommand(),
	)

	return viewsCmd
}

func newViewsGenerateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate Go code from Templ templates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTempl("generate")
		},
	}
}

func newViewsFormatCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "format",
		Short: "Format Templ templates in views and email directories",
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

