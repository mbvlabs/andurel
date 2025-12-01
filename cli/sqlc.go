package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/layout/versions"
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
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	configPath := filepath.Join(rootDir, "database", "sqlc.yaml")
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(
				"sqlc config not found at %s",
				configPath,
			)
		}
		return err
	}

	var cmd *exec.Cmd

	if os.Getenv("ANDUREL_SKIP_BUILD") == "true" {
		cmd = exec.Command("go", "run", "github.com/sqlc-dev/sqlc/cmd/sqlc@"+versions.Sqlc, "-f", configPath, action)
	} else {
		sqlcBin := filepath.Join(rootDir, "bin", "sqlc")
		if _, err := os.Stat(sqlcBin); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf(
					"sqlc binary not found at %s\nRun 'andurel tool sync' to download it",
					sqlcBin,
				)
			}
			return err
		}
		cmd = exec.Command(sqlcBin, "-f", configPath, action)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = rootDir

	return cmd.Run()
}
