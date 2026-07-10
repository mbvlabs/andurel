package layout

import (
	"slices"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout/extensions"
)

type mockExtension struct {
	name         string
	dependencies []string
}

func (m mockExtension) Name() string {
	return m.name
}

func (m mockExtension) Dependencies() []string {
	return m.dependencies
}

func (m mockExtension) Apply(ctx *extensions.Context) error {
	return nil
}

func TestResolveExtensions(t *testing.T) {
	registerMockExtensions(t,
		mockExtension{name: "test-resolve-logging"},
		mockExtension{name: "test-resolve-metrics", dependencies: []string{"test-resolve-logging"}},
		mockExtension{name: "test-resolve-dashboard", dependencies: []string{"test-resolve-logging", "test-resolve-metrics"}},
	)

	tests := []struct {
		name     string
		input    []string
		expected []string
		wantErr  string
	}{
		{
			name:     "single extension with no dependencies",
			input:    []string{"test-resolve-logging"},
			expected: []string{"test-resolve-logging"},
		},
		{
			name:     "extension with single dependency",
			input:    []string{"test-resolve-metrics"},
			expected: []string{"test-resolve-logging", "test-resolve-metrics"},
		},
		{
			name:     "extension with transitive dependencies",
			input:    []string{"test-resolve-dashboard"},
			expected: []string{"test-resolve-logging", "test-resolve-metrics", "test-resolve-dashboard"},
		},
		{
			name:     "multiple extensions preserve dependency order",
			input:    []string{"test-resolve-metrics", "test-resolve-logging"},
			expected: []string{"test-resolve-logging", "test-resolve-metrics"},
		},
		{
			name:     "duplicate requests get deduplicated",
			input:    []string{"test-resolve-logging", "test-resolve-logging"},
			expected: []string{"test-resolve-logging"},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: nil,
		},
		{
			name:    "unknown extension",
			input:   []string{"test-resolve-nonexistent"},
			wantErr: "unknown extension",
		},
		{
			name:    "empty string in input",
			input:   []string{""},
			wantErr: "extension name cannot be empty",
		},
		{
			name:    "whitespace-only string",
			input:   []string{"  "},
			wantErr: "extension name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolveExtensions(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveExtensions failed: %v", err)
			}

			got := extensionNames(result)
			if !slices.Equal(got, tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func TestAvailableExtensionNames(t *testing.T) {
	names, err := AvailableExtensionNames()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"aws-ses", "css-components", "docker"} {
		if !slices.Contains(names, want) {
			t.Fatalf("available extensions = %v, missing %q", names, want)
		}
	}
}

func TestResolveExtensions_CircularDependency(t *testing.T) {
	registerMockExtensions(t,
		mockExtension{name: "test-cycle-a", dependencies: []string{"test-cycle-b"}},
		mockExtension{name: "test-cycle-b", dependencies: []string{"test-cycle-a"}},
	)

	_, err := resolveExtensions([]string{"test-cycle-a"})
	if err == nil || !strings.Contains(err.Error(), "circular dependency") {
		t.Fatalf("expected circular dependency error, got %v", err)
	}
}

func TestResolveExtensions_SelfDependency(t *testing.T) {
	registerMockExtensions(t,
		mockExtension{name: "test-self-dep", dependencies: []string{"test-self-dep"}},
	)

	_, err := resolveExtensions([]string{"test-self-dep"})
	if err == nil || !strings.Contains(err.Error(), "cannot depend on itself") {
		t.Fatalf("expected self dependency error, got %v", err)
	}
}

func TestResolveExtensions_ComplexDependencyGraph(t *testing.T) {
	registerMockExtensions(t,
		mockExtension{name: "test-complex-base"},
		mockExtension{name: "test-complex-logging", dependencies: []string{"test-complex-base"}},
		mockExtension{name: "test-complex-database", dependencies: []string{"test-complex-base"}},
		mockExtension{name: "test-complex-api", dependencies: []string{"test-complex-logging", "test-complex-database"}},
		mockExtension{name: "test-complex-admin", dependencies: []string{"test-complex-api", "test-complex-logging"}},
	)

	result, err := resolveExtensions([]string{"test-complex-admin"})
	if err != nil {
		t.Fatalf("resolveExtensions failed: %v", err)
	}

	positions := make(map[string]int, len(result))
	for i, ext := range result {
		positions[ext.Name()] = i
	}

	assertBefore(t, positions, "test-complex-base", "test-complex-logging")
	assertBefore(t, positions, "test-complex-base", "test-complex-database")
	assertBefore(t, positions, "test-complex-logging", "test-complex-api")
	assertBefore(t, positions, "test-complex-database", "test-complex-api")
	assertBefore(t, positions, "test-complex-api", "test-complex-admin")
	assertBefore(t, positions, "test-complex-logging", "test-complex-admin")
}

func registerMockExtensions(t *testing.T, exts ...mockExtension) {
	t.Helper()
	for _, ext := range exts {
		if err := extensions.Register(ext); err != nil {
			t.Fatalf("register %s: %v", ext.Name(), err)
		}
	}
}

func extensionNames(exts []extensions.Extension) []string {
	names := make([]string, 0, len(exts))
	for _, ext := range exts {
		names = append(names, ext.Name())
	}
	return names
}

func assertBefore(t *testing.T, positions map[string]int, before, after string) {
	t.Helper()
	beforePos, beforeOK := positions[before]
	afterPos, afterOK := positions[after]
	if !beforeOK || !afterOK {
		t.Fatalf("missing extension positions for %q or %q in %v", before, after, positions)
	}
	if beforePos >= afterPos {
		t.Fatalf("expected %s before %s, positions: %v", before, after, positions)
	}
}
