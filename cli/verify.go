package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mbvlabs/andurel/v2/cli/internal/layers"
	"github.com/mbvlabs/andurel/v2/cli/output"
	"github.com/spf13/cobra"
)

// verifyViolation is one actionable finding: which rule failed, where, and
// the exact fix an agent should apply.
type verifyViolation struct {
	Rule    string `json:"rule"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

const (
	verifyStatusOK      = "ok"
	verifyStatusFail    = "fail"
	verifyStatusSkipped = "skipped"
)

// verifySkippedPackage is a test package that has tests but ran none of them,
// for example because its TestMain exited before m.Run when Docker was
// unavailable. Reason is what the package printed.
type verifySkippedPackage struct {
	Package string `json:"package"`
	Reason  string `json:"reason"`
}

type verifyStep struct {
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	OK         bool              `json:"ok"`
	Reason     string            `json:"reason,omitempty"`
	Detail     string            `json:"detail,omitempty"`
	Violations []verifyViolation `json:"violations,omitempty"`
	// SkippedPackages lists go test packages that ran no tests. It does not
	// change the step's status: the run still passes.
	SkippedPackages []verifySkippedPackage `json:"skipped_packages,omitempty"`
}

type verifyReport struct {
	OK    bool         `json:"ok"`
	Steps []verifyStep `json:"steps"`
}

func verifyStepOK(name string) verifyStep {
	return verifyStep{Name: name, Status: verifyStatusOK, OK: true}
}

func verifyStepFailed(name string) verifyStep {
	return verifyStep{Name: name, Status: verifyStatusFail}
}

// verifyStepPassed reports whether a step lets the overall run pass. A skip is
// an explicit decision and does not fail verification; anything else must be ok.
func verifyStepPassed(step verifyStep) bool {
	return step.Status == verifyStatusOK || step.Status == verifyStatusSkipped
}

func verifyRuleHelp() string {
	var b strings.Builder
	writeRuleSection(&b, layers.Catalog())
	return b.String()
}

func newVerifyCommand() *cobra.Command {
	var withTests bool
	var skipGenerated bool
	var noFrontend bool
	cmd := &cobra.Command{
		Use:   "verify [packages...]",
		Short: "Check formatting, vet, layer boundaries, frontend, and generated code",
		Long: `Run the Andurel verification loop against the current project.

andurel verify checks, in order:
  • gofmt     — every Go file is gofmt-clean
  • go fix    — the toolchain reports no automatic rewrites
  • go vet    — the module vets clean
  • layers    — the typed persistence, transaction, and domain transport rules
  • frontend  — the package.json typecheck script passes (when defined)
  • generated — templ, email, narsilc, routes.ts, and payloads.ts output is up to date

Pass Go package patterns to scope gofmt, go fix, go vet, and layers to those
packages, for example 'andurel verify ./internal/docs/...'. The frontend and
generated steps always check the whole project.

The layers step loads the active build target, like go vet: files excluded by
GOOS, GOARCH, or build constraints are not type-checked or analyzed.

` + verifyRuleHelp() + `Pass --skip-generated to mark the generated step 'skipped'; an explicit skip
does not fail the run. Pass --no-frontend to skip the frontend typecheck.

The frontend step runs the project package manager's 'run typecheck' script
when package.json defines one; without node_modules it fails and tells you to
install dependencies. Projects without package.json or without the script show
the step as 'skipped'.

Pass --with-tests to also run go test ./... (DB-backed tests need Docker for a
Postgres container). A test package that has tests but ran none, such as one
whose TestMain exits early without Docker, does not fail the run: the go test
step stays 'ok' and lists it under skipped_packages as {package, reason}.
verify never rewrites files; fix reported issues and re-run.

Each step reports status 'ok', 'fail', or 'skipped'. Every failing step lists
violations as {rule, file, line, message, fix}. A failed verification exits
with code 9 (verification_failed); usage errors exit with code 1.`,
		Example: `  andurel verify
  andurel verify --json
  andurel verify ./internal/docs/...
  andurel verify --with-tests
  andurel verify --skip-generated
  andurel verify --no-frontend`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVerify(cmd, withTests, skipGenerated, noFrontend, args)
		},
	}
	cmd.Flags().BoolVar(&withTests, "with-tests", false, "Also run go test ./...")
	cmd.Flags().
		BoolVar(&skipGenerated, "skip-generated", false, "Skip the generated-code drift step")
	cmd.Flags().
		BoolVar(&noFrontend, "no-frontend", false, "Skip the frontend typecheck step")
	setAgentMetadata(
		cmd,
		"verification",
		"Read-only verification: gofmt, go fix, go vet, the domain persistence and transaction boundary, the package.json typecheck script, and generated-code drift. Pass Go package patterns to scope gofmt, go fix, go vet, and layers. With --with-tests, test packages that ran no tests (for example DB tests without Docker) are listed in the go test step's skipped_packages; they do not fail the run. Fails with exit code 9 and structured violations; each rule and its fix is listed in verify --help and the rules field of andurel commands --json.",
	)
	setStandardHelp(cmd)
	return cmd
}

func runVerify(
	cmd *cobra.Command,
	withTests, skipGenerated, noFrontend bool,
	patterns []string,
) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	report := verifyReport{}
	report.Steps = append(report.Steps, verifyGofmt(rootDir, patterns))
	report.Steps = append(report.Steps, verifyGoFix(rootDir, patterns))
	report.Steps = append(report.Steps, verifyGoVet(rootDir, patterns))
	report.Steps = append(report.Steps, verifyLayerBoundaries(rootDir, patterns))
	if noFrontend {
		report.Steps = append(report.Steps, verifyFrontendSkipped("--no-frontend"))
	} else {
		report.Steps = append(report.Steps, verifyFrontend(rootDir))
	}
	if skipGenerated {
		report.Steps = append(report.Steps, verifyStep{
			Name:   "generated",
			Status: verifyStatusSkipped,
			Reason: "--skip-generated",
		})
	} else {
		report.Steps = append(report.Steps, verifyGenerated(rootDir))
	}
	if withTests {
		report.Steps = append(report.Steps, verifyGoTest(rootDir))
	}
	report.OK = true
	for _, step := range report.Steps {
		if !verifyStepPassed(step) {
			report.OK = false
			break
		}
	}

	opts, err := output.ParseOptions(cmd)
	if err != nil {
		return err
	}
	if opts.Mode == output.ModeHuman && !opts.Quiet {
		if err := renderVerifyHuman(cmd, report); err != nil {
			return err
		}
	}
	if !report.OK {
		failure := output.NewError(
			output.CodeVerificationFailed,
			verifySummary(report),
			output.ExitVerification,
			"Apply each violation's fix and run andurel verify again.",
		)
		failure.Data = report
		return failure
	}
	if opts.Mode == output.ModeHuman {
		return nil
	}
	return output.OK(cmd, report, verifySummary(report))
}

func renderVerifyHuman(cmd *cobra.Command, report verifyReport) error {
	out := cmd.OutOrStdout()
	for _, step := range report.Steps {
		status := "ok"
		switch step.Status {
		case verifyStatusFail:
			status = "FAIL"
		case verifyStatusSkipped:
			status = "SKIP"
		}
		if n := len(step.SkippedPackages); n > 0 {
			noun := "packages"
			if n == 1 {
				noun = "package"
			}
			status += fmt.Sprintf(" (%d %s skipped: no tests ran)", n, noun)
		}
		if _, err := fmt.Fprintf(out, "%-10s %s\n", step.Name, status); err != nil {
			return err
		}
		for _, skipped := range step.SkippedPackages {
			if _, err := fmt.Fprintf(out, "  skipped %s: %s\n", skipped.Package, skipped.Reason); err != nil {
				return err
			}
		}
		for _, violation := range step.Violations {
			if _, err := fmt.Fprintf(out, "  %s\n", formatViolation(violation)); err != nil {
				return err
			}
		}
		if step.Reason != "" {
			if _, err := fmt.Fprintf(out, "  reason: %s\n", step.Reason); err != nil {
				return err
			}
		}
		if step.Detail != "" {
			if _, err := fmt.Fprintln(out, step.Detail); err != nil {
				return err
			}
		}
	}
	return nil
}

func formatViolation(v verifyViolation) string {
	location := v.File
	if location != "" && v.Line > 0 {
		location = fmt.Sprintf("%s:%d", v.File, v.Line)
	}
	line := fmt.Sprintf("[%s] %s", v.Rule, v.Message)
	if location != "" {
		line = fmt.Sprintf("%s: %s", location, line)
	}
	if v.Fix != "" {
		line += " — fix: " + v.Fix
	}
	return line
}

func verifySummary(report verifyReport) string {
	if report.OK {
		return "Verification passed"
	}
	failed := make([]string, 0)
	for _, step := range report.Steps {
		if !verifyStepPassed(step) {
			failed = append(failed, step.Name)
		}
	}
	return "Verification failed: " + strings.Join(failed, ", ")
}

// goPatternArgs returns the go command patterns for a scoped verify. No
// patterns means the whole module.
func goPatternArgs(patterns []string) []string {
	if len(patterns) == 0 {
		return []string{"./..."}
	}
	return patterns
}

func verifyGofmt(rootDir string, patterns []string) verifyStep {
	paths := []string{"."}
	if len(patterns) > 0 {
		dirs, err := goPackageDirs(rootDir, patterns)
		if err != nil {
			step := verifyStepFailed("gofmt")
			step.Detail = err.Error()
			return step
		}
		paths = dirs
	}

	args := append([]string{"-l"}, paths...)
	out, err := runVerifyCommand(rootDir, "gofmt", args...)
	detail := strings.TrimSpace(out)
	if err != nil {
		step := verifyStepFailed("gofmt")
		step.Detail = commandDetail(err, detail)
		return step
	}
	if detail == "" {
		return verifyStepOK("gofmt")
	}
	step := verifyStepFailed("gofmt")
	for file := range strings.SplitSeq(detail, "\n") {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		file = relativeVerifyFile(rootDir, file)
		step.Violations = append(step.Violations, verifyViolation{
			Rule:    "gofmt",
			File:    file,
			Message: "file is not gofmt-clean",
			Fix:     "gofmt -w " + file,
		})
	}
	return step
}

// goPackageDirs resolves package patterns to their source directories for
// gofmt, which accepts paths rather than package patterns.
func goPackageDirs(rootDir string, patterns []string) ([]string, error) {
	args := append([]string{"list", "-f", "{{.Dir}}"}, patterns...)
	out, err := runVerifyCommand(rootDir, "go", args...)
	if err != nil {
		return nil, fmt.Errorf("resolve package patterns: %s", strings.TrimSpace(out))
	}
	seen := map[string]bool{}
	dirs := make([]string, 0)
	for line := range strings.SplitSeq(out, "\n") {
		dir := strings.TrimSpace(line)
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("no packages matched %s", strings.Join(patterns, " "))
	}
	return dirs, nil
}

func relativeVerifyFile(rootDir, file string) string {
	if filepath.IsAbs(file) {
		if rel, err := filepath.Rel(rootDir, file); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(file)
}

func verifyGoFix(rootDir string, patterns []string) verifyStep {
	args := append([]string{"fix", "-diff"}, goPatternArgs(patterns)...)
	out, err := runVerifyCommand(rootDir, "go", args...)
	detail := strings.TrimSpace(out)
	if err != nil {
		step := verifyStepFailed("go fix")
		step.Detail = commandDetail(err, detail)
		return step
	}
	if detail == "" {
		return verifyStepOK("go fix")
	}
	step := verifyStepFailed("go fix")
	step.Detail = detail
	step.Violations = []verifyViolation{{
		Rule:    "go fix",
		Message: "go fix reports automatic rewrites",
		Fix:     "go fix " + strings.Join(goPatternArgs(patterns), " "),
	}}
	return step
}

func verifyGoVet(rootDir string, patterns []string) verifyStep {
	out, err := runGoVetPatterns(rootDir, patterns)
	detail := strings.TrimSpace(out)
	if err != nil {
		step := verifyStepFailed("go vet")
		step.Detail = commandDetail(err, detail)
		return step
	}
	return verifyStepOK("go vet")
}

func verifyGoTest(rootDir string) verifyStep {
	cmd := exec.Command("go", "test", "-json", "./...")
	cmd.Dir = rootDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	result := parseGoTestJSON(stdout.Bytes())
	if err != nil {
		step := verifyStepFailed("go test")
		step.Detail = commandDetail(err, result.failureDetail(stderr.String()))
		return step
	}
	step := verifyStepOK("go test")
	step.SkippedPackages = result.skipped
	return step
}

// goTestEvent is one line of go test -json output (cmd/test2json), including
// the build-output events the go command emits for packages that fail to build.
type goTestEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
}

type goTestPackage struct {
	ranTests bool
	failed   bool
	passed   bool
	output   []string
	pkgLines []string
}

type goTestResult struct {
	order    []string
	packages map[string]*goTestPackage
	build    []string
	other    []string
	skipped  []verifySkippedPackage
}

// parseGoTestJSON folds go test -json events per package. A package whose test
// binary passed without a single test-level run event has tests but ran none,
// typically because its TestMain exited before m.Run (for example when Docker
// is unavailable); it is reported as skipped with its own output as the reason.
func parseGoTestJSON(data []byte) goTestResult {
	result := goTestResult{packages: map[string]*goTestPackage{}}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event goTestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil || event.Action == "" {
			result.other = append(result.other, line)
			continue
		}
		if event.Action == "build-output" {
			result.build = append(result.build, strings.TrimRight(event.Output, "\n"))
			continue
		}
		if event.Package == "" {
			continue
		}
		pkg, ok := result.packages[event.Package]
		if !ok {
			pkg = &goTestPackage{}
			result.packages[event.Package] = pkg
			result.order = append(result.order, event.Package)
		}
		switch event.Action {
		case "run":
			if event.Test != "" {
				pkg.ranTests = true
			}
		case "output":
			text := strings.TrimRight(event.Output, "\n")
			pkg.output = append(pkg.output, text)
			if event.Test == "" {
				pkg.pkgLines = append(pkg.pkgLines, text)
			}
		case "fail":
			if event.Test == "" {
				pkg.failed = true
			}
		case "pass":
			if event.Test == "" {
				pkg.passed = true
			}
		}
	}

	for _, name := range result.order {
		pkg := result.packages[name]
		if !pkg.passed || pkg.failed || pkg.ranTests {
			continue
		}
		result.skipped = append(result.skipped, verifySkippedPackage{
			Package: name,
			Reason:  goTestSkipReason(name, pkg.pkgLines),
		})
	}
	slices.SortFunc(result.skipped, func(a, b verifySkippedPackage) int {
		return strings.Compare(a.Package, b.Package)
	})
	return result
}

// goTestSkipReason keeps what the package printed itself, dropping the test
// framework's own PASS and "ok <pkg>" summary lines.
func goTestSkipReason(pkg string, lines []string) string {
	reasons := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "PASS" || strings.HasPrefix(trimmed, "ok  \t"+pkg) ||
			strings.HasPrefix(trimmed, "ok "+pkg) {
			continue
		}
		reasons = append(reasons, trimmed)
	}
	if len(reasons) == 0 {
		return "no tests ran"
	}
	return strings.Join(reasons, "; ")
}

// failureDetail renders the output a developer needs for a failed go test run:
// build errors, every line from failing packages, and anything the go command
// printed outside the JSON stream.
func (r goTestResult) failureDetail(stderr string) string {
	lines := append([]string{}, r.build...)
	for _, name := range r.order {
		if pkg := r.packages[name]; pkg.failed {
			lines = append(lines, pkg.output...)
		}
	}
	lines = append(lines, r.other...)
	if text := strings.TrimSpace(stderr); text != "" {
		lines = append(lines, text)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// verifyGenerated reports generated-code drift by reusing doctor's read-only
// generation checks. A missing tool binary fails the step with a missing-tool
// violation; out-of-date output fails it with the sync command that fixes it.
func verifyGenerated(rootDir string) verifyStep {
	step := verifyStepOK("generated")
	notes := make([]string, 0)
	for _, result := range codeGenerationChecks(rootDir, false) {
		switch result.status {
		case statusFail:
			step.Status = verifyStatusFail
			step.OK = false
			step.Violations = append(step.Violations, generatedViolations(result)...)
		case statusWarn:
			if result.missingTool != "" {
				step.Status = verifyStatusFail
				step.OK = false
				step.Violations = append(step.Violations, verifyViolation{
					Rule:    "missing-tool",
					Message: result.missingTool + " binary not found",
					Fix:     "andurel tool sync",
				})
				continue
			}
			notes = append(notes, result.name+": "+result.message)
		}
	}
	step.Detail = strings.Join(notes, "\n")
	return step
}

// generatedViolations turns one failed generation check into findings. When
// regeneration lists the changed files, each finding names its source file so a
// whole-tree generated run still points at the code that owns the drift.
func generatedViolations(result checkResult) []verifyViolation {
	file, fix := generatedTarget(result)
	if len(result.changed) == 0 {
		message := result.name + ": " + result.message
		if len(result.details) > 0 {
			message += " (" + strings.Join(result.details, "; ") + ")"
		}
		return []verifyViolation{{
			Rule:    "generated",
			File:    file,
			Message: message,
			Fix:     fix,
		}}
	}

	violations := make([]verifyViolation, 0, len(result.changed))
	for _, path := range result.changed {
		path = filepath.ToSlash(path)
		violations = append(violations, verifyViolation{
			Rule:    "generated",
			File:    generatedSourceFile(result.name, path),
			Message: fmt.Sprintf("%s: %s is out of date", result.name, path),
			Fix:     fix,
		})
	}
	return violations
}

// generatedSourceFile maps a generated path back to the authored file an agent
// should edit. templ and email renderers are named after their .templ source.
func generatedSourceFile(checkName, path string) string {
	switch checkName {
	case "templ generate", "email generate":
		if before, ok := strings.CutSuffix(path, "_templ.go"); ok {
			return before + ".templ"
		}
	}
	return path
}

// generatedTarget maps a doctor generation check to the output it owns and
// the command that regenerates it.
func generatedTarget(result checkResult) (file, fix string) {
	switch result.name {
	case "templ generate":
		return "", "andurel build"
	case "email generate":
		return "email", "andurel sync email"
	case "narsilc generate":
		return "domains/internal/queries", "andurel sync queries"
	case "routes.ts":
		return "resources/js/routes.ts", "andurel sync routes"
	case "payloads.ts":
		return "resources/js/types/payloads.ts", "andurel sync payloads"
	default:
		return "", doctorHint(result)
	}
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

// runGoVet runs go vet ./... in dir and returns the combined output. It is
// the single vet implementation shared by andurel verify (the write-path
// gate) and andurel doctor (read-only diagnostics); each formats the result
// for its own report type.
func runGoVet(dir string) (string, error) {
	return runGoVetPatterns(dir, nil)
}

// runGoVetPatterns is runGoVet for scoped package patterns.
func runGoVetPatterns(dir string, patterns []string) (string, error) {
	args := append([]string{"vet"}, goPatternArgs(patterns)...)
	return runVerifyCommand(dir, "go", args...)
}
