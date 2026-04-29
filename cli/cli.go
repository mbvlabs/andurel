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

type helpCommand struct {
	Use         string
	Description string
}

func setStandardHelp(cmd *cobra.Command, commands ...helpCommand) {
	helpOwner := cmd
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// Output description
		if cmd.Long != "" {
			fmt.Println(cmd.Long)
			fmt.Println()
		} else if cmd.Short != "" {
			fmt.Println(cmd.Short)
			fmt.Println()
		}

		// Output commands
		if cmd == helpOwner && len(commands) > 0 {
			fmt.Println("Commands:")
			maxUseLength := 0
			for _, command := range commands {
				if len(command.Use) > maxUseLength {
					maxUseLength = len(command.Use)
				}
			}
			for _, command := range commands {
				fmt.Printf("  %-*s", maxUseLength, command.Use)
				if command.Description != "" {
					fmt.Printf("  %s", command.Description)
				}
				fmt.Println()
			}
			fmt.Println()
		} else if cmd.HasAvailableSubCommands() {
			fmt.Println("Commands:")
			for _, sub := range cmd.Commands() {
				if sub.IsAvailableCommand() || sub.Hidden {
					fmt.Printf("  %-12s %s\n", sub.Name(), sub.Short)
				}
			}
			fmt.Println()
		}

		// Output examples
		if cmd.HasExample() {
			fmt.Println("Examples:")
			fmt.Print(cmd.Example)
			fmt.Println()
			fmt.Println()
		}

		// Output flags
		if cmd.HasAvailableLocalFlags() {
			fmt.Println("Flags:")
			usage := cmd.LocalFlags().FlagUsages()
			// FlagUsages already has leading spaces, just print as-is
			fmt.Print(usage)
		}
	})
}

func NewRootCommand(version, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "andurel",
		Short:        "Andurel - The Go Web development framework",
		Long:         `Andurel is a comprehensive web development framework for Go,`,
		Version:      fmt.Sprintf("%s (built: %s)", version, date),
		SilenceUsage: true,
		Run: func(cmd *cobra.Command, args []string) {
			printBanner()
			cmd.Help()
		},
	}

	rootCmd.AddCommand(newRunAppCommand())

	rootCmd.AddCommand(newProjectCommand(version))
	rootCmd.AddCommand(newControllerRootCommand())
	rootCmd.AddCommand(newViewRootCommand())
	rootCmd.AddCommand(newResourceRootCommand())
	rootCmd.AddCommand(newDatabaseCommand())
	rootCmd.AddCommand(newMigrateCommand())

	rootCmd.AddCommand(newAppCommand())
	rootCmd.AddCommand(newConsoleCommand())
	rootCmd.AddCommand(newDblabCommand())
	rootCmd.AddCommand(newMailpitCommand())
	rootCmd.AddCommand(newLlmCommand())
	rootCmd.AddCommand(newToolCommand())
	rootCmd.AddCommand(newExtensionCommand())
	rootCmd.AddCommand(newModelRootCommand())
	rootCmd.AddCommand(newUpgradeCommand(version))
	rootCmd.AddCommand(newDoctorCommand(version))

	return rootCmd
}

func newRunAppCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run",
		Aliases: []string{"r"},
		Short:   "Runs the app",
		Args:    cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			if err := checkBinaries(rootDir); err != nil {
				return err
			}

			binPath := filepath.Join(rootDir, "bin", "shadowfax")

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

	binPath := filepath.Join(rootDir, "bin", "shadowfax")
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("bin/shadowfax not found. Run 'andurel tool sync' to download it")
	}

	return nil
}
