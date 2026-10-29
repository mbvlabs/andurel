package cli

import (
	"go/ast"
	"path"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/internal/layers"
)

// verifyLayerBoundaries enforces the persistence, transaction, and domain
// transport boundaries with typed analyzers. The rules and their fixes live in
// cli/internal/layers so verify --help and commands --json share one catalog.
// ruleIDs restricts the analyzers to the selected catalog rules; an empty list
// runs every layer rule.
//
// Load and type errors fail the step instead of producing an empty green
// result: a project that cannot be type-checked has not been verified.
func verifyLayerBoundaries(rootDir string, patterns []string, ruleIDs []string) verifyStep {
	violations, err := layers.AnalyzePatternsRules(rootDir, patterns, ruleIDs)
	if err != nil {
		step := verifyStepFailed("layers")
		step.Detail = err.Error()
		return step
	}
	if len(violations) == 0 {
		return verifyStepOK("layers")
	}

	step := verifyStepFailed("layers")
	step.Violations = make([]verifyViolation, 0, len(violations))
	for _, violation := range violations {
		step.Violations = append(step.Violations, verifyViolation{
			Rule:    violation.Rule,
			File:    violation.File,
			Line:    violation.Line,
			Message: violation.Message,
			Fix:     violation.Fix,
		})
	}
	return step
}

// importNames maps each import's local package name to its path.
func importNames(file *ast.File) map[string]string {
	names := make(map[string]string, len(file.Imports))
	for _, imported := range file.Imports {
		value := strings.Trim(imported.Path.Value, `"`)
		name := path.Base(value)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		names[name] = value
	}
	return names
}

// localName returns the name file uses for importPath, or "" when the file
// does not import it (or imports it blank or dot).
func localName(imports map[string]string, importPath string) string {
	for name, value := range imports {
		if value == importPath && name != "_" && name != "." {
			return name
		}
	}
	return ""
}
