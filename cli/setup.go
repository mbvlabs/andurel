package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/v2/layout"
)

// defaultPackageManager is the frontend package manager used when andurel.lock
// records none. npm ships with Node, so its install command always exists.
const defaultPackageManager = "npm"

// setupState is the first-run setup a generated project documents before any
// check can run: the tool binaries pinned in andurel.lock and the frontend
// dependencies of the selected package manager. andurel new stays offline, so
// a fresh tree is expected to be missing both; that is setup, not drift.
type setupState struct {
	MissingTools       []string
	NodeModulesMissing bool
	PackageManager     string
	Inertia            bool
}

// detectSetupState inspects rootDir without running anything. A missing or
// invalid andurel.lock is reported as an error: the state is unknown then.
func detectSetupState(rootDir string) (setupState, error) {
	lock, err := layout.ReadLockFile(rootDir)
	if err != nil {
		return setupState{}, fmt.Errorf("read %s: %w", layout.ProjectTomlName, err)
	}

	state := setupState{}
	if lock.ScaffoldConfig != nil &&
		layout.IsSupportedInertiaAdapter(lock.ScaffoldConfig.Inertia) {
		state.Inertia = true
		state.PackageManager = lock.ScaffoldConfig.PackageManager()
		if state.PackageManager == "" {
			state.PackageManager = defaultPackageManager
		}
		if _, err := os.Stat(filepath.Join(rootDir, "node_modules")); err != nil {
			state.NodeModulesMissing = true
		}
	}

	toolNames := make([]string, 0, len(lock.Tools))
	for name := range lock.Tools {
		toolNames = append(toolNames, name)
	}
	slices.Sort(toolNames)
	for _, name := range toolNames {
		if _, err := os.Stat(filepath.Join(rootDir, "bin", name)); err != nil {
			state.MissingTools = append(state.MissingTools, name)
		}
	}

	return state, nil
}

// Missing reports whether the documented setup has not run yet.
func (state setupState) Missing() bool {
	return len(state.MissingTools) > 0 || state.NodeModulesMissing
}

// InstallCommand is the exact frontend dependency command, or "" when the
// frontend dependencies are installed or the project has no frontend.
func (state setupState) InstallCommand() string {
	if !state.NodeModulesMissing {
		return ""
	}

	return state.PackageManager + " install"
}

// Fix returns the exact commands that complete setup, in documented order.
func (state setupState) Fix() string {
	commands := make([]string, 0, 2)
	if len(state.MissingTools) > 0 {
		commands = append(commands, "andurel tool sync")
	}
	if command := state.InstallCommand(); command != "" {
		commands = append(commands, command)
	}

	return strings.Join(commands, " && ")
}

// Detail summarizes what is missing, one fact per line.
func (state setupState) Detail() []string {
	details := make([]string, 0, 2)
	if len(state.MissingTools) > 0 {
		details = append(
			details,
			"missing tools: "+strings.Join(state.MissingTools, ", "),
		)
	}
	if state.NodeModulesMissing {
		details = append(details, "frontend dependencies are not installed (node_modules is missing)")
	}

	return details
}

// checkProjectSetup reports the documented first-run setup. Missing setup is a
// warning, not a failure: the project is still fresh, and the hint names the
// exact commands. verify turns the same state into its own setup step so a
// missing tool is never reported as generated-code drift.
func checkProjectSetup(rootDir string) checkResult {
	const name = "Project setup"

	state, err := detectSetupState(rootDir)
	if err != nil {
		return checkResult{
			name:    name,
			status:  statusSkip,
			message: "cannot read project setup",
			details: []string{err.Error()},
			hint:    "Fix andurel.lock first; run andurel doctor again afterwards.",
		}
	}
	if !state.Missing() {
		return checkResult{
			name:    name,
			status:  statusPass,
			message: "tool binaries and frontend dependencies are installed",
		}
	}

	return checkResult{
		name:    name,
		status:  statusWarn,
		message: "setup is incomplete",
		details: state.Detail(),
		hint:    "Run " + state.Fix() + " from the project root.",
	}
}
