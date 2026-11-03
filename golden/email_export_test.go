package golden

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

// TestEmailExportMatchesCommittedTemplates runs the real React Email export in
// a generated project and requires byte-identical committed templates. It also
// checks the placeholder guard rejects a literal "{{" in authored TSX.
//
// The export needs Node, pnpm and installed frontend dependencies, so this is
// opt-in with the full goldens and skips when the install cannot run (offline
// developer machines and CI without a pnpm store).
func TestEmailExportMatchesCommittedTemplates(t *testing.T) {
	goldentest.RequireBinary(t)
	goldentest.RequireFullScaffold(t)
	for _, tool := range []string{"node", "pnpm"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not found in PATH; skipping email export comparison", tool)
		}
	}

	parent := t.TempDir()
	goldentest.RunCLIFull(t, parent, "new", "app")
	project := filepath.Join(parent, "app")

	installCtx, cancelInstall := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelInstall()
	install := exec.CommandContext(installCtx, "pnpm", "install", "--reporter=append-only")
	install.Dir = project
	install.Env = os.Environ()
	if out, err := install.CombinedOutput(); err != nil {
		t.Skipf("pnpm install unavailable, skipping email export comparison: %v\n%s", err, out)
	}

	templatePaths, err := filepath.Glob(filepath.Join(project, "emails", "templates", "*.tmpl"))
	if err != nil {
		t.Fatalf("glob email templates: %v", err)
	}
	if len(templatePaths) == 0 {
		t.Fatal("generated project has no emails/templates/*.tmpl")
	}
	before := make(map[string][]byte, len(templatePaths))
	for _, path := range templatePaths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		before[path] = content
	}

	goldentest.RunCLI(t, project, "sync", "emails")
	for path, want := range before {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read exported %s: %v", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf(
				"%s is not byte-identical to the committed template\nexport:\n%s\ncommitted:\n%s",
				filepath.Base(path),
				got,
				want,
			)
		}
	}

	// A literal "{{" anywhere in the rendered output would break Go
	// text/template, so the export must reject it before writing templates.
	layoutPath := filepath.Join(project, "emails", "components", "layout.tsx")
	layout, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Fatalf("read layout.tsx: %v", err)
	}
	guarded := strings.Replace(
		string(layout),
		"const emailStyles = `",
		"const emailStyles = `\n.email-guard::before { content: \"{{\"; }",
		1,
	)
	if guarded == string(layout) {
		t.Fatal("could not inject the literal placeholder into layout.tsx")
	}
	if err := os.WriteFile(layoutPath, []byte(guarded), 0o644); err != nil {
		t.Fatalf("write guarded layout.tsx: %v", err)
	}

	out, exitCode := goldentest.RunCLIExit(t, project, "sync", "emails")
	if exitCode == 0 {
		t.Fatalf("email export accepted a literal \"{{\":\n%s", out)
	}
	if !strings.Contains(string(out), "literal") {
		t.Fatalf("email export failed without the placeholder guard message:\n%s", out)
	}
}
