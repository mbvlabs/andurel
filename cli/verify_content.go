package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/internal/content"
)

const contentManifestName = content.ManifestName

// verifyContent runs the declared content invariants. A project without an
// andurel.content.toml has nothing to check: the step is skipped and marked
// not applicable instead of claiming content was verified. selectedRules is
// the --rules subset: an empty selection runs every content rule, while a
// non-empty selection executes only those built-ins and checker declarations.
func verifyContent(rootDir string, selectedRules []string) (verifyStep, []string) {
	requested := selectedContentRules(selectedRules)
	cfg, loadDiagnostics := content.Load(rootDir)
	if cfg == nil && len(loadDiagnostics) == 0 {
		step := verifyStep{
			Name:          "content",
			Status:        verifyStatusSkipped,
			Reason:        "no content configuration (" + content.ManifestName + ")",
			NotApplicable: true,
		}
		return step, nil
	}
	step := verifyStepOK("content")
	diagnostics := loadDiagnostics
	executed := requested
	if len(diagnostics) == 0 {
		diagnostics = content.CheckSelected(rootDir, cfg, requested)
	} else {
		// Load performed configuration validation even when a rule filter was
		// given; only content-config actually ran before the failure.
		executed = []string{content.RuleConfig}
	}
	for _, diagnostic := range diagnostics {
		if len(selectedRules) > 0 && diagnostic.Rule != content.RuleConfig &&
			!containsString(selectedRules, diagnostic.Rule) {
			continue
		}
		step.Status = verifyStatusFail
		step.OK = false
		step.Violations = append(step.Violations, verifyViolation{
			Rule:    diagnostic.Rule,
			File:    diagnostic.File,
			Line:    diagnostic.Line,
			Message: diagnostic.Message,
			Fix:     diagnostic.Fix,
		})
	}
	if len(step.Violations) > 0 && step.Detail == "" {
		step.Detail = fmt.Sprintf(
			"%d content invariant violation(s) in the declared content tree",
			len(step.Violations),
		)
	}
	return step, executed
}

func selectedContentRules(selectedRules []string) []string {
	if len(selectedRules) == 0 {
		return append([]string(nil), contentRuleIDs...)
	}
	executed := make([]string, 0, len(selectedRules))
	for _, rule := range selectedRules {
		if strings.HasPrefix(rule, "content-") {
			executed = append(executed, rule)
		}
	}
	return executed
}

func containsString(values []string, want string) bool {
	return slices.Contains(values, want)
}
