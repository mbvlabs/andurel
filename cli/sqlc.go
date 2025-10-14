package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/pkg/cache"
	"github.com/spf13/cobra"
)

func newSqlcCommand() *cobra.Command {
	sqlcCmd := &cobra.Command{
		Use:     "sqlc",
		Aliases: []string{"s"},
		Short:   "SQLC code generation helpers",
		Long:    "Manage SQLC code generation for the current project.",
	}

	sqlcCmd.AddCommand(
		newSqlcCompileCommand(),
		newSqlcGenerateCommand(),
	)

	return sqlcCmd
}

func newSqlcCompileCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "compile",
		Short: "Compile SQLC queries to check for errors",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSqlc("compile")
		},
	}
}

func newSqlcGenerateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate Go code from SQL queries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSqlc("generate")
		},
	}
}

func runSqlc(action string) error {
	wd, err := findGoModRoot()
	if err != nil {
		return err
	}

	configPath := filepath.Join(wd, "database", "sqlc.yaml")
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"sqlc config not found at %s",
				configPath,
			)
		}
		return err
	}

	cmd := exec.Command("go", "tool", "sqlc", "-f", configPath, action)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = wd

	return cmd.Run()
}

func findGoModRoot() (string, error) {
	return cache.GetDirectoryRoot("go_mod_root", func() (string, error) {
		dir, err := os.Getwd()
		if err != nil {
			return "", errors.New("could not get working directory")
		}

		for {
			goModPath := filepath.Join(dir, "go.mod")
			if _, err := os.Stat(goModPath); err == nil {
				return dir, nil
			}

			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}

		return "", errors.New("go mod could not be found")
	})
}
