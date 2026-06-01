package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

func newGenerateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate new code (model, controller, scaffold, job)",
		Long: `Generates new code for your Andurel application. The following
generators are available:

  model       Generate a model from the existing migration, or update one
              with --update
  views       Generate Go code from Templ templates (templ generate)
  controller  Generate a controller, views, and routes
  scaffold    Generate a complete resource with model, controller, views, and routes
  job         Generate a background job with a worker`,
		Example: `  andurel generate model Post
  andurel generate model Post --update
  andurel generate views
  andurel generate controller users index show
  andurel generate scaffold Product
  andurel generate job SendWelcomeEmail`,
	}

	cmd.AddCommand(
		newGenerateModelCommand(),
		newGenerateViewsCommand(),
		newGenerateControllerCommand(),
		newGenerateScaffoldCommand(),
		newGenerateJobCommand(),
	)

	setStandardHelp(cmd,
		helpCommand{
			Use:         "generate model NAME",
			Description: "generates a new model from migration",
		},
		helpCommand{
			Use:         "generate views",
			Description: "generates Go code from Templ templates",
		},
		helpCommand{
			Use:         "generate controller NAME [action ...]",
			Description: "generates a new controller",
		},
		helpCommand{
			Use:         "generate scaffold NAME",
			Description: "generates a complete scaffold resource",
		},
		helpCommand{
			Use:         "generate job NAME",
			Description: "generates a new background job",
		},
	)

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
