package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldReactInertiaAssets(t *testing.T) {
	projectDir := t.TempDir()

	if err := Scaffold(projectDir, "testapp", "postgresql", "tailwind", "test", nil, "react", ""); err != nil {
		t.Fatalf("scaffold react inertia project: %v", err)
	}

	assertFileContains(t, projectDir, "resources/js/app.tsx", "@inertiajs/react")
	assertFileContains(t, projectDir, "resources/js/Pages/Welcome.tsx", "Inertia + React")
	assertFileContains(t, projectDir, "package.json", "@vitejs/plugin-react")
	assertFileContains(t, projectDir, "vite.config.ts", "resources/js/app.tsx")
	assertFileContains(t, projectDir, "tsconfig.json", "resources/js/**/*.tsx")
	assertFileContains(t, projectDir, "cmd/app/main.go", "internal/inertia")
	assertFileContains(t, projectDir, "router/router.go", "inertia.Middleware()")
	assertFileContains(t, projectDir, "go.mod", "github.com/romsar/gonertia")
	assertFileMissing(t, projectDir, "resources/js/app.ts")
	assertFileMissing(t, projectDir, "resources/js/Pages/Welcome.vue")
}

func assertFileContains(t *testing.T, root, relPath, want string) {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(root, relPath))
	if err != nil {
		t.Fatalf("read %s: %v", relPath, err)
	}
	if !strings.Contains(string(content), want) {
		t.Fatalf("%s does not contain %q", relPath, want)
	}
}

func assertFileMissing(t *testing.T, root, relPath string) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(root, relPath)); err == nil {
		t.Fatalf("%s exists unexpectedly", relPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", relPath, err)
	}
}
