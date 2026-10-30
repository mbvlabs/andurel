package layout

import "testing"

// TestGeneratedQueueAttachesTelemetryToJobContext guards beta.4 fix #3: River
// derives every job context from the context passed to Start, so the worker
// queue must attach telemetry there instead of only passing the River logger.
func TestGeneratedQueueAttachesTelemetryToJobContext(t *testing.T) {
	assertTemplateContains(t, "runtime_queue.tmpl",
		`"github.com/mbvlabs/andurel/pkg/telemetry"`,
		"telemetry *telemetry.Telemetry",
		"q.telemetry = tel",
		"q.processor.Start(q.telemetry.Context(ctx))",
	)
	assertTemplateMissing(t, "runtime_queue.tmpl", "q.processor.Start(ctx)")
}
