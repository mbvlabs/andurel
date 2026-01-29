package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/layout/versions"
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

func runTempl(action string) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	var cmd *exec.Cmd

	if os.Getenv("ANDUREL_SKIP_BUILD") == "true" {
		cmd = exec.Command("go", "run", "github.com/a-h/templ/cmd/templ@"+versions.Templ, action)
	} else {
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
		cmd = exec.Command(templBin, action)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir

	return cmd.Run()
}
