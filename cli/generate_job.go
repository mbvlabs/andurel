package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/generator/templates"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
	"github.com/spf13/cobra"
)

type jobTemplateData struct {
	PascalName string
	SnakeName  string
	QueueName  string
}

type workerTemplateData struct {
	ModulePath string
	PascalName string
}

func newGenerateJobCommand() *cobra.Command {
	var queueName string

	cmd := &cobra.Command{
		Use:     "job NAME",
		Aliases: []string{"j"},
		Short: "Generate a new background job",
		Long: `Generates a new background job with the given name. Pass the job name
in CamelCase.

This creates a job argument definition in queue/jobs/ and a worker
implementation in queue/workers/, then registers the worker in
queue/workers/workers.go.

Use the --queue flag to assign the job to a specific queue. This
generates an InsertOpts method on the args struct that River uses
when inserting the job.`,
		Example: `  andurel generate job SendWelcomeEmail

      Creates a SendWelcomeEmail job and worker on the default queue.
      Job:    queue/jobs/send_welcome_email.go
      Worker: queue/workers/send_welcome_email.go

  andurel generate job ProcessPayment --queue=financial

      Creates a ProcessPayment job on the "financial" queue with an
      InsertOpts method.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) > 1 {
				return fmt.Errorf("too many arguments: job takes exactly 1 argument (the job name)")
			}
			name := args[0]

			if err := chdirToProjectRoot(); err != nil {
				return err
			}

			return withGenerateCleanup(func(_ *cobra.Command, _ []string) error {
				return generateJob(name, queueName)
			})(cmd, args)
		},
	}

	cmd.Flags().StringVar(&queueName, "queue", "", "Assign the job to a specific queue")

	return cmd
}

func generateJob(name, queueName string) error {
	modulePath, err := readModulePath()
	if err != nil {
		return fmt.Errorf("failed to read module path: %w", err)
	}

	snakeName := naming.ToSnakeCase(name)
	pascalName := naming.ToPascalCase(snakeName)

	// Generate queue/jobs/<snake>.go
	jobPath := filepath.Join("queue", "jobs", snakeName+".go")
	if err := generateFromTemplate("job.tmpl", jobPath, jobTemplateData{
		PascalName: pascalName,
		SnakeName:  snakeName,
		QueueName:  queueName,
	}); err != nil {
		return fmt.Errorf("failed to generate job file: %w", err)
	}

	// Generate queue/workers/<snake>.go
	workerPath := filepath.Join("queue", "workers", snakeName+".go")
	if err := generateFromTemplate("worker.tmpl", workerPath, workerTemplateData{
		ModulePath: modulePath,
		PascalName: pascalName,
	}); err != nil {
		return fmt.Errorf("failed to generate worker file: %w", err)
	}

	// Register worker in queue/workers/workers.go
	if err := registerWorkerInWorkersGo(pascalName); err != nil {
		return fmt.Errorf("failed to register worker: %w", err)
	}

	fmt.Printf("Successfully generated job %s\n", name)
	return nil
}

func generateFromTemplate(tmplName, outputPath string, data any) error {
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("file %s already exists", outputPath)
	}

	content, err := templates.RenderTemplateUsingGlobal(tmplName, data)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, []byte(content), constants.FilePermissionPrivate); err != nil {
		return err
	}

	return files.FormatGoFile(outputPath)
}

func registerWorkerInWorkersGo(pascalName string) error {
	workersGoPath := filepath.Join("queue", "workers", "workers.go")
	content, err := os.ReadFile(workersGoPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", workersGoPath, err)
	}

	contentStr := string(content)
	marker := "// andurel:worker-registration-point"
	if !strings.Contains(contentStr, marker) {
		printManualWorkerRegistration(pascalName)
		return nil
	}

	registrationLine := fmt.Sprintf(`
	if err := river.AddWorkerSafely(wrks, New%sWorker()); err != nil {
		return nil, err
	}

`, pascalName)

	newContent := strings.Replace(contentStr, marker, registrationLine+marker, 1)

	if err := os.WriteFile(workersGoPath, []byte(newContent), constants.FilePermissionPrivate); err != nil {
		return err
	}

	if err := files.FormatGoFile(workersGoPath); err != nil {
		return err
	}

	return nil
}

func printManualWorkerRegistration(pascalName string) {
	fmt.Printf(`
INFO: Could not find worker registration marker in queue/workers/workers.go.
Add the following line before the "return wrks, nil" statement in the Register function:

	if err := river.AddWorkerSafely(wrks, New%sWorker()); err != nil {
		return nil, err
	}

`, pascalName)
}
