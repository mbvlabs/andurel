// Package cli provides the command-line interface for the Andurel framework.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/internal/cache"
	"github.com/mbvlabs/andurel/layout"
	"github.com/spf13/cobra"
)

type helpCommand struct {
	Use         string
	Description string
}

func setStandardHelp(cmd *cobra.Command, _ ...helpCommand) {
	cmd.SetHelpFunc(renderHumanHelp)
}

func isInAndurelProject() bool {
	_, err := findGoModRoot()
	return err == nil
}

// NewRootCommand creates a new root command.
func NewRootCommand(version, date string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "andurel",
		Short: "Space-grade Go framework for humans and agents",
		Long: `Andurel is a space-grade Go framework for humans and agents.

Everything you and your agent(s) need to build robust and performant applications that will scale to the far-side of the moon.`,
		Version:           fmt.Sprintf("%s (built: %s)", version, date),
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: validateProjectionFlags,
		Run: func(cmd *cobra.Command, args []string) {
			printBanner()
			fmt.Println()
			_ = cmd.Help()
		},
	}

	output.RegisterPersistentFlags(rootCmd)

	rootCmd.AddCommand(newProjectCommand(version))
	rootCmd.AddCommand(newGenerateCommand())
	rootCmd.AddCommand(newSyncGroupCommand())
	rootCmd.AddCommand(newInspectCommand())
	rootCmd.AddCommand(newFmtCommand())
	rootCmd.AddCommand(newDatabaseCommand())
	rootCmd.AddCommand(newRunAppCommand())
	rootCmd.AddCommand(newToolCommand())
	rootCmd.AddCommand(newBuildCommand())
	rootCmd.AddCommand(newUpgradeCommand(version))
	rootCmd.AddCommand(newPackagesCommand())
	rootCmd.AddCommand(newDoctorCommand(version))
	rootCmd.AddCommand(newCommandsCommand(rootCmd))
	rootCmd.AddCommand(newSkillCommand())

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	if err := configureProjectionContracts(rootCmd); err != nil {
		panic(err)
	}
	registerAllCommandMeta(rootCmd)

	rootCmd.SetHelpFunc(renderHumanHelp)

	return rootCmd
}

func newRunAppCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run",
		Aliases: []string{"r"},
		Short:   "Start the development server",
		Long: `Start the development server (shadowfax) for your Andurel application.

The server auto-reloads on file changes, including Go, Templ, CSS, and
narsilc query files. For Inertia projects, shadowfax also runs the Vite
dev server. Development SSR is served by Vite's /__inertia_ssr endpoint.
cmd/ssr is the production Node owner. Run this from your project root.`,
		Example: `  andurel run`,
		Args:    cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			if err := checkBinaries(rootDir); err != nil {
				return err
			}
			if err := generateNarsilcIfNeeded(rootDir); err != nil {
				return err
			}
			watchContext, stopWatching := context.WithCancel(cmd.Context())
			defer stopWatching()
			if hasEmailCompilerInputs(rootDir) {
				if err := compileEmailProject(watchContext, rootDir); err != nil {
					return err
				}
				if err := watchEmailProject(
					watchContext,
					rootDir,
					filepath.Join(rootDir, "bin", "tailwindcli"),
				); err != nil {
					return err
				}
			}

			binPath := filepath.Join(rootDir, "bin", "shadowfax")
			shadowfaxArgs, err := shadowfaxRunArgs(rootDir)
			if err != nil {
				return err
			}

			runCmd := exec.Command(binPath, shadowfaxArgs...)
			runCmd.Stdout = os.Stdout
			runCmd.Stderr = os.Stderr
			runCmd.Stdin = os.Stdin
			runCmd.Dir = rootDir

			return runCmd.Run()
		},
	}

	return cmd
}

// shadowfaxRunArgs builds the explicit CLI contract passed to Shadowfax.
// Inertia identity and package manager come from andurel.lock. Development
// SSR is owned by Vite; cmd/ssr settings stay in app config for production.
func shadowfaxRunArgs(rootDir string) ([]string, error) {
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		// Missing or incomplete lock: run Shadowfax without Inertia flags.
		return nil, nil
	}
	if lock.ScaffoldConfig == nil || lock.ScaffoldConfig.Inertia == "" {
		return nil, nil
	}

	packageManager := lock.ScaffoldConfig.PackageManager()
	if packageManager == "" {
		packageManager = "npm"
	}
	return []string{
		"--inertia",
		"--js-package-manager", packageManager,
	}, nil
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
		return output.NewError(
			output.CodeMissingTool,
			"bin/shadowfax not found",
			output.ExitDependency,
			"Run 'andurel tool sync' to download it.",
		)
	}

	return nil
}

func findProjectRoot() (string, error) {
	return findGoModRoot()
}
