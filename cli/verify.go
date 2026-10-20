package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

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

type verifyStep struct {
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	OK         bool              `json:"ok"`
	Reason     string            `json:"reason,omitempty"`
	Detail     string            `json:"detail,omitempty"`
	Violations []verifyViolation `json:"violations,omitempty"`
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

func newVerifyCommand() *cobra.Command {
	var withTests bool
	var skipGenerated bool
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Check formatting, vet, layer boundaries, and generated code",
		Long: `Run the Andurel verification loop against the current project.

andurel verify checks, in order:
  • gofmt     — every Go file is gofmt-clean
  • go fix    — the toolchain reports no automatic rewrites
  • go vet    — the module vets clean
  • layers    — the persistence boundary:
                queries-import  only domains import domains/internal/queries
                db-handle       outside domains, internal/runtime, cmd, seeds,
                                and migrations nothing references
                                storage.Connection or storage.Transaction,
                                or imports pgx
                tx-unwrap       storage.Unwrap is called only in domains
                tx-escape       a storage.AtomicOps callback uses only
                                dependencies bound with .InTx(tx)
                tx-public-call  a storage.AtomicOps callback does not call an
                                exported method on its own receiver
  • generated — templ, narsilc, routes.ts, and payloads.ts output is up to date

A missing templ or narsilc binary fails the generated step with the
missing-tool rule; run 'andurel tool sync' to install it. Pass
--skip-generated to mark that step 'skipped' instead; an explicit skip does
not fail the run.

Pass --with-tests to also run go test ./... (requires a Postgres container for
domain tests). verify never rewrites files; fix reported issues and re-run.

Each step reports status 'ok', 'fail', or 'skipped'. Every failing step lists
violations as {rule, file, line, message, fix}. A failed verification exits
with code 9 (verification_failed); usage errors exit with code 1.`,
		Example: `  andurel verify
  andurel verify --json
  andurel verify --with-tests
  andurel verify --skip-generated`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVerify(cmd, withTests, skipGenerated)
		},
	}
	cmd.Flags().BoolVar(&withTests, "with-tests", false, "Also run go test ./...")
	cmd.Flags().
		BoolVar(&skipGenerated, "skip-generated", false, "Skip the generated-code drift step")
	setAgentMetadata(
		cmd,
		"verification",
		"Read-only verification: gofmt, go fix, go vet, the domain persistence and transaction boundary, and generated-code drift. A missing templ or narsilc binary fails generated with missing-tool; run 'andurel tool sync'. Fails with exit code 9 and structured violations.",
	)
	setStandardHelp(cmd)
	return cmd
}

func runVerify(cmd *cobra.Command, withTests, skipGenerated bool) error {
	rootDir, err := findGoModRoot()
	if err != nil {
		return err
	}

	report := verifyReport{}
	report.Steps = append(report.Steps, verifyGofmt(rootDir))
	report.Steps = append(report.Steps, verifyGoFix(rootDir))
	report.Steps = append(report.Steps, verifyGoVet(rootDir))
	report.Steps = append(report.Steps, verifyLayerBoundaries(rootDir))
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
		if _, err := fmt.Fprintf(out, "%-10s %s\n", step.Name, status); err != nil {
			return err
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

func verifyGofmt(rootDir string) verifyStep {
	out, err := runVerifyCommand(rootDir, "gofmt", "-l", ".")
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
		step.Violations = append(step.Violations, verifyViolation{
			Rule:    "gofmt",
			File:    filepath.ToSlash(file),
			Message: "file is not gofmt-clean",
			Fix:     "gofmt -w " + file,
		})
	}
	return step
}

func verifyGoFix(rootDir string) verifyStep {
	out, err := runVerifyCommand(rootDir, "go", "fix", "-diff", "./...")
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
		Fix:     "go fix ./...",
	}}
	return step
}

func verifyGoVet(rootDir string) verifyStep {
	out, err := runGoVet(rootDir)
	detail := strings.TrimSpace(out)
	if err != nil {
		step := verifyStepFailed("go vet")
		step.Detail = commandDetail(err, detail)
		return step
	}
	return verifyStepOK("go vet")
}

func verifyGoTest(rootDir string) verifyStep {
	out, err := runVerifyCommand(rootDir, "go", "test", "./...")
	detail := strings.TrimSpace(out)
	if err != nil {
		step := verifyStepFailed("go test")
		step.Detail = commandDetail(err, detail)
		return step
	}
	return verifyStepOK("go test")
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
			message := result.name + ": " + result.message
			if len(result.details) > 0 {
				message += " (" + strings.Join(result.details, "; ") + ")"
			}
			file, fix := generatedTarget(result)
			step.Violations = append(step.Violations, verifyViolation{
				Rule:    "generated",
				File:    file,
				Message: message,
				Fix:     fix,
			})
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

// generatedTarget maps a doctor generation check to the output it owns and
// the command that regenerates it.
func generatedTarget(result checkResult) (file, fix string) {
	switch result.name {
	case "templ generate":
		return "", "andurel build"
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
	return runVerifyCommand(dir, "go", "vet", "./...")
}
