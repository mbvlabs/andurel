package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mbvlabs/andurel/cli/output"
	"github.com/mbvlabs/andurel/layout"

	"github.com/spf13/cobra"
)

func newProjectCommand(version string) *cobra.Command {
	var dryRun bool
	var diff bool
	projectCmd := &cobra.Command{
		Use:     "new [project-name]",
		Aliases: []string{"n"},
		Short:   "Create a new Andurel project",
		Long: `Scaffold a complete Andurel project with the given name.

Generates the full project structure including controllers, models, views,
database migrations, router, services, and configuration files. After
creation, run 'andurel tool sync' to download required binaries.`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) <= 1 {
				return nil
			}
			return output.NewError(
				output.CodeUsage,
				"andurel new accepts exactly one project name",
				output.ExitUsage,
				"Run andurel new <project-name> with a single safe directory name.",
			)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if isInAndurelProject() {
				return output.NewError(output.CodeUnsafeAction, "cannot create a new project inside an existing Andurel project", output.ExitUnsafe, "Run andurel new from a parent directory outside an existing project.")
			}
			return newProject(cmd, args, version, dryRun, diff)
		},
	}

	projectCmd.Flags().
		StringSliceP("extensions", "e", nil, "Extensions to enable (comma-separated list)")

	projectCmd.Flags().
		String("inertia", "", "Inertia adapter to use (vue, react). Optionally append /npm|pnpm|bun|yarn to specify the JS runtime (default: npm)")
	projectCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview project files without creating them")
	projectCmd.Flags().BoolVar(&diff, "diff", false, "Include a text diff preview in structured output")

	return projectCmd
}

var newProjectNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type newProjectDestination struct {
	projectName      string
	path             string
	currentDirectory bool
}

func newProject(cmd *cobra.Command, args []string, version string, dryRun bool, diff bool) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	destination, err := resolveNewProjectDestination(args[0])
	if err != nil {
		return err
	}
	projectName := destination.projectName

	database := "postgresql"

	inertiaFlag, err := cmd.Flags().GetString("inertia")
	if err != nil {
		return err
	}

	adapter := inertiaFlag
	javascriptRuntime := ""
	if inertiaFlag != "" {
		parts := strings.SplitN(inertiaFlag, "/", 2)
		adapter = parts[0]
		if len(parts) == 2 {
			javascriptRuntime = parts[1]
		} else {
			javascriptRuntime = "npm"
		}

		if !layout.IsSupportedInertiaAdapter(adapter) {
			return fmt.Errorf(
				"invalid inertia adapter: %s - valid options are 'vue', 'react'",
				adapter,
			)
		}
		if !layout.IsSupportedJavaScriptRuntime(javascriptRuntime) {
			return fmt.Errorf(
				"invalid JavaScript runtime: %s - valid options are 'npm', 'pnpm', 'bun', 'yarn'",
				javascriptRuntime,
			)
		}
	}

	extensions, err := cmd.Flags().GetStringSlice("extensions")
	if err != nil {
		return err
	}
	scaffold := func(target string) error {
		return layout.Scaffold(target, projectName, database, version, extensions, adapter, javascriptRuntime)
	}
	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if dryRun || output.SuppressesHumanOutput(opts) {
		silence := output.SuppressesHumanOutput(opts)
		report, err := newProjectReport(projectName, destination.path, dryRun, diff, func(target string) error {
			return runWithOptionalStdoutSilence(silence, func() error {
				if dryRun {
					return scaffold(target)
				}
				return scaffoldNewProject(destination, scaffold)
			})
		})
		if err != nil {
			return wrapNewProjectScaffoldError(err)
		}
		return output.OK(cmd, report, mutationSummary(report), output.Breadcrumb{Command: "andurel tool sync"}, output.Breadcrumb{Command: "andurel database migrate up"}, output.Breadcrumb{Command: "andurel run"})
	}

	if err := scaffoldNewProject(destination, scaffold); err != nil {
		return wrapNewProjectScaffoldError(err)
	}

	fmt.Printf("\n🎉 Successfully created project: %s\n", projectName)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  cd %s\n", args[0])
	fmt.Printf("  andurel tool sync\n")
	fmt.Printf("  cp .env.example .env\n")
	fmt.Printf("  fill in your database connection details in .env\n")
	fmt.Printf("  (andurel database create - if database does not exist\n")
	fmt.Printf("  andurel database migrate up\n")
	if layout.IsSupportedInertiaAdapter(adapter) {
		fmt.Printf("  %s install\n", javascriptRuntime)
	}
	fmt.Printf("  andurel run\n")

	return nil
}

func resolveNewProjectDestination(projectArgument string) (newProjectDestination, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return newProjectDestination{}, output.WrapError(
			output.CodeGenerationFailed,
			fmt.Errorf("resolve current directory: %w", err),
			output.ExitGeneration,
			"Run the command from a readable parent directory.",
		)
	}

	return normalizeNewProjectDestination(cwd, projectArgument)
}

func normalizeNewProjectDestination(cwd, projectArgument string) (newProjectDestination, error) {
	if projectArgument != "." {
		if err := validateNewProjectName(projectArgument); err != nil {
			return newProjectDestination{}, err
		}
	}

	absCWD, err := filepath.Abs(cwd)
	if err != nil {
		return newProjectDestination{}, newProjectPathError("normalize current directory", err)
	}
	cleanCWD := filepath.Clean(absCWD)
	resolvedCWD, err := filepath.EvalSymlinks(cleanCWD)
	if err != nil {
		return newProjectDestination{}, newProjectPathError("resolve current directory", err)
	}

	if projectArgument == "." {
		projectName := filepath.Base(resolvedCWD)
		if err := validateNewProjectName(projectName); err != nil {
			return newProjectDestination{}, output.NewError(
				output.CodeUsage,
				fmt.Sprintf("current directory name %q is not a valid project name", projectName),
				output.ExitUsage,
				"Rename the directory to a safe basename matching [A-Za-z0-9][A-Za-z0-9._-]* and retry.",
			)
		}

		entries, err := os.ReadDir(resolvedCWD)
		if err != nil {
			return newProjectDestination{}, newProjectPathError("read current directory", err)
		}
		if len(entries) != 0 {
			return newProjectDestination{}, output.NewError(
				output.CodeUnsafeAction,
				"current directory is not empty",
				output.ExitUnsafe,
				"Use an empty directory or choose a new project name.",
			)
		}

		return newProjectDestination{
			projectName:      projectName,
			path:             resolvedCWD,
			currentDirectory: true,
		}, nil
	}

	target := filepath.Clean(filepath.Join(resolvedCWD, projectArgument))
	if filepath.Dir(target) != resolvedCWD {
		return newProjectDestination{}, output.NewError(
			output.CodeUsage,
			"project destination must be a direct child of the current directory",
			output.ExitUsage,
			"Use a single safe directory name without path separators.",
		)
	}
	if _, err := os.Lstat(target); err == nil {
		return newProjectDestination{}, output.NewError(
			output.CodeUnsafeAction,
			fmt.Sprintf("project destination %q already exists", target),
			output.ExitUnsafe,
			"Choose a different project name or remove the existing destination after reviewing its contents.",
		)
	} else if !errors.Is(err, os.ErrNotExist) {
		return newProjectDestination{}, newProjectPathError("inspect project destination", err)
	}

	return newProjectDestination{projectName: projectArgument, path: target}, nil
}

func validateNewProjectName(projectName string) error {
	if newProjectNamePattern.MatchString(projectName) {
		return nil
	}
	return output.NewError(
		output.CodeUsage,
		fmt.Sprintf("invalid project name %q", projectName),
		output.ExitUsage,
		"Use a basename matching [A-Za-z0-9][A-Za-z0-9._-]* without spaces or path separators.",
	)
}

func newProjectPathError(action string, err error) error {
	return output.WrapError(
		output.CodeGenerationFailed,
		fmt.Errorf("%s: %w", action, err),
		output.ExitGeneration,
		"Check the destination path and filesystem permissions, then retry.",
	)
}

func wrapNewProjectScaffoldError(err error) error {
	var cliErr *output.CLIError
	if errors.As(err, &cliErr) {
		return err
	}
	return output.WrapError(
		output.CodeGenerationFailed,
		err,
		output.ExitGeneration,
		"No partial scaffold was retained. Check the scaffold inputs and filesystem permissions, then retry.",
	)
}

func scaffoldNewProject(destination newProjectDestination, scaffold func(string) error) (resultErr error) {
	parent := filepath.Dir(destination.path)
	stagingRoot, err := os.MkdirTemp(parent, ".andurel-new-*")
	if err != nil {
		return fmt.Errorf("create scaffold staging directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(stagingRoot); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove scaffold staging directory: %w", err))
		}
	}()

	stagedProject := filepath.Join(stagingRoot, destination.projectName)
	if err := scaffold(stagedProject); err != nil {
		return fmt.Errorf("generate staged scaffold: %w", err)
	}

	if !destination.currentDirectory {
		if _, err := os.Lstat(destination.path); err == nil {
			return fmt.Errorf("publish scaffold: destination %q now exists", destination.path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect scaffold destination before publishing: %w", err)
		}
		if err := os.Rename(stagedProject, destination.path); err != nil {
			return fmt.Errorf("publish scaffold: %w", err)
		}
		return nil
	}

	return publishStagedProjectContents(stagedProject, destination.path)
}

func publishStagedProjectContents(stagedProject, destination string) error {
	destinationEntries, err := os.ReadDir(destination)
	if err != nil {
		return fmt.Errorf("read project destination: %w", err)
	}
	if len(destinationEntries) != 0 {
		return fmt.Errorf("publish scaffold: current directory is no longer empty")
	}

	entries, err := os.ReadDir(stagedProject)
	if err != nil {
		return fmt.Errorf("read staged scaffold: %w", err)
	}

	published := make([]string, 0, len(entries))
	for _, entry := range entries {
		source := filepath.Join(stagedProject, entry.Name())
		target := filepath.Join(destination, entry.Name())
		if err := os.Rename(source, target); err != nil {
			return errors.Join(
				fmt.Errorf("publish scaffold entry %q: %w", entry.Name(), err),
				rollbackPublishedProjectEntries(stagedProject, destination, published),
			)
		}
		published = append(published, entry.Name())
	}

	return nil
}

func rollbackPublishedProjectEntries(stagedProject, destination string, published []string) error {
	var rollbackErr error
	for index := len(published) - 1; index >= 0; index-- {
		name := published[index]
		source := filepath.Join(destination, name)
		target := filepath.Join(stagedProject, name)
		if err := os.Rename(source, target); err != nil {
			if removeErr := os.RemoveAll(source); removeErr != nil {
				rollbackErr = errors.Join(
					rollbackErr,
					fmt.Errorf("roll back scaffold entry %q: %w", name, err),
					fmt.Errorf("remove partially published scaffold entry %q: %w", name, removeErr),
				)
			}
		}
	}
	return rollbackErr
}

func newProjectReport(projectName, basePath string, dryRun bool, diff bool, scaffold func(target string) error) (report mutationReport, err error) {
	var targetPath string
	if dryRun {
		tempDir, err := os.MkdirTemp("", "andurel-new-dry-run-*")
		if err != nil {
			return mutationReport{}, err
		}
		defer func() { err = errors.Join(err, os.RemoveAll(tempDir)) }()
		targetPath = filepath.Join(tempDir, projectName)
		if err := scaffold(targetPath); err != nil {
			return mutationReport{}, err
		}
	} else {
		targetPath = basePath
		if err := scaffold(targetPath); err != nil {
			return mutationReport{}, err
		}
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return mutationReport{}, err
	}
	after, err := snapshotFilesForReport(absTarget)
	if err != nil {
		return mutationReport{}, err
	}
	report = buildMutationReport(mutationOptions{
		Action:   "new project",
		Resource: projectName,
		DryRun:   dryRun,
		Diff:     diff,
	}, fileSnapshot{}, after)
	return report, nil
}
