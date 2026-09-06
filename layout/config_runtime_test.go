package layout

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	layouttemplates "github.com/mbvlabs/andurel/layout/templates"
)

func TestGeneratedConfigRuntime(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated config runtime check in short mode")
	}

	root := t.TempDir()
	data := &TemplateData{ModuleName: "testapp", ProjectName: "testapp"}
	for _, name := range []string{
		"config", "helper", "app", "http", "session", "database", "queue", "telemetry", "email", "auth",
	} {
		if err := renderTemplate(
			root, "config_"+name+".tmpl", "config/"+name+".go", layouttemplates.Files, data,
		); err != nil {
			t.Fatalf("render config %s: %v", name, err)
		}
	}

	module := fmt.Sprintf(`module testapp

go %s

require (
	github.com/gosimple/slug v1.15.0
	github.com/joho/godotenv v1.5.1
	go.uber.org/fx v1.24.0
)
`, goVersion)
	for _, name := range []string{"email", "server", "storage", "validation"} {
		path, err := filepath.Abs(filepath.Join("..", "pkg", name))
		if err != nil {
			t.Fatal(err)
		}
		module += fmt.Sprintf(
			"\nrequire github.com/mbvlabs/andurel/pkg/%s v0.0.0\n"+
				"replace github.com/mbvlabs/andurel/pkg/%s => %q\n",
			name, name, filepath.ToSlash(path),
		)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module), 0o644); err != nil {
		t.Fatal(err)
	}
	probe, err := filepath.Abs(filepath.Join("testdata", "configruntime", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "-mod=mod", probe)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated config runtime check: %v\n%s", err, output)
	}
}
