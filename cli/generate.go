package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mbvlabs/andurel/generator"

	"github.com/spf13/cobra"
)

// chdirToProjectRoot finds the go.mod root and changes the working directory
// to it so that the generator's relative paths resolve correctly regardless
// of where the command was invoked.
func chdirToProjectRoot() error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return fmt.Errorf("not in an andurel project directory: %w", err)
	}
	return os.Chdir(rootDir)
}

func newControllerRootCommand() *cobra.Command {
	var withViews bool

	cmd := &cobra.Command{
		Use:   "controller <name> <command>",
		Short: "Controller management commands",
		Long:  "Manage resource controllers.\n\n<ResourceName> is the associated model name used for generation.",
		Example: `  controller User create
  controller User create --with-views`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if len(args) > 2 {
				return fmt.Errorf("too many arguments\nRun 'andurel controller --help' for usage")
			}
			name := args[0]
			switch args[1] {
			case "create":
				if err := chdirToProjectRoot(); err != nil {
					return err
				}
				return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
					gen, err := generator.New()
					if err != nil {
						return err
					}
					return gen.GenerateControllerFromModel(name, withViews)
				})(cmd, args)
			default:
				return fmt.Errorf("unknown controller command %q\nRun 'andurel controller --help' for usage", args[1])
			}
		},
	}

	setStandardHelp(cmd, helpCommand{
		Use:         "controller <ResourceName> create",
		Description: "creates a resource controller",
	})

	cmd.Flags().BoolVar(&withViews, "with-views", false, "Generate views along with the controller")

	cmd.AddCommand(newControllerCreateCommand())

	return cmd
}

func newControllerCreateCommand() *cobra.Command {
	var withViews bool

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new controller",
		Example: `  controller User create
  controller User create --with-views`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := chdirToProjectRoot(); err != nil {
				return err
			}
			return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
				gen, err := generator.New()
				if err != nil {
					return err
				}
				return gen.GenerateControllerFromModel(name, withViews)
			})(cmd, args)
		},
	}

	cmd.Flags().BoolVar(&withViews, "with-views", false, "Generate views along with the controller")

	return cmd
}

func newViewRootCommand() *cobra.Command {
	var withController bool

	cmd := &cobra.Command{
		Use:   "view",
		Short: "View management commands",
		Long:  "Manage view templates and Templ code generation.\n\n<ResourceName> is the associated model name used for generation.",
		Example: `  view User create
  view User create --with-controller
  view generate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if len(args) > 2 {
				return fmt.Errorf("too many arguments\nRun 'andurel view --help' for usage")
			}
			name := args[0]
			switch args[1] {
			case "create":
				if err := chdirToProjectRoot(); err != nil {
					return err
				}
				return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
					gen, err := generator.New()
					if err != nil {
						return err
					}
					return gen.GenerateViewFromModel(name, withController)
				})(cmd, args)
			default:
				return fmt.Errorf("unknown view command %q\nRun 'andurel view --help' for usage", args[1])
			}
		},
	}

	setStandardHelp(cmd,
		helpCommand{
			Use:         "view <ResourceName> create",
			Description: "creates a resource view",
		},
		helpCommand{
			Use:         "view generate",
			Description: "generates Go code from Templ templates",
		},
		helpCommand{
			Use:         "view format",
			Description: "formats Templ templates in views and email directories",
		},
	)

	cmd.Flags().BoolVar(&withController, "with-controller", false, "Generate controller along with the views")

	cmd.AddCommand(
		newViewCreateCommand(),
		newTemplGenerateCommand(),
		newTemplFormatCommand(),
	)

	return cmd
}

func newViewCreateCommand() *cobra.Command {
	var withController bool

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create new views for a model",
		Example: `  view User create
  view User create --with-controller`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := chdirToProjectRoot(); err != nil {
				return err
			}
			return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
				gen, err := generator.New()
				if err != nil {
					return err
				}
				return gen.GenerateViewFromModel(name, withController)
			})(cmd, args)
		},
	}

	cmd.Flags().BoolVar(&withController, "with-controller", false, "Generate controller along with the views")

	return cmd
}

func newResourceRootCommand() *cobra.Command {
	var tableName string

	cmd := &cobra.Command{
		Use:   "resource <name> <command>",
		Short: "Resource management commands",
		Long:  "Generate complete resources (model, controller, views, and routes).\n\n<ResourceName> is the associated model name used for generation.",
		Example: `  resource Product create
  resource Product create --table-name=inventory`,
	}

	setStandardHelp(cmd, helpCommand{
		Use:         "resource <ResourceName> create",
		Description: "creates a complete resource",
	})

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return cmd.Help()
		}
		if len(args) > 2 {
			return fmt.Errorf("too many arguments\nRun 'andurel resource --help' for usage")
		}
		name := args[0]
		switch args[1] {
		case "create":
			if err := chdirToProjectRoot(); err != nil {
				return err
			}
			return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
				gen, err := generator.New()
				if err != nil {
					return err
				}
				if err := gen.GenerateModel(name, tableName, false); err != nil {
					return err
				}
				return gen.GenerateControllerFromModel(name, true)
			})(cmd, args)
		default:
			return fmt.Errorf("unknown resource command %q\nRun 'andurel resource --help' for usage", args[1])
		}
	}

	cmd.Flags().StringVar(&tableName, "table-name", "", "Override the default table name (defaults to plural form of model name)")

	return cmd
}

type createdFileTracker struct {
	rootDir       string
	existingFiles map[string]struct{}
}

func withGenerateCleanup(run func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		tracker, trackerErr := newCreatedFileTracker()

		runErr := run(cmd, args)
		if runErr == nil {
			return nil
		}

		if trackerErr != nil {
			return formatGenerateFailure(runErr, nil, nil, trackerErr)
		}

		removedFiles, cleanupFailures, cleanupErr := tracker.cleanupCreatedFiles()
		if cleanupErr != nil {
			return formatGenerateFailure(runErr, nil, nil, cleanupErr)
		}

		return formatGenerateFailure(runErr, removedFiles, cleanupFailures, nil)
	}
}

func newCreatedFileTracker() (*createdFileTracker, error) {
	rootDir, err := findProjectRoot()
	if err != nil {
		return nil, err
	}

	existingFiles, err := snapshotFiles(rootDir)
	if err != nil {
		return nil, err
	}

	return &createdFileTracker{
		rootDir:       rootDir,
		existingFiles: existingFiles,
	}, nil
}

func (t *createdFileTracker) cleanupCreatedFiles() ([]string, []string, error) {
	currentFiles, err := snapshotFiles(t.rootDir)
	if err != nil {
		return nil, nil, err
	}

	createdFiles := make([]string, 0)
	for relPath := range currentFiles {
		if _, exists := t.existingFiles[relPath]; !exists {
			createdFiles = append(createdFiles, relPath)
		}
	}
	sort.Strings(createdFiles)

	removedFiles := make([]string, 0, len(createdFiles))
	cleanupFailures := make([]string, 0)

	for _, relPath := range createdFiles {
		fullPath := filepath.Join(t.rootDir, relPath)
		if err := os.Remove(fullPath); err != nil {
			cleanupFailures = append(cleanupFailures, fmt.Sprintf("%s (%v)", relPath, err))
			continue
		}
		removedFiles = append(removedFiles, relPath)
	}

	return removedFiles, cleanupFailures, nil
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, statErr := os.Stat(goModPath); statErr == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("unable to locate project root containing go.mod")
		}
		dir = parent
	}
}

func snapshotFiles(rootDir string) (map[string]struct{}, error) {
	files := make(map[string]struct{})

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		files[filepath.ToSlash(relPath)] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

func formatGenerateFailure(runErr error, removedFiles []string, cleanupFailures []string, trackerErr error) error {
	if trackerErr != nil {
		return fmt.Errorf("%w\n\nUnable to clean up created files automatically: %v", runErr, trackerErr)
	}

	var msg strings.Builder
	msg.WriteString(runErr.Error())

	if len(removedFiles) == 0 && len(cleanupFailures) == 0 {
		msg.WriteString("\n\nNo new files were created before the failure.")
		return errors.New(msg.String())
	}

	msg.WriteString("\n\nGeneration failed and automatic cleanup ran.")
	if len(removedFiles) > 0 {
		msg.WriteString(fmt.Sprintf("\nRemoved %d created file(s):", len(removedFiles)))
		msg.WriteString(formatPathList(removedFiles, 12))
	}

	if len(cleanupFailures) > 0 {
		msg.WriteString(fmt.Sprintf("\nCould not remove %d file(s):", len(cleanupFailures)))
		msg.WriteString(formatPathList(cleanupFailures, 12))
		msg.WriteString("\nPlease remove these files manually.")
	}

	return errors.New(msg.String())
}

func formatPathList(paths []string, maxItems int) string {
	var out strings.Builder
	limit := len(paths)
	if limit > maxItems {
		limit = maxItems
	}

	for i := 0; i < limit; i++ {
		out.WriteString("\n  - ")
		out.WriteString(paths[i])
	}

	if len(paths) > limit {
		out.WriteString(fmt.Sprintf("\n  - ... and %d more", len(paths)-limit))
	}

	return out.String()
}
