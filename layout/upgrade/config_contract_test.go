package upgrade

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/layout/templates"
)

func TestEditedRCConfigAddsTemplateFieldsAndPreservesIndependentDeclarations(t *testing.T) {
	current := mustReadFixture(t, "rc3", "pristine", "config/app.go")
	current = append(current, []byte("\nconst IndependentConfigValue = true\n")...)
	target, err := renderTemplateToBytes("config_app.tmpl", templates.Files, nil)
	if err != nil {
		t.Fatal(err)
	}
	spec := rcFileTransforms[0]
	transformed, conflict, err := transformEditedRCFile(current, target, "testapp", spec)
	if err != nil || conflict != "" {
		t.Fatalf("transform: conflict=%q err=%v", conflict, err)
	}
	for _, marker := range []string{
		"IndependentConfigValue",
		"SessionMaxAge        int      `env:\"SESSION_MAX_AGE\" envDefault:\"604800\"`",
		"CORSAllowedOrigins   []string `env:\"CORS_ALLOWED_ORIGINS\" envSeparator:\",\"`",
	} {
		if !bytes.Contains(transformed, []byte(marker)) {
			t.Fatalf("transformed config missing %q:\n%s", marker, transformed)
		}
	}
}

func TestEditedRCConfigRejectsConflictingAndAmbiguousFields(t *testing.T) {
	target, err := renderTemplateToBytes("config_app.tmpl", templates.Files, nil)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		mutate  func([]byte) []byte
		message string
	}{
		{
			name: "wrong type",
			mutate: func(content []byte) []byte {
				return bytes.Replace(content, []byte("TokenSigningKey      string"), []byte("SessionMaxAge        string"), 1)
			},
			message: "conflicting type or tag",
		},
		{
			name: "duplicate field",
			mutate: func(content []byte) []byte {
				return bytes.Replace(content, []byte("TokenSigningKey      string"), []byte("SessionMaxAge int\n\tSessionMaxAge int\n\tTokenSigningKey string"), 1)
			},
			message: "defined more than once",
		},
		{
			name: "duplicate struct",
			mutate: func(content []byte) []byte {
				return append(content, []byte("\ntype app struct{}\n")...)
			},
			message: "expected exactly one type app struct",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := test.mutate(mustReadFixture(t, "rc3", "pristine", "config/app.go"))
			_, conflict, transformErr := transformEditedRCFile(current, target, "testapp", rcFileTransforms[0])
			if transformErr != nil {
				t.Fatal(transformErr)
			}
			if !strings.Contains(conflict, test.message) {
				t.Fatalf("conflict = %q, want %q", conflict, test.message)
			}
		})
	}
}

func TestEditedRCConfigAcceptsOneCorrectFieldAndAddsTheOther(t *testing.T) {
	current := mustReadFixture(t, "rc3", "pristine", "config/app.go")
	current = bytes.Replace(
		current,
		[]byte("\tTokenSigningKey      string"),
		[]byte("\tSessionMaxAge        int      `env:\"SESSION_MAX_AGE\" envDefault:\"604800\"`\n\tTokenSigningKey      string"),
		1,
	)
	target, err := renderTemplateToBytes("config_app.tmpl", templates.Files, nil)
	if err != nil {
		t.Fatal(err)
	}
	transformed, conflict, err := transformEditedRCFile(current, target, "testapp", rcFileTransforms[0])
	if err != nil || conflict != "" {
		t.Fatalf("transform: conflict=%q err=%v", conflict, err)
	}
	if bytes.Count(transformed, []byte("SessionMaxAge")) != 1 || bytes.Count(transformed, []byte("CORSAllowedOrigins")) != 1 {
		t.Fatalf("required field counts are not exact:\n%s", transformed)
	}
}

func TestRouterConfigContractUsesEffectivePlannedContents(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "router"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	router := []byte("package router\n\nfunc setup() { _ = cfg.App.SessionMaxAge }\n")
	legacyConfig := mustReadFixture(t, "rc3", "pristine", "config/app.go")
	if err := os.WriteFile(filepath.Join(root, "router", "router.go"), router, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "app.go"), legacyConfig, 0o644); err != nil {
		t.Fatal(err)
	}
	plan := &migrationPlan{files: []plannedFile{{path: "router/router.go", after: router}}}
	if err := validatePlannedFiles(root, plan); err == nil || !strings.Contains(err.Error(), "config/app.go does not define app.SessionMaxAge") {
		t.Fatalf("incomplete plan validation = %v", err)
	}
}
