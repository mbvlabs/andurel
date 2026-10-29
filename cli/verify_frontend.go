package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/v2/layout"
)

// verifyFrontendStage runs the selected frontend rules: the package manager's
// typecheck script and the route-usage analysis. It returns the rules it
// actually executed so the caller can report selected/executed/skipped.
func verifyFrontendStage(rootDir string, runTypecheck, runRouteUsage bool) (verifyStep, []string) {
	executed := make([]string, 0, 2)
	step := verifyStepOK("frontend")
	details := make([]string, 0, 2)
	skippedChecks := 0
	if runTypecheck {
		typecheck := verifyFrontend(rootDir)
		executed = append(executed, "frontend")
		switch typecheck.Status {
		case verifyStatusFail:
			step.Status = verifyStatusFail
			step.OK = false
			step.Violations = append(step.Violations, typecheck.Violations...)
			appendVerifyDetail(&step, typecheck.Detail)
		case verifyStatusSkipped:
			skippedChecks++
			details = append(details, "typecheck: "+typecheck.Reason)
		}
	}
	if runRouteUsage {
		violations, skippedReason, applicable := analyzeRouteUsage(rootDir)
		switch {
		case !applicable:
			skippedChecks++
			details = append(details, "route-usage: "+skippedReason)
		case skippedReason != "":
			step.Warnings = append(step.Warnings, "frontend-route-usage: "+skippedReason)
		default:
			executed = append(executed, "frontend-route-usage")
			if len(violations) > 0 {
				step.Status = verifyStatusFail
				step.OK = false
				step.Violations = append(step.Violations, violations...)
			}
		}
	}
	runChecks := 0
	if runTypecheck {
		runChecks++
	}
	if runRouteUsage {
		runChecks++
	}
	if step.Status == verifyStatusOK && skippedChecks == runChecks {
		return verifyStep{
			Name:   "frontend",
			Status: verifyStatusSkipped,
			Reason: strings.Join(details, "; "),
		}, executed
	}
	if len(details) > 0 {
		step.Detail = strings.Join(append([]string{step.Detail}, details...), "; ")
		step.Detail = strings.TrimPrefix(step.Detail, "; ")
	}
	return step, executed
}

func appendVerifyDetail(step *verifyStep, detail string) {
	if strings.TrimSpace(detail) == "" {
		return
	}
	if step.Detail == "" {
		step.Detail = detail
		return
	}
	step.Detail += "\n" + detail
}

// verifyFrontend runs the package.json typecheck script with the project's
// package manager. A missing script means there is nothing to check, so the
// step is skipped rather than failed.
func verifyFrontend(rootDir string) verifyStep {
	packagePath := filepath.Join(rootDir, "package.json")
	contents, err := os.ReadFile(packagePath)
	if errors.Is(err, fs.ErrNotExist) {
		return verifyStep{
			Name:   "frontend",
			Status: verifyStatusSkipped,
			Reason: "no package.json",
		}
	}
	if err != nil {
		step := verifyStepFailed("frontend")
		step.Detail = err.Error()
		return step
	}

	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		step := verifyStepFailed("frontend")
		step.Detail = "package.json is not valid JSON: " + err.Error()
		return step
	}
	if strings.TrimSpace(manifest.Scripts["typecheck"]) == "" {
		return verifyStep{
			Name:   "frontend",
			Status: verifyStatusSkipped,
			Reason: "package.json has no typecheck script",
		}
	}

	packageManager := frontendPackageManager(rootDir)
	installFix := packageManager + " install"
	if _, err := os.Stat(filepath.Join(rootDir, "node_modules")); err != nil {
		step := verifyStepFailed("frontend")
		step.Violations = []verifyViolation{{
			Rule:    "frontend",
			File:    "package.json",
			Message: "node_modules is missing, so the typecheck script cannot run",
			Fix:     installFix,
		}}
		step.Detail = "Install frontend dependencies first: " + installFix
		return step
	}

	run := exec.Command(packageManager, "run", "typecheck")
	run.Dir = rootDir
	var output bytes.Buffer
	run.Stdout = &output
	run.Stderr = &output
	if err := run.Run(); err != nil {
		step := verifyStepFailed("frontend")
		detail := strings.TrimSpace(output.String())
		if detail == "" {
			detail = err.Error()
		}
		message := "typecheck script failed"
		fix := "fix the reported TypeScript errors and re-run " + packageManager + " run typecheck"
		if errors.Is(err, exec.ErrNotFound) {
			message = packageManager + " binary not found, so the typecheck script cannot run"
			fix = installFix
		}
		step.Detail = detail
		step.Violations = []verifyViolation{{
			Rule:    "frontend",
			File:    "package.json",
			Message: message,
			Fix:     fix,
		}}
		return step
	}
	return verifyStepOK("frontend")
}

// frontendPackageManager reads the package manager from andurel.toml. Projects
// without an Andurel lock file (or an empty value) default to pnpm, matching
// the scaffold.
func frontendPackageManager(rootDir string) string {
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil || lock.ScaffoldConfig == nil {
		return "pnpm"
	}
	if manager := lock.ScaffoldConfig.PackageManager(); manager != "" {
		return manager
	}
	return "pnpm"
}

func verifyFrontendSkipped(reason string) verifyStep {
	return verifyStep{Name: "frontend", Status: verifyStatusSkipped, Reason: reason}
}
