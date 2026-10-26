package layout

import "testing"

func TestGeneratedLifecycleSeparatesServingFromStartup(t *testing.T) {
	assertTemplateContains(t, "cmd_app_main.tmpl",
		"app.Shutdown, lifecycle.WithBlockingStart()",
	)
	assertTemplateContains(t, "cmd_queue_main.tmpl",
		"lifecycle.Run(ctx, worker.Start, worker.Shutdown)",
	)
	assertTemplateMissing(t, "cmd_ssr_main.tmpl", "WithBlockingStart")
}
