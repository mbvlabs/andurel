package upgrade

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout"
)

func TestShouldUpdateTool_NoDowngrade(t *testing.T) {
	tests := []struct {
		name            string
		existingVersion string
		expectedVersion string
		shouldUpdate    bool
		reason          string
	}{
		{
			name:            "should upgrade when framework version is higher",
			existingVersion: "v0.3.857",
			expectedVersion: "v0.3.960",
			shouldUpdate:    true,
			reason:          "expected > existing",
		},
		{
			name:            "should NOT downgrade when user has higher version",
			existingVersion: "v0.3.960",
			expectedVersion: "v0.3.857",
			shouldUpdate:    false,
			reason:          "expected < existing (no downgrade)",
		},
		{
			name:            "should NOT update when versions are the same",
			existingVersion: "v0.3.960",
			expectedVersion: "v0.3.960",
			shouldUpdate:    false,
			reason:          "expected == existing",
		},
		{
			name:            "should upgrade from older major version",
			existingVersion: "v0.2.100",
			expectedVersion: "v0.3.0",
			shouldUpdate:    true,
			reason:          "expected > existing (major bump)",
		},
		{
			name:            "should NOT downgrade major version",
			existingVersion: "v1.0.0",
			expectedVersion: "v0.9.999",
			shouldUpdate:    false,
			reason:          "expected < existing (no major downgrade)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := &layout.Tool{
				Source:  "github.com/example/tool",
				Version: tt.existingVersion,
			}

			expected := &layout.Tool{
				Source:  "github.com/example/tool",
				Version: tt.expectedVersion,
			}

			result := shouldUpdateTool(existing, expected)

			if result != tt.shouldUpdate {
				t.Errorf("shouldUpdateTool(%s, %s) = %v, want %v (reason: %s)",
					tt.existingVersion, tt.expectedVersion, result, tt.shouldUpdate, tt.reason)
			}
		})
	}
}

func TestShouldUpdateTool_BuiltTools(t *testing.T) {
	tests := []struct {
		name            string
		existingPath    string
		existingVersion string
		expectedPath    string
		expectedVersion string
		shouldUpdate    bool
		reason          string
	}{
		{
			name:            "should NOT update when path and version are the same",
			existingPath:    "cmd/tool/main.go",
			existingVersion: "1.0.0",
			expectedPath:    "cmd/tool/main.go",
			expectedVersion: "1.0.0",
			shouldUpdate:    false,
			reason:          "path and version match",
		},
		{
			name:            "should update when version changes",
			existingPath:    "cmd/tool/main.go",
			existingVersion: "1.0.0",
			expectedPath:    "cmd/tool/main.go",
			expectedVersion: "1.1.0",
			shouldUpdate:    true,
			reason:          "version differs",
		},
		{
			name:            "should update when path changes",
			existingPath:    "cmd/tool/main.go",
			existingVersion: "1.0.0",
			expectedPath:    "cmd/server/main.go",
			expectedVersion: "1.0.0",
			shouldUpdate:    true,
			reason:          "path differs",
		},
		{
			name:            "should update when both path and version change",
			existingPath:    "cmd/tool/main.go",
			existingVersion: "1.0.0",
			expectedPath:    "cmd/server/main.go",
			expectedVersion: "2.0.0",
			shouldUpdate:    true,
			reason:          "both path and version differ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := &layout.Tool{
				Path:    tt.existingPath,
				Version: tt.existingVersion,
			}

			expected := &layout.Tool{
				Path:    tt.expectedPath,
				Version: tt.expectedVersion,
			}

			result := shouldUpdateTool(existing, expected)

			if result != tt.shouldUpdate {
				t.Errorf("shouldUpdateTool(path=%s,v=%s -> path=%s,v=%s) = %v, want %v (reason: %s)",
					tt.existingPath, tt.existingVersion, tt.expectedPath, tt.expectedVersion,
					result, tt.shouldUpdate, tt.reason)
			}
		})
	}
}

func TestShouldUpdateTool_UsesSemverForVersionedTools(t *testing.T) {
	existing := &layout.Tool{
		Source:  "github.com/example/tool",
		Version: "v1.0.0",
	}

	expected := &layout.Tool{
		Source:  "github.com/example/tool",
		Version: "v2.0.0",
	}

	if !shouldUpdateTool(existing, expected) {
		t.Error("shouldUpdateTool should return true when expected semver is higher")
	}
}

func TestSyncToolsToFrameworkVersion_PreservesNonFrameworkTools(t *testing.T) {
	upgrader := &Upgrader{
		lock: &layout.AndurelLock{
			Version: "v0.1.0",
			Tools: map[string]*layout.Tool{
				"templ": {
					Source:  "github.com/a-h/templ",
					Version: "v0.3.900",
				},
				"my-custom-tool": {
					Source:  "github.com/acme/my-custom-tool",
					Version: "v1.2.3",
				},
			},
			ScaffoldConfig: &layout.ScaffoldConfig{
				ProjectName: "myapp",
				Database:    "postgres",
			},
		},
	}

	result, err := upgrader.syncToolsToFrameworkVersion()
	if err != nil {
		t.Fatalf("syncToolsToFrameworkVersion returned error: %v", err)
	}

	custom, ok := upgrader.lock.Tools["my-custom-tool"]
	if !ok {
		t.Fatal("expected custom tool to be preserved in lock file")
	}
	if custom.Version != "v1.2.3" {
		t.Fatalf("expected custom tool version to remain v1.2.3, got %s", custom.Version)
	}
	if custom.Source != "github.com/acme/my-custom-tool" {
		t.Fatalf("expected custom tool source to remain unchanged, got %s", custom.Source)
	}

	for _, removed := range result.Removed {
		if removed == "my-custom-tool" {
			t.Fatal("custom tool should never be removed during upgrade")
		}
	}
}

func TestSyncToolsToFrameworkVersion_PrefersHigherExistingVersion(t *testing.T) {
	upgrader := &Upgrader{
		lock: &layout.AndurelLock{
			Version: "v0.1.0",
			Tools: map[string]*layout.Tool{
				"templ": {
					Source:  "github.com/a-h/templ",
					Version: "v99.0.0",
				},
			},
			ScaffoldConfig: &layout.ScaffoldConfig{
				ProjectName: "myapp",
				Database:    "postgres",
			},
		},
	}

	result, err := upgrader.syncToolsToFrameworkVersion()
	if err != nil {
		t.Fatalf("syncToolsToFrameworkVersion returned error: %v", err)
	}

	if got := upgrader.lock.Tools["templ"].Version; got != "v99.0.0" {
		t.Fatalf("expected existing higher templ version to be preserved, got %s", got)
	}

	for _, updated := range result.Updated {
		if strings.HasPrefix(updated, "templ:") {
			t.Fatalf("templ should not be updated when existing version is higher, got update: %s", updated)
		}
	}
}

func TestSyncToolsToFrameworkVersion_InitializesMissingToolsMap(t *testing.T) {
	upgrader := &Upgrader{
		lock: &layout.AndurelLock{
			Version: "v0.1.0",
			ScaffoldConfig: &layout.ScaffoldConfig{
				ProjectName: "myapp",
				Database:    "postgres",
			},
		},
	}

	result, err := upgrader.syncToolsToFrameworkVersion()
	if err != nil {
		t.Fatalf("syncToolsToFrameworkVersion returned error: %v", err)
	}

	if upgrader.lock.Tools == nil {
		t.Fatal("expected tools map to be initialized")
	}
	if _, ok := upgrader.lock.Tools["templ"]; !ok {
		t.Fatal("expected templ to be added to missing tools map")
	}
	if _, ok := upgrader.lock.Tools["tailwindcli"]; !ok {
		t.Fatal("expected tailwindcli to be added for tailwind projects")
	}
	if len(result.Added) == 0 {
		t.Fatal("expected tools to be reported as added")
	}
}

func TestValidatePreconditions_RejectsMissingProjectDirectory(t *testing.T) {
	t.Parallel()

	upgrader := &Upgrader{
		projectRoot: filepath.Join(t.TempDir(), "missing"),
		lock:        &layout.AndurelLock{Version: "v0.1.0"},
		git:         NewGitAnalyzer(t.TempDir()),
	}

	err := upgrader.validatePreconditions()
	if err == nil {
		t.Fatal("expected missing project directory to fail preconditions")
	}
	if !strings.Contains(err.Error(), "project directory does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePreconditions_RejectsNonGitProject(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	upgrader := &Upgrader{
		projectRoot: projectRoot,
		lock:        &layout.AndurelLock{Version: "v0.1.0"},
		git:         NewGitAnalyzer(projectRoot),
	}

	err := upgrader.validatePreconditions()
	if err == nil {
		t.Fatal("expected non-git project to fail preconditions")
	}
	if !strings.Contains(err.Error(), "git validation failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePreconditions_RejectsMissingLock(t *testing.T) {
	t.Parallel()

	projectRoot := newGitUpgradeProject(t)
	upgrader := &Upgrader{
		projectRoot: projectRoot,
		git:         NewGitAnalyzer(projectRoot),
	}

	err := upgrader.validatePreconditions()
	if err == nil {
		t.Fatal("expected missing lock to fail preconditions")
	}
	if !strings.Contains(err.Error(), "andurel.lock file not found or invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePreconditions_RejectsMissingVersion(t *testing.T) {
	t.Parallel()

	projectRoot := newGitUpgradeProject(t)
	upgrader := &Upgrader{
		projectRoot: projectRoot,
		lock:        &layout.AndurelLock{},
		git:         NewGitAnalyzer(projectRoot),
	}

	err := upgrader.validatePreconditions()
	if err == nil {
		t.Fatal("expected missing lock version to fail preconditions")
	}
	if !strings.Contains(err.Error(), "lock file missing template version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncToolsToFrameworkVersion_RemovesLegacyRunTool(t *testing.T) {
	t.Parallel()

	upgrader := &Upgrader{
		lock: &layout.AndurelLock{
			Version: "v0.1.0",
			Tools: map[string]*layout.Tool{
				"run": layout.NewBuiltTool("cmd/run/main.go", "v0.1.0"),
			},
			ScaffoldConfig: &layout.ScaffoldConfig{ProjectName: "myapp"},
		},
	}

	result, err := upgrader.syncToolsToFrameworkVersion()
	if err != nil {
		t.Fatalf("syncToolsToFrameworkVersion returned error: %v", err)
	}

	if _, ok := upgrader.lock.Tools["run"]; ok {
		t.Fatal("expected legacy run tool to be removed")
	}
	if !slices.Contains(result.Removed, "run") {
		t.Fatalf("removed tools = %v, want run", result.Removed)
	}
}

func TestIsFrameworkManagedTool_RecognizesOnlyKnownFrameworkTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tool *layout.Tool
		want bool
	}{
		{name: "templ", tool: layout.NewGoTool("templ", "github.com/a-h/templ", "v0.0.1"), want: true},
		{name: "tailwindcli", tool: layout.NewBinaryTool("tailwindcli", "v0.0.1"), want: true},
		{name: "custom", tool: &layout.Tool{Source: "github.com/example/custom", Version: "v1.0.0"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isFrameworkManagedTool(tt.name, tt.tool); got != tt.want {
				t.Fatalf("isFrameworkManagedTool(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestSyncToolsToFrameworkVersion_UpdatesBuiltToolPathAndVersion(t *testing.T) {
	t.Parallel()

	upgrader := &Upgrader{
		lock: &layout.AndurelLock{
			Version: "v0.1.0",
			Tools: map[string]*layout.Tool{
				"templ": layout.NewBuiltTool("cmd/old-templ/main.go", "v0.0.1"),
			},
			ScaffoldConfig: &layout.ScaffoldConfig{ProjectName: "myapp"},
		},
	}
	expected := layout.GetExpectedTools(upgrader.lock.ScaffoldConfig)["templ"]

	result, err := upgrader.syncToolsToFrameworkVersion()
	if err != nil {
		t.Fatalf("syncToolsToFrameworkVersion returned error: %v", err)
	}

	templ := upgrader.lock.Tools["templ"]
	if templ.Path != expected.Path {
		t.Fatalf("templ path = %q, want %q", templ.Path, expected.Path)
	}
	if templ.Version != expected.Version {
		t.Fatalf("templ version = %q, want %q", templ.Version, expected.Version)
	}
	if len(result.Updated) == 0 {
		t.Fatal("expected built tool update to be reported")
	}
}

func TestSyncToolsToFrameworkVersion_RefreshesMetadataWithoutVersionChange(t *testing.T) {
	t.Parallel()

	expected := layout.GetExpectedTools(&layout.ScaffoldConfig{ProjectName: "myapp"})["templ"]
	upgrader := &Upgrader{
		lock: &layout.AndurelLock{
			Version: "v0.1.0",
			Tools: map[string]*layout.Tool{
				"templ": {
					Version: expected.Version,
					Source:  "stale/source",
				},
			},
			ScaffoldConfig: &layout.ScaffoldConfig{ProjectName: "myapp"},
		},
	}

	result, err := upgrader.syncToolsToFrameworkVersion()
	if err != nil {
		t.Fatalf("syncToolsToFrameworkVersion returned error: %v", err)
	}

	templ := upgrader.lock.Tools["templ"]
	if templ.Source != expected.Source {
		t.Fatalf("templ source = %q, want %q", templ.Source, expected.Source)
	}
	if templ.Download == nil {
		t.Fatal("expected missing download metadata to be restored")
	}
	if templ.VersionCheck == nil {
		t.Fatal("expected missing version check metadata to be restored")
	}
	for _, updated := range result.Updated {
		if strings.HasPrefix(updated, "templ:") {
			t.Fatalf("metadata-only refresh should not be reported as version update: %v", result.Updated)
		}
	}
}

func TestObsoleteManagedInternalFiles_RemovesInertiaWhenNotConfigured(t *testing.T) {
	projectRoot := t.TempDir()
	inertiaDir := filepath.Join(projectRoot, "internal", "inertia")
	if err := os.MkdirAll(inertiaDir, 0o755); err != nil {
		t.Fatalf("failed to create inertia dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inertiaDir, "render.go"), []byte("package inertia\n"), 0o644); err != nil {
		t.Fatalf("failed to write inertia file: %v", err)
	}

	upgrader := &Upgrader{
		projectRoot: projectRoot,
		lock: &layout.AndurelLock{
			ScaffoldConfig: &layout.ScaffoldConfig{},
		},
	}

	obsolete := upgrader.obsoleteManagedInternalFiles()
	if len(obsolete) != 1 {
		t.Fatalf("obsolete files = %v, want one file", obsolete)
	}
	if obsolete[0] != "internal/inertia/render.go" {
		t.Fatalf("obsolete file = %q, want internal/inertia/render.go", obsolete[0])
	}
}

func TestObsoleteManagedInternalFiles_KeepsConfiguredInertiaFiles(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	renderPath := filepath.Join(projectRoot, "internal", "inertia", "render.go")
	if err := os.MkdirAll(filepath.Dir(renderPath), 0o755); err != nil {
		t.Fatalf("failed to create inertia dir: %v", err)
	}
	if err := os.WriteFile(renderPath, []byte("package inertia\n"), 0o644); err != nil {
		t.Fatalf("failed to write inertia file: %v", err)
	}

	upgrader := &Upgrader{
		projectRoot: projectRoot,
		lock: &layout.AndurelLock{
			ScaffoldConfig: &layout.ScaffoldConfig{Inertia: "react"},
		},
	}

	if obsolete := upgrader.obsoleteManagedInternalFiles(); len(obsolete) != 0 {
		t.Fatalf("obsolete files = %v, want none", obsolete)
	}
}

func TestCleanupObsoleteBinaries_RemovesExistingBinaries(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	binDir := filepath.Join(projectRoot, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}
	runPath := filepath.Join(binDir, "run")
	if err := os.WriteFile(runPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("failed to write run binary: %v", err)
	}

	upgrader := &Upgrader{projectRoot: projectRoot}
	if err := upgrader.cleanupObsoleteBinaries([]string{"run", "missing"}); err != nil {
		t.Fatalf("cleanupObsoleteBinaries returned error: %v", err)
	}

	if _, err := os.Stat(runPath); !os.IsNotExist(err) {
		t.Fatalf("expected run binary to be removed, stat err = %v", err)
	}
}

func TestCleanupObsoleteBinaries_IgnoresMissingBinDirectory(t *testing.T) {
	t.Parallel()

	upgrader := &Upgrader{projectRoot: t.TempDir()}
	if err := upgrader.cleanupObsoleteBinaries([]string{"run"}); err != nil {
		t.Fatalf("cleanupObsoleteBinaries returned error: %v", err)
	}
}

func TestExecuteDryRun_ReportsRenderedFilesAndTools(t *testing.T) {
	t.Parallel()

	projectRoot := newGitUpgradeProject(t)
	lock := &layout.AndurelLock{
		SchemaVersion: 1,
		Version:       "v0.1.0",
		Tools: map[string]*layout.Tool{
			"templ": {Version: "v0.0.1", Path: "bin/templ", VersionCheck: &layout.VersionCheck{Args: []string{"--version"}}},
			"run":   layout.NewBuiltTool("cmd/run/main.go", "v0.1.0"),
		},
		ScaffoldConfig: &layout.ScaffoldConfig{
			ProjectName: "myapp",
			Database:    "postgres",
		},
	}
	if err := lock.WriteLockFile(projectRoot); err != nil {
		t.Fatalf("failed to write lock file: %v", err)
	}

	upgrader, err := NewUpgrader(projectRoot, UpgradeOptions{
		DryRun:        true,
		TargetVersion: "v0.2.0",
	})
	if err != nil {
		t.Fatalf("NewUpgrader returned error: %v", err)
	}

	report, err := upgrader.Execute()
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if !report.Success {
		t.Fatal("expected dry-run report to be successful")
	}
	if report.FilesReplaced == 0 {
		t.Fatal("expected dry run to report rendered framework files")
	}
	if !slices.Contains(report.RemovedTools, "run") {
		t.Fatalf("removed tools = %v, want run", report.RemovedTools)
	}
	if report.ToolsUpdated == 0 {
		t.Fatal("expected stale templ tool to be reported as updated")
	}
	persisted, err := layout.ReadLockFile(projectRoot)
	if err != nil {
		t.Fatalf("failed to reread lock file: %v", err)
	}
	if persisted.Version != "v0.1.0" {
		t.Fatalf("dry run should not rewrite lock version, got %q", persisted.Version)
	}
}

func TestExecuteDryRun_ReturnsScaffoldConfigError(t *testing.T) {
	t.Parallel()

	projectRoot := newGitUpgradeProject(t)
	lock := &layout.AndurelLock{
		SchemaVersion: 1,
		Version:       "v0.1.0",
		Tools:         map[string]*layout.Tool{},
	}
	if err := lock.WriteLockFile(projectRoot); err != nil {
		t.Fatalf("failed to write lock file: %v", err)
	}

	upgrader, err := NewUpgrader(projectRoot, UpgradeOptions{
		DryRun:        true,
		TargetVersion: "v0.2.0",
	})
	if err != nil {
		t.Fatalf("NewUpgrader returned error: %v", err)
	}

	report, err := upgrader.Execute()
	if err == nil {
		t.Fatal("expected missing scaffold config to fail dry-run execution")
	}
	if report == nil {
		t.Fatal("expected report to be returned with error")
	}
	if !strings.Contains(err.Error(), "lock file missing scaffold config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newGitUpgradeProject(t *testing.T) string {
	t.Helper()

	projectRoot := t.TempDir()
	goMod := "module github.com/example/myapp\n\ngo 1.24.0\n"
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, string(output))
	}

	return projectRoot
}
