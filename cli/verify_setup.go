package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/v2/layout/cmds"
)

// verifySetupNeeds says which setup a run's selected stages require.
type verifySetupNeeds struct {
	GenerationTools bool
	FrontendDeps    bool
}

// verifySetup is the preflight step for F04: missing managed tools and missing
// frontend dependencies are reported here, with their exact installation
// command, before any dependent stage runs. Dependent stages are then
// reported as blocked instead of blaming generated-code drift.
func verifySetup(rootDir string, needs verifySetupNeeds) (verifyStep, []string) {
	executed := make([]string, 0)
	if !needs.GenerationTools && !needs.FrontendDeps {
		return verifyStep{
			Name:   "setup",
			Status: verifyStatusSkipped,
			Reason: "no setup requirements for the selected stages",
		}, executed
	}
	step := verifyStepOK("setup")
	if needs.GenerationTools {
		executed = append(executed, "missing-tool")
		for _, tool := range missingGenerationTools(rootDir) {
			step.Status = verifyStatusFail
			step.OK = false
			step.Violations = append(step.Violations, verifyViolation{
				Rule:    "missing-tool",
				Message: tool + " binary not found",
				Fix:     "andurel tool sync",
			})
		}
	}
	if needs.FrontendDeps {
		executed = append(executed, "setup")
		if missing, fix := missingFrontendSetup(rootDir); missing {
			step.Status = verifyStatusFail
			step.OK = false
			step.Violations = append(step.Violations, verifyViolation{
				Rule:    "setup",
				File:    "package.json",
				Message: "node_modules is missing, so the frontend typecheck and route-usage checks cannot run",
				Fix:     fix,
			})
			step.Detail = "Install frontend dependencies first: " + fix
		}
	}
	return step, executed
}

// missingGenerationTools lists the managed tools the selected generation
// checks need. Installed means the project-local bin/<name> binary exists, the
// same predicate doctor uses.
func missingGenerationTools(rootDir string) []string {
	needed := []string{"templ"}
	if hasEmailCompilerInputs(rootDir) && hasEmailTemplates(rootDir) {
		needed = append(needed, "tailwindcli")
	}
	if hasQueries, err := cmds.HasQueryFiles(rootDir); err == nil && hasQueries {
		needed = append(needed, "narsilc")
	}
	return missingToolBinaries(rootDir, needed)
}

// missingFrontendSetup reports whether package.json declares a typecheck
// script but node_modules has not been installed, or the recorded package
// manager binary is unavailable. It prefers the project's andurel.lock so the
// install command matches the adapter and package manager recorded at
// creation.
func missingFrontendSetup(rootDir string) (bool, string) {
	if state, err := detectSetupState(rootDir); err == nil && state.Inertia {
		if !state.NodeModulesMissing {
			return false, ""
		}
		install := state.PackageManager + " install"
		if _, lookErr := exec.LookPath(state.PackageManager); lookErr != nil {
			return true, state.PackageManager +
				" is not installed; install it, then run " + install
		}
		return true, install
	}

	packageManager := frontendPackageManager(rootDir)
	install := packageManager + " install"
	contents, err := os.ReadFile(filepath.Join(rootDir, "package.json"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, ""
		}
		return true, "make package.json readable, then run " + install
	}
	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return false, ""
	}
	if strings.TrimSpace(manifest.Scripts["typecheck"]) == "" {
		// The route-usage check reads the tree directly, so a project without a
		// typecheck script has no required frontend setup for verify.
		return false, ""
	}
	if _, err := os.Stat(filepath.Join(rootDir, "node_modules")); err != nil {
		if _, lookErr := exec.LookPath(packageManager); lookErr != nil {
			return true, packageManager +
				" is not installed; install it, then run " + install
		}
		return true, install
	}
	return false, ""
}

// blockedStep marks a selected stage that cannot run until setup is complete.
func blockedStep(stepName, setupDetail string) verifyStep {
	return verifyStep{
		Name:   stepName,
		Status: verifyStatusBlocked,
		Reason: "project setup is incomplete; run the commands listed in the setup step",
		Detail: setupDetail,
	}
}
