package layers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

func wantRule(analyzer *analysis.Analyzer, rule string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: analyzer.Name,
		Doc:  analyzer.Doc,
		Run: func(pass *analysis.Pass) (any, error) {
			original := pass.Report
			pass.Report = func(diagnostic analysis.Diagnostic) {
				diagnostic.Message = rule + ": " + diagnostic.Message
				original(diagnostic)
			}
			return analyzer.Run(pass)
		},
	}
}

func testAnalyzers() map[string]*analysis.Analyzer {
	cfg := Config{Module: "app"}
	analyzers := make(map[string]*analysis.Analyzer)
	for _, analyzer := range Analyzers(cfg) {
		analyzers[ruleIDForAnalyzer(analyzer)] = analyzer
	}
	return analyzers
}

func TestAnalyzers(t *testing.T) {
	analyzers := testAnalyzers()
	testdata := analysistest.TestData()

	cases := []struct {
		name  string
		pkg   string
		rules []string
	}{
		{name: "shape1-alias", pkg: "app/services/shape1", rules: []string{"tx-escape"}},
		{name: "shape2-method-value", pkg: "app/services/shape2", rules: []string{"tx-escape"}},
		{name: "shape3-postgres", pkg: "app/services/shape3", rules: []string{"db-handle"}},
		{name: "shape3b-indirect-postgres", pkg: "app/services/shape3b", rules: []string{"db-handle"}},
		{name: "shape4-pool", pkg: "app/services/shape4", rules: []string{"db-handle"}},
		{name: "shape5-database-sql", pkg: "app/services/shape5", rules: []string{"db-handle"}},
		{name: "embedded-postgres", pkg: "app/services/embedded", rules: []string{"db-handle"}},
		{name: "shape6-free-function", pkg: "app/services/shape6", rules: []string{"tx-escape"}},
		{name: "nested-closure-params", pkg: "app/services/nested", rules: []string{"tx-escape"}},
		{name: "typed-candidates", pkg: "app/services/candidates", rules: []string{"tx-escape"}},
		{name: "branch-joins", pkg: "app/services/branches", rules: []string{"tx-escape"}},
		{name: "wrong-transaction", pkg: "app/services/txwrong", rules: []string{"tx-escape"}},
		{name: "generic-callbacks", pkg: "app/services/generic", rules: []string{"tx-escape"}},
		{name: "package-initializer", pkg: "app/services/initops", rules: []string{"tx-escape"}},
		{name: "mixed-in-tx-contracts", pkg: "app/services/intxcontracts", rules: []string{"tx-escape"}},
		{name: "shape6-free-function", pkg: "app/services/shape6", rules: []string{"tx-escape"}},
		{name: "shape7-handler-transactor", pkg: "app/handlers/shape7", rules: []string{"tx-layer"}},
		{name: "shape8-domain-transport", pkg: "app/domains/transportprobe", rules: []string{"domain-transport"}},
		{name: "shape9-control", pkg: "app/services/shape9", rules: []string{"tx-escape"}},
		{name: "import-alias", pkg: "app/services/alias", rules: []string{"tx-escape"}},
		{name: "dot-import", pkg: "app/services/dot", rules: []string{"tx-escape"}},
		{name: "tx-public-call", pkg: "app/services/public", rules: []string{"tx-public-call"}},
		{name: "tx-unwrap", pkg: "app/services/unwrap", rules: []string{"tx-unwrap"}},
		{name: "connection-and-transaction", pkg: "app/services/connection", rules: []string{"db-handle"}},
		{name: "unanalyzable-callback", pkg: "app/services/unknown", rules: []string{"tx-escape"}},
	}
	for _, testCase := range cases {
		for _, rule := range testCase.rules {
			analyzer, ok := analyzers[rule]
			if !ok {
				t.Fatalf("no analyzer for rule %q", rule)
			}
			t.Run(testCase.name+"/"+rule, func(t *testing.T) {
				analysistest.Run(t, testdata, wantRule(analyzer, rule), testCase.pkg)
			})
		}
	}
}

func TestAnalyzersAllowBoundAndDomainCode(t *testing.T) {
	analyzers := testAnalyzers()
	testdata := analysistest.TestData()

	cleanPackages := []struct {
		name string
		pkg  string
	}{
		{name: "bound-locals", pkg: "app/services/clean"},
		{name: "mixed-in-tx-contracts", pkg: "app/services/intxcontracts"},
		{name: "domains-own-the-handle", pkg: "app/domains"},
		{name: "runtime-wiring", pkg: "app/internal/runtime"},
	}
	for _, clean := range cleanPackages {
		t.Run(clean.name, func(t *testing.T) {
			for name, analyzer := range analyzers {
				t.Run(name, func(t *testing.T) {
					analysistest.Run(t, testdata, analyzer, clean.pkg)
				})
			}
		})
	}
}

func TestIsStorageTxRequiresExactType(t *testing.T) {
	pkg := types.NewPackage(StorageImportPath, "storage")
	txName := types.NewTypeName(token.NoPos, pkg, "Tx", nil)
	tx := types.NewNamed(txName, types.NewStruct(nil, nil), nil)

	if !isStorageTx(tx) {
		t.Fatal("storage.Tx should be a transaction")
	}
	if isStorageTx(types.NewPointer(tx)) {
		t.Fatal("*storage.Tx must not satisfy the exact storage.Tx callback signature")
	}
}

func TestCatalogMatchesAnalyzers(t *testing.T) {
	seen := map[string]bool{}
	rules := LayerRules()
	analyzers := testAnalyzers()
	if len(rules) != len(analyzers) {
		t.Fatalf("catalog has %d layer rules, analyzers has %d", len(rules), len(analyzers))
	}
	for _, rule := range rules {
		if rule.ID == "" || rule.Summary == "" || rule.Fix == "" || rule.Layer == "" {
			t.Fatalf("incomplete rule: %#v", rule)
		}
		if seen[rule.ID] {
			t.Fatalf("duplicate rule id %q", rule.ID)
		}
		seen[rule.ID] = true
		if _, ok := analyzers[rule.ID]; !ok {
			t.Fatalf("catalog rule %q has no analyzer", rule.ID)
		}
	}
	for id := range analyzers {
		if !seen[id] {
			t.Fatalf("analyzer %q is not in the layer catalog", id)
		}
	}
}

func TestQueriesImportRule(t *testing.T) {
	// Go's internal-package rule makes a non-domains import of
	// app/domains/internal/queries impossible to type-check. Exercise this
	// import-only rule directly rather than weakening it for testdata.
	file, err := parser.ParseFile(
		token.NewFileSet(),
		"services/fake.go",
		"package services\n\nimport _ \"app/domains/internal/queries\"\n",
		0,
	)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	analyzers := testAnalyzers()
	analyzer, ok := analyzers["queries-import"]
	if !ok {
		t.Fatal("no queries-import analyzer")
	}
	reported := make([]analysis.Diagnostic, 0)
	pass := &analysis.Pass{
		Analyzer: analyzer,
		Files:    []*ast.File{file},
		Pkg:      types.NewPackage("app/services/fake", "services"),
		Report: func(diagnostic analysis.Diagnostic) {
			reported = append(reported, diagnostic)
		},
		ResultOf: map[*analysis.Analyzer]any{},
	}
	if _, err := analyzer.Run(pass); err != nil {
		t.Fatalf("run queries-import: %v", err)
	}
	if len(reported) != 1 {
		t.Fatalf("queries-import diagnostics = %#v, want one", reported)
	}

	// The same import inside domains is legal.
	domainsFile, err := parser.ParseFile(
		token.NewFileSet(),
		"domains/fake.go",
		"package domains\n\nimport _ \"app/domains/internal/queries\"\n",
		0,
	)
	if err != nil {
		t.Fatalf("parse domains fixture: %v", err)
	}
	reported = nil
	pass.Files = []*ast.File{domainsFile}
	pass.Pkg = types.NewPackage("app/domains", "domains")
	if _, err := analyzer.Run(pass); err != nil {
		t.Fatalf("run queries-import on domains: %v", err)
	}
	if len(reported) != 0 {
		t.Fatalf("domains import diagnostics = %#v, want none", reported)
	}
}

func TestAnalyzeFailsOnTypeErrors(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "go.mod", "module app\n\ngo 1.21\n")
	writeProjectFile(t, root, "main.go", "package main\n\nfunc main() {\n\tvar value int = \"not an int\"\n\t_ = value\n}\n")

	_, err := Analyze(root)
	if err == nil {
		t.Fatal("Analyze should fail on an ill-typed project")
	}
	if !strings.Contains(err.Error(), "type-check project") {
		t.Fatalf("error = %v, want type-check failure", err)
	}
}

func TestAnalyzeFailsOnEmptyProject(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "go.mod", "module app\n\ngo 1.21\n")

	_, err := Analyze(root)
	if err == nil {
		t.Fatal("Analyze should fail when there are no Go packages")
	}
}

func TestLoadIncludesTestFiles(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "go.mod", "module app\n\ngo 1.21\n")
	writeProjectFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	writeProjectFile(
		t,
		root,
		"main_test.go",
		"package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}\n",
	)

	project, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, pkg := range project.Packages {
		for _, file := range pkg.Files {
			if strings.HasSuffix(file, "_test.go") {
				return
			}
		}
	}
	t.Fatal("Load did not include any test files")
}

func TestAnalyzeCleanProject(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "go.mod", "module app\n\ngo 1.21\n")
	writeProjectFile(t, root, "main.go", "package main\n\nfunc main() {}\n")

	violations, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze clean project: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("clean project violations = %#v", violations)
	}
}

func TestLayerDirUsesSourceDirectoryForExternalTests(t *testing.T) {
	root := t.TempDir()
	writeStorageStubProject(t, root)
	writeProjectFile(t, root, "services/service.go", "package services\n\ntype Service struct{}\n")
	writeProjectFile(t, root, "services/external_test.go", `package services_test

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/storage"
)

func helper(ctx context.Context, t storage.Transactor) error {
	return storage.AtomicOps(ctx, t, func(ctx context.Context, tx storage.Tx) error {
		return nil
	})
}
`)

	project, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	violations, err := project.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, violation := range violations {
		if violation.Rule == "tx-layer" {
			t.Fatalf("external services test was treated as services_test: %#v", violation)
		}
	}

	sawExternalTest := false
	for _, pkg := range project.Packages {
		if strings.HasSuffix(pkg.Path, "_test") && filepath.Base(pkg.Dir) == "services" {
			sawExternalTest = true
		}
	}
	if !sawExternalTest {
		for _, pkg := range project.Packages {
			t.Logf("package id=%q path=%q name=%q fortest=%q dir=%q", pkg.ID, pkg.Path, pkg.Name, pkg.ForTest, pkg.Dir)
		}
		t.Fatal("loader did not include the external services test package")
	}
}

func TestLayerDirDoesNotAllowProductionServicesTest(t *testing.T) {
	root := t.TempDir()
	writeStorageStubProject(t, root)
	writeProjectFile(t, root, "services_test/prod.go", `package services_test

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Runner struct {
	transactor storage.Transactor
}

func (r Runner) Run(ctx context.Context) error {
	return storage.AtomicOps(ctx, r.transactor, func(ctx context.Context, tx storage.Tx) error {
		return nil
	})
}
`)

	violations, err := Analyze(root)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	for _, violation := range violations {
		if violation.Rule == "tx-layer" && violation.File == "services_test/prod.go" {
			return
		}
	}
	t.Fatalf("production services_test package escaped tx-layer: %#v", violations)
}

func writeStorageStubProject(t *testing.T, root string) {
	t.Helper()
	writeProjectFile(t, root, "go.mod", `module app

go 1.21

require github.com/mbvlabs/andurel/pkg/storage v0.0.0

replace github.com/mbvlabs/andurel/pkg/storage => ./stubs/storage
`)
	writeProjectFile(t, root, "stubs/storage/go.mod", `module github.com/mbvlabs/andurel/pkg/storage

go 1.21
`)
	writeProjectFile(t, root, "stubs/storage/storage.go", `package storage

import "context"

type Tx struct{}

type Transactor struct{}

func AtomicOps(ctx context.Context, t Transactor, fn func(context.Context, Tx) error) error {
	return fn(ctx, Tx{})
}
`)
}

func writeProjectFile(t *testing.T, root, rel, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func TestNoPackagesErrorReportsGoCommandFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module app\n\ngo 1.27.1\n\nnot-a-directive\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := noPackagesError(dir, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("error does not carry the go command's message: %v", err)
	}
}
