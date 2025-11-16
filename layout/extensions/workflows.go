package extensions

import (
	"fmt"
	"time"

	"github.com/mbvlabs/andurel/layout/cmds"
)

type Workflows struct{}

func (w Workflows) Name() string {
	return "workflows"
}

func (w Workflows) Apply(ctx *Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("workflows: context or data is nil")
	}

	if ctx.Data.DatabaseDialect() != "postgresql" {
		return fmt.Errorf("workflows: requires PostgreSQL database (SQLite not supported)")
	}

	builder := ctx.Builder()
	moduleName := ctx.Data.GetModuleName()

	builder.AddMainImport(fmt.Sprintf("%s/queue/workflow", moduleName))

	builder.AddMainInitialization(
		"dependencyChecker",
		"workflow.NewDependencyChecker(pool, 30*time.Second)",
		"pool",
	)

	builder.AddBackgroundWorker(
		"workflowDependencyChecker",
		"dependencyChecker.Start(ctx)",
		"dependencyChecker",
	)

	if err := w.renderTemplates(ctx); err != nil {
		return fmt.Errorf("workflows: failed to render templates: %w", err)
	}

	ctx.AddPostStep(func(targetDir string) error {
		return cmds.RunGoModTidy(targetDir)
	})

	return nil
}

func (w Workflows) Dependencies() []string {
	return nil
}

func (w Workflows) renderTemplates(ctx *Context) error {
	baseTime := time.Now()
	if ctx.NextMigrationTime != nil && !ctx.NextMigrationTime.IsZero() {
		baseTime = *ctx.NextMigrationTime
	}

	templates := map[string]string{
		"queue_workflow_workflow.tmpl":           "queue/workflow/workflow.go",
		"queue_workflow_lifecycle.tmpl":          "queue/workflow/lifecycle.go",
		"queue_workflow_lifecycle_task.tmpl":     "queue/workflow/lifecycle_task.go",
		"queue_workflow_lifecycle_types.tmpl":    "queue/workflow/lifecycle_types.go",
		"queue_workflow_lifecycle_control.tmpl":  "queue/workflow/lifecycle_control.go",
		"queue_workflow_lifecycle_internal.tmpl": "queue/workflow/lifecycle_internal.go",
		"queue_workflow_worker.tmpl":             "queue/workflow/worker.go",
		"queue_workflow_outputs.tmpl":            "queue/workflow/outputs.go",
		"queue_workflow_checker.tmpl":            "queue/workflow/checker.go",

		"database_migrations_workflow_indexes.tmpl": fmt.Sprintf(
			"database/migrations/%v_add_workflow_indexes.sql",
			baseTime.Format("20060102150405"),
		),
		"database_migrations_workflow_outputs.tmpl": fmt.Sprintf(
			"database/migrations/%v_create_workflow_outputs_table.sql",
			baseTime.Add(1*time.Second).Format("20060102150405"),
		),
	}

	for tmpl, target := range templates {
		templatePath := fmt.Sprintf("templates/workflows/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, nil); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}
