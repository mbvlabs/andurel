package cli

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newTemplCommand() *cobra.Command {
	templCmd := &cobra.Command{
		Use:     "templ",
		Aliases: []string{"t"},
		Short:   "Templ code generation helpers",
		Long:    "Manage Templ code generation for the current project.",
	}

	templCmd.AddCommand(
		newTemplGenerateCommand(),
	)

	return templCmd
}

func newTemplGenerateCommand() *cobra.Command {
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
	wd, err := findGoModRoot()
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "tool", "templ", action)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = wd

	return cmd.Run()
}
