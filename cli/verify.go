package cli

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

type verifyStep struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

type verifyReport struct {
	OK    bool         `json:"ok"`
	Steps []verifyStep `json:"steps"`
}

func newVerifyCommand() *cobra.Command {
	var withTests bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Check formatting, vet, and layer boundaries",
		Long: `Run the Andurel verification loop against the current project.

andurel verify checks, in order:
  • gofmt   — every Go file is gofmt-clean
  • go fix  — the toolchain reports no automatic rewrites
  • go vet  — the module vets clean
  • layers  — domains never import another domain, and only domains import
              domains/internal/queries

Pass --with-tests to also run go test ./... (requires a Postgres container for
domain tests). verify never rewrites files; fix reported issues and re-run.`,
		Example: `  andurel verify
  andurel verify --json
  andurel verify --with-tests`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVerify(cmd, withTests)
		},
	}
	cmd.Flags().BoolVar(&withTests, "with-tests", false, "Also run go test ./...")
	setAgentMetadata(
		cmd,
		"verification",
		"Read-only verification: gofmt, go fix, go vet, and the domains/service import boundary.",
	)
	setStandardHelp(cmd)
	return cmd
}

func runVerify(cmd *cobra.Command, withTests bool) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	report := verifyReport{OK: true}
	report.Steps = append(report.Steps, verifyGofmt(rootDir))
	report.Steps = append(report.Steps, verifyGoFix(rootDir))
	report.Steps = append(report.Steps, verifyGoVet(rootDir))
	report.Steps = append(report.Steps, verifyLayerBoundaries(rootDir))
	if withTests {
		report.Steps = append(report.Steps, verifyGoTest(rootDir))
	}
	for _, step := range report.Steps {
		if !step.OK {
			report.OK = false
		}
	}

	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if opts.Mode == output.ModeHuman {
		return renderVerifyHuman(cmd, report, opts)
	}
	return output.OK(cmd, report, verifySummary(report))
}

func renderVerifyHuman(cmd *cobra.Command, report verifyReport, opts output.Options) error {
	if opts.Quiet {
		if report.OK {
			return nil
		}
		return output.NewError(
			output.CodeError,
			"verification failed",
			output.ExitUsage,
			verifySummary(report),
		)
	}
	out := cmd.OutOrStdout()
	for _, step := range report.Steps {
		status := "ok"
		if !step.OK {
			status = "FAIL"
		}
		if _, err := fmt.Fprintf(out, "%-10s %s\n", step.Name, status); err != nil {
			return err
		}
		if step.Detail != "" {
			if _, err := fmt.Fprintln(out, step.Detail); err != nil {
				return err
			}
		}
	}
	if !report.OK {
		return output.NewError(
			output.CodeError,
			"verification failed",
			output.ExitUsage,
			"Fix the reported issues and run andurel verify again.",
		)
	}
	return nil
}

func verifySummary(report verifyReport) string {
	if report.OK {
		return "Verification passed"
	}
	failed := make([]string, 0)
	for _, step := range report.Steps {
		if !step.OK {
			failed = append(failed, step.Name)
		}
	}
	return "Verification failed: " + strings.Join(failed, ", ")
}

func verifyGofmt(rootDir string) verifyStep {
	out, err := runVerifyCommand(rootDir, "gofmt", "-l", ".")
	detail := strings.TrimSpace(out)
	if err != nil {
		return verifyStep{Name: "gofmt", OK: false, Detail: commandDetail(err, detail)}
	}
	return verifyStep{Name: "gofmt", OK: detail == "", Detail: detail}
}

func verifyGoFix(rootDir string) verifyStep {
	out, err := runVerifyCommand(rootDir, "go", "fix", "-diff", "./...")
	detail := strings.TrimSpace(out)
	if err != nil {
		return verifyStep{Name: "go fix", OK: false, Detail: commandDetail(err, detail)}
	}
	return verifyStep{Name: "go fix", OK: detail == "", Detail: detail}
}

func verifyGoVet(rootDir string) verifyStep {
	out, err := runVerifyCommand(rootDir, "go", "vet", "./...")
	detail := strings.TrimSpace(out)
	if err != nil {
		return verifyStep{Name: "go vet", OK: false, Detail: commandDetail(err, detail)}
	}
	return verifyStep{Name: "go vet", OK: true}
}

func verifyGoTest(rootDir string) verifyStep {
	out, err := runVerifyCommand(rootDir, "go", "test", "./...")
	detail := strings.TrimSpace(out)
	if err != nil {
		return verifyStep{Name: "go test", OK: false, Detail: commandDetail(err, detail)}
	}
	return verifyStep{Name: "go test", OK: true}
}

func commandDetail(err error, output string) string {
	if output == "" {
		return err.Error()
	}
	return output
}

func runVerifyCommand(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// verifyLayerBoundaries enforces the domain import rules from the RFC:
//   - only domains/** may import domains/internal/queries
//   - a domain package never imports another domain package
func verifyLayerBoundaries(rootDir string) verifyStep {
	module, _, err := readGoModMetadata(rootDir)
	if err != nil {
		return verifyStep{Name: "layers", OK: false, Detail: err.Error()}
	}

	queriesImport := module + "/domains/internal/queries"
	domainsImportPrefix := module + "/domains/"

	violations := make([]string, 0)
	walkErr := filepath.WalkDir(rootDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "bin", "node_modules", "vendor", "dist", "resources":
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: %v", rel, err))
			return nil
		}

		inDomains := strings.HasPrefix(rel, "domains/")
		owner := domainOwner(rel)
		for _, imported := range file.Imports {
			value := strings.Trim(imported.Path.Value, `"`)
			switch {
			case value == queriesImport && !inDomains:
				violations = append(
					violations,
					fmt.Sprintf("%s imports %s; only domains/** may import it", rel, queriesImport),
				)
			case strings.HasPrefix(value, domainsImportPrefix) && owner != "":
				target := strings.TrimPrefix(value, domainsImportPrefix)
				if target == owner || strings.HasPrefix(target, owner+"/") {
					continue
				}
				if strings.HasPrefix(target, "internal/") || target == "internal" {
					continue
				}
				violations = append(
					violations,
					fmt.Sprintf("%s imports %s; %s must not import another domain", rel, value, owner),
				)
			}
		}
		return nil
	})
	if walkErr != nil {
		return verifyStep{Name: "layers", OK: false, Detail: walkErr.Error()}
	}
	if len(violations) == 0 {
		return verifyStep{Name: "layers", OK: true}
	}
	return verifyStep{Name: "layers", OK: false, Detail: strings.Join(violations, "\n")}
}

// domainOwner returns the domain directory name for a project-relative path
// under domains/<name>/..., or "" when the file is not inside a domain package.
func domainOwner(rel string) string {
	if !strings.HasPrefix(rel, "domains/") {
		return ""
	}
	rest := strings.TrimPrefix(rel, "domains/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) < 2 {
		// domains/errors.go: the shared vocabulary package, not a domain.
		return ""
	}
	name := parts[0]
	if name == "internal" {
		return ""
	}
	return name
}
