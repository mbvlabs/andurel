package upgrade

import (
	"os"
	"path/filepath"
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
