package golden

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/cli/patterns"
	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

// migrationPatternSQL is the migration pattern applied to the products table
// the domain and handler patterns use.
func migrationPatternSQL(t *testing.T) string {
	t.Helper()

	migrationPattern, ok := patterns.Get("migration")
	if !ok {
		t.Fatal("migration pattern not found")
	}
	for _, file := range migrationPattern.Files {
		if strings.HasSuffix(file.Path, ".sql") {
			return file.Content
		}
	}
	t.Fatal("migration pattern has no SQL file")

	return ""
}

// compileProjectEnv satisfies the config commands that read the environment.
// A scaffold ships .env.example, not .env.
const compileProjectEnv = `DB_KIND=postgres
DB_HOST=127.0.0.1
DB_PORT=5432
DB_NAME=app
DB_USER=postgres
DB_PASSWORD=postgres
DB_SSL_MODE=disable
`

// TestPatternsCompile writes every compile-tested pattern snippet into a fresh
// full scaffold and requires the result to vet and to compile its tests. A
// snippet that drifts from the scaffold APIs fails here, not in an agent's
// project.
func TestPatternsCompile(t *testing.T) {
	goldentest.RequireBinary(t)
	goldentest.RequireFullScaffold(t)

	parent := t.TempDir()
	goldentest.RunCLIFull(t, parent, "new", "app")
	project := filepath.Join(parent, "app")
	goldentest.SeedProjectTools(t, project)
	goldentest.WriteProjectFile(t, project, ".env", compileProjectEnv)

	writeProductsMigration(t, project)

	for _, pattern := range patterns.All() {
		for _, file := range pattern.Files {
			if !patternFileWritten(file) {
				continue
			}
			goldentest.WriteProjectFile(t, project, file.Path, file.Content)
		}
	}

	// The route pattern's path is generic (<resource>), so it is not written by
	// the loop above. Resolve it for the products example the handler pattern
	// depends on.
	writeProductsRoutes(t, project)

	goldentest.RunCLI(t, project, "sync", "queries")

	runGoCommand(t, project, "vet", "./...")
	// -run XXX_NONE compiles every test package without running a test body.
	runGoCommand(t, project, "test", "-run", "XXX_NONE", "./...")
}

func writeProductsMigration(t *testing.T, project string) {
	t.Helper()

	goldentest.RunCLI(t, project, "generate", "migration", "create_products_table")

	entries, err := os.ReadDir(filepath.Join(project, "migrations"))
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	const suffix = "_create_products_table.sql"
	matches := make([]string, 0, 1)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
			matches = append(matches, filepath.Join("migrations", entry.Name()))
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one migrations/*%s, found %v", suffix, matches)
	}

	goldentest.WriteProjectFile(t, project, filepath.ToSlash(matches[0]), migrationPatternSQL(t))
}

func writeProductsRoutes(t *testing.T, project string) {
	t.Helper()

	routePattern, ok := patterns.Get("route")
	if !ok {
		t.Fatal("route pattern not found")
	}
	for _, file := range routePattern.Files {
		if !strings.HasSuffix(file.Path, ".go") {
			continue
		}
		content := strings.NewReplacer(
			"User", "Product",
			"users", "products",
		).Replace(file.Content)
		// The handler pattern calls routes.ProductShow; fail loudly if the
		// route example stops being a users example the replacer can resolve.
		if !strings.Contains(content, "ProductShow ") || strings.Contains(strings.ToLower(content), "user") {
			t.Fatalf("route pattern no longer resolves to a products example:\n%s", content)
		}
		goldentest.WriteProjectFile(t, project, "routes/products.go", content)
		return
	}
	t.Fatal("route pattern has no Go file to resolve for products")
}

func patternFileWritten(file patterns.File) bool {
	// Go files are compile-tested; concrete SQL files are not compiled, but the
	// domain pattern's .sql is what `sync queries` turns into the queries the
	// Go example calls.
	return file.CompileTested() || (file.Concrete() && strings.HasSuffix(file.Path, ".sql"))
}

func runGoCommand(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %v: %v\n%s", args, err, out)
	}
}
