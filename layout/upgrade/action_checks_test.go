package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeActionCheckFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func actionChecksByID(checks []ActionCheck) map[string]ActionCheck {
	byID := make(map[string]ActionCheck, len(checks))
	for _, check := range checks {
		byID[check.ID] = check
	}
	return byID
}

func TestManualActionDetectorsCoverEveryAction(t *testing.T) {
	actions, err := manualActionsForUpgrade("v2.0.0-alpha.0", "v2.0.0", true)
	if err != nil {
		t.Fatalf("manualActionsForUpgrade: %v", err)
	}
	detectors := make(map[string]bool, len(manualActionDetectors))
	for _, detector := range manualActionDetectors {
		if detectors[detector.id] {
			t.Fatalf("duplicate detector for %s", detector.id)
		}
		detectors[detector.id] = true
	}
	seen := make(map[string]bool, len(actions))
	for _, action := range actions {
		seen[action.ID] = true
		if !detectors[action.ID] {
			t.Errorf("manual action %s has no detector", action.ID)
		}
	}
	for id := range detectors {
		if !seen[id] {
			t.Errorf("detector %s does not match an emitted manual action", id)
		}
	}
}

// writeAppliedActionProject writes the markers every detector expects, so the
// all-applied table below fails when a detector's marker drifts from the
// action's instructions.
func writeAppliedActionProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeActionCheckFile(t, root, "app/runtime.go", `package app

func newRuntime() {
	_ = anduin.NewServer(
	routing.NewHosts(nil)
	routes.ViteBuild.FullURL()
	_ = isViteAssetRequest
	_ = isSafeViteAssetRequest
	_ = anduin.RequestHost(ctx)
	handlers.NewPagesHandler(renderer).Register(router)
	_ = inertia.WithSSRDefault(cfg.App.IsProduction())
}
`)
	writeActionCheckFile(t, root, "handlers/errors.go", "package handlers\n\nvar _ = anduin.ErrMethodNotAllowed\n")
	writeActionCheckFile(t, root, "handlers/api.go", "package handlers\n\nfunc (h readinessHandler) ReadinessReason() string { return \"\" }\n")
	writeActionCheckFile(t, root, "config/email.go", "package config\n\nvar _ = SMTPHost\n\nfunc senderHost() {}\n")
	writeActionCheckFile(t, root, "domains/user.sql", "-- name: Bump :exec\nUPDATE users SET session_version = session_version + 1;\n")
	writeActionCheckFile(t, root, "cmd/configcheck/main.go", "package main\n\nfunc main() {}\n")
	writeActionCheckFile(t, root, "vite.config.ts", "export default { ssr: { noExternal: true } }\n")
	writeActionCheckFile(t, root, "resources/root.templ", "package resources\n\nif data.SSR == nil {\n}\n")
	writeActionCheckFile(t, root, "resources/js/routes.ts", "export const configureRouteHostScope = () => {}\n")
	return root
}

func TestCheckManualActionsReportsApplied(t *testing.T) {
	root := writeAppliedActionProject(t)
	checks := CheckManualActions(root, "v2.0.0-beta.4", "v2.0.0-beta.4", true)
	if len(checks) != len(manualActionDetectors) {
		t.Fatalf("checks = %d, want %d", len(checks), len(manualActionDetectors))
	}
	for _, check := range checks {
		if check.Status != ActionApplied {
			t.Errorf("action %s = %s (%s), want applied", check.ID, check.Status, check.Detail)
		}
		if check.Title == "" || check.Version == "" {
			t.Errorf("action %s is missing title or version: %#v", check.ID, check)
		}
	}
}

// TestCheckManualActionsDetectsLegacyInternalRuntime keeps upgrade detection
// working for projects whose composition root predates the app/ layout.
func TestCheckManualActionsDetectsLegacyInternalRuntime(t *testing.T) {
	root := t.TempDir()
	writeActionCheckFile(t, root, "internal/runtime/runtime.go", `package runtime

func newRuntime() {
	_ = isViteAssetRequest
	_ = isSafeViteAssetRequest
}
`)
	writeActionCheckFile(t, root, "internal/configcheck/main.go", "package main\n\nfunc main() {}\n")

	checks := actionChecksByID(CheckManualActions(root, "v2.0.0-beta.4", "v2.0.0-beta.4", true))
	for _, id := range []string{
		"cors-host-origins-v2.0.0-beta.3",
		"csrf-asset-skip-v2.0.0-beta.3",
		"configcheck-v2.0.0-beta.4",
	} {
		if got := checks[id].Status; got != ActionApplied {
			t.Errorf("legacy action %s = %s, want applied", id, got)
		}
	}
}

func TestCheckManualActionsReportsNotApplied(t *testing.T) {
	root := t.TempDir()
	checks := actionChecksByID(CheckManualActions(root, "v2.0.0-beta.4", "v2.0.0-beta.4", true))

	for _, id := range []string{
		"cors-host-origins-v2.0.0-beta.3",
		"csrf-asset-skip-v2.0.0-beta.3",
		"method-not-allowed-page-v2.0.0-beta.3",
		"email-sender-v2.0.0-beta.2",
		"configcheck-v2.0.0-beta.4",
	} {
		check, ok := checks[id]
		if !ok {
			t.Fatalf("missing check %s", id)
		}
		if check.Status != ActionNotApplied {
			t.Errorf("action %s = %s, want not_applied", id, check.Status)
		}
		if check.Detail == "" {
			t.Errorf("action %s has no detector detail", id)
		}
	}

	// An empty project has no generated Pages registration and no domains
	// package, so those actions are not applicable rather than pending.
	for _, id := range []string{"public-home-v2.0.0-beta.4", "validation-validate-v2.0.0-alpha.5"} {
		if got := checks[id].Status; got != ActionNotApplicable {
			t.Errorf("action %s = %s, want not_applicable", id, got)
		}
	}

	stale := t.TempDir()
	writeActionCheckFile(t, stale, "services/service_test.go", "package services\n\nvar _ = storage.TestCluster{}\n")
	checks = actionChecksByID(CheckManualActions(stale, "v2.0.0-beta.4", "v2.0.0-beta.4", true))
	if got := checks["storage-storagetest-v2.0.0-alpha.5"].Status; got != ActionNotApplied {
		t.Fatalf("storage-storagetest = %s, want not_applied for storage.TestCluster", got)
	}
}

func TestCheckManualActionsVersionAndInertiaGates(t *testing.T) {
	root := t.TempDir()
	checks := actionChecksByID(CheckManualActions(root, "v2.0.0-beta.2", "v2.0.0-beta.4", true))

	for _, id := range []string{
		"cors-host-origins-v2.0.0-beta.3",
		"method-not-allowed-page-v2.0.0-beta.3",
		"public-home-v2.0.0-beta.4",
	} {
		check := checks[id]
		if check.Status != ActionNotApplicable {
			t.Errorf("future action %s = %s, want not_applicable", id, check.Status)
		}
		if !strings.Contains(check.Detail, "requires "+check.Version) {
			t.Errorf("future action %s detail = %q, want the required version", id, check.Detail)
		}
	}
	if _, ok := checks["vite-ssr-bundle-v2.0.0-beta.2"]; !ok {
		t.Fatalf("beta.2 action missing from a beta.2 project: %#v", checks)
	}

	nonInertia := actionChecksByID(CheckManualActions(root, "v2.0.0-beta.4", "v2.0.0-beta.4", false))
	for _, id := range []string{
		"inertia-ssr-default-v2.0.0-beta.4",
		"routes-host-scope-v2.0.0-beta.4",
	} {
		if got := nonInertia[id].Status; got != ActionNotApplicable {
			t.Errorf("inertia action %s on a non-Inertia project = %s, want not_applicable", id, got)
		}
		if got := nonInertia[id].Detail; !strings.Contains(got, "Inertia") {
			t.Errorf("inertia action %s detail = %q, want the Inertia reason", id, got)
		}
	}
}

func TestCheckManualActionsWithoutVersion(t *testing.T) {
	if checks := CheckManualActions(t.TempDir(), "", "v2.0.0-beta.4", true); len(checks) != 0 {
		t.Fatalf("checks without a project version = %#v, want none", checks)
	}
	if checks := CheckManualActions(t.TempDir(), "not-a-version", "v2.0.0-beta.4", true); len(checks) != 0 {
		t.Fatalf("checks with an invalid project version = %#v, want none", checks)
	}
}
