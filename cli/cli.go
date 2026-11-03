// Package cli provides the command-line interface for the Andurel framework.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/mbvlabs/andurel/v2/cli/internal/devserver"
	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/mbvlabs/andurel/v2/internal/cache"
	"github.com/mbvlabs/andurel/v2/layout"
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
	output.SetCLIVersion(version)
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
	rootCmd.AddCommand(newEmailCommand())
	rootCmd.AddCommand(newInspectCommand())
	rootCmd.AddCommand(newPatternsCommand())
	rootCmd.AddCommand(newVerifyCommand())
	rootCmd.AddCommand(newFmtCommand())
	rootCmd.AddCommand(newDatabaseCommand())
	rootCmd.AddCommand(newConsoleCommand())
	rootCmd.AddCommand(newRunAppCommand())
	rootCmd.AddCommand(newToolCommand())
	rootCmd.AddCommand(newBuildCommand())
	rootCmd.AddCommand(newUpgradeCommand(version))
	rootCmd.AddCommand(newPackagesCommand())
	rootCmd.AddCommand(newDoctorCommand(version))
	rootCmd.AddCommand(newPortsCommand())
	rootCmd.AddCommand(newCommandsCommand(rootCmd))
	rootCmd.AddCommand(newSkillCommand())

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	if err := configureCommandCapabilities(rootCmd); err != nil {
		panic(err)
	}
	registerAllCommandMeta(rootCmd)
	configureGroupCommandBehavior(rootCmd)

	rootCmd.SetHelpFunc(renderHumanHelp)

	return rootCmd
}

func newRunAppCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "run",
		Aliases: []string{"r"},
		Short:   "Start the development server",
		Long: `Start the development server for your Andurel application.

andurel run builds and runs cmd/app and cmd/queue, and serves the app through
a live-reload proxy on PROXY_PORT (default 3000). The app itself listens on
PORT (default 8080). It also runs the Vite dev server, which serves Inertia
pages with HMR and development SSR (/__inertia_ssr). cmd/ssr is the
production SSR process and is not started here.

It watches:
  - Go files: rebuild and restart the app and the queue worker
  - resources/*.html: rebuild and restart the app (the embedded Inertia root)
  - emails/*.go and emails/**: andurel sync emails regenerates props, manifest
    and emails/templates, then the app and queue rebuild
  - narsilc queries are generated once at start

Log lines from the runner itself start with [run]. In a terminal the logs
are shown in a tabbed view (app, queue, build, and each sidecar).
Ctrl-C, SIGINT or SIGTERM stops Vite, the app, the queue and any sidecars.
Run this from your project root.

Pass --tools to start allowlisted sidecars (mailpit) alongside the runner.`,
		Example: `  andurel run
  andurel run --tools mailpit`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectStructuredOutput(cmd, "andurel run"); err != nil {
				return err
			}
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}

			if err := generateNarsilcIfNeeded(rootDir); err != nil {
				return err
			}
			toolsFlag, err := cmd.Flags().GetString("tools")
			if err != nil {
				return err
			}

			opts, err := devServerOptions(rootDir, toolsFlag)
			if err != nil {
				return err
			}
			if hasReactEmails(rootDir) {
				opts.SyncEmails = func(ctx context.Context, w io.Writer) error {
					// These are progress messages; a log failure must not stop
					// the sync.
					_, _ = fmt.Fprintln(w, "[run] Syncing React Email templates...")
					report, err := generateEmailsFiles(rootDir)
					if err != nil {
						return err
					}
					_, _ = fmt.Fprintf(
						w,
						"[run] Generated %d email templates\n",
						report.TemplateCount,
					)
					return runEmailsExport(ctx, rootDir, w, w)
				}
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runDevServerFunc(ctx, opts)
		},
	}

	cmd.Flags().String(
		"tools",
		"",
		"comma-separated sidecar tools to start with the runner (mailpit)",
	)

	return cmd
}

// devServerOptions builds the in-process dev server options. The JavaScript
// package manager comes from andurel.toml. Development SSR is owned by Vite;
// cmd/ssr settings stay in app config for production.
func devServerOptions(rootDir, toolsFlag string) (devserver.Options, error) {
	opts := devserver.Options{
		Dir:            rootDir,
		PackageManager: "pnpm",
		Version:        output.CLIVersion(),
		Verbose:        os.Getenv("ANDUREL_RUN_VERBOSE") == "true",
		ClearLogs:      os.Getenv("ANDUREL_RUN_CLEAR_LOGS") != "",
	}

	lock, err := layout.ReadLockFile(rootDir)
	if err == nil && lock.ScaffoldConfig != nil {
		if packageManager := lock.ScaffoldConfig.PackageManager(); packageManager != "" {
			opts.PackageManager = packageManager
		}
	}

	toolNames, err := parseRunTools(toolsFlag)
	if err != nil {
		return devserver.Options{}, err
	}
	if len(toolNames) > 0 {
		if err := checkToolBinaries(rootDir, toolNames); err != nil {
			return devserver.Options{}, err
		}
		opts.Tools = toolNames
	}

	return opts, nil
}

// allowedRunTools is the v1 allowlist for `andurel run --tools`.
var allowedRunTools = map[string]struct{}{
	"mailpit": {},
}

func parseRunTools(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		if raw != "" {
			return nil, fmt.Errorf("--tools: empty list")
		}
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return nil, fmt.Errorf("--tools: empty name")
		}
		if _, ok := allowedRunTools[name]; !ok {
			return nil, fmt.Errorf("--tools: unknown tool %q (allowed: mailpit)", name)
		}
		if _, dup := seen[name]; dup {
			return nil, fmt.Errorf("--tools: duplicate tool %q", name)
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--tools: empty list")
	}
	return out, nil
}

func checkToolBinaries(rootDir string, names []string) error {
	for _, name := range names {
		binPath := filepath.Join(rootDir, "bin", name)
		if _, err := os.Stat(binPath); err != nil {
			return output.NewError(
				output.CodeMissingTool,
				fmt.Sprintf("bin/%s not found", name),
				output.ExitDependency,
				"Run 'andurel tool sync' to download it.",
			)
		}
	}
	return nil
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
