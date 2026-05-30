package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/generator/files"
	"github.com/mbvlabs/andurel/pkg/constants"
	"github.com/mbvlabs/andurel/pkg/naming"
	"github.com/spf13/cobra"
)

func newGenerateJobCommand() *cobra.Command {
	var queueName string

	cmd := &cobra.Command{
		Use:   "job NAME",
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
	pascalName := naming.ToPascalCase(name)

	// Generate queue/jobs/<snake>.go
	jobPath := filepath.Join("queue", "jobs", snakeName+".go")
	if err := generateJobFile(jobPath, modulePath, pascalName, snakeName, queueName); err != nil {
		return fmt.Errorf("failed to generate job file: %w", err)
	}

	// Generate queue/workers/<snake>.go
	workerPath := filepath.Join("queue", "workers", snakeName+".go")
	if err := generateWorkerFile(workerPath, modulePath, pascalName, snakeName); err != nil {
		return fmt.Errorf("failed to generate worker file: %w", err)
	}

	// Register worker in queue/workers/workers.go
	if err := registerWorkerInWorkersGo(pascalName); err != nil {
		return fmt.Errorf("failed to register worker: %w", err)
	}

	fmt.Printf("Successfully generated job %s\n", name)
	return nil
}

func generateJobFile(jobPath, modulePath, pascalName, snakeName, queueName string) error {
	if _, err := os.Stat(jobPath); err == nil {
		return fmt.Errorf("job file %s already exists", jobPath)
	}

	var sb strings.Builder
	sb.WriteString("package jobs\n\n")

	if queueName != "" {
		sb.WriteString("import \"github.com/riverqueue/river\"\n\n")
	}

	sb.WriteString(fmt.Sprintf("type %sArgs struct{}\n\n", pascalName))
	sb.WriteString(fmt.Sprintf("func (%sArgs) Kind() string { return %q }\n", pascalName, snakeName))

	if queueName != "" {
		sb.WriteString(fmt.Sprintf("\nfunc (%sArgs) InsertOpts() river.InsertOpts {\n", pascalName))
		sb.WriteString("\treturn river.InsertOpts{\n")
		sb.WriteString(fmt.Sprintf("\t\tQueue: %q,\n", queueName))
		sb.WriteString("\t}\n")
		sb.WriteString("}\n")
	}

	if err := os.MkdirAll(filepath.Dir(jobPath), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(jobPath, []byte(sb.String()), constants.FilePermissionPrivate); err != nil {
		return err
	}

	if err := files.FormatGoFile(jobPath); err != nil {
		return err
	}

	return nil
}

func generateWorkerFile(workerPath, modulePath, pascalName, snakeName string) error {
	if _, err := os.Stat(workerPath); err == nil {
		return fmt.Errorf("worker file %s already exists", workerPath)
	}

	var sb strings.Builder
	sb.WriteString("package workers\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"context\"\n")
	sb.WriteString("\n")
	sb.WriteString("\t\"github.com/riverqueue/river\"\n")
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("\t%q\n", modulePath+"/queue/jobs"))
	sb.WriteString(")\n\n")
	sb.WriteString(fmt.Sprintf("type %sWorker struct {\n", pascalName))
	sb.WriteString(fmt.Sprintf("\triver.WorkerDefaults[jobs.%sArgs]\n", pascalName))
	sb.WriteString("}\n\n")
	sb.WriteString(fmt.Sprintf("func New%sWorker() *%sWorker {\n", pascalName, pascalName))
	sb.WriteString(fmt.Sprintf("\treturn &%sWorker{}\n", pascalName))
	sb.WriteString("}\n\n")
	sb.WriteString(fmt.Sprintf("func (w *%sWorker) Work(ctx context.Context, job *river.Job[jobs.%sArgs]) error {\n", pascalName, pascalName))
	sb.WriteString("\t_ = ctx\n")
	sb.WriteString("\t_ = job\n")
	sb.WriteString("\treturn nil\n")
	sb.WriteString("}\n")

	if err := os.MkdirAll(filepath.Dir(workerPath), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(workerPath, []byte(sb.String()), constants.FilePermissionPrivate); err != nil {
		return err
	}

	if err := files.FormatGoFile(workerPath); err != nil {
		return err
	}

	return nil
}

func registerWorkerInWorkersGo(pascalName string) error {
	workersGoPath := filepath.Join("queue", "workers", "workers.go")
	content, err := os.ReadFile(workersGoPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", workersGoPath, err)
	}

	registrationLine := fmt.Sprintf("\n\tif err := river.AddWorkerSafely(wrks, New%sWorker()); err != nil {\n\t\treturn nil, err\n\t}\n", pascalName)

	contentStr := string(content)
	target := "return wrks, nil"
	idx := strings.LastIndex(contentStr, target)
	if idx == -1 {
		return fmt.Errorf("could not find '%s' in %s", target, workersGoPath)
	}

	newContent := contentStr[:idx] + registrationLine + contentStr[idx:]

	if err := os.WriteFile(workersGoPath, []byte(newContent), constants.FilePermissionPrivate); err != nil {
		return err
	}

	if err := files.FormatGoFile(workersGoPath); err != nil {
		return err
	}

	return nil
}
