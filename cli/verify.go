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
	// verifyStatusBlocked marks a step that could not run because project
	// setup is incomplete. It fails the run, like a failure, but names the
	// missing setup instead of blaming generated code.
	verifyStatusBlocked = "blocked"
)

// verifySkippedPackage is a test package that has tests but ran none of them,
// for example because its TestMain exited before m.Run when Docker was
// unavailable. Reason is what the package printed.
type verifySkippedPackage struct {
	Package string `json:"package"`
	Reason  string `json:"reason"`
}

// verifySkippedTest is a test that called t.Skip. The go test step stays ok:
// the test author's explicit skip does not fail the run.
type verifySkippedTest struct {
	Package string `json:"package"`
	Test    string `json:"test"`
	Reason  string `json:"reason,omitempty"`
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
	// SkippedTests lists tests that called t.Skip. Like SkippedPackages it
	// does not change the step's status.
	SkippedTests []verifySkippedTest `json:"skipped_tests,omitempty"`
	// Packages is the per-package go test accounting. Totals counts top-level
	// tests only; subtests are reported separately so totals are not doubled.
	Packages []verifyTestPackage `json:"packages,omitempty"`
	Totals   *verifyTestTotals   `json:"totals,omitempty"`
	// Rules reports the stage's rule selection when verify runs with --only
	// or --rules. Steps without rule granularity omit it.
	Rules *verifyStepRules `json:"rules,omitempty"`
	// NotApplicable marks a skipped step that has nothing to check in this
	// project, such as a content stage with no content configuration.
	NotApplicable bool `json:"not_applicable,omitempty"`
	// Warnings are advisory findings that do not fail the step. The generated
	// step uses them for payload roots andurel sync payloads cannot resolve.
	Warnings []string `json:"warnings,omitempty"`
}

// verifyTestPackage is one go test package's outcome. Passed, Failed, and
// Skipped count top-level tests; Subtests counts nested tests separately.
type verifyTestPackage struct {
	Package         string              `json:"package"`
	Status          string              `json:"status"`
	Passed          int                 `json:"passed"`
	Failed          int                 `json:"failed"`
	Skipped         int                 `json:"skipped"`
	Subtests        *verifyTestSubtests `json:"subtests,omitempty"`
	DurationSeconds float64             `json:"duration_seconds,omitempty"`
	Cached          bool                `json:"cached,omitempty"`
	NoTestFiles     bool                `json:"no_test_files,omitempty"`
	NoTestsRan      bool                `json:"no_tests_ran,omitempty"`
	BuildFailed     bool                `json:"build_failed,omitempty"`
	SetupFailed     bool                `json:"setup_failed,omitempty"`
	TimedOut        bool                `json:"timed_out,omitempty"`
	RaceDetected    bool                `json:"race_detected,omitempty"`
	Reason          string              `json:"reason,omitempty"`
}

// verifyTestSubtests counts nested tests separately from their parents.
type verifyTestSubtests struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// verifyTestTotals sums the per-package accounting. Passed/Failed/Skipped
// count top-level tests; a parent test and its subtests are never both counted
// in the same total.
type verifyTestTotals struct {
	Packages        int     `json:"packages"`
	Passed          int     `json:"passed"`
	Failed          int     `json:"failed"`
	Skipped         int     `json:"skipped"`
	SubtestsPassed  int     `json:"subtests_passed,omitempty"`
	SubtestsFailed  int     `json:"subtests_failed,omitempty"`
	SubtestsSkipped int     `json:"subtests_skipped,omitempty"`
	NoTestFiles     int     `json:"no_test_files,omitempty"`
	NoTestsRan      int     `json:"no_tests_ran,omitempty"`
	CachedPackages  int     `json:"cached_packages,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
}

// verifyStepRules records the rule selection applied to one stage.
type verifyStepRules struct {
	Selected []string `json:"selected"`
	Executed []string `json:"executed"`
	Skipped  []string `json:"skipped,omitempty"`
}

type verifyReport struct {
	OK    bool         `json:"ok"`
	Steps []verifyStep `json:"steps"`
	Scope verifyScope  `json:"scope"`
}

func verifyStepOK(name string) verifyStep {
	return verifyStep{Name: name, Status: verifyStatusOK, OK: true}
}

func verifyStepFailed(name string) verifyStep {
	return verifyStep{Name: name, Status: verifyStatusFail}
}

// verifyStepPassed reports whether a step lets the overall run pass. A skip is
// an explicit decision and does not fail verification; a blocked step fails
// because the project setup it needs is incomplete.
func verifyStepPassed(step verifyStep) bool {
	return step.Status == verifyStatusOK || step.Status == verifyStatusSkipped
}

func verifyRuleHelp() string {
	var b strings.Builder
	writeRuleSection(&b, layers.Catalog())
	return b.String()
}

func newVerifyCommand() *cobra.Command {
	var options verifyOptions
	cmd := &cobra.Command{
		Use:   "verify [packages...]",
		Short: "Check formatting, vet, layer boundaries, frontend, content, and generated code",
		Long: `Run the Andurel verification loop against the current project.

andurel verify checks, in order:
  • setup     — managed tool binaries and frontend dependencies are installed
  • gofmt     — every Go file is gofmt-clean
  • go fix    — the toolchain reports no automatic rewrites
  • go vet    — the module vets clean
  • layers    — the typed persistence, transaction, and domain transport rules
  • frontend  — the package.json typecheck script and the route-usage rule
  • generated — templ, email, narsilc, routes.ts, and payloads.ts output is up to date
  • content   — the invariants declared in andurel.content.toml
  • go test   — go test -json (only with --with-tests or --only go-test)

Payload roots that andurel sync payloads cannot resolve are listed under the
generated step's warnings; they do not fail verify. Run andurel sync payloads
--strict to turn them into errors. verify never rewrites files.

Scope:
  --only <stage[,stage]>   run only the named stages. Stage IDs: setup, gofmt,
                           go-fix, go-vet, layers, frontend, generated,
                           content, go-test. Unknown names are usage errors.
  --rules <id[,id]>        run only the named catalog rules; a rule selects
                           its owning stage. Rule IDs are listed below.
  --changed                check only changed files. The default base is HEAD
                           (staged, unstaged, and untracked changes). Deleted
                           and renamed paths are included. A bad base, a
                           non-Git project, or --changed together with Go
                           package patterns is a usage error, never an empty
                           green run.
  --base <ref>             base revision for --changed (requires --changed).

In --changed mode Go stages cover the changed packages and their in-module
reverse dependencies, so an exported change also checks its dependents. A
change to go.mod, go.sum, go.work, go.work.sum, andurel.toml, andurel.lock, or
bin/ widens Go stages to ./... . The frontend, generated, and content stages
always check the whole project when they run; the report records that widening
under scope.widened. An empty change set is an explicit no-op: every selected
stage is skipped, scope.noop is true, and the run does not claim whole-project
health.

Pass Go package patterns to scope gofmt, go fix, go vet, layers, and go test,
for example 'andurel verify ./internal/docs/...'. Package patterns cannot be
combined with --changed.

The layers step loads the active build target, like go vet: files excluded by
GOOS, GOARCH, or build constraints are not type-checked or analyzed.

` + verifyRuleHelp() + `The setup step fails with the exact installation command when a selected
stage needs a missing binary or node_modules. Dependent stages then report
status 'blocked' (which fails the run) instead of blaming generated-code
drift.

The frontend stage runs the project package manager's 'run typecheck' script
when package.json defines one and analyzes frontend route usage. The
route-usage rule flags full-URL route helper calls evaluated at module
initialization, because configureRouteHosts runs after boot. It is token-based:
relative ESM imports and tsconfig path aliases, namespace/named imports,
module-scope aliases and destructuring, immediately invoked function
expressions and bare top-level calls, and one level of local wrapper functions
are recognized; re-exports, dynamic property access, and cross-module wrappers
are not. Render-time calls, event handlers, function bodies, and explicit .path()
relative helpers are allowed. The Inertia runtime's request-scoped host context
remains authoritative for the cases static analysis cannot prove.

The content stage reads andurel.content.toml (schemaVersion = 1):
  include/exclude  root-relative Markdown globs; '*' stays in one segment,
                   '**' crosses segments
  required         paths that must exist
  catalogFile      optional TOML file with [[entry]] slug/file tables
  [[catalog]]      inline slug/file entries
  [slugs]          pattern (default '^[a-z0-9]+(?:-[a-z0-9]+)*$'; empty disables)
  [links]          anchors = "ignore"|"check"; external must stay "ignore"
  [[checkers]]     name, run = [argv...], optional dir, timeoutSeconds

Content built-ins validate catalog targets, duplicate and pattern-violating
slugs, required paths, and local relative links/images (case-sensitive,
project-root confined). External URLs, site-absolute '/...' paths, fenced code,
and inline code spans are not checked; the network is never used. A [[checkers]]
entry runs its argv without a shell from dir (default project root, bounded to
the project) with a bounded timeout (default 120s, max 600s); exit 0 passes and
nonzero output lines shaped 'path:line: message' become file/line diagnostics.
Projects without andurel.content.toml report the content stage as skipped and
not_applicable, never as checked.

Pass --skip-generated to mark the generated step 'skipped'; an explicit skip
does not fail the run. Pass --no-frontend to skip the frontend stage.

Pass --with-tests to also run go test -json ./... (or the selected Go packages).
DB-backed tests need Docker for a Postgres container. A test package that has
tests but ran none, such as one whose TestMain exits early without Docker, does
not fail the run: the go test step stays 'ok' and lists it under
skipped_packages as {package, reason}. Tests that call t.Skip are listed under
skipped_tests as {package, test, reason}; they keep the step 'ok' too. The step
reports per-package status, top-level pass/fail/skip counts, subtests
separately, duration, cache/no-test-files information, and totals. Build,
setup, timeout, and race failures are marked explicitly.

Each step reports status 'ok', 'fail', 'blocked', or 'skipped'. Every failing
step lists violations as {rule, file, line, message, fix}. A failed or blocked
verification exits with code 9 (verification_failed); usage errors exit with
code 1.`,
		Example: `  andurel verify
  andurel verify --json
  andurel verify ./internal/docs/...
  andurel verify --with-tests
  andurel verify --only layers,frontend
  andurel verify --rules db-handle,frontend-route-usage
  andurel verify --changed
  andurel verify --changed --base origin/main
  andurel verify --skip-generated
  andurel verify --no-frontend`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.patterns = args
			return runVerify(cmd, options)
		},
	}
	cmd.Flags().BoolVar(&options.withTests, "with-tests", false, "Also run go test -json")
	cmd.Flags().
		BoolVar(&options.skipGenerated, "skip-generated", false, "Skip the generated-code drift stage")
	cmd.Flags().
		BoolVar(&options.noFrontend, "no-frontend", false, "Skip the frontend typecheck and route-usage stage")
	cmd.Flags().StringSliceVar(
		&options.only,
		"only",
		nil,
		"Run only these stages (comma-separated or repeated; see --help)",
	)
	cmd.Flags().StringSliceVar(
		&options.rules,
		"rules",
		nil,
		"Run only these catalog rules (comma-separated or repeated; see --help)",
	)
	cmd.Flags().BoolVar(
		&options.changed,
		"changed",
		false,
		"Check changed files (tracked staged/unstaged, untracked, deletions, renames)",
	)
	cmd.Flags().StringVar(
		&options.base,
		"base",
		verifyDefaultBase,
		"Git revision compared by --changed",
	)
	setAgentMetadata(
		cmd,
		"verification",
		"Read-only verification: setup preflight, gofmt, go fix, go vet, the domain persistence and transaction boundary, the package.json typecheck plus frontend route-usage rule, generated-code drift, and declared content invariants. Scope with Go package patterns, --only stages, --rules catalog rules, or --changed [--base ref]; changed mode follows changed packages and their reverse dependencies and records the applied scope and widening in JSON. Missing tools or node_modules fail the setup step with the exact install command and make dependent steps blocked, never generated drift. With --with-tests, the go test step reports per-package status, top-level pass/fail/skip counts, subtests separately, totals, skipped_packages, and skipped_tests with reasons; neither skip fails the run. Content invariants come from andurel.content.toml and include catalog/slug/required/local-link checks plus declared argv checkers; projects without a declaration are not_applicable. Fails with exit code 9 and structured violations; each rule and its fix is listed in verify --help and the rules field of andurel commands --json.",
	)
	setStandardHelp(cmd)
	return cmd
}

// verifyOptions is one verify invocation's parsed flags.
type verifyOptions struct {
	withTests     bool
	skipGenerated bool
	noFrontend    bool
	only          []string
	rules         []string
	changed       bool
	base          string
	patterns      []string
}

func runVerify(cmd *cobra.Command, options verifyOptions) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}
	if strings.TrimSpace(options.base) != "" && options.base != verifyDefaultBase && !options.changed {
		return output.NewError(
			output.CodeUsage,
			"verify --base requires --changed",
			output.ExitUsage,
			"Run 'andurel verify --changed --base <ref>', or omit --base.",
		)
	}
	selection, err := parseVerifySelection(options.only, options.rules)
	if err != nil {
		return err
	}
	selection = applyVerifyFlagSelection(selection, options)
	plan, err := buildVerifyPlan(rootDir, selection, options.changed, options.base, options.patterns)
	if err != nil {
		return err
	}

	runner := &verifyRunner{
		rootDir:   rootDir,
		plan:      plan,
		selection: selection,
		options:   options,
	}
	runner.run()

	report := verifyReport{Steps: runner.steps, Scope: plan.scope}
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

// applyVerifyFlagSelection folds the legacy --with-tests/--skip-generated/
// --no-frontend flags into the stage selection.
func applyVerifyFlagSelection(selection verifySelection, options verifyOptions) verifySelection {
	if options.skipGenerated {
		selection = selection.without(verifyStageGenerated)
	}
	if options.noFrontend {
		selection = selection.without(verifyStageFrontend)
	}
	// go test is opt-in: --with-tests, or an explicit --only go-test. Plain
	// verify never runs the project's test suite.
	explicitTests := slices.Contains(splitVerifyNames(options.only), verifyStageGoTest)
	if options.withTests || explicitTests {
		selection = selection.with(verifyStageGoTest)
	} else {
		selection = selection.without(verifyStageGoTest)
	}
	return selection
}

// verifyRunner executes the selected stages in canonical order.
type verifyRunner struct {
	rootDir   string
	plan      verifyPlan
	selection verifySelection
	options   verifyOptions
	steps     []verifyStep
	setupFail string
	// toolsMissing and frontendMissing are the two independent setup
	// prerequisites, so one missing prerequisite does not block an unrelated
	// stage.
	toolsMissing    bool
	frontendMissing bool
}

func (r *verifyRunner) run() {
	r.runSetup()
	r.runStage(verifyStageGofmt, r.runGofmt)
	r.runStage(verifyStageGoFix, r.runGoFix)
	r.runStage(verifyStageGoVet, r.runGoVet)
	r.runStage(verifyStageLayers, r.runLayers)
	r.runStage(verifyStageFrontend, r.runFrontend)
	r.runStage(verifyStageGenerated, r.runGenerated)
	r.runStage(verifyStageContent, r.runContent)
	r.runStage(verifyStageGoTest, r.runGoTest)
}

func (r *verifyRunner) runSetup() {
	needs := verifySetupNeeds{
		GenerationTools: r.plan.run[verifyStageGenerated],
		FrontendDeps:    r.plan.run[verifyStageFrontend],
	}
	// An explicit --only setup (or --rules setup/missing-tool) asks for the
	// project's full setup preflight, not only the prerequisites of stages
	// that happen to be selected with it. Being part of the default selection
	// is not an explicit request.
	explicitSetup := slices.Contains(splitVerifyNames(r.options.only), verifyStageSetup)
	for _, rule := range splitVerifyNames(r.options.rules) {
		if rule == "setup" || rule == "missing-tool" {
			explicitSetup = true
		}
	}
	if explicitSetup && !r.plan.run[verifyStageGenerated] && !r.plan.run[verifyStageFrontend] {
		needs = verifySetupNeeds{GenerationTools: true, FrontendDeps: true}
	}
	step, executed := verifySetup(r.rootDir, needs)
	if step.Status == verifyStatusFail {
		fixes := make([]string, 0, len(step.Violations))
		for _, violation := range step.Violations {
			switch violation.Rule {
			case "missing-tool":
				r.toolsMissing = true
			case "setup":
				r.frontendMissing = true
			}
			if violation.Fix != "" && !slices.Contains(fixes, violation.Fix) {
				fixes = append(fixes, violation.Fix)
			}
		}
		r.setupFail = strings.Join(fixes, "; ")
	}
	stage, _ := verifyStageByID(verifyStageSetup)
	step.Rules = r.selection.stageRules(stage, executed)
	r.steps = append(r.steps, step)
}

// runStage appends a skipped step when the stage is not selected or has no
// work in the applied scope; otherwise it executes the stage function.
func (r *verifyRunner) runStage(id string, run func() verifyStep) {
	stage, ok := verifyStageByID(id)
	if !ok {
		return
	}
	if !stageSelected(r.selection.StageIDs, id) {
		reason := "not selected by --only/--rules"
		switch {
		case id == verifyStageGenerated && r.options.skipGenerated:
			reason = "--skip-generated"
		case id == verifyStageFrontend && r.options.noFrontend:
			reason = "--no-frontend"
		case id == verifyStageGoTest && !r.options.withTests:
			reason = "--with-tests not set"
		}
		r.steps = append(r.steps, verifyStep{Name: stage.StepName, Status: verifyStatusSkipped, Reason: reason})
		return
	}
	if !r.plan.run[id] {
		reason := r.plan.skipReasons[id]
		if reason == "" {
			reason = "no work in scope"
		}
		r.steps = append(r.steps, verifyStep{Name: stage.StepName, Status: verifyStatusSkipped, Reason: reason})
		return
	}
	r.steps = append(r.steps, run())
}

func (r *verifyRunner) ruleSelected(rule string) bool {
	return !r.selection.ruleFiltered || slices.Contains(r.selection.RuleIDs, rule)
}

func (r *verifyRunner) stageRuleIDs(executed []string, id string) *verifyStepRules {
	stage, ok := verifyStageByID(id)
	if !ok {
		return nil
	}
	return r.selection.stageRules(stage, executed)
}

func (r *verifyRunner) runGofmt() verifyStep {
	if !r.ruleSelected("gofmt") {
		return verifyStep{Name: "gofmt", Status: verifyStatusSkipped, Reason: "gofmt rule not selected"}
	}
	var step verifyStep
	switch {
	case r.plan.changed && r.plan.goWhole:
		step = verifyGofmt(r.rootDir, nil)
	case r.plan.changed && len(r.plan.goFiles) > 0:
		step = verifyGofmtPaths(r.rootDir, r.plan.goFiles)
	case r.plan.changed:
		return verifyStep{
			Name:   "gofmt",
			Status: verifyStatusSkipped,
			Reason: "no changed Go files remain to format (deletions and renames are scoped to the owning packages)",
		}
	default:
		step = verifyGofmt(r.rootDir, r.options.patterns)
	}
	step.Rules = r.stageRuleIDs([]string{"gofmt"}, verifyStageGofmt)
	return step
}

func (r *verifyRunner) runGoFix() verifyStep {
	if !r.ruleSelected("go fix") {
		return verifyStep{Name: "go fix", Status: verifyStatusSkipped, Reason: "go fix rule not selected"}
	}
	step := verifyGoFix(r.rootDir, r.plan.patterns)
	step.Rules = r.stageRuleIDs([]string{"go fix"}, verifyStageGoFix)
	return step
}

func (r *verifyRunner) runGoVet() verifyStep {
	return verifyGoVet(r.rootDir, r.plan.patterns)
}

func (r *verifyRunner) runLayers() verifyStep {
	layerRules := make([]string, 0, len(layerRuleIDs()))
	for _, rule := range layerRuleIDs() {
		if r.ruleSelected(rule) {
			layerRules = append(layerRules, rule)
		}
	}
	if len(layerRules) == 0 {
		return verifyStep{Name: "layers", Status: verifyStatusSkipped, Reason: "no layer rule selected"}
	}
	step := verifyLayerBoundaries(r.rootDir, r.plan.patterns, layerRules)
	step.Rules = r.stageRuleIDs(layerRules, verifyStageLayers)
	return step
}

func (r *verifyRunner) runFrontend() verifyStep {
	if r.frontendMissing {
		return blockedStep("frontend", "Run: "+r.setupFail)
	}
	runTypecheck := r.ruleSelected("frontend")
	runRouteUsage := r.ruleSelected("frontend-route-usage")
	if !runTypecheck && !runRouteUsage {
		return verifyStep{Name: "frontend", Status: verifyStatusSkipped, Reason: "no frontend rule selected"}
	}
	step, executed := verifyFrontendStage(r.rootDir, runTypecheck, runRouteUsage)
	step.Rules = r.stageRuleIDs(executed, verifyStageFrontend)
	return step
}

func (r *verifyRunner) runGenerated() verifyStep {
	if !r.ruleSelected("generated") {
		return verifyStep{Name: "generated", Status: verifyStatusSkipped, Reason: "generated rule not selected"}
	}
	if r.toolsMissing {
		return blockedStep("generated", "Run: "+r.setupFail)
	}
	step := verifyGenerated(r.rootDir)
	step.Rules = r.stageRuleIDs([]string{"generated"}, verifyStageGenerated)
	return step
}

func (r *verifyRunner) runContent() verifyStep {
	var selected []string
	if r.selection.ruleFiltered {
		selected = contentSelectedRules(r.selection.RuleIDs)
		if len(selected) == 0 {
			return verifyStep{Name: "content", Status: verifyStatusSkipped, Reason: "no content rule selected"}
		}
	}
	step, executed := verifyContent(r.rootDir, selected)
	if step.NotApplicable {
		return step
	}
	step.Rules = r.stageRuleIDs(executed, verifyStageContent)
	return step
}

func contentSelectedRules(ruleIDs []string) []string {
	selected := make([]string, 0, len(contentRuleIDs))
	for _, rule := range ruleIDs {
		if slices.Contains(contentRuleIDs, rule) {
			selected = append(selected, rule)
		}
	}
	return selected
}

func (r *verifyRunner) runGoTest() verifyStep {
	step := verifyGoTest(r.rootDir, r.plan.patterns)
	return step
}

func renderVerifyHuman(cmd *cobra.Command, report verifyReport) error {
	out := cmd.OutOrStdout()
	if report.Scope.Mode == verifyScopeChanged {
		if _, err := fmt.Fprintf(
			out,
			"scope      changed (base %s): %d changed files, %d Go packages\n",
			report.Scope.Base,
			len(report.Scope.ChangedFiles),
			len(report.Scope.GoPackages),
		); err != nil {
			return err
		}
		for _, widening := range report.Scope.Widened {
			if _, err := fmt.Fprintf(out, "           widened %s: %s\n", widening.Step, widening.Reason); err != nil {
				return err
			}
		}
	}
	if report.Scope.Noop {
		if _, err := fmt.Fprintln(out, "No changes in scope; verify did not check whole-project health."); err != nil {
			return err
		}
	}
	for _, step := range report.Steps {
		status := "ok"
		switch step.Status {
		case verifyStatusFail:
			status = "FAIL"
		case verifyStatusSkipped:
			status = "SKIP"
		case verifyStatusBlocked:
			status = "BLOCK"
		}
		notes := make([]string, 0, 3)
		if step.Totals != nil {
			notes = append(notes, fmt.Sprintf(
				"%d passed, %d failed, %d skipped in %d packages",
				step.Totals.Passed,
				step.Totals.Failed,
				step.Totals.Skipped,
				step.Totals.Packages,
			))
			if step.Totals.CachedPackages > 0 {
				notes = append(notes, fmt.Sprintf("%d cached", step.Totals.CachedPackages))
			}
		}
		if n := len(step.SkippedPackages); n > 0 {
			notes = append(notes, fmt.Sprintf("%d %s skipped: no tests ran", n, verifyPlural(n, "package")))
		}
		if n := len(step.SkippedTests); n > 0 {
			notes = append(notes, fmt.Sprintf("%d %s skipped", n, verifyPlural(n, "test")))
		}
		if len(notes) > 0 {
			status += " (" + strings.Join(notes, ", ") + ")"
		}
		if _, err := fmt.Fprintf(out, "%-10s %s\n", step.Name, status); err != nil {
			return err
		}
		if step.Rules != nil && len(step.Rules.Skipped) > 0 {
			if _, err := fmt.Fprintf(out, "  rules skipped: %s\n", strings.Join(step.Rules.Skipped, ", ")); err != nil {
				return err
			}
		}
		for _, skipped := range step.SkippedPackages {
			if _, err := fmt.Fprintf(out, "  skipped %s: %s\n", skipped.Package, skipped.Reason); err != nil {
				return err
			}
		}
		for _, skipped := range step.SkippedTests {
			line := fmt.Sprintf("  skipped %s: %s", skipped.Package, skipped.Test)
			if skipped.Reason != "" {
				line += " (" + skipped.Reason + ")"
			}
			if _, err := fmt.Fprintln(out, line); err != nil {
				return err
			}
		}
		for _, warning := range step.Warnings {
			if _, err := fmt.Fprintf(out, "  warning: %s\n", warning); err != nil {
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

func verifyPlural(count int, noun string) string {
	if count == 1 {
		return noun
	}
	return noun + "s"
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
		if report.Scope.Noop {
			return "No changes in scope; verify did not check whole-project health"
		}
		return "Verification passed"
	}
	failed := make([]string, 0)
	blocked := make([]string, 0)
	for _, step := range report.Steps {
		if verifyStepPassed(step) {
			continue
		}
		if step.Status == verifyStatusBlocked {
			blocked = append(blocked, step.Name)
			continue
		}
		failed = append(failed, step.Name)
	}
	if len(blocked) > 0 && len(failed) == 0 {
		return "Verification blocked: setup incomplete (" + strings.Join(blocked, ", ") + ")"
	}
	if len(blocked) > 0 {
		return "Verification failed: " + strings.Join(failed, ", ") +
			"; blocked: " + strings.Join(blocked, ", ")
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
	return verifyGofmtPaths(rootDir, paths)
}

// verifyGofmtPaths checks exactly the given files or directories. Changed
// scope passes the changed Go files so an unrelated unformatted file outside
// the change set does not fail the run.
func verifyGofmtPaths(rootDir string, paths []string) verifyStep {
	if len(paths) == 0 {
		return verifyStepOK("gofmt")
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

func verifyGoTest(rootDir string, patterns []string) verifyStep {
	args := append([]string{"test", "-json"}, goPatternArgs(patterns)...)
	cmd := exec.Command("go", args...)
	cmd.Dir = rootDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	result := parseGoTestJSON(stdout.Bytes())
	step := verifyStepOK("go test")
	step.SkippedPackages = result.skipped
	step.SkippedTests = result.skippedTests
	step.Packages = result.packageReports()
	totals := result.totals()
	step.Totals = &totals
	if len(result.other) > 0 {
		step.Warnings = append(step.Warnings, fmt.Sprintf(
			"go test output contained %d line(s) that are not test2json events; see detail",
			len(result.other),
		))
		step.Detail = strings.Join(result.other, "\n")
	}
	if err != nil {
		step.Status = verifyStatusFail
		step.OK = false
		step.Detail = commandDetail(err, result.failureDetail(stderr.String()))
		return step
	}
	return step
}

// goTestEvent is one line of go test -json output (cmd/test2json), including
// the build-output events the go command emits for packages that fail to build.
type goTestEvent struct {
	Action      string  `json:"Action"`
	Package     string  `json:"Package"`
	Test        string  `json:"Test"`
	Output      string  `json:"Output"`
	Elapsed     float64 `json:"Elapsed"`
	FailedBuild string  `json:"FailedBuild"`
}

type goTestPackage struct {
	ranTests  bool
	failed    bool
	passed    bool
	skipped   bool
	elapsed   float64
	output    []string
	pkgLines  []string
	testLines map[string][]string
	// testState records each test's terminal action. Top-level tests and
	// subtests share the map; they are counted separately.
	testState map[string]string
}

type goTestResult struct {
	order        []string
	packages     map[string]*goTestPackage
	build        []string
	other        []string
	skipped      []verifySkippedPackage
	skippedTests []verifySkippedTest
}

// parseGoTestJSON folds go test -json events per package. A package whose test
// binary passed without a single test-level run event has tests but ran none,
// typically because its TestMain exited before m.Run (for example when Docker
// is unavailable); it is reported as skipped with its own output as the reason.
// Packages without test files and build/setup failures are kept so a package
// that never ran tests is never reported as passing tests that did not run.
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
			if event.Action != "start" {
				result.other = append(result.other, line)
			}
			continue
		}
		pkg, ok := result.packages[event.Package]
		if !ok {
			pkg = &goTestPackage{
				testLines: map[string][]string{},
				testState: map[string]string{},
			}
			result.packages[event.Package] = pkg
			result.order = append(result.order, event.Package)
		}
		switch event.Action {
		case "run":
			if event.Test != "" {
				pkg.ranTests = true
				pkg.testState[event.Test] = "run"
			}
		case "output":
			text := strings.TrimRight(event.Output, "\n")
			pkg.output = append(pkg.output, text)
			if event.Test == "" {
				pkg.pkgLines = append(pkg.pkgLines, text)
			} else {
				pkg.testLines[event.Test] = append(pkg.testLines[event.Test], text)
			}
		case "skip":
			if event.Test != "" {
				pkg.testState[event.Test] = "skip"
				result.skippedTests = append(result.skippedTests, verifySkippedTest{
					Package: event.Package,
					Test:    event.Test,
					Reason:  goTestTestSkipReason(pkg.testLines[event.Test]),
				})
			} else {
				pkg.skipped = true
			}
		case "fail":
			if event.Test != "" {
				pkg.testState[event.Test] = "fail"
				break
			}
			pkg.failed = true
			pkg.elapsed = event.Elapsed
		case "pass":
			if event.Test != "" {
				pkg.testState[event.Test] = "pass"
				break
			}
			pkg.passed = true
			pkg.elapsed = event.Elapsed
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
	slices.SortFunc(result.skippedTests, func(a, b verifySkippedTest) int {
		if order := strings.Compare(a.Package, b.Package); order != 0 {
			return order
		}
		return strings.Compare(a.Test, b.Test)
	})
	return result
}

// verifyTestStatus values for packages that did not run tests.
const (
	verifyTestStatusNoTestFiles = "no_test_files"
	verifyTestStatusNoTestsRan  = "no_tests_ran"
)

// packageReports renders the per-package accounting. Status is explicit: a
// package that never ran tests is never reported as a passing test package.
func (r goTestResult) packageReports() []verifyTestPackage {
	reports := make([]verifyTestPackage, 0, len(r.order))
	for _, name := range r.order {
		pkg := r.packages[name]
		report := verifyTestPackage{
			Package:         name,
			DurationSeconds: pkg.elapsed,
		}
		subtests := &verifyTestSubtests{}
		for test, state := range pkg.testState {
			if state == "run" {
				continue
			}
			if strings.Contains(test, "/") {
				switch state {
				case "pass":
					subtests.Passed++
				case "fail":
					subtests.Failed++
				case "skip":
					subtests.Skipped++
				}
				continue
			}
			switch state {
			case "pass":
				report.Passed++
			case "fail":
				report.Failed++
			case "skip":
				report.Skipped++
			}
		}
		if subtests.Passed+subtests.Failed+subtests.Skipped > 0 {
			report.Subtests = subtests
		}

		output := strings.Join(pkg.output, "\n")
		report.Cached = strings.Contains(output, "(cached)")
		report.NoTestFiles = strings.Contains(output, "[no test files]")
		report.BuildFailed = strings.Contains(output, "[build failed]")
		report.SetupFailed = strings.Contains(output, "[setup failed]")
		report.TimedOut = strings.Contains(output, "panic: test timed out") ||
			strings.Contains(output, "test timed out after")
		report.RaceDetected = strings.Contains(output, "WARNING: DATA RACE")

		switch {
		case pkg.failed:
			report.Status = verifyStatusFail
		case report.NoTestFiles:
			report.Status = verifyTestStatusNoTestFiles
			report.Reason = "no test files"
		case pkg.passed && !pkg.ranTests:
			report.Status = verifyTestStatusNoTestsRan
			report.NoTestsRan = true
			report.Reason = goTestSkipReason(name, pkg.pkgLines)
		case pkg.passed:
			report.Status = verifyStatusOK
		default:
			report.Status = verifyStatusSkipped
			report.Reason = "go test reported no result for this package"
		}
		reports = append(reports, report)
	}
	slices.SortFunc(reports, func(a, b verifyTestPackage) int {
		return strings.Compare(a.Package, b.Package)
	})
	return reports
}

// totals sums top-level tests across packages. Subtests are summed separately
// so a parent test and its subtests never inflate the same total.
func (r goTestResult) totals() verifyTestTotals {
	totals := verifyTestTotals{}
	for _, report := range r.packageReports() {
		if report.NoTestFiles {
			totals.NoTestFiles++
		} else {
			totals.Packages++
		}
		if report.NoTestsRan {
			totals.NoTestsRan++
		}
		if report.Cached {
			totals.CachedPackages++
		}
		totals.Passed += report.Passed
		totals.Failed += report.Failed
		totals.Skipped += report.Skipped
		totals.DurationSeconds += report.DurationSeconds
		if report.Subtests != nil {
			totals.SubtestsPassed += report.Subtests.Passed
			totals.SubtestsFailed += report.Subtests.Failed
			totals.SubtestsSkipped += report.Subtests.Skipped
		}
	}
	return totals
}

// goTestTestSkipReason extracts the message a test passed to t.Skip. test2json
// prints that message before the test's "--- SKIP:" marker, so the marker is
// only the fallback for a bare t.Skip(). Runner frames (=== RUN/PAUSE/CONT and
// --- PASS/FAIL) are not reasons, and neither is the empty file:line frame a
// bare t.Skip leaves behind.
func goTestTestSkipReason(lines []string) string {
	reason := ""
	marker := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "",
			strings.HasPrefix(trimmed, "=== RUN"),
			strings.HasPrefix(trimmed, "=== PAUSE"),
			strings.HasPrefix(trimmed, "=== CONT"),
			strings.HasPrefix(trimmed, "--- PASS:"),
			strings.HasPrefix(trimmed, "--- FAIL:"),
			goTestLogLocationOnly(trimmed):
			continue
		case strings.HasPrefix(trimmed, "--- SKIP:"):
			if marker == "" {
				marker = strings.TrimSpace(strings.TrimPrefix(trimmed, "--- SKIP:"))
			}
		default:
			reason = trimmed
		}
	}
	if reason != "" {
		return reason
	}
	return marker
}

// goTestLogLocationOnly reports a test2json log frame with no message, such as
// "fixture_test.go:42:" from a bare t.Skip().
func goTestLogLocationOnly(line string) bool {
	_, rest, ok := strings.Cut(line, ".go:")
	if !ok {
		return false
	}
	lineNumber, message, ok := strings.Cut(rest, ":")
	if !ok || message != "" || lineNumber == "" {
		return false
	}
	for _, digit := range lineNumber {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
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
	step.Warnings = payloadScanWarnings(rootDir)
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

// payloadScanWarnings lists inertia.FromStruct roots the payload scanner cannot
// resolve statically. andurel sync payloads --strict rejects these; verify only
// warns so projects can adopt --strict incrementally. The generated step's
// payloads.ts check already reports parse and generation failures.
func payloadScanWarnings(rootDir string) []string {
	if !projectUsesInertia(rootDir) {
		return nil
	}
	scan, err := scanControllerPayloads(rootDir)
	if err != nil {
		return nil
	}
	warnings := make([]string, 0)
	for _, skipped := range scan.skipped {
		if skipped.Reason != unresolvedFromStructReason {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"payloads.ts: %s:%d: %s; pass a named composite literal or a typed variable",
			skipped.SourceFile,
			skipped.Line,
			skipped.Reason,
		))
	}
	return warnings
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
