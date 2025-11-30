// Package cli provides the command-line interface for the Andurel framework.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/pkg/cache"
	"github.com/spf13/cobra"
)

func NewRootCommand(version, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "andurel",
		Short:   "Andurel - The Go Web development framework",
		Long:    `Andurel is a comprehensive web development framework for Go,`,
		Version: fmt.Sprintf("%s (built: %s)", version, date),
		SilenceUsage: true,
	}

	rootCmd.AddCommand(newRunAppCommand())

	rootCmd.AddCommand(newProjectCommand(version))
	rootCmd.AddCommand(newGenerateCommand())
	rootCmd.AddCommand(newMigrationCommand())
	rootCmd.AddCommand(newSqlcCommand())
	rootCmd.AddCommand(newTemplCommand())

	rootCmd.AddCommand(newAppCommand())
	rootCmd.AddCommand(newLlmCommand())
	rootCmd.AddCommand(newToolCommand())
	rootCmd.AddCommand(newExtensionCommand())

	return rootCmd
}

func newRunAppCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs the app",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			if err := checkBinaries(rootDir); err != nil {
				return err
			}

			binPath := filepath.Join(rootDir, "bin", "run")

			runCmd := exec.Command(binPath)
			runCmd.Stdout = os.Stdout
			runCmd.Stderr = os.Stderr
			runCmd.Stdin = os.Stdin
			runCmd.Dir = rootDir

			return runCmd.Run()
		},
	}

	return cmd
}

func findGoModRoot() (string, error) {
	return cache.GetDirectoryRoot("go_mod_root", func() (string, error) {
		dir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("could not get working directory: %w", err)
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

		return "", fmt.Errorf("not in an andurel project: go.mod could not be found")
	})
}

func checkBinaries(rootDir string) error {
	lockPath := filepath.Join(rootDir, "andurel.lock")
	if _, err := os.Stat(lockPath); err != nil {
		return nil
	}

	lock, err := os.ReadFile(lockPath)
	if err != nil {
		return nil
	}

	if len(lock) == 0 {
		return nil
	}

	binPath := filepath.Join(rootDir, "bin", "run")
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("bin/run not found. Run 'andurel tool sync' to build it")
	}

	return nil
}
