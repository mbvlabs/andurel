package golden

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mbvlabs/andurel/v2/internal/goldentest"
)

type probeViolation struct {
	Rule string `json:"rule"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type probeSpans struct {
	funcs   map[string][2]int
	imports map[string]int
}

const probeServiceFile = `package services

import (
	"context"
	"database/sql"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type Probe struct {
	transactor storage.Transactor
	users      domains.Users
	pg         *storage.Postgres
	sqlDB      *sql.DB
}

// Shape 1: alias escape.
func (p Probe) AliasEscape(ctx context.Context) error {
	users := p.users
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := users.Find(ctx, [16]byte{})
		return err
	})
}

// Shape 2: method value callback.
func (p Probe) MethodValue(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, p.work)
}

func (p Probe) work(ctx context.Context, tx storage.Tx) error {
	_, err := p.users.Find(ctx, [16]byte{})
	return err
}

// Shape 3: *storage.Postgres outside domains.
func (p Probe) RawSQL(ctx context.Context) error {
	_, err := p.pg.Exec(ctx, "delete from users")
	return err
}

// Shape 4: pool escape hatch.
func (p Probe) Pool(ctx context.Context) error {
	_, err := p.pg.Pool().Exec(ctx, "delete from users")
	return err
}

// Shape 5: database/sql outside domains.
func (p Probe) StdlibSQL(ctx context.Context) error {
	_, err := p.sqlDB.ExecContext(ctx, "delete from users")
	return err
}

// Shape 6: free function with a captured unbound dependency.
func PlainFunc(ctx context.Context, t storage.Transactor, users domains.Users) error {
	return storage.AtomicOps(ctx, t, func(ctx context.Context, tx storage.Tx) error {
		_, err := users.Find(ctx, [16]byte{})
		return err
	})
}

// Shape 9 (control): already caught before this workstream.
func (p Probe) Control(ctx context.Context) error {
	return storage.AtomicOps(ctx, p.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := p.users.Find(ctx, [16]byte{})
		return err
	})
}
`

const probeHandlerFile = `package handlers

import (
	"context"

	"app/domains"

	"github.com/mbvlabs/andurel/pkg/storage"
)

type ProbeHandler struct {
	transactor storage.Transactor
	users      domains.Users
}

func (h ProbeHandler) Show(ctx context.Context) error {
	return storage.AtomicOps(ctx, h.transactor, func(ctx context.Context, tx storage.Tx) error {
		_, err := h.users.InTx(tx).Find(ctx, [16]byte{})
		return err
	})
}
`

const probeDomainFile = `package domains

import (
	"context"

	"github.com/mbvlabs/andurel/pkg/anduin"
)

func ProbeHTTP(ctx context.Context) string {
	return anduin.Query(ctx, "x")
}
`

// TestVerifyLayerProbes is the real-code counterpart of the analysistest
// fixtures: a fresh scaffold verifies green, then the W4 probe files produce
// exactly the four expected rules.
func TestVerifyLayerProbes(t *testing.T) {
	goldentest.RequireBinary(t)
	goldentest.RequireFullScaffold(t)

	parent := t.TempDir()
	goldentest.RunCLIFull(t, parent, "new", "app")
	project := filepath.Join(parent, "app")
	goldentest.SeedProjectTools(t, project)

	// A fresh scaffold must remain green.
	goldentest.RunCLI(t, project, "verify", "--no-frontend", "--json")

	goldentest.WriteProjectFile(t, project, "services/probe.go", probeServiceFile)
	goldentest.WriteProjectFile(t, project, "handlers/probe.go", probeHandlerFile)
	goldentest.WriteProjectFile(t, project, "domains/probe_http.go", probeDomainFile)

	output, exitCode := goldentest.RunCLIExit(t, project, "verify", "--no-frontend", "--json")
	if exitCode != 9 {
		t.Fatalf("verify --json exited %d, want 9\n%s", exitCode, output)
	}

	var envelope struct {
		Data struct {
			Steps []struct {
				Name       string `json:"name"`
				Violations []struct {
					Rule string `json:"rule"`
					File string `json:"file"`
					Line int    `json:"line"`
				} `json:"violations"`
			} `json:"steps"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output), &envelope); err != nil {
		t.Fatalf("decode verify JSON: %v\n%s", err, output)
	}

	rules := map[string]bool{}
	violations := make([]probeViolation, 0)
	for _, step := range envelope.Data.Steps {
		for _, violation := range step.Violations {
			rules[violation.Rule] = true
			violations = append(violations, probeViolation{
				Rule: violation.Rule,
				File: violation.File,
				Line: violation.Line,
			})
		}
	}
	got := make([]string, 0, len(rules))
	for rule := range rules {
		got = append(got, rule)
	}
	sort.Strings(got)

	want := []string{"db-handle", "domain-transport", "tx-escape", "tx-layer"}
	if len(got) != len(want) {
		t.Fatalf("verify rules = %v, want %v\n%s", got, want, output)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("verify rules = %v, want %v\n%s", got, want, output)
		}
	}

	serviceSpans := parseProbeSpans(t, "services/probe.go", probeServiceFile)
	handlerSpans := parseProbeSpans(t, "handlers/probe.go", probeHandlerFile)
	domainSpans := parseProbeSpans(t, "domains/probe_http.go", probeDomainFile)

	assertRuleInFunc(t, violations, "services/probe.go", serviceSpans, "AliasEscape", "tx-escape")
	assertRuleInFunc(t, violations, "services/probe.go", serviceSpans, "work", "tx-escape")
	assertRuleInFunc(t, violations, "services/probe.go", serviceSpans, "RawSQL", "db-handle")
	assertRuleInFunc(t, violations, "services/probe.go", serviceSpans, "Pool", "db-handle")
	assertRuleAtLine(t, violations, "services/probe.go", serviceSpans.imports["database/sql"], "db-handle")
	assertRuleInFunc(t, violations, "services/probe.go", serviceSpans, "PlainFunc", "tx-escape")
	assertRuleInFunc(t, violations, "services/probe.go", serviceSpans, "Control", "tx-escape")
	assertRuleInFunc(t, violations, "handlers/probe.go", handlerSpans, "Show", "tx-layer")
	assertRuleAtLine(t, violations, "domains/probe_http.go", domainSpans.imports["github.com/mbvlabs/andurel/pkg/anduin"], "domain-transport")
}

func parseProbeSpans(t *testing.T, filename, source string) probeSpans {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	spans := probeSpans{
		funcs:   make(map[string][2]int),
		imports: make(map[string]int),
	}
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			spans.funcs[typed.Name.Name] = [2]int{
				fset.Position(typed.Pos()).Line,
				fset.Position(typed.End()).Line,
			}
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				imported, ok := spec.(*ast.ImportSpec)
				if !ok {
					continue
				}
				path := strings.Trim(imported.Path.Value, `"`)
				spans.imports[path] = fset.Position(imported.Path.Pos()).Line
			}
		}
	}
	return spans
}

func assertRuleInFunc(
	t *testing.T,
	violations []probeViolation,
	filename string,
	spans probeSpans,
	function string,
	rule string,
) {
	t.Helper()

	span, ok := spans.funcs[function]
	if !ok {
		t.Fatalf("%s has no function %s", filename, function)
	}
	for _, violation := range violations {
		if violation.File == filename && violation.Rule == rule &&
			violation.Line >= span[0] && violation.Line <= span[1] {
			return
		}
	}
	t.Fatalf("%s:%s (%d-%d) has no %s violation: %#v", filename, function, span[0], span[1], rule, violations)
}

func assertRuleAtLine(
	t *testing.T,
	violations []probeViolation,
	filename string,
	line int,
	rule string,
) {
	t.Helper()

	if line == 0 {
		t.Fatalf("%s has no import line for the assertion", filename)
	}
	for _, violation := range violations {
		if violation.File == filename && violation.Rule == rule && violation.Line == line {
			return
		}
	}
	t.Fatalf("%s:%d has no %s violation: %#v", filename, line, rule, violations)
}
