package extensions

import (
	"fmt"

	"github.com/mbvlabs/andurel/layout/cmds"
)

type QueueUI struct{}

func (e QueueUI) Name() string {
	return "queueui"
}

func (e QueueUI) Dependencies() []string {
	return []string{}
}

func (e QueueUI) Apply(ctx *Context) error {
	if ctx == nil || ctx.Data == nil {
		return fmt.Errorf("queueui: context or data is nil")
	}

	if ctx.Data.DatabaseDialect() != "postgresql" {
		return fmt.Errorf("queueui extension requires postgresql database")
	}

	builder := ctx.Builder()

	moduleName := ctx.Data.GetModuleName()

	builder.AddControllerImport(fmt.Sprintf("%s/database", moduleName))
	builder.AddControllerDependency("db", "database.Postgres")
	builder.AddControllerField("Queue", "Queue")
	builder.AddControllerConstructor("queue", "newQueue(db)")

	builder.AddRouteImport("net/http")

	if err := e.renderTemplates(ctx); err != nil {
		return fmt.Errorf("queueui: failed to render templates: %w", err)
	}

	ctx.AddPostStep(func(targetDir string) error {
		return cmds.RunSqlcGenerate(targetDir)
	})

	return nil
}

func (e QueueUI) renderTemplates(ctx *Context) error {
	templates := map[string]string{
		"controllers_queue_controller.tmpl": "controllers/queue_controller.go",
		"router_routes_queue.tmpl":          "router/routes/queue.go",
		"database_queries_queue_ui.tmpl":    "database/queries/queue_ui.sql",
		"models_queue.tmpl":                 "models/queue.go",

		"tw_views_queue_layout.tmpl":                     "views/queue_layout.templ",
		"tw_views_queue_dashboard.tmpl":                  "views/queue_dashboard.templ",
		"tw_views_queue_jobs.tmpl":                       "views/queue_jobs.templ",
		"tw_views_queue_job_detail.tmpl":                 "views/queue_job_detail.templ",
		"tw_views_queue_queues.tmpl":                     "views/queue_queues.templ",
		"tw_views_queue_queue_detail.tmpl":               "views/queue_queue_detail.templ",
		"tw_views_components_queue_job_state.tmpl":       "views/components/queue_job_state.templ",
		"tw_views_components_queue_time.tmpl":            "views/components/queue_time.templ",
		"tw_views_components_queue_pagination.tmpl":      "views/components/queue_pagination.templ",
		"tw_views_components_queue_filters.tmpl":         "views/components/queue_filters.templ",
		"tw_views_components_queue_job_row.tmpl":         "views/components/queue_job_row.templ",
		"tw_views_components_queue_actions.tmpl":         "views/components/queue_actions.templ",
		"vanilla_views_queue_layout.tmpl":                "views/queue_layout.templ",
		"vanilla_views_queue_dashboard.tmpl":             "views/queue_dashboard.templ",
		"vanilla_views_queue_jobs.tmpl":                  "views/queue_jobs.templ",
		"vanilla_views_queue_job_detail.tmpl":            "views/queue_job_detail.templ",
		"vanilla_views_queue_queues.tmpl":                "views/queue_queues.templ",
		"vanilla_views_queue_queue_detail.tmpl":          "views/queue_queue_detail.templ",
		"vanilla_views_components_queue_job_state.tmpl":  "views/components/queue_job_state.templ",
		"vanilla_views_components_queue_time.tmpl":       "views/components/queue_time.templ",
		"vanilla_views_components_queue_pagination.tmpl": "views/components/queue_pagination.templ",
		"vanilla_views_components_queue_filters.tmpl":    "views/components/queue_filters.templ",
		"vanilla_views_components_queue_job_row.tmpl":    "views/components/queue_job_row.templ",
		"vanilla_views_components_queue_actions.tmpl":    "views/components/queue_actions.templ",
	}

	for tmpl, target := range templates {
		templatePath := fmt.Sprintf("templates/queueui/%s", tmpl)
		if err := ctx.ProcessTemplate(templatePath, target, ctx.Data); err != nil {
			return fmt.Errorf("failed to process %s: %w", tmpl, err)
		}
	}

	return nil
}
