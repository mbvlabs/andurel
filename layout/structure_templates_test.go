package layout

import (
	"maps"
	"strings"
	"testing"
)

// TestScaffoldLayoutHasNoTopLevelInternal guards the beta.4 structure
// decisions: composition lives in app/, payload structs in payloads/, and
// the scaffold ships no AGENTS.md.
func TestScaffoldLayoutHasNoTopLevelInternal(t *testing.T) {
	mappings := map[TmplTarget]TmplTargetPath{}
	maps.Copy(mappings, baseTemplateMappings)
	maps.Copy(mappings, inertiaSharedTemplateMappings)

	for templateName, targetPath := range mappings {
		if strings.HasPrefix(string(targetPath), "internal/") {
			t.Fatalf(
				"template %s maps to %s; generated apps have no top-level internal/",
				templateName,
				targetPath,
			)
		}
	}
	for _, want := range []struct {
		template string
		target   string
	}{
		{template: "runtime_runtime.tmpl", target: "app/runtime.go"},
		{template: "runtime_queue.tmpl", target: "app/queue.go"},
		{template: "runtime_main_test.tmpl", target: "app/main_test.go"},
		{template: "runtime_runtime_test.tmpl", target: "app/runtime_test.go"},
		{template: "payloads_payloads.tmpl", target: "payloads/payloads.go"},
	} {
		if got := mappings[TmplTarget(want.template)]; string(got) != want.target {
			t.Fatalf("%s maps to %q, want %q", want.template, got, want.target)
		}
	}
	if _, ok := mappings["agents.tmpl"]; ok {
		t.Fatal("scaffold still registers agents.tmpl; guidance ships as CLI-owned skills")
	}
}
