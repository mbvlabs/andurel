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

func isInAndurelProject() bool {
	_, err := findGoModRoot()
	if err != nil {
		return false
	}
	return true
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
			fmt.Println()
			cmd.Help()
		},
	}

	rootCmd.AddCommand(newProjectCommand(version))
	rootCmd.AddCommand(newGenerateCommand())
	rootCmd.AddCommand(newFmtCommand())
	rootCmd.AddCommand(newDatabaseCommand())

	rootCmd.AddCommand(newRunAppCommand())
	rootCmd.AddCommand(newConsoleCommand())
	rootCmd.AddCommand(newLlmCommand())
	rootCmd.AddCommand(newToolCommand())
	rootCmd.AddCommand(newExtensionCommand())
	rootCmd.AddCommand(newBuildCommand())
	rootCmd.AddCommand(newUpgradeCommand(version))
	rootCmd.AddCommand(newDoctorCommand(version))

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if c.Parent() != nil {
			c.Print(c.Short)
			c.Println()
			c.Println()
			c.Println("Usage:")
			c.Printf("  %s\n", c.UseLine())
			if c.HasAvailableSubCommands() {
				c.Println()
				c.Println("Available Commands:")
				for _, s := range c.Commands() {
					if !s.IsAvailableCommand() || s.Hidden {
						continue
					}
					c.Printf("  %-12s %s\n", s.Name(), s.Short)
				}
			}
			if c.HasAvailableLocalFlags() {
				c.Println()
				c.Println("Flags:")
				c.Print(c.LocalFlags().FlagUsages())
			}
			return
		}
		if isInAndurelProject() {
			c.Println("Usage:")
			c.Println("  andurel [command]")
			c.Println()
			c.Println("Commands:")
			for _, sub := range c.Commands() {
				if !sub.IsAvailableCommand() || sub.Hidden {
					continue
				}
				if sub.Name() == "new" || sub.Name() == "help" || sub.Name() == "completion" {
					continue
				}
				c.Printf("  %-12s %s\n", sub.Name(), sub.Short)
			}
			c.Println()
			c.Println("Flags:")
			c.Println(c.LocalFlags().FlagUsages())
			c.Println("Use \"andurel [command] --help\" for more information about a command.")
		} else {
			fmt.Println("Usage:")
			fmt.Println("  andurel COMMAND [options]")
			fmt.Println()
			fmt.Println("You must specify a command:")
			fmt.Println()
			fmt.Printf("  %-14s %s\n", "new", "Create a new Andurel project")
			fmt.Println()
			fmt.Println("All commands can be run with -h (or --help) for more information.")
			fmt.Println()
			fmt.Println("Inside an Andurel application directory, some common commands are:")
			fmt.Println()
			fmt.Printf("  %-14s %s\n", "generate", "Generate new code")
			fmt.Printf("  %-14s %s\n", "console", "Interactive database console")
			fmt.Printf("  %-14s %s\n", "migrate", "Run database migrations")
		}
	})

	return rootCmd
}

func newRunAppCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run",
		Aliases: []string{"r"},
		Short:   "Start the development server",
		Long:    "Start the development server (shadowfax) for your Andurel application.",
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

var findGoModRoot = func() (string, error) {
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

func findProjectRoot() (string, error) {
	return findGoModRoot()
}
