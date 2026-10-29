package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
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
// checks need, mirroring doctor's check conditions without running generation.
func missingGenerationTools(rootDir string) []string {
	missing := make([]string, 0)
	require := func(name string) {
		if _, err := os.Stat(filepath.Join(rootDir, "bin", name)); err != nil {
			missing = append(missing, name)
		}
	}
	require("templ")
	if hasEmailCompilerInputs(rootDir) && hasEmailTemplates(rootDir) {
		require("tailwindcli")
	}
	if hasQueries, err := cmds.HasQueryFiles(rootDir); err == nil && hasQueries {
		require("narsilc")
	}
	return missing
}

// missingFrontendSetup reports whether package.json declares a typecheck
// script but node_modules has not been installed, or the package manager
// binary is unavailable.
func missingFrontendSetup(rootDir string) (bool, string) {
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
		// The route-usage check still needs node_modules? It reads the tree
		// directly, so a project without a typecheck script has no required
		// frontend setup for verify.
		return false, ""
	}
	if _, err := os.Stat(filepath.Join(rootDir, "node_modules")); err != nil {
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
