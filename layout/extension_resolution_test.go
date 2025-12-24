package layout

// import (
// 	"testing"
//
// 	"github.com/mbvlabs/andurel/layout/extensions"
// )
//
// // Mock extensions for testing
// type mockExtension struct {
// 	name         string
// 	dependencies []string
// }
//
// func (m mockExtension) Name() string {
// 	return m.name
// }
//
// func (m mockExtension) Dependencies() []string {
// 	return m.dependencies
// }
//
// func (m mockExtension) Apply(ctx *extensions.Context) error {
// 	return nil
// }
//
// func TestResolveExtensions(t *testing.T) {
// 	// Register mock extensions for testing
// 	logging := mockExtension{name: "logging", dependencies: []string{}}
// 	metrics := mockExtension{name: "metrics", dependencies: []string{"logging"}}
// 	dashboard := mockExtension{name: "dashboard", dependencies: []string{"logging", "metrics"}}
//
// 	if err := extensions.Register(logging); err != nil {
// 		t.Fatalf("Failed to register logging extension: %v", err)
// 	}
// 	if err := extensions.Register(metrics); err != nil {
// 		t.Fatalf("Failed to register metrics extension: %v", err)
// 	}
// 	if err := extensions.Register(dashboard); err != nil {
// 		t.Fatalf("Failed to register dashboard extension: %v", err)
// 	}
//
// 	tests := []struct {
// 		name     string
// 		input    []string
// 		expected []string
// 		wantErr  bool
// 	}{
// 		{
// 			name:     "Single extension with no dependencies",
// 			input:    []string{"logging"},
// 			expected: []string{"logging"},
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "Extension with single dependency",
// 			input:    []string{"metrics"},
// 			expected: []string{"logging", "metrics"},
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "Extension with transitive dependencies",
// 			input:    []string{"dashboard"},
// 			expected: []string{"logging", "metrics", "dashboard"},
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "Multiple extensions in order (logging, metrics)",
// 			input:    []string{"logging", "metrics"},
// 			expected: []string{"logging", "metrics"},
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "Multiple extensions reverse order (metrics, logging)",
// 			input:    []string{"metrics", "logging"},
// 			expected: []string{"logging", "metrics"},
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "All extensions specified",
// 			input:    []string{"dashboard", "logging", "metrics"},
// 			expected: []string{"logging", "metrics", "dashboard"},
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "All extensions in different order",
// 			input:    []string{"metrics", "dashboard", "logging"},
// 			expected: []string{"logging", "metrics", "dashboard"},
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "Only top-level extension (auto-includes dependencies)",
// 			input:    []string{"dashboard"},
// 			expected: []string{"logging", "metrics", "dashboard"},
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "Duplicate requests get deduplicated",
// 			input:    []string{"logging", "logging"},
// 			expected: []string{"logging"},
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "Empty input",
// 			input:    []string{},
// 			expected: nil,
// 			wantErr:  false,
// 		},
// 		{
// 			name:     "Unknown extension",
// 			input:    []string{"nonexistent"},
// 			expected: nil,
// 			wantErr:  true,
// 		},
// 		{
// 			name:     "Empty string in input",
// 			input:    []string{""},
// 			expected: nil,
// 			wantErr:  true,
// 		},
// 		{
// 			name:     "Whitespace-only string",
// 			input:    []string{"  "},
// 			expected: nil,
// 			wantErr:  true,
// 		},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result, err := resolveExtensions(tt.input)
//
// 			if tt.wantErr {
// 				if err == nil {
// 					t.Errorf("Expected error but got none")
// 				}
// 				return
// 			}
//
// 			if err != nil {
// 				t.Errorf("Unexpected error: %v", err)
// 				return
// 			}
//
// 			if len(result) != len(tt.expected) {
// 				t.Errorf("Expected %d extensions, got %d", len(tt.expected), len(result))
// 				return
// 			}
//
// 			for i, expected := range tt.expected {
// 				if result[i].Name() != expected {
// 					t.Errorf("Position %d: expected %s, got %s", i, expected, result[i].Name())
// 				}
// 			}
// 		})
// 	}
// }
//
// func TestResolveExtensions_CircularDependency(t *testing.T) {
// 	// Create circular dependency: A -> B -> A
// 	extA := mockExtension{name: "circular-a", dependencies: []string{"circular-b"}}
// 	extB := mockExtension{name: "circular-b", dependencies: []string{"circular-a"}}
//
// 	if err := extensions.Register(extA); err != nil {
// 		t.Fatalf("Failed to register circular-a extension: %v", err)
// 	}
// 	if err := extensions.Register(extB); err != nil {
// 		t.Fatalf("Failed to register circular-b extension: %v", err)
// 	}
//
// 	_, err := resolveExtensions([]string{"circular-a"})
// 	if err == nil {
// 		t.Error("Expected circular dependency error but got none")
// 	}
//
// 	if err != nil && !contains(err.Error(), "circular dependency") {
// 		t.Errorf("Expected circular dependency error, got: %v", err)
// 	}
// }
//
// func TestResolveExtensions_SelfDependency(t *testing.T) {
// 	// Create self-referencing extension
// 	selfExt := mockExtension{name: "self-dep", dependencies: []string{"self-dep"}}
//
// 	if err := extensions.Register(selfExt); err != nil {
// 		t.Fatalf("Failed to register self-dep extension: %v", err)
// 	}
//
// 	_, err := resolveExtensions([]string{"self-dep"})
// 	if err == nil {
// 		t.Error("Expected self-dependency error but got none")
// 	}
//
// 	if err != nil && !contains(err.Error(), "depend on itself") {
// 		t.Errorf("Expected self-dependency error, got: %v", err)
// 	}
// }
//
// func TestResolveExtensions_ComplexDependencyGraph(t *testing.T) {
// 	// Create a more complex dependency graph:
// 	//   base (no deps)
// 	//   logging -> base
// 	//   database -> base
// 	//   api -> logging, database
// 	//   admin -> api, logging
//
// 	base := mockExtension{name: "base", dependencies: []string{}}
// 	logging := mockExtension{name: "logging", dependencies: []string{"base"}}
// 	database := mockExtension{name: "database", dependencies: []string{"base"}}
// 	api := mockExtension{name: "api", dependencies: []string{"logging", "database"}}
// 	admin := mockExtension{name: "admin", dependencies: []string{"api", "logging"}}
//
// 	for _, ext := range []mockExtension{base, logging, database, api, admin} {
// 		if err := extensions.Register(ext); err != nil {
// 			t.Fatalf("Failed to register %s extension: %v", ext.Name(), err)
// 		}
// 	}
//
// 	result, err := resolveExtensions([]string{"admin"})
// 	if err != nil {
// 		t.Fatalf("Unexpected error: %v", err)
// 	}
//
// 	// Verify the order satisfies all dependencies
// 	positions := make(map[string]int)
// 	for i, ext := range result {
// 		positions[ext.Name()] = i
// 	}
//
// 	// base should come before everything
// 	if positions["base"] >= positions["logging"] ||
// 		positions["base"] >= positions["database"] {
// 		t.Error("base should come before logging and database")
// 	}
//
// 	// logging and database should come before api
// 	if positions["logging"] >= positions["api"] ||
// 		positions["database"] >= positions["api"] {
// 		t.Error("logging and database should come before api")
// 	}
//
// 	// api should come before admin
// 	if positions["api"] >= positions["admin"] {
// 		t.Error("api should come before admin")
// 	}
//
// 	// logging should come before admin
// 	if positions["logging"] >= positions["admin"] {
// 		t.Error("logging should come before admin")
// 	}
// }
//
// func contains(s, substr string) bool {
// 	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
// }
//
// func containsSubstring(s, substr string) bool {
// 	for i := 0; i <= len(s)-len(substr); i++ {
// 		if s[i:i+len(substr)] == substr {
// 			return true
// 		}
// 	}
// 	return false
// }
