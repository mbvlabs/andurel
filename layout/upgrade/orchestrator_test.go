package upgrade

import (
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
