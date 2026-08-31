package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mbvlabs/andurel/emailcompiler"
	"github.com/mbvlabs/andurel/layout"
	"github.com/mbvlabs/andurel/layout/versions"
	"github.com/spf13/cobra"
)

func newEmailCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "email",
		Short: "Compile email templates",
		Long: `Compile Tailwind-authored email templates into email-compatible Go renderers.

The compiler leaves authored .templ files unchanged. Tailwind utilities are
resolved in memory, converted to inline styles and compatibility attributes,
then emitted through Templ as the normal *_templ.go files.`,
	}
	command.AddCommand(newEmailCompileCommand())
	return command
}

func newEmailCompileCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "compile",
		Short: "Compile Tailwind classes in email templates",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, err := findGoModRoot()
			if err != nil {
				return err
			}
			return compileEmailProject(cmd.Context(), rootDir)
		},
	}
}

func compileEmailProject(ctx context.Context, rootDir string) error {
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return fmt.Errorf("failed to read andurel.lock: %w", err)
	}

	tailwindTool, ok := lock.Tools["tailwindcli"]
	if !ok {
		tailwindTool = layout.NewBinaryTool("tailwindcli", versions.TailwindCLI)
	}
	if err := syncSingleToolFunc(
		rootDir,
		"tailwindcli",
		tailwindTool,
		runtime.GOOS,
		runtime.GOARCH,
	); err != nil {
		return fmt.Errorf("failed to sync tailwind CLI: %w", err)
	}

	return compileEmailWithTailwind(ctx, rootDir, filepath.Join(rootDir, "bin", "tailwindcli"))
}

func compileEmailWithTailwind(ctx context.Context, rootDir, tailwindPath string) error {
	fmt.Println("Compiling Tailwind email templates...")
	if err := emailcompiler.Compile(ctx, emailcompiler.Config{
		ProjectRoot:  rootDir,
		TailwindPath: tailwindPath,
	}); err != nil {
		return fmt.Errorf("email compilation failed: %w", err)
	}
	fmt.Println("Email templates compiled.")
	return nil
}

func hasEmailCompilerInputs(rootDir string) bool {
	for _, path := range []string{
		filepath.Join(rootDir, "email"),
		filepath.Join(rootDir, "css", "email.css"),
	} {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}
